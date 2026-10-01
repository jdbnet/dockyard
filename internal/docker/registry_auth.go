package docker

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/registry"
)

type dockerConfigFile struct {
	Auths       map[string]dockerAuthEntry `json:"auths"`
	CredsStore  string                     `json:"credsStore"`
	CredHelpers map[string]string          `json:"credHelpers"`
}

type dockerAuthEntry struct {
	Auth     string `json:"auth"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type credentialHelperResponse struct {
	Username string `json:"Username"`
	Secret   string `json:"Secret"`
}

// encodedRegistryAuth returns the X-Registry-Auth value for imageRef using the
// same Docker config and credential helpers as the docker CLI. An empty string
// means no credentials are stored for that registry.
func encodedRegistryAuth(imageRef string) (string, error) {
	path := dockerConfigPath()
	if path == "" {
		return "", nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var cfg dockerConfigFile
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("parse docker config: %w", err)
	}
	auth, ok, err := cfg.authForImage(imageRef)
	if err != nil || !ok {
		return "", err
	}
	if auth.Username == "" && auth.Password == "" && auth.IdentityToken == "" && auth.RegistryToken == "" {
		return "", nil
	}
	return registry.EncodeAuthConfig(auth)
}

func dockerConfigPath() string {
	if dir := strings.TrimSpace(os.Getenv("DOCKER_CONFIG")); dir != "" {
		return filepath.Join(dir, "config.json")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".docker", "config.json")
}

func (cfg dockerConfigFile) authForImage(imageRef string) (registry.AuthConfig, bool, error) {
	host := registryHost(imageRef)
	if host == "" {
		return registry.AuthConfig{}, false, nil
	}
	key, entry, found := cfg.findAuth(host)
	auth := registry.AuthConfig{ServerAddress: host}
	if found {
		auth = entry.toAuthConfig(host)
	}
	helper := cfg.helperFor(host, key)
	if helper == "" {
		if !found || (auth.Username == "" && auth.Password == "") {
			return registry.AuthConfig{}, false, nil
		}
		return auth, true, nil
	}
	username, secret, err := getCredential(helper, credentialServer(key, host))
	if err != nil {
		return registry.AuthConfig{}, false, err
	}
	if username == "" && secret == "" {
		return registry.AuthConfig{}, false, nil
	}
	auth.Username = username
	auth.Password = secret
	auth.ServerAddress = host
	return auth, true, nil
}

func (cfg dockerConfigFile) findAuth(host string) (string, dockerAuthEntry, bool) {
	if len(cfg.Auths) == 0 {
		return "", dockerAuthEntry{}, false
	}
	for _, key := range authKeys(host) {
		if entry, ok := cfg.Auths[key]; ok {
			return key, entry, true
		}
	}
	return "", dockerAuthEntry{}, false
}

func authKeys(host string) []string {
	keys := []string{host, "https://" + host, "https://" + host + "/v1/"}
	switch host {
	case "docker.io", "index.docker.io", "registry-1.docker.io":
		keys = append(keys,
			"https://index.docker.io/v1/",
			"https://index.docker.io/v1",
			"docker.io",
		)
	}
	return keys
}

func (cfg dockerConfigFile) helperFor(host, configKey string) string {
	if cfg.CredHelpers != nil {
		if helper, ok := cfg.CredHelpers[host]; ok && helper != "" {
			return helper
		}
		if configKey != "" {
			if helper, ok := cfg.CredHelpers[configKey]; ok && helper != "" {
				return helper
			}
		}
	}
	return cfg.CredsStore
}

func credentialServer(configKey, host string) string {
	if configKey != "" {
		return configKey
	}
	return host
}

func (e dockerAuthEntry) toAuthConfig(host string) registry.AuthConfig {
	auth := registry.AuthConfig{
		Username:      e.Username,
		Password:      e.Password,
		ServerAddress: host,
	}
	if e.Auth == "" {
		return auth
	}
	decoded, err := base64.StdEncoding.DecodeString(e.Auth)
	if err != nil {
		decoded, err = base64.URLEncoding.DecodeString(e.Auth)
	}
	if err != nil {
		return auth
	}
	user, pass, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return auth
	}
	auth.Username = user
	auth.Password = pass
	return auth
}

func getCredential(helper, server string) (string, string, error) {
	cmd := exec.Command("docker-credential-"+helper, "get")
	cmd.Stdin = strings.NewReader(server + "\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		if strings.Contains(strings.ToLower(msg), "credentials not found") {
			return "", "", nil
		}
		return "", "", fmt.Errorf("docker credential helper %s: %s", helper, msg)
	}
	var resp credentialHelperResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return "", "", fmt.Errorf("docker credential helper %s: %w", helper, err)
	}
	return resp.Username, resp.Secret, nil
}

func registryHost(imageRef string) string {
	named, err := reference.ParseNormalizedNamed(imageRef)
	if err != nil {
		return ""
	}
	return reference.Domain(named)
}

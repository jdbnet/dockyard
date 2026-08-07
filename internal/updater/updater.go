package updater

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"golang.org/x/mod/semver"
)

const (
	defaultUpdateBaseURL = "https://apps.jdbnet.co.uk"
	etagFileName         = ".dockyard-etag"
	downloadTimeout      = 2 * time.Minute
	headTimeout          = 30 * time.Second
)

var (
	updateBaseURL         = defaultUpdateBaseURL
	resolveExecutablePath = resolveExecutable
	execSelf              = func(path string, args []string, env []string) error {
		return syscall.Exec(path, args, env)
	}
	httpClient = &http.Client{Timeout: downloadTimeout}
)

// MaybeUpdate checks for a newer binary and replaces the current executable if one
// is available. On success it exec-restarts and does not return.
func MaybeUpdate(currentVersion string) {
	if err := maybeUpdate(currentVersion); err != nil {
		log.Printf("auto-update: %v", err)
	}
}

func maybeUpdate(currentVersion string) error {
	name, err := binaryName(runtime.GOARCH)
	if err != nil {
		log.Printf("auto-update skipped: %v", err)
		return nil
	}

	exePath, err := resolveExecutablePath()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	if !isWritable(exePath) {
		log.Print("auto-update skipped: binary not writable")
		return nil
	}

	exeDir := filepath.Dir(exePath)
	etagPath := filepath.Join(exeDir, etagFileName)
	updateURL := updateBaseURL + "/" + name

	etag, err := fetchETag(updateURL)
	if err != nil {
		return fmt.Errorf("check for update: %w", err)
	}
	if etag == "" {
		return fmt.Errorf("check for update: missing ETag header")
	}

	cachedETag, _ := os.ReadFile(etagPath)
	if string(cachedETag) == etag {
		return nil
	}

	tempPath := filepath.Join(exeDir, name+".new")
	if err := downloadFile(updateURL, tempPath); err != nil {
		return fmt.Errorf("download update: %w", err)
	}
	defer os.Remove(tempPath)

	if err := os.Chmod(tempPath, 0o755); err != nil {
		return fmt.Errorf("chmod update: %w", err)
	}

	remoteVersion, err := binaryVersion(tempPath)
	if err != nil {
		return fmt.Errorf("verify update: %w", err)
	}

	if !shouldApplyUpdate(currentVersion, remoteVersion) {
		log.Printf("auto-update skipped: remote version %s is older than local %s", remoteVersion, currentVersion)
		if err := os.WriteFile(etagPath, []byte(etag), 0o644); err != nil {
			return fmt.Errorf("cache etag: %w", err)
		}
		return nil
	}

	log.Printf("auto-update: updating from %s to %s", currentVersion, remoteVersion)

	if err := os.Rename(tempPath, exePath); err != nil {
		return fmt.Errorf("install update: %w", err)
	}

	if err := os.WriteFile(etagPath, []byte(etag), 0o644); err != nil {
		return fmt.Errorf("cache etag: %w", err)
	}

	if err := execSelf(exePath, os.Args, os.Environ()); err != nil {
		return fmt.Errorf("restart after update: %w", err)
	}
	return nil
}

func binaryName(goarch string) (string, error) {
	switch goarch {
	case "amd64":
		return "dockyard-amd64", nil
	case "arm64":
		return "dockyard-arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture %q", goarch)
	}
}

func shouldApplyUpdate(local, remote string) bool {
	localV := normalizeVersion(local)
	remoteV := normalizeVersion(remote)
	if !semver.IsValid(localV) || !semver.IsValid(remoteV) {
		return false
	}
	return semver.Compare(remoteV, localV) >= 0
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if v == "" {
		return ""
	}
	return "v" + v
}

func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}

func isWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

func fetchETag(url string) (string, error) {
	client := &http.Client{Timeout: headTimeout}
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %s", resp.Status)
	}

	etag := strings.Trim(resp.Header.Get("ETag"), `"`)
	return etag, nil
}

func downloadFile(url, dest string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		os.Remove(dest)
		return err
	}
	return nil
}

func binaryVersion(path string) (string, error) {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("run %s --version: %w", path, err)
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		return "", fmt.Errorf("empty version from %s", path)
	}
	return version, nil
}

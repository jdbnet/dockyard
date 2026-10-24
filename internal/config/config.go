package config

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Path        string            `yaml:"-"`
	Docker      DockerConfig      `yaml:"docker"`
	Compose     ComposeConfig     `yaml:"compose"`
	Web         WebConfig         `yaml:"web"`
	Auth        AuthConfig        `yaml:"auth"`
	Stats       StatsConfig       `yaml:"stats"`
	Events      EventsConfig      `yaml:"events"`
	RestartLoop RestartLoopConfig `yaml:"restart_loop"`
	TUI         TUIConfig         `yaml:"tui"`
	Updates     UpdatesConfig     `yaml:"updates"`
	Logging     LoggingConfig     `yaml:"logging"`
	APIKeys       []APIKey            `yaml:"api_keys,omitempty"`
	Notifications NotificationsConfig `yaml:"notifications"`
}

// NotificationsConfig drives update webhook delivery.
type NotificationsConfig struct {
	Enabled   bool                 `yaml:"enabled"`
	PublicURL string               `yaml:"public_url"`
	Schedule  NotifyScheduleConfig `yaml:"schedule"`
	Webhooks  []WebhookConfig      `yaml:"webhooks,omitempty"`
}

type NotifyScheduleConfig struct {
	Mode     string   `yaml:"mode"`
	Interval Duration `yaml:"interval"`
	At       string   `yaml:"at"`
}

// WebhookField is one key in a custom webhook JSON body. The value is a template string.
type WebhookField struct {
	Key   string `yaml:"key" json:"key"`
	Value string `yaml:"value" json:"value"`
}

type WebhookConfig struct {
	ID              string         `yaml:"id"`
	Name            string         `yaml:"name"`
	Type            string         `yaml:"type"`
	URL             string         `yaml:"url"`
	Auth            string         `yaml:"auth"`
	Username        string         `yaml:"username"`
	Token           string         `yaml:"token"`
	Enabled         bool           `yaml:"enabled"`
	MessageTemplate string         `yaml:"message_template,omitempty"`
	TitleTemplate   string         `yaml:"title_template,omitempty"`
	CustomFields    []WebhookField `yaml:"custom_fields,omitempty"`
}

// APIKey is a stored external-API credential. The raw key is never written.
type APIKey struct {
	ID         string     `yaml:"id" json:"id"`
	Name       string     `yaml:"name" json:"name"`
	Prefix     string     `yaml:"prefix" json:"prefix"`
	Hash       string     `yaml:"sha256" json:"-"`
	Scope      string     `yaml:"scope" json:"scope"`
	CreatedAt  time.Time  `yaml:"created_at" json:"created_at"`
	LastUsedAt *time.Time `yaml:"last_used_at,omitempty" json:"last_used_at,omitempty"`
}

type DockerConfig struct {
	Socket string `yaml:"socket"`
}

type ComposeConfig struct {
	StacksDir string `yaml:"stacks_dir"`
}

type WebConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Bind    string `yaml:"bind"`
}

type AuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type StatsConfig struct {
	Interval   Duration `yaml:"interval"`
	BufferSize int      `yaml:"buffer_size"`
}

type EventsConfig struct {
	FallbackRefresh Duration `yaml:"fallback_refresh"`
}

type RestartLoopConfig struct {
	Threshold int      `yaml:"threshold"`
	Window    Duration `yaml:"window"`
}

type TUIConfig struct {
	RefreshRate Duration `yaml:"refresh_rate"`
}

type UpdatesConfig struct {
	Interval Duration `yaml:"interval"`
}

type LoggingConfig struct {
	Enabled          bool   `yaml:"enabled"`
	URL              string `yaml:"url"`
	Type             string `yaml:"type"`
	Auth             string `yaml:"auth"`
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	Token            string `yaml:"token"`
	CollectByDefault bool   `yaml:"collect_by_default"`
}

// Duration wraps time.Duration for YAML unmarshaling.
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}

func (d Duration) MarshalYAML() (any, error) {
	return FormatDuration(d.Duration), nil
}

// FormatDuration renders a duration the same way config.yaml writes it.
func FormatDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "0s"
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	case d%time.Second == 0:
		return fmt.Sprintf("%ds", d/time.Second)
	default:
		return d.String()
	}
}

func LoadConfig(path string) (*Config, error) {
	cfg := defaultConfig()

	file, err := os.Open(path)
	if err != nil && os.IsNotExist(err) && os.Getenv("DOCKYARD_DOCKER_SOCKET") == "" {
		log.Printf("No config file found, using defaults")
	} else if err == nil {
		defer file.Close()
		if err := yaml.NewDecoder(file).Decode(cfg); err != nil {
			return nil, fmt.Errorf("parse config file: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("open config file: %w", err)
	}

	applyEnvOverrides(cfg)
	if cfg.Updates.Interval.Duration <= 0 {
		cfg.Updates.Interval.Duration = time.Hour
	}
	cfg.Path = path
	if cfg.Logging.Type == "" {
		cfg.Logging.Type = "gologs"
	}
	if cfg.Logging.Auth == "" {
		cfg.Logging.Auth = "none"
	}
	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Docker: DockerConfig{Socket: "/var/run/docker.sock"},
		Compose: ComposeConfig{
			StacksDir: "/opt/stacks",
		},
		Web: WebConfig{Enabled: false, Port: 8080, Bind: "127.0.0.1"},
		Stats: StatsConfig{
			Interval:   Duration{5 * time.Second},
			BufferSize: 60,
		},
		Events: EventsConfig{
			FallbackRefresh: Duration{30 * time.Second},
		},
		RestartLoop: RestartLoopConfig{
			Threshold: 3,
			Window:    Duration{5 * time.Minute},
		},
		TUI: TUIConfig{
			RefreshRate: Duration{100 * time.Millisecond},
		},
		Updates: UpdatesConfig{
			Interval: Duration{time.Hour},
		},
		Logging: LoggingConfig{
			Type:             "gologs",
			Auth:             "none",
			CollectByDefault: true,
		},
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("DOCKYARD_DOCKER_SOCKET"); v != "" {
		cfg.Docker.Socket = v
	}
	if v := os.Getenv("DOCKYARD_WEB_ENABLED"); v != "" {
		cfg.Web.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("DOCKYARD_WEB_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Web.Port = port
		}
	}
	if v := os.Getenv("DOCKYARD_WEB_BIND"); v != "" {
		cfg.Web.Bind = v
	}
	if v := os.Getenv("DOCKYARD_STACKS_DIR"); v != "" {
		cfg.Compose.StacksDir = v
	}
	if v := os.Getenv("DOCKYARD_AUTH_USER"); v != "" {
		cfg.Auth.Username = v
	}
	if v := os.Getenv("DOCKYARD_AUTH_PASS"); v != "" {
		cfg.Auth.Password = v
	}
	if v := os.Getenv("DOCKYARD_UPDATES_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			cfg.Updates.Interval.Duration = d
		}
	}
}

// SaveAPIKeys rewrites the api_keys list and leaves every other key as it is.
func (c *Config) SaveAPIKeys() error {
	keys := c.APIKeys
	if keys == nil {
		keys = []APIKey{}
	}
	return c.saveKeys(map[string]any{"api_keys": keys})
}

// SaveNotifications rewrites the notifications key and leaves every other key as it is.
func (c *Config) SaveNotifications(notifications NotificationsConfig) error {
	if notifications.Webhooks == nil {
		notifications.Webhooks = []WebhookConfig{}
	}
	return c.saveKeys(map[string]any{"notifications": notifications})
}

// SaveLogging rewrites the logging key in the config file and leaves every other key as it is.
func (c *Config) SaveLogging(logging LoggingConfig) error {
	return c.saveKeys(map[string]any{"logging": logging})
}

// SaveGeneral rewrites every settings section except logging.
func (c *Config) SaveGeneral() error {
	return c.saveKeys(map[string]any{
		"docker":       c.Docker,
		"compose":      c.Compose,
		"web":          c.Web,
		"auth":         c.Auth,
		"stats":        c.Stats,
		"events":       c.Events,
		"restart_loop": c.RestartLoop,
		"tui":          c.TUI,
		"updates":      c.Updates,
	})
}

func (c *Config) saveKeys(keys map[string]any) error {
	if c.Path == "" {
		return fmt.Errorf("config path is not set")
	}
	raw, err := os.ReadFile(c.Path)
	var doc yaml.Node
	switch {
	case err != nil && os.IsNotExist(err):
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	case err != nil:
		return fmt.Errorf("read config: %w", err)
	case len(bytes.TrimSpace(raw)) == 0:
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	default:
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("parse config: %w", err)
		}
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("config root is not a map")
	}
	for key, value := range keys {
		var node yaml.Node
		if err := node.Encode(value); err != nil {
			return err
		}
		upsertMapping(doc.Content[0], key, &node)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(c.Path); statErr == nil {
		mode = info.Mode().Perm()
	}
	tmp := c.Path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), mode); err != nil {
		return err
	}
	return os.Rename(tmp, c.Path)
}

func upsertMapping(root *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i < len(root.Content)-1; i += 2 {
		if root.Content[i].Value == key {
			root.Content[i+1] = value
			return
		}
	}
	root.Content = append(root.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"},
		value,
	)
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Web.Bind, c.Web.Port)
}

func (c *Config) AuthEnabled() bool {
	return c.Auth.Username != "" && c.Auth.Password != ""
}

func (c *Config) WebBindIsLoopback() bool {
	host := strings.TrimSpace(c.Web.Bind)
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

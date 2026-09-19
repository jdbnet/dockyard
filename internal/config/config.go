package config

import (
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
	Docker      DockerConfig      `yaml:"docker"`
	Compose     ComposeConfig     `yaml:"compose"`
	Web         WebConfig         `yaml:"web"`
	Auth        AuthConfig        `yaml:"auth"`
	Stats       StatsConfig       `yaml:"stats"`
	Events      EventsConfig      `yaml:"events"`
	RestartLoop RestartLoopConfig `yaml:"restart_loop"`
	TUI         TUIConfig         `yaml:"tui"`
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

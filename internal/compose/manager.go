package compose

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	ComposeFileName = "compose.yaml"
)

var composeFilenames = []string{
	"compose.yaml", "compose.yml",
	"docker-compose.yaml", "docker-compose.yml",
}

// Stack represents a compose project on disk.
type Stack struct {
	Name           string `json:"name"`
	Project        string `json:"project"`
	Path           string `json:"path"`
	ComposeFile    string `json:"compose_file"`
	Managed        bool   `json:"managed"`
	Editable       bool   `json:"editable"`
	ContainerCount int    `json:"container_count"`
	RunningCount   int    `json:"running_count"`
}

// Manager handles compose stack files and docker compose CLI operations.
type Manager struct {
	stacksDir string
}

func NewManager(stacksDir string) (*Manager, error) {
	dir, err := expandHome(stacksDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create stacks dir: %w", err)
	}
	return &Manager{stacksDir: dir}, nil
}

func (m *Manager) StacksDir() string {
	return m.stacksDir
}

// List scans the managed stacks directory.
func (m *Manager) List() ([]Stack, error) {
	entries, err := os.ReadDir(m.stacksDir)
	if err != nil {
		return nil, err
	}
	var stacks []Stack
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		path := filepath.Join(m.stacksDir, name)
		composeFile, err := findComposeFile(path)
		if err != nil {
			continue
		}
		stacks = append(stacks, Stack{
			Name:        name,
			Project:     name,
			Path:        path,
			ComposeFile: composeFile,
			Managed:     true,
			Editable:    true,
		})
	}
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Name < stacks[j].Name })
	return stacks, nil
}

// Read returns the compose file contents for a managed stack.
func (m *Manager) Read(name string) (string, error) {
	path, err := m.stackPath(name)
	if err != nil {
		return "", err
	}
	composeFile, err := findComposeFile(path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(composeFile)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Write saves compose content to a managed stack (creates compose.yaml).
func (m *Manager) Write(name, content string) error {
	path, err := m.stackPath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	target := filepath.Join(path, ComposeFileName)
	return os.WriteFile(target, []byte(content), 0o644)
}

// Create creates a new managed stack with the given compose content.
func (m *Manager) Create(name, content string) error {
	if err := validateStackName(name); err != nil {
		return err
	}
	path := filepath.Join(m.stacksDir, name)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("stack %q already exists", name)
	}
	return m.Write(name, content)
}

// Delete removes a managed stack directory.
func (m *Manager) Delete(name string) error {
	path, err := m.stackPath(name)
	if err != nil {
		return err
	}
	return os.RemoveAll(path)
}

// Run executes docker compose in the stack directory.
func (m *Manager) Run(ctx context.Context, stack Stack, args ...string) (string, error) {
	if stack.ComposeFile == "" {
		var err error
		stack.ComposeFile, err = findComposeFile(stack.Path)
		if err != nil {
			return "", err
		}
	}
	project := stack.Project
	if project == "" {
		project = stack.Name
	}
	base := []string{"compose", "-f", stack.ComposeFile, "-p", project}
	cmd := exec.CommandContext(ctx, "docker", append(base, args...)...)
	cmd.Dir = stack.Path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (m *Manager) Pull(ctx context.Context, stack Stack) error {
	_, err := m.Run(ctx, stack, "pull")
	return err
}

func (m *Manager) Up(ctx context.Context, stack Stack) error {
	_, err := m.Run(ctx, stack, "up", "-d", "--remove-orphans")
	return err
}

func (m *Manager) Down(ctx context.Context, stack Stack) error {
	_, err := m.Run(ctx, stack, "down")
	return err
}

// Update pulls latest images and recreates containers.
func (m *Manager) Update(ctx context.Context, stack Stack) error {
	if _, err := m.Run(ctx, stack, "pull"); err != nil {
		return err
	}
	_, err := m.Run(ctx, stack, "up", "-d", "--remove-orphans")
	return err
}

// UpdateService pulls and recreates a single service.
func (m *Manager) UpdateService(ctx context.Context, stack Stack, service string) error {
	if _, err := m.Run(ctx, stack, "pull", service); err != nil {
		return err
	}
	_, err := m.Run(ctx, stack, "up", "-d", service)
	return err
}

// StackFromPath builds a Stack for an external compose project directory.
func StackFromPath(name, path string) (Stack, error) {
	composeFile, err := findComposeFile(path)
	if err != nil {
		return Stack{}, err
	}
	return Stack{
		Name:        name,
		Project:     name,
		Path:        path,
		ComposeFile: composeFile,
		Managed:     false,
		Editable:    fileWritable(composeFile),
	}, nil
}

func (m *Manager) stackPath(name string) (string, error) {
	if err := validateStackName(name); err != nil {
		return "", err
	}
	path := filepath.Join(m.stacksDir, name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("stack %q not found", name)
	}
	return path, nil
}

func findComposeFile(dir string) (string, error) {
	for _, name := range composeFilenames {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no compose file in %s", dir)
}

func validateStackName(name string) error {
	if name == "" {
		return fmt.Errorf("stack name required")
	}
	if strings.ContainsAny(name, "/\\..") {
		return fmt.Errorf("invalid stack name")
	}
	return nil
}

func expandHome(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, path[2:]), nil
	}
	return path, nil
}

const DefaultTemplate = `services:
  app:
    image: nginx:alpine
    ports:
      - "8080:80"
    restart: unless-stopped
`

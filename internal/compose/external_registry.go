package compose

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// ExternalEntry records a compose project discovered outside the managed stacks dir.
type ExternalEntry struct {
	Project string `json:"project"`
	Path    string `json:"path"`
}

// ExternalRegistry persists known external compose project paths.
type ExternalRegistry struct {
	mu    sync.Mutex
	path  string
	items map[string]ExternalEntry
}

func NewExternalRegistry(stacksDir string) (*ExternalRegistry, error) {
	dir, err := expandHome(stacksDir)
	if err != nil {
		return nil, err
	}
	dataDir := filepath.Dir(dir)
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	reg := &ExternalRegistry{
		path:  filepath.Join(dataDir, "external-stacks.json"),
		items: make(map[string]ExternalEntry),
	}
	if err := reg.load(); err != nil {
		return nil, err
	}
	return reg, nil
}

func (r *ExternalRegistry) Remember(project, workDir string) error {
	if project == "" || workDir == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if existing, ok := r.items[project]; ok && existing.Path == workDir {
		return nil
	}
	r.items[project] = ExternalEntry{Project: project, Path: workDir}
	return r.saveLocked()
}

func (r *ExternalRegistry) Forget(project string) error {
	if project == "" {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[project]; !ok {
		return nil
	}
	delete(r.items, project)
	return r.saveLocked()
}

func (r *ExternalRegistry) List() []ExternalEntry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]ExternalEntry, 0, len(r.items))
	for _, e := range r.items {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Project < out[j].Project })
	return out
}

func (r *ExternalRegistry) load() error {
	b, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read external stacks: %w", err)
	}
	var entries []ExternalEntry
	if err := json.Unmarshal(b, &entries); err != nil {
		return fmt.Errorf("parse external stacks: %w", err)
	}
	for _, e := range entries {
		if e.Project == "" || e.Path == "" {
			continue
		}
		r.items[e.Project] = e
	}
	return nil
}

func (r *ExternalRegistry) saveLocked() error {
	entries := make([]ExternalEntry, 0, len(r.items))
	for _, e := range r.items {
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Project < entries[j].Project })
	b, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("write external stacks: %w", err)
	}
	return os.Rename(tmp, r.path)
}

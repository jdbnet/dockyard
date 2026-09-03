package engine

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/jdbnet/dockyard/internal/compose"
	"github.com/jdbnet/dockyard/internal/docker"
)

// Stack is a compose stack (alias for API consumers).
type Stack = compose.Stack

func (e *Engine) Stacks(ctx context.Context) ([]Stack, error) {
	managed, err := e.stacks.List()
	if err != nil {
		return nil, err
	}

	containers, _ := e.Containers(ctx)
	counts := make(map[string]struct{ total, running int })
	dirs := make(map[string]string)

	for _, c := range containers {
		if c.ComposeProject == "" {
			continue
		}
		n := counts[c.ComposeProject]
		n.total++
		if c.State == "running" {
			n.running++
		}
		counts[c.ComposeProject] = n
		if c.ComposeWorkDir != "" {
			dirs[c.ComposeProject] = c.ComposeWorkDir
		}
	}

	out := make([]Stack, 0, len(managed)+len(dirs))
	seen := make(map[string]bool)
	for i := range managed {
		s := managed[i]
		if n, ok := counts[s.Project]; ok {
			s.ContainerCount = n.total
			s.RunningCount = n.running
		}
		out = append(out, s)
		seen[s.Name] = true
	}

	for project, wd := range dirs {
		if seen[project] {
			continue
		}
		if err := e.external.Remember(project, wd); err != nil {
			return nil, fmt.Errorf("remember external stack %q: %w", project, err)
		}
		ext, err := compose.StackFromPath(project, wd)
		if err != nil {
			continue
		}
		applyStackCounts(&ext, counts[project].total, counts[project].running)
		out = append(out, ext)
		seen[project] = true
	}

	for _, entry := range e.external.List() {
		if seen[entry.Project] {
			continue
		}
		ext, err := compose.StackFromPath(entry.Project, entry.Path)
		if err != nil {
			_ = e.external.Forget(entry.Project)
			continue
		}
		applyStackCounts(&ext, counts[entry.Project].total, counts[entry.Project].running)
		out = append(out, ext)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func applyStackCounts(s *Stack, total, running int) {
	s.ContainerCount = total
	s.RunningCount = running
}

func (e *Engine) stackByName(ctx context.Context, name string) (Stack, error) {
	stacks, err := e.Stacks(ctx)
	if err != nil {
		return Stack{}, err
	}
	for _, s := range stacks {
		if s.Name == name || s.Project == name {
			return s, nil
		}
	}
	return Stack{}, fmt.Errorf("stack %q not found", name)
}

// StackCompose is compose file content and edit permissions for a stack.
type StackCompose struct {
	Name     string `json:"name"`
	Content  string `json:"content"`
	Managed  bool   `json:"managed"`
	Editable bool   `json:"editable"`
	Path     string `json:"path"`
}

func (e *Engine) GetStackCompose(ctx context.Context, name string) (StackCompose, error) {
	stack, err := e.stackByName(ctx, name)
	if err != nil {
		return StackCompose{}, err
	}
	content, err := e.readStackContent(stack)
	if err != nil {
		return StackCompose{}, err
	}
	return StackCompose{
		Name:     stack.Name,
		Content:  content,
		Managed:  stack.Managed,
		Editable: stack.IsEditable(),
		Path:     stack.Path,
	}, nil
}

func (e *Engine) readStackContent(stack Stack) (string, error) {
	if stack.Managed {
		return e.stacks.Read(stack.Name)
	}
	b, err := os.ReadFile(stack.ComposeFile)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (e *Engine) ReadStackCompose(ctx context.Context, name string) (string, error) {
	info, err := e.GetStackCompose(ctx, name)
	if err != nil {
		return "", err
	}
	return info.Content, nil
}

func (e *Engine) SaveStackCompose(ctx context.Context, name, content string) error {
	if err := compose.ValidateYAML(content); err != nil {
		return err
	}
	stack, err := e.stackByName(ctx, name)
	if err != nil {
		return e.stacks.Create(name, content)
	}
	if !stack.IsEditable() {
		return fmt.Errorf("stack %q is read-only", name)
	}
	if stack.Managed {
		if err := e.stacks.Write(name, content); err != nil {
			return err
		}
		return e.refreshAll(ctx)
	}
	if err := os.WriteFile(stack.ComposeFile, []byte(content), 0o644); err != nil {
		return err
	}
	return e.refreshAll(ctx)
}

func (e *Engine) CreateStack(ctx context.Context, name, content string, start bool) error {
	if err := compose.ValidateYAML(content); err != nil {
		return err
	}
	if err := e.stacks.Create(name, content); err != nil {
		return err
	}
	if !start {
		return e.refreshAll(ctx)
	}
	stack, err := e.stackByName(ctx, name)
	if err != nil {
		return err
	}
	if err := e.stacks.Up(ctx, stack); err != nil {
		return err
	}
	return e.refreshAll(ctx)
}

func (e *Engine) UpdateStack(ctx context.Context, name string) error {
	stack, err := e.stackByName(ctx, name)
	if err != nil {
		return err
	}
	if err := e.stacks.Update(ctx, stack); err != nil {
		return err
	}
	return e.refreshAll(ctx)
}

func (e *Engine) StackUp(ctx context.Context, name string) error {
	stack, err := e.stackByName(ctx, name)
	if err != nil {
		return err
	}
	if err := e.stacks.Up(ctx, stack); err != nil {
		return err
	}
	return e.refreshAll(ctx)
}

func (e *Engine) StackDown(ctx context.Context, name string) error {
	stack, err := e.stackByName(ctx, name)
	if err != nil {
		return err
	}
	if err := e.stacks.Down(ctx, stack); err != nil {
		return err
	}
	return e.refreshAll(ctx)
}

func (e *Engine) DeleteStack(ctx context.Context, name string) error {
	stack, err := e.stackByName(ctx, name)
	if err != nil {
		return err
	}
	if !stack.Managed {
		return fmt.Errorf("cannot delete external stack %q", name)
	}
	_ = e.stacks.Down(ctx, stack)
	if err := e.stacks.Delete(name); err != nil {
		return err
	}
	return e.refreshAll(ctx)
}

func (e *Engine) UpdateContainer(ctx context.Context, id string) error {
	insp, err := e.docker.InspectContainer(ctx, id)
	if err != nil {
		return err
	}
	project, service, workingDir := docker.ContainerComposeInfo(insp.Summary.Labels)

	if project != "" && service != "" {
		stack, stackErr := e.stackByName(ctx, project)
		if stackErr == nil {
			if err := e.stacks.UpdateService(ctx, stack, service); err == nil {
				return e.refreshAll(ctx)
			}
		}
		if workingDir != "" {
			ext, err := compose.StackFromPath(project, workingDir)
			if err == nil {
				if err := e.stacks.UpdateService(ctx, ext, service); err == nil {
					return e.refreshAll(ctx)
				}
			}
		}
	}

	if _, err := e.docker.RecreateContainer(ctx, id); err != nil {
		return err
	}
	return e.refreshAll(ctx)
}

func (e *Engine) StacksDir() string {
	return e.stacks.StacksDir()
}

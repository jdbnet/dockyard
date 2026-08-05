package engine

import (
	"context"
	"fmt"
	"os"
	"sort"

	"git.jdbnet.co.uk/jamie/dockyard/internal/compose"
	"git.jdbnet.co.uk/jamie/dockyard/internal/docker"
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
		ext, err := compose.StackFromPath(project, wd)
		if err != nil {
			continue
		}
		if n, ok := counts[project]; ok {
			ext.ContainerCount = n.total
			ext.RunningCount = n.running
		}
		out = append(out, ext)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
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

func (e *Engine) ReadStackCompose(_ context.Context, name string) (string, error) {
	stack, err := e.stackByName(context.Background(), name)
	if err != nil {
		return "", err
	}
	if stack.Managed {
		return e.stacks.Read(name)
	}
	b, err := os.ReadFile(stack.ComposeFile)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (e *Engine) SaveStackCompose(ctx context.Context, name, content string) error {
	stack, err := e.stackByName(ctx, name)
	if err != nil {
		return e.stacks.Create(name, content)
	}
	if !stack.Managed {
		return fmt.Errorf("cannot edit external stack %q (copy to stacks dir first)", name)
	}
	if err := e.stacks.Write(name, content); err != nil {
		return err
	}
	return e.refreshAll(ctx)
}

func (e *Engine) CreateStack(ctx context.Context, name, content string, start bool) error {
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

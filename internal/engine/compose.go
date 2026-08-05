package engine

import (
	"sort"
	"strings"
)

func groupComposeProjects(containers []Container) []ComposeProject {
	byProject := make(map[string][]Container)
	for _, c := range containers {
		project := c.ComposeProject
		if project == "" {
			project = StandaloneProject
		}
		byProject[project] = append(byProject[project], c)
	}

	names := make([]string, 0, len(byProject))
	for name := range byProject {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]ComposeProject, 0, len(names))
	for _, name := range names {
		list := byProject[name]
		sort.Slice(list, func(i, j int) bool {
			si := list[i].ComposeService
			sj := list[j].ComposeService
			if si == "" && sj == "" {
				return strings.ToLower(list[i].Name) < strings.ToLower(list[j].Name)
			}
			if si == "" {
				return false
			}
			if sj == "" {
				return true
			}
			return strings.ToLower(si) < strings.ToLower(sj)
		})
		out = append(out, ComposeProject{Name: name, Containers: list})
	}
	return out
}

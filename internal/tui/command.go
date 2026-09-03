package tui

import (
	"strings"
)

type commandDef struct {
	name    string
	aliases []string
	view    viewKind
	quit    bool
}

var navigatorCommands = []commandDef{
	{name: "containers", view: viewContainers},
	{name: "stacks", view: viewStacks, aliases: []string{"compose", "c"}},
	{name: "images", view: viewImages, aliases: []string{"i"}},
	{name: "volumes", view: viewVolumes, aliases: []string{"v"}},
	{name: "networks", view: viewNetworks, aliases: []string{"n"}},
	{name: "ports", view: viewPorts, aliases: []string{"p"}},
	{name: "quit", quit: true, aliases: []string{"q"}},
}

func normalizeCommandInput(input string) string {
	return strings.ToLower(strings.TrimSpace(input))
}

func commandCompletions(input string) []string {
	input = normalizeCommandInput(input)
	if input == "" {
		return nil
	}
	if strings.HasPrefix(input, "exec") || strings.HasPrefix("exec", input) {
		return nil
	}

	seen := make(map[string]struct{})
	var matches []string
	for _, def := range navigatorCommands {
		if strings.HasPrefix(def.name, input) {
			if _, ok := seen[def.name]; !ok {
				seen[def.name] = struct{}{}
				matches = append(matches, def.name)
			}
			continue
		}
		for _, alias := range def.aliases {
			if strings.HasPrefix(alias, input) {
				if _, ok := seen[def.name]; !ok {
					seen[def.name] = struct{}{}
					matches = append(matches, def.name)
				}
				break
			}
		}
	}
	return matches
}

func commandSuggestion(input string) (canonical, suffix string) {
	matches := commandCompletions(input)
	if len(matches) == 0 {
		return "", ""
	}
	canonical = matches[0]
	input = normalizeCommandInput(input)
	if strings.HasPrefix(canonical, input) {
		return canonical, canonical[len(input):]
	}
	for _, def := range navigatorCommands {
		if def.name != canonical {
			continue
		}
		for _, alias := range def.aliases {
			if strings.HasPrefix(alias, input) {
				return canonical, canonical[len(input):]
			}
		}
	}
	return canonical, ""
}

func resolveNavigatorCommand(input string) (def commandDef, ok bool) {
	input = normalizeCommandInput(input)
	if input == "" {
		return commandDef{}, false
	}
	for _, def := range navigatorCommands {
		if input == def.name {
			return def, true
		}
		for _, alias := range def.aliases {
			if input == alias {
				return def, true
			}
		}
	}
	matches := commandCompletions(input)
	if len(matches) == 1 {
		for _, def := range navigatorCommands {
			if def.name == matches[0] {
				return def, true
			}
		}
	}
	return commandDef{}, false
}

func execSuggestion(input string) (suffix string) {
	input = normalizeCommandInput(input)
	if input == "" || strings.HasPrefix(input, "exec ") {
		return ""
	}
	if strings.HasPrefix("exec", input) {
		return "exec"[len(input):]
	}
	return ""
}

package tui

import "testing"

func TestCommandCompletionsUniquePrefix(t *testing.T) {
	matches := commandCompletions("im")
	if len(matches) != 1 || matches[0] != "images" {
		t.Fatalf("expected [images], got %v", matches)
	}
}

func TestCommandSuggestionSuffix(t *testing.T) {
	_, suffix := commandSuggestion("im")
	if suffix != "ages" {
		t.Fatalf("expected suffix ages, got %q", suffix)
	}
}

func TestResolveNavigatorCommandPartial(t *testing.T) {
	def, ok := resolveNavigatorCommand("im")
	if !ok || def.name != "images" || def.view != viewImages {
		t.Fatalf("expected images view, got ok=%v def=%+v", ok, def)
	}
}

func TestResolveNavigatorCommandAlias(t *testing.T) {
	def, ok := resolveNavigatorCommand("i")
	if !ok || def.name != "images" {
		t.Fatalf("expected images, got ok=%v def=%+v", ok, def)
	}
}

func TestCommandCompletionsAmbiguous(t *testing.T) {
	matches := commandCompletions("c")
	if len(matches) < 2 {
		t.Fatalf("expected multiple matches for c, got %v", matches)
	}
}

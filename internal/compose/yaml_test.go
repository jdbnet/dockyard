package compose

import "testing"

func TestValidateYAML_valid(t *testing.T) {
	if err := ValidateYAML("services:\n  app:\n    image: nginx\n"); err != nil {
		t.Fatalf("expected valid yaml: %v", err)
	}
}

func TestValidateYAML_invalid(t *testing.T) {
	if err := ValidateYAML("services:\n  app: [\n"); err == nil {
		t.Fatal("expected yaml error")
	}
}

func TestValidateYAML_empty(t *testing.T) {
	if err := ValidateYAML(""); err != nil {
		t.Fatalf("empty yaml should parse: %v", err)
	}
}

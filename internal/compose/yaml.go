package compose

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidateYAML checks compose content parses as YAML.
func ValidateYAML(content string) error {
	var doc any
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		return formatYAMLError(err)
	}
	return nil
}

func formatYAMLError(err error) error {
	var ye *yaml.TypeError
	if ok := asTypeError(err, &ye); ok {
		return fmt.Errorf("yaml type error: %v", ye)
	}
	// yaml.v3 line errors embed line number in message for ScannerError
	if strings.Contains(err.Error(), "yaml:") {
		return err
	}
	return fmt.Errorf("invalid yaml: %w", err)
}

func asTypeError(err error, target **yaml.TypeError) bool {
	if err == nil {
		return false
	}
	if te, ok := err.(*yaml.TypeError); ok {
		*target = te
		return true
	}
	return false
}

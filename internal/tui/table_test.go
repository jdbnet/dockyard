package tui

import "testing"

func TestFormatTableRowAlignsColumns(t *testing.T) {
	widths := []int{20, 12, 8}
	row := formatTableRow([]string{"uptime-kuma-uptime-kuma-1", "uptime-kuma", "running"}, widths)
	header := formatTableRow([]string{"NAME", "STACK", "STATE"}, widths)

	// Header and row should be same length (fixed-width columns)
	if len(row) != len(header) {
		t.Fatalf("header/row length mismatch: header=%d row=%d\nheader: %q\nrow: %q", len(header), len(row), header, row)
	}
}

func TestPadCellTruncatesLongValues(t *testing.T) {
	got := padCell("very-long-container-name-that-exceeds-limit", 10)
	if len(got) != 10 {
		t.Fatalf("expected width 10, got %q (%d)", got, len(got))
	}
}

func TestComputeWidthsUsesTerminalWidth(t *testing.T) {
	specs := stackCols()
	rows := []rowItem{{
		cols: []string{"my-stack", "1/2", "managed", "/home/jamie/stacks/my-stack"},
	}}
	widths := computeWidths(specs, rows, 118)
	if tableLineWidth(widths) < 118 {
		t.Fatalf("expected table to use content width, got %d (widths=%v)", tableLineWidth(widths), widths)
	}
	if widths[0] < len("my-stack") {
		t.Fatalf("NAME column too narrow: %d", widths[0])
	}
	if widths[3] < len("/home/jamie/stacks/my-stack") {
		t.Fatalf("PATH column too narrow: %d", widths[3])
	}
}

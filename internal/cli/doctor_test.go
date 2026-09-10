package cli

import (
	"bytes"
	"testing"
)

func TestPrintDoctorLineColor(t *testing.T) {
	var out bytes.Buffer
	printDoctorLine(&out, true, colorGreen, "OS: %s", "linux")
	got := out.String()
	if got != "\033[32mOS: linux\033[0m\n" {
		t.Fatalf("colored output = %q", got)
	}
}

func TestShouldColorModes(t *testing.T) {
	var out bytes.Buffer
	if !shouldColor(&out, "always") {
		t.Fatal("always should enable color")
	}
	if shouldColor(&out, "never") {
		t.Fatal("never should disable color")
	}
	if shouldColor(&out, "auto") {
		t.Fatal("non-terminal buffer should disable auto color")
	}
}

func TestNoColorDisablesAutoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var out bytes.Buffer
	if shouldColor(&out, "auto") {
		t.Fatal("NO_COLOR should disable auto color")
	}
}

func TestParseCodexVersion(t *testing.T) {
	tests := []struct {
		output string
		want   string
	}{
		{"WARNING: path setup failed\ncodex-cli 0.153.4\n", "codex-cli 0.153.4"},
		{"codex 1.2.3\n", "codex 1.2.3"},
	}
	for _, tt := range tests {
		got, err := parseCodexVersion(tt.output)
		if err != nil {
			t.Fatalf("parseCodexVersion(%q): %v", tt.output, err)
		}
		if got != tt.want {
			t.Fatalf("parseCodexVersion(%q) = %q, want %q", tt.output, got, tt.want)
		}
	}
}

func TestParseCodexVersionRejectsUnexpectedOutput(t *testing.T) {
	if _, err := parseCodexVersion("WARNING: no version here"); err == nil {
		t.Fatal("expected unexpected output error")
	}
}

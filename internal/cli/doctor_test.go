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

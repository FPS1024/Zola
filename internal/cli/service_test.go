package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindLaunchdServiceFromOverrides(t *testing.T) {
	dir := t.TempDir()
	launchctl := filepath.Join(dir, "launchctl")
	plist := filepath.Join(dir, "service.plist")
	if err := os.WriteFile(launchctl, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plist, []byte("<plist/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZOLA_LAUNCHCTL", launchctl)
	t.Setenv("ZOLA_LAUNCHD_PLIST", plist)

	service, err := findLaunchdService()
	if err != nil {
		t.Fatalf("findLaunchdService() error = %v", err)
	}
	if service.launchctl != launchctl {
		t.Fatalf("launchctl = %q, want %q", service.launchctl, launchctl)
	}
	if service.plist != plist {
		t.Fatalf("plist = %q, want %q", service.plist, plist)
	}
}

func TestFirstLines(t *testing.T) {
	if got := firstLines("one\ntwo\nthree\nfour", 2); got != "one | two" {
		t.Fatalf("firstLines() = %q", got)
	}
}

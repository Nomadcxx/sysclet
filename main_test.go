package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRenderCommandWritesANSIArt(t *testing.T) {
	fontPath := filepath.Join(t.TempDir(), "tiny.tdf")
	if err := os.WriteFile(fontPath, testTDF(), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"render", "--font", fontPath, "A"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
	want := "\x1b[0;31;44m─\x1b[0m\n\x1b[0;94m│\x1b[0m\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
}

func TestRunExportCreatesMissingProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new-profile.conf")
	var stdout, stderr bytes.Buffer
	code := run([]string{"export", "--font-name", "Bleach", "--slot", "ascii_1", "--config", path, "A"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "created "+path) || strings.Contains(stderr.String(), "backup:") {
		t.Fatalf("stderr = %q, want created path without a backup", stderr.String())
	}
	profile, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(profile), "ansi_style=true\nascii_1=\"\"\"") {
		t.Fatalf("created profile = %q, missing ANSI slot", profile)
	}
}

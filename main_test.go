package main

import (
	"bytes"
	"os"
	"path/filepath"
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

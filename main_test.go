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

func TestRunRenderCommandAcceptsMultipleRows(t *testing.T) {
	fontPath := filepath.Join(t.TempDir(), "tiny.tdf")
	if err := os.WriteFile(fontPath, testTDF(), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{"render", "--font", fontPath, "A\nA"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run() = %d, stderr = %q", code, stderr.String())
	}
	row := "\x1b[0;31;44m─\x1b[0m\n\x1b[0;94m│\x1b[0m"
	if want := row + "\n" + row + "\n"; stdout.String() != want {
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

func TestVersionFlagPrintsStampedVersion(t *testing.T) {
	defer func(old string) { version = old }(version)
	version = "v9.9.9"
	for _, arg := range []string{"--version", "-V", "version"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{arg}, &stdout, &stderr); code != 0 || stdout.String() != "sysclet v9.9.9\n" {
			t.Errorf("run(%q) = %d, stdout %q", arg, code, stdout.String())
		}
	}
}

func TestFontsCommandFiltersCatalog(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"fonts", "rivendell"}, &stdout, &stderr); code != 0 {
		t.Fatalf("run(fonts) = %d, stderr %q", code, stderr.String())
	}
	if got := strings.Fields(stdout.String()); len(got) == 0 || got[0] != "Rivendell" {
		t.Fatalf("fonts rivendell = %q, want Rivendell first", stdout.String())
	}
	if code := run([]string{"fonts", "no-such-font-zz"}, &stdout, &stderr); code != 1 {
		t.Fatalf("run(fonts no match) = %d, want 1", code)
	}
}

func TestExportFormatsWriteStdoutFilesAndSyscTargets(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"export", "--font-name", "Bleach", "--format", "txt", "SYSC"}, &stdout, &stderr); code != 0 {
		t.Fatalf("txt export = %d, stderr %q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "\x1b") || !strings.Contains(stdout.String(), "▄") {
		t.Fatalf("txt export = %q, want plain block art", stdout.String())
	}

	out := filepath.Join(t.TempDir(), "header")
	if code := run([]string{"export", "--font-name", "Bleach", "--format", "ANSI", "--output", out, "SYSC"}, &stdout, &stderr); code != 0 {
		t.Fatalf("ansi file export = %d, stderr %q", code, stderr.String())
	}
	if data, err := os.ReadFile(out + ".ans"); err != nil || !strings.Contains(string(data), "\x1b[") {
		t.Fatalf("ansi file = %q, %v; want ANSI art", data, err)
	}

	t.Setenv("HOME", t.TempDir())
	stderr.Reset()
	if code := run([]string{"export", "--font-name", "Bleach", "--format", "sysc-terminal", "--output", "hdr", "SYSC"}, &stdout, &stderr); code != 0 {
		t.Fatalf("sysc-terminal export = %d, stderr %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "sysc-terminal --effect beam-text --file") {
		t.Fatalf("sysc-terminal export stderr = %q, want the run command", stderr.String())
	}

	for _, args := range [][]string{
		{"export", "--font-name", "Bleach", "--format", "png", "SYSC"},
		{"export", "--font-name", "Bleach", "--format", "txt", "--slot", "ascii_1", "SYSC"},
		{"export", "--font-name", "Bleach", "--format", "sysc-walls", "SYSC"},
		{"export", "--font-name", "Bleach", "--slot", "ascii_1", "--output", out, "SYSC"},
	} {
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("run(%q) = %d, want usage error 2", args, code)
		}
	}
}

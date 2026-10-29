package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestANSIExportPreservesSGRSequences(t *testing.T) {
	art := "\x1b[0;31mheader\x1b[0m\n"
	got, err := generateExportContent(exportANSI, "", art)
	if err != nil {
		t.Fatal(err)
	}
	if got != art {
		t.Fatalf("ANSI export = %q, want exact input %q", got, art)
	}
	if extension := exportExtension(exportANSI); extension != ".ans" {
		t.Fatalf("ANSI extension = %q, want .ans", extension)
	}
}

func TestAppendExportExtensionAcceptsUppercaseExtension(t *testing.T) {
	if got := appendExportExtension("header.ANS", exportANSI); got != "header.ANS" {
		t.Fatalf("appendExportExtension() = %q, want header.ANS", got)
	}
}

func TestExportDisplayHints(t *testing.T) {
	path := "/tmp/my header"
	tests := []struct {
		format exportFormat
		want   string
	}{
		{exportSyscGreet, "Select the Beams effect in sysc-greet to animate this profile art."},
		{exportANSI, "Display in a terminal: cat '/tmp/my header.ans' or less -R '/tmp/my header.ans'"},
		{exportTXT, "View in a terminal: cat '/tmp/my header.txt' or less '/tmp/my header.txt' · plain text; ANSI colors are removed."},
		{exportGO, "Run in a terminal: go run '/tmp/my header.go'"},
		{exportJS, "Run in a terminal: node '/tmp/my header.js'"},
		{exportPY, "Run in a terminal: python3 '/tmp/my header.py'"},
		{exportRS, "Run in a terminal: rustc '/tmp/my header.rs' -o sysclet-header && ./sysclet-header"},
		{exportSH, "Run in a terminal: sh '/tmp/my header.sh'"},
	}
	for _, test := range tests {
		t.Run(string(test.format), func(t *testing.T) {
			if got := exportDisplayHint(test.format, path); got != test.want {
				t.Fatalf("exportDisplayHint() = %q, want %q", got, test.want)
			}
		})
	}
	want := `Display in a terminal: cat '/tmp/a'\''b.ans' or less -R '/tmp/a'\''b.ans'`
	if got := exportDisplayHint(exportANSI, "/tmp/a'b"); got != want {
		t.Fatalf("exportDisplayHint() with an apostrophe = %q, want %q", got, want)
	}
}

func TestClipExportPreviewKeepsWidthAndLineLimits(t *testing.T) {
	got := clipExportPreview("\x1b[0;31m123456789\x1b[0m\nsecond line\nthird line\n", 5, 2)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("clipExportPreview() returned %d lines, want 2: %q", len(lines), got)
	}
	for i, line := range lines {
		if width := ansi.StringWidth(line); width > 5 {
			t.Errorf("preview line %d has visible width %d, want at most 5", i+1, width)
		}
	}
	if !strings.Contains(lines[0], "\x1b[0;31m") {
		t.Fatalf("truncated ANSI preview lost its color sequence: %q", got)
	}
}

func TestRepositoryHeaderIsLiteralSyscGreetProfile(t *testing.T) {
	profile, err := os.ReadFile("assets/sysclet-header.conf")
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(string(profile), '\x1b') {
		t.Fatal("header profile must store literal \\033 escapes")
	}
	if !strings.HasPrefix(string(profile), "ansi_style=true\nascii_1=\"\"\"\n") || !strings.HasSuffix(string(profile), "\n\"\"\"\n") {
		t.Fatal("header profile is missing the sysc-greet setting or slot delimiters")
	}
	sum := sha256.Sum256(profile)
	got := hex.EncodeToString(sum[:])
	if want := "cfc3b5c3bc1392e34d2f77afd890fa9197374b434d657b3045b431ff24399334"; got != want {
		t.Fatalf("header profile hash = %s, want %s", got, want)
	}
}

func TestSyscWallsExportWritesPlainArtAndKeepsConfigLines(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	configPath := home + "/.config/sysc-walls/daemon.conf"
	if err := os.MkdirAll(home+"/.config/sysc-walls", 0o700); err != nil {
		t.Fatal(err)
	}
	config := "# mine\n[animation]\neffect = matrix-art\n# file = old\ntheme = rama\n\n[terminal]\nkitty = true\n"
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}

	path, _, configBackup, err := saveSyscArt("header", exportSyscWalls, "\x1b[0;31m▄█\x1b[0m\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := home + "/.local/share/syscgo/walls/header.txt"; path != want {
		t.Fatalf("art path = %q, want %q", path, want)
	}
	if art, _ := os.ReadFile(path); string(art) != "▄█\n" {
		t.Fatalf("art = %q, want ANSI stripped", art)
	}
	got, _ := os.ReadFile(configPath)
	want := "# mine\n[animation]\neffect = beam-text\n# file = old\ntheme = dracula\nfile = " + path + "\ncycle = false\n\n[terminal]\nkitty = true\n"
	if string(got) != want {
		t.Fatalf("daemon.conf =\n%s\nwant\n%s", got, want)
	}
	if backup, _ := os.ReadFile(configBackup); string(backup) != config {
		t.Fatalf("config backup = %q, want original", backup)
	}
	if _, err := resolveSyscArtPath("/etc/header"); err == nil {
		t.Fatal("resolveSyscArtPath accepted a path sysc-walls cannot read")
	}
}

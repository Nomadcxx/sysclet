package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileSnippetEscapesANSIForSyscGreet(t *testing.T) {
	got, err := ProfileSnippet("ascii_2", "\x1b[0;31mX\x1b[0m\nY")
	if err != nil {
		t.Fatal(err)
	}
	want := "ansi_style=true\nascii_2=\"\"\"\n\\033[0;31mX\\033[0m\nY\n\"\"\"\n"
	if got != want {
		t.Fatalf("ProfileSnippet() = %q, want %q", got, want)
	}
	if strings.ContainsRune(got, '\x1b') {
		t.Fatal("profile snippet contains an actual ESC byte")
	}
}

func TestUpdateProfileReplacesOnlySelectedSlotAndCreatesBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greeter.conf")
	original := "name=demo\nansi_style=false\nascii_1=\"\"\"\nold\n\"\"\"\nascii_2=\"\"\"\nkeep=art\n\"\"\"\ncolor=green\n"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}

	backupPath, err := UpdateProfile(path, "ascii_1", "\x1b[0;31mnew\x1b[0m\nline")
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatalf("backup = %q, want original profile", backup)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(updated)
	for _, want := range []string{
		"name=demo",
		"ansi_style=true",
		"ascii_1=\"\"\"\n\\033[0;31mnew\\033[0m\nline\n\"\"\"",
		"ascii_2=\"\"\"\nkeep=art\n\"\"\"",
		"color=green",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("updated profile does not contain %q:\n%s", want, got)
		}
	}
}

func TestUpdateProfileRejectsInvalidSlotWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greeter.conf")
	original := "name=demo\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateProfile(path, "ascii_0", "art"); err == nil {
		t.Fatal("UpdateProfile accepted ascii_0")
	}
	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(updated) != original {
		t.Fatalf("profile changed after invalid slot: %q", updated)
	}
}

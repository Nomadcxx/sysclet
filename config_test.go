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

func TestUpdateProfileCreatesMissingProfile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new-profile.conf")
	backupPath, err := UpdateProfile(path, "ascii_2", "\x1b[0;31mnew\x1b[0m")
	if err != nil {
		t.Fatal(err)
	}
	if backupPath != "" {
		t.Fatalf("new profile backup = %q, want none", backupPath)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ProfileSnippet("ascii_2", "\x1b[0;31mnew\x1b[0m")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("new profile = %q, want %q", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o200 == 0 || info.Mode().Perm()&0o022 != 0 {
		t.Fatalf("new profile mode = %04o, want owner writable and not group/world writable", info.Mode().Perm())
	}
}

func TestUpdateProfileReplacesOnlySelectedSlotAndCreatesBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greeter.conf")
	original := "name=demo\nansi_style=false\nascii_1=\"\"\"\nold\n\"\"\"\nascii_2=\"\"\"\nkeep=art\n\"\"\"\ncolor=green\n"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	originalInfo, err := os.Stat(path)
	if err != nil {
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
	updatedInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if updatedInfo.Mode().Perm() != originalInfo.Mode().Perm() {
		t.Fatalf("updated profile mode = %04o, want original mode %04o", updatedInfo.Mode().Perm(), originalInfo.Mode().Perm())
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

func TestUpdateProfilePreservesMixedLineEndingsOutsideSlot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "greeter.conf")
	original := "name=demo\r\n# keep LF\nansi_style=false\r\nascii_1=\"\"\"\r\nold\r\n\"\"\"\r\ncolor=green\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateProfile(path, "ascii_1", "\x1b[0;31mnew\x1b[0m"); err != nil {
		t.Fatal(err)
	}

	updated, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updated), "name=demo\r\n# keep LF\nansi_style=true\r\n") {
		t.Fatalf("unrelated line endings changed: %q", updated)
	}
	if !strings.HasSuffix(string(updated), "color=green\n") {
		t.Fatalf("trailing LF line ending changed: %q", updated)
	}
}

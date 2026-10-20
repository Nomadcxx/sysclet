package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWithEnvReplacesExistingValue(t *testing.T) {
	got := withEnv([]string{"PATH=/usr/bin", "GOBIN=/old", "GOBIN=/older"}, "GOBIN", "/home/user/bin")

	var values []string
	for _, entry := range got {
		key, value, ok := strings.Cut(entry, "=")
		if ok && key == "GOBIN" {
			values = append(values, value)
		}
	}
	if len(values) != 1 || values[0] != "/home/user/bin" {
		t.Fatalf("GOBIN entries = %q, want one value /home/user/bin", values)
	}
}

func TestUserInstallDir(t *testing.T) {
	got, err := userInstallDir("/home/alice")
	if err != nil {
		t.Fatalf("userInstallDir returned an error: %v", err)
	}
	if want := "/home/alice/.local/bin"; got != want {
		t.Fatalf("userInstallDir = %q, want %q", got, want)
	}

	if _, err := userInstallDir("relative/home"); err == nil {
		t.Fatal("userInstallDir accepted a relative home directory")
	}
}

func TestInstallCmdTargetsUserBinAndLatestModule(t *testing.T) {
	temp := t.TempDir()
	binDir := filepath.Join(temp, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(temp, "go-args")
	fakeGo := filepath.Join(binDir, "go")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" \"$GOBIN\" > \"$CAPTURE\"\n"
	if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("CAPTURE", capture)

	installDir := filepath.Join(temp, "user", ".local", "bin")
	message := installCmd(context.Background(), installDir)()
	result, ok := message.(installResultMsg)
	if !ok {
		t.Fatalf("installCmd returned %T, want installResultMsg", message)
	}
	if result.err != nil {
		t.Fatalf("installCmd returned an error: %v", result.err)
	}

	got, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	want := "install " + syscletModule + "\n" + installDir + "\n"
	if string(got) != want {
		t.Fatalf("go invocation = %q, want %q", got, want)
	}
}

func TestInstallStartsOnlyAfterEnter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newModel(ctx, cancel, "/tmp/sysclet-test-bin")
	if m.installing {
		t.Fatal("installer started before user confirmation")
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updatedModel, ok := updated.(model)
	if !ok {
		t.Fatalf("Update returned %T, want model", updated)
	}
	if !updatedModel.installing || cmd == nil {
		t.Fatal("Enter did not start the install action")
	}
}

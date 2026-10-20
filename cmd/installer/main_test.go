package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" \"$GOBIN\" > \"$CAPTURE\"\nprintf 'go: downloading test.module v1.0.0\\n' >&2\nprintf 'build complete\\n'\n"
	if err := os.WriteFile(fakeGo, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)
	t.Setenv("CAPTURE", capture)

	installDir := filepath.Join(temp, "user", ".local", "bin")
	message := installCmd(context.Background(), installDir)()
	started, ok := message.(installStartedMsg)
	if !ok {
		t.Fatalf("installCmd returned %T, want installStartedMsg", message)
	}
	if started.err != nil {
		t.Fatalf("installCmd returned an error: %v", started.err)
	}
	if started.stream == nil {
		t.Fatal("installCmd did not return a live output stream")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newModel(ctx, cancel, installDir)
	m.installing = true
	m.stream = started.stream
	seen := map[string]bool{}
	read := readInstallOutputCmd(started.stream)
	for i := 0; i < 2; i++ {
		lineMessage := read()
		line, ok := lineMessage.(installOutputMsg)
		if !ok {
			t.Fatalf("readInstallOutputCmd returned %T, want installOutputMsg", lineMessage)
		}
		seen[line.line] = true
		updated, next := m.Update(line)
		m = updated.(model)
		if !strings.Contains(m.output, line.line) {
			t.Fatalf("progress view state %q does not contain streamed line %q", m.output, line.line)
		}
		read = next
	}
	if !seen["go: downloading test.module v1.0.0"] || !seen["build complete"] {
		t.Fatalf("streamed output = %#v, want download and build lines", seen)
	}
	doneMessage := read()
	result, ok := doneMessage.(installDoneMsg)
	if !ok {
		t.Fatalf("readInstallOutputCmd returned %T, want installDoneMsg", doneMessage)
	}
	if result.err != nil {
		t.Fatalf("install command failed: %v", result.err)
	}
	updated, _ := m.Update(result)
	m = updated.(model)
	if !m.done || !strings.Contains(m.output, filepath.Join(installDir, "sysclet")) {
		t.Fatalf("completed installer state = done %v, output %q", m.done, m.output)
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

func TestInstallerProgressShowsArtifactAndExactDestination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newModel(ctx, cancel, "/home/alice/.local/bin")
	m.installing = true
	m.output = "go: downloading example.org/sysclet-fonts v1.0.0"
	view := ansi.Strip(m.View())
	for _, want := range []string{
		"github.com/Nomadcxx/sysclet@latest",
		"/home/alice/.local/bin/sysclet",
		"TDF fonts are bundled in the binary",
		"go: downloading example.org/sysclet-fonts v1.0.0",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("installer view is missing %q", want)
		}
	}
}

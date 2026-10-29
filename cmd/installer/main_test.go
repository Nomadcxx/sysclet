package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

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

func TestInstallerViewShowsReleaseInstallAndExactDestination(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := newModel(ctx, cancel, "/home/alice/.local/bin")
	m.installing = true
	view := ansi.Strip(m.View())
	for _, want := range []string{
		"Downloading and verifying sysclet release",
		"/home/alice/.local/bin/sysclet",
		"TDF fonts are bundled in the binary",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("installer view is missing %q", want)
		}
	}
}

func TestInstallerViewWrapsLongInstallPaths(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deepPath := strings.TrimSuffix(strings.Repeat("deep"+string(filepath.Separator), 20), string(filepath.Separator))
	installDir := filepath.Join(t.TempDir(), "alice smith", deepPath, "bin")
	installPath := filepath.Join(installDir, "sysclet")
	m := newModel(ctx, cancel, installDir)
	m.width = 80
	m.height = 50
	m.installing = true

	views := []string{m.View()}
	m.installing = false
	m.done = true
	views = append(views, m.View())

	widestLine := 0
	for _, view := range views {
		plain := ansi.Strip(view)
		compact := strings.Join(strings.Fields(plain), "")
		for _, line := range strings.Split(plain, "\n") {
			if width := ansi.StringWidth(line); width > widestLine {
				widestLine = width
			}
		}
		if !strings.Contains(compact, strings.ReplaceAll(installPath, " ", "")) {
			t.Errorf("installer view lost part of install path %q", installPath)
		}
		if !strings.Contains(plain, "alice smith") {
			t.Error("installer view did not preserve spaces in the install path")
		}
	}
	if widestLine > m.width {
		t.Errorf("installer view line is %d columns wide, terminal is %d", widestLine, m.width)
	}
}

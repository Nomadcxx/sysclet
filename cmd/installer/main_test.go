package main

import (
	"context"
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

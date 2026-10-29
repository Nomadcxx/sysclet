package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallReleaseReplacesBinaryWithVerifiedLinuxAsset(t *testing.T) {
	want := []byte("verified sysclet binary")
	manifest := checksumLine(want, "sysclet-linux-amd64")
	server := releaseTestServer(t, manifest, http.StatusOK, want)
	installDir := t.TempDir()
	binaryPath := filepath.Join(installDir, "sysclet")
	if err := os.WriteFile(binaryPath, []byte("old binary"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := installRelease(context.Background(), server.Client(), server.URL, installDir, "linux", "amd64"); err != nil {
		t.Fatalf("installRelease returned an error: %v", err)
	}

	got, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("installed bytes = %q, want %q", got, want)
	}
	info, err := os.Stat(binaryPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o755); got != want {
		t.Fatalf("installed mode = %04o, want %04o", got, want)
	}
	entries, err := os.ReadDir(installDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "sysclet" {
		t.Fatalf("install directory entries = %v, want only sysclet", entries)
	}
}

func TestInstallReleaseRejectsInvalidChecksumsAndPreservesExistingBinary(t *testing.T) {
	asset := []byte("new binary")
	tests := []struct {
		name     string
		manifest string
	}{
		{name: "mismatch", manifest: strings.Repeat("0", 64) + "  sysclet-linux-amd64\n"},
		{name: "missing entry", manifest: checksumLine(asset, "sysclet-linux-arm64")},
		{name: "malformed digest", manifest: "not-a-digest  sysclet-linux-amd64\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := releaseTestServer(t, test.manifest, http.StatusOK, asset)
			installDir := t.TempDir()
			binaryPath := filepath.Join(installDir, "sysclet")
			if err := os.WriteFile(binaryPath, []byte("existing binary"), 0o700); err != nil {
				t.Fatal(err)
			}

			err := installRelease(context.Background(), server.Client(), server.URL, installDir, "linux", "amd64")
			if err == nil {
				t.Fatal("installRelease succeeded with an invalid or missing checksum")
			}
			got, readErr := os.ReadFile(binaryPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(got) != "existing binary" {
				t.Fatalf("existing binary changed to %q", got)
			}
		})
	}
}

func TestInstallReleaseRejectsHTTPErrorAndPreservesExistingBinary(t *testing.T) {
	asset := []byte("new binary")
	manifest := checksumLine(asset, "sysclet-linux-amd64")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/SHA256SUMS" {
			_, _ = w.Write([]byte(manifest))
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	installDir := t.TempDir()
	binaryPath := filepath.Join(installDir, "sysclet")
	if err := os.WriteFile(binaryPath, []byte("existing binary"), 0o700); err != nil {
		t.Fatal(err)
	}

	err := installRelease(context.Background(), server.Client(), server.URL, installDir, "linux", "amd64")
	if err == nil {
		t.Fatal("installRelease succeeded after an HTTP error")
	}
	got, readErr := os.ReadFile(binaryPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "existing binary" {
		t.Fatalf("existing binary changed to %q", got)
	}
}

func TestInstallReleaseHonorsCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("canceled installation made an HTTP request")
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := installRelease(ctx, server.Client(), server.URL, t.TempDir(), "linux", "amd64")
	if err == nil || !strings.Contains(err.Error(), context.Canceled.Error()) {
		t.Fatalf("installRelease error = %v, want cancellation", err)
	}
}

func TestInstallReleaseRejectsUnsupportedPlatform(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unsupported platform made an HTTP request")
	}))
	defer server.Close()

	if err := installRelease(context.Background(), server.Client(), server.URL, t.TempDir(), "darwin", "arm64"); err == nil {
		t.Fatal("installRelease accepted an unsupported operating system")
	}
}

func releaseTestServer(t *testing.T, manifest string, assetStatus int, asset []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/SHA256SUMS":
			_, _ = w.Write([]byte(manifest))
		case "/sysclet-linux-amd64":
			w.WriteHeader(assetStatus)
			_, _ = w.Write(asset)
		default:
			http.NotFound(w, r)
		}
	}))
}

func checksumLine(data []byte, name string) string {
	digest := sha256.Sum256(data)
	return fmt.Sprintf("%x  %s\n", digest, name)
}

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallScriptSelectsAndRunsVerifiedLinuxInstaller(t *testing.T) {
	installer := []byte("#!/bin/sh\nprintf 'installer ran' > \"$CAPTURE\"\n")
	for _, test := range []struct {
		name      string
		machine   string
		assetName string
	}{
		{name: "amd64", machine: "x86_64", assetName: "sysclet-installer-linux-amd64"},
		{name: "arm64", machine: "aarch64", assetName: "sysclet-installer-linux-arm64"},
	} {
		t.Run(test.name, func(t *testing.T) {
			manifest := checksumLine(installer, test.assetName)
			capture, output, err := runInstallScript(t, test.machine, test.assetName, manifest, installer, true)
			if err != nil {
				t.Fatalf("install.sh failed: %v\n%s", err, output)
			}
			if capture != "installer ran" {
				t.Fatalf("installer output = %q, want installer ran", capture)
			}
		})
	}
}

func TestInstallScriptFallsBackWhenReleaseHasNoInstallerAsset(t *testing.T) {
	manifest := checksumLine([]byte("old app binary"), "sysclet-linux-amd64")
	capture, output, err := runInstallScript(t, "x86_64", "sysclet-installer-linux-amd64", manifest, nil, true)
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	if !strings.Contains(capture, "go run github.com/Nomadcxx/sysclet/cmd/installer@latest") {
		t.Fatalf("fallback invocation = %q, want Go source launcher", capture)
	}
}

func TestInstallScriptFallsBackOnUnsupportedArchitecture(t *testing.T) {
	manifest := checksumLine([]byte("app binary"), "sysclet-linux-amd64")
	capture, output, err := runInstallScript(t, "riscv64", "unsupported-installer", manifest, nil, true)
	if err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, output)
	}
	if !strings.Contains(capture, "go run github.com/Nomadcxx/sysclet/cmd/installer@latest") {
		t.Fatalf("fallback invocation = %q, want Go source launcher", capture)
	}
}

func TestInstallScriptStopsOnInstallerChecksumMismatch(t *testing.T) {
	installer := []byte("#!/bin/sh\nprintf 'installer ran' > \"$CAPTURE\"\n")
	manifest := fmt.Sprintf("%064d  sysclet-installer-linux-amd64\n", 0)
	capture, output, err := runInstallScript(t, "x86_64", "sysclet-installer-linux-amd64", manifest, installer, true)
	if err == nil {
		t.Fatalf("install.sh succeeded with a wrong checksum; output %s", output)
	}
	if capture != "" {
		t.Fatalf("a command ran despite checksum mismatch: %q", capture)
	}
}

func TestInstallScriptReportsGoFallbackWhenLegacyReleaseHasNoGo(t *testing.T) {
	manifest := checksumLine([]byte("old app binary"), "sysclet-linux-amd64")
	capture, output, err := runInstallScript(t, "x86_64", "sysclet-installer-linux-amd64", manifest, nil, false)
	if err == nil {
		t.Fatalf("install.sh succeeded without an installer asset or Go; output %s", output)
	}
	if capture != "" || !strings.Contains(output, "Go 1.21+") {
		t.Fatalf("capture = %q, output = %q; want actionable Go fallback", capture, output)
	}
}

func runInstallScript(t *testing.T, machine, assetName, manifest string, installer []byte, withGo bool) (string, string, error) {
	t.Helper()
	script, err := exec.LookPath("script")
	if err != nil {
		t.Skip("script(1) is required to run install.sh with its terminal check")
	}
	temp := t.TempDir()
	binDir := filepath.Join(temp, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sh", "awk", "sha256sum", "mktemp", "rm", "cp", "chmod"} {
		target, err := exec.LookPath(name)
		if err != nil {
			t.Fatalf("find %s for test PATH: %v", name, err)
		}
		if err := os.Symlink(target, filepath.Join(binDir, name)); err != nil {
			t.Fatal(err)
		}
	}
	manifestPath := filepath.Join(temp, "SHA256SUMS")
	if err := os.WriteFile(manifestPath, []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	installerPath := filepath.Join(temp, assetName)
	if installer != nil {
		if err := os.WriteFile(installerPath, installer, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	capturePath := filepath.Join(temp, "capture")

	writeStub(t, binDir, "id", "#!/bin/sh\nprintf '1000\\n'\n")
	writeStub(t, binDir, "uname", fmt.Sprintf("#!/bin/sh\nprintf '%s\\n'\n", machine))
	writeStub(t, binDir, "curl", "#!/bin/sh\n"+
		"out=\nurl=\n"+
		"while [ \"$#\" -gt 0 ]; do\n"+
		"  case \"$1\" in\n"+
		"    -o) out=$2; shift 2 ;;\n"+
		"    -w) shift 2 ;;\n"+
		"    -*) shift ;;\n"+
		"    *) url=$1; shift ;;\n"+
		"  esac\n"+
		"done\n"+
		"case \"$url\" in\n"+
		"  */SHA256SUMS) src=$TEST_MANIFEST ;;\n"+
		"  */sysclet-installer-linux-*) src=$TEST_INSTALLER ;;\n"+
		"  *) : > \"$out\"; printf '404'; exit 0 ;;\n"+
		"esac\n"+
		"if [ ! -f \"$src\" ]; then : > \"$out\"; printf '404'; exit 0; fi\n"+
		"cp \"$src\" \"$out\"\n"+
		"printf '200'\n")
	if withGo {
		writeStub(t, binDir, "go", "#!/bin/sh\nprintf 'go %s' \"$*\" > \"$CAPTURE\"\n")
	}

	installScript, err := filepath.Abs("../../install.sh")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(script, "-q", "-e", "-c", "sh "+shellQuote(installScript), "/dev/null")
	command.Env = []string{
		"PATH=" + binDir,
		"CAPTURE=" + capturePath,
		"TEST_MANIFEST=" + manifestPath,
		"TEST_INSTALLER=" + installerPath,
		"TERM=xterm",
		"HOME=" + temp,
	}
	output, runErr := command.CombinedOutput()
	capture, readErr := os.ReadFile(capturePath)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	return string(capture), string(output), runErr
}

func writeStub(t *testing.T, dir, name, contents string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o755); err != nil {
		t.Fatal(err)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

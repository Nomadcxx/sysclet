# sysclet installer design

## Goal

Provide a short, cross-distro path to install or update sysclet for the current user.

## Design

The POSIX `install.sh` checks for Go and an interactive terminal, then starts the Bubble Tea installer with `go run github.com/Nomadcxx/sysclet/cmd/installer@latest`. The TUI runs `go install github.com/Nomadcxx/sysclet@latest` with `GOBIN` set to `~/.local/bin`. The installer reuses sysc-greet's SYSC ASCII header and reports success or the captured Go error.

The installer rejects root execution. It writes only to the current user's home directory and does not call a package manager or request elevated privileges. Go 1.20 or newer and network access are required. This works across Linux distributions that support Go and POSIX `sh`; it does not depend on release assets, which sysclet does not publish yet.

## Verification

Build the repository, run the installer package's focused checks, validate `install.sh` with `sh -n`, and visually launch the TUI from a terminal. Do not run the install action during automated verification because it downloads and installs the current remote module.

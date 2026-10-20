# sysclet Installer Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a small cross-distro TUI installer that installs sysclet for the current user.

**Architecture:** A POSIX bootstrap script launches `cmd/installer` with `go run`. The Bubble Tea TUI runs `go install` with `GOBIN` set to `~/.local/bin`. It uses no package manager, root privileges, or release artifacts.

**Tech Stack:** Go 1.20+, Bubble Tea v1, Bubbles, Lip Gloss, POSIX `sh`.

---

### Task 1: Add the installer TUI

**Files:**
- Create: `cmd/installer/main.go`
- Create: `cmd/installer/main_test.go`

1. Add the user install path helper and environment helper.
2. Add a one-screen TUI with the SYSC header, install/update action, spinner, and success/error result.
3. Reject root execution and explain that the installer must run as the target user.
4. Run `go test -count=1 -p 2 ./cmd/installer` and build with `-p 2`.

### Task 2: Add the bootstrap script

**Files:**
- Create: `install.sh`

1. Check for Go and an interactive terminal.
2. Reject root execution.
3. Run `go run github.com/Nomadcxx/sysclet/cmd/installer@latest` with input from `/dev/tty`.
4. Run `sh -n install.sh`.

### Task 3: Document installation

**Files:**
- Modify: `README.md`

1. Add the `curl | sh` command and direct Go install fallback.
2. State the Go 1.20+ and network requirements and the `~/.local/bin` destination.
3. Run `git diff --check`.

### Task 4: Verify and prepare visual review

1. Run `go test -count=1 -p 2 ./cmd/installer` and `go build -p 2 ./...`.
2. Launch `go run ./cmd/installer`, confirm the TUI draws, then quit without starting installation.
3. Confirm installation only starts after Enter.
4. Review the diff and commit the feature without pushing it.

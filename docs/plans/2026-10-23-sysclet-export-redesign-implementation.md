# Sysclet Export Redesign Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make export format switching obvious, preserve ANSI output when requested, select the active sysc-greet profile automatically, and append new profile slots safely.

**Architecture:** Keep interaction and compositor detection in the existing Bubble Tea model, format generation in `export.go`, and profile parsing in `config.go`. Reuse the current file picker and backup-safe update path; add only standard-library profile detection and slot selection.

**Tech Stack:** Go, Bubble Tea, Bubbles, Lip Gloss, standard library.

---

### Task 1: Append profile slots by default

**Files:**
- Modify: `config.go`
- Test: `config_test.go`
- Modify: `tui.go`
- Test: `tui_test.go`

1. Add a failing `config_test.go` case for a profile with `ascii_1` and `ascii_4`; expect `nextProfileSlot` to return `ascii_5`. Also cover a missing profile (`ascii_1`) and malformed profile input.
2. Run `GOMAXPROCS=4 go test -count=1 -p 2 ./...`; confirm the new helper is missing or the assertion fails.
3. Implement `nextProfileSlot` by scanning profile assignments with the existing profile-entry parser. Return the first slot for a missing file and an error for malformed entries.
4. Add a failing TUI test showing that Ctrl+S appends to the next slot and repeated saves keep updating that same slot. Cover manual slot editing as explicit replacement.
5. Run the focused tests, then `GOMAXPROCS=4 go test -count=1 -p 2 ./...`.

### Task 2: Detect and select sysc-greet compositor profiles

**Files:**
- Modify: `tui.go`
- Test: `tui_test.go`

1. Add table tests for `SYSC_COMPOSITOR`, `NIRI_SOCKET`, `SWAYSOCK`, `HYPRLAND_INSTANCE_SIGNATURE`, and desktop aliases (`GNOME`, `Plasma`, `i3`, `bspwm`). Use temporary profile directories.
2. Run the focused compositor tests and confirm the detection helper is missing.
3. Implement name normalization and choose a matching `.conf` from the existing sysc-greet profile directories. Keep the path editable and let Ctrl+O open the existing `.conf` picker.
4. Add tests that opening export defaults to an available detected profile and preserves a user-entered path.
5. Run the focused tests, then `GOMAXPROCS=4 go test -count=1 -p 2 ./...`.

### Task 3: Add format tabs and ANSI display guidance

**Files:**
- Modify: `export.go`
- Create: `export_test.go`
- Modify: `tui.go`
- Test: `tui_test.go`
- Modify: `README.md`

1. Add a failing export test requiring an ANSI format to preserve SGR bytes and use the `.ans` extension. Add table coverage for the run/display hint for each output format.
2. Run `GOMAXPROCS=4 go test -count=1 -p 2 ./...`; confirm the ANSI format and hint are missing.
3. Implement raw ANSI generation and concise format-specific display commands. Keep TXT output plain and label its ANSI-stripping behavior.
4. Add failing key tests for Tab/Shift+Tab through individual format tabs, with the next Tab moving to the output destination and, for sysc-greet, the slot.
5. Implement explicit selected/focused format tabs, profile/path focus, compact output guidance, and previews that fit both full-screen and compact terminals.
6. Update the README keyboard map and format guidance. Run the focused tests and `GOMAXPROCS=4 go test -count=1 -p 2 ./...`.

### Task 4: Add the repo header source and transparent beam wordmark

**Files:**
- Create: `assets/sysclet-header.conf`
- Replace: `assets/sysclet-wordmark.webp`
- Modify: `README.md`
- Modify: `docs/design.md`
- Test: `export_test.go`

1. Add an asset test for the profile structure, literal `\\033` escapes, absence of raw escape bytes, and a fixed SHA-256 value for the complete supplied header.
2. Run the focused test and confirm the asset is missing.
3. Add the exact supplied profile block. Render the wordmark from that source with a transparent canvas and a restrained moving beam, retaining the terminal's explicit background-colored art cells.
4. Update the README alt text and document that sysc-greet's Beams display effect is selected independently from the profile file.
5. Verify alpha and animation with `file`/`identify`, inspect a rendered frame, and run `GOMAXPROCS=4 go test -count=1 -p 2 ./...`.

### Task 5: Review, merge, and publish

**Files:**
- Review the complete branch diff and resulting PR.

1. Run `gofmt -w` on changed Go files and `GOMAXPROCS=4 go test -count=1 -p 2 ./...`.
2. Review the diff locally for slot overwrite risk, focus order, narrow-terminal clipping, and exact asset bytes.
3. Push the branch, open a PR, review the PR diff manually, and merge when LGTM.
4. Bump the project version, tag the release, update `sysclet-git` on AUR, and verify the published package points at the release commit.

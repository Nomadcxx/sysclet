# sysclet TDF Header Generator Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a standalone Go CLI that renders TheDraw TDF fonts as ANSI-colored text and exports the art to sysc-greet profile slots.

**Architecture:** Keep the program in a small root `main` package. Parse TDF records into glyph rows of `(rune, color attribute)` cells, render those cells using the TDF color mapping, and reuse the rendered result for terminal output and sysc-greet config export. Config edits must save a timestamped backup and atomically replace only the selected slot.

**Tech Stack:** Go standard library only.

---

### Task 1: Add TDF parsing and ANSI rendering

**Files:**
- Create: `tdf.go`
- Test: `tdf_test.go`

**Steps:**
1. Build a tiny valid TDF byte fixture in a test with one colored glyph and known foreground/background attributes.
2. Run `go test ./...` and confirm the new test fails because the parser does not exist.
3. Implement header/record validation, printable glyph lookup, CP437 conversion, TDF spacing, per-cell SGR, line resets, and useful malformed-input errors.
4. Run `go test ./...`; verify expected glyph text and exact ANSI color sequences, then add a malformed offset assertion.

### Task 2: Add paste-ready export and safe profile updates

**Files:**
- Create: `config.go`
- Test: `config_test.go`

**Steps:**
1. Add tests for the generated `ansi_style=true` and `ascii_N` block, replacement of one existing slot, preservation of other content, backup creation, and invalid slot rejection.
2. Run `go test ./...` and confirm the new tests fail before implementation.
3. Implement snippet generation and profile replacement. Back up the exact original bytes to a unique timestamped sibling file before writing the replacement through a same-directory temporary file and rename.
4. Run `go test ./...`; verify the selected slot changes while other variants remain byte-for-byte intact.

### Task 3: Add the CLI and README

**Files:**
- Create: `main.go`
- Create: `go.mod`
- Create: `README.md`

**Steps:**
1. Implement `render` and `export` subcommands with `--font`, optional `--font-name`, `--slot`, and optional `--config` flags. Treat all remaining arguments as the text.
2. Run `go build ./...` and confirm the command compiles.
3. Document install, obtaining a TDF font, rendering, exporting, and updating a profile with its backup behavior.
4. Run `gofmt`, `go vet ./...`, `go test ./...`, and build an executable.
5. Render `SYSC` using the cached Bleach TDF file and inspect that output contains ANSI color transitions and the expected glyph rows. Confirm export contains readable `\\033` config escapes.
6. Review the diff, commit the implementation, and push the `main` branch to the authorized sysclet repository.

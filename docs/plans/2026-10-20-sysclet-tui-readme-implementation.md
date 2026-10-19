# sysclet TUI and README Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add an interactive TDF header editor with sysc-greet export and present it through a BIT-inspired README and showcase.

**Architecture:** Keep the app in the existing root `main` package. Reuse `Font.Render`, `ProfileSnippet`, and `UpdateProfile`; use Bubble Tea for terminal ownership, Bubbles for text/file inputs, and Lip Gloss for layout. Keep TDF SGR output unchanged.

**Tech Stack:** Go 1.20, Bubble Tea v1, Bubbles, Lip Gloss, existing TDF parser and config writer.

---

### Task 1: CLI entry and font discovery

**Files:**
- Modify: `main.go`
- Create: `tui.go`
- Test: `main_test.go`, `tui_test.go`
- Modify: `go.mod`, `go.sum`

**Steps:**
1. Add tests that `tui` is accepted and that font discovery honors an absolute `XDG_DATA_HOME`, falling back to `~/.local/share/sysclet/fonts`.
2. Run the focused tests and confirm they fail because dispatch and discovery are missing.
3. Add Bubble Tea, Bubbles, and Lip Gloss at the versions used by BIT; make no arguments and `tui` launch the app while preserving help, `render`, and `export`.
4. Add the minimal data-directory helper and font loader that returns all named fonts from one TDF file.
5. Run focused tests and `go test ./...`.

### Task 2: Live preview and keyboard navigation

**Files:**
- Create: `tui.go`
- Create: `tui_test.go`

**Steps:**
1. Add a test that changing text updates the ANSI preview immediately and retains TDF SGR codes.
2. Run the test and confirm it fails because the TUI model is missing.
3. Implement the Text, TDF Font, and Export control strip, BIT-like preview frame, and keyboard guide. Keep the TDF renderer's colors and spacing authoritative.
4. Run the focused test and `go test ./...`.

### Task 3: TDF file and named-font selection

**Files:**
- Modify: `tui.go`
- Modify: `tui_test.go`

**Steps:**
1. Add coverage for selecting a named record from a multi-font TDF and rejecting a selected non-TDF file.
2. Run the focused tests and confirm they fail.
3. Use the Bubbles file picker to browse local `.tdf` files and offer record names when a file contains multiple fonts. Explain how to load a font when none is selected.
4. Run the focused tests and `go test ./...`.

### Task 4: sysc-greet profile export

**Files:**
- Modify: `tui.go`
- Modify: `tui_test.go`

**Steps:**
1. Add coverage that export renders a snippet for the selected slot and calls the existing backup-safe profile update path only after explicit confirmation.
2. Run the focused test and confirm it fails.
3. Add editable profile path and `ascii_N` controls, show the snippet preview, and reuse `ProfileSnippet` or `UpdateProfile` for output.
4. Run the focused test and `go test ./...`.

### Task 5: README and TUI showcase

**Files:**
- Modify: `README.md`
- Replace: `assets/sysclet-showcase.webp`

**Steps:**
1. Record the TUI at testbench resolution while generating `SYSC`, `SEE`, `YOU`, `IN`, `SPACE`, and `COWBOY`, then export one result to a sysc-greet profile.
2. Convert the capture to a compact animated WebP while preserving the readable terminal text and color.
3. Rework README hierarchy to match BIT's wordmark, badges, anchor links, full-width showcase, and concise install/feature/usage sections. Keep sysclet's smaller feature set and TDF license details accurate.
4. Apply the stop-slop review to the README and PR text.

### Task 6: Package and release checks

**Files:**
- Review: `packaging/aur/sysclet-git/PKGBUILD`

**Steps:**
1. Verify AUR metadata still builds the TUI with module dependencies and adds no runtime dependency.
2. Run `gofmt`, `go test ./...`, `go vet ./...`, `go build`, and the AUR package build.
3. Launch the binary in a full-screen terminal and verify edit, font selection, and export flows.
4. Commit the feature, push the branch, and open a pull request for review.

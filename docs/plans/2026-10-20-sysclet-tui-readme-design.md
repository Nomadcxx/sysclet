# sysclet TUI and README design

## Goal

Add a full-screen TUI that follows bit's ANSI-art editor layout and lets users generate TheDraw headers, then export them to sysc-greet. Refresh sysclet's README to use bit's visual hierarchy while keeping the content and showcase specific to sysclet.

## TUI design

Running `sysclet` with no arguments opens the TUI; `sysclet tui` is also available. Existing `render` and `export` commands keep their current behavior.

Use bit's overall screen structure: a compact title, a large framed ANSI preview, a bottom control strip, and a keyboard guide. The control strip contains only the controls sysclet owns: text, TDF font, and sysc-greet export. Left/right moves between controls; up/down changes a selection; Enter edits text or opens a selection; Escape returns from a modal; `e` opens export; `q` exits. Re-render after every text edit so the preview stays current.

The font control discovers `.tdf` files in `$XDG_DATA_HOME/sysclet/fonts`, falling back to `~/.local/share/sysclet/fonts`. It can browse to another local TDF file. When a TDF file contains multiple font records, the user can choose the font by name. If no font is loaded, the UI explains how to select one. It does not download fonts or embed third-party fonts.

The preview displays `Font.Render` output with its SGR codes and cell colors intact. The TDF remains authoritative for glyph geometry, foreground/background colors, and spacing. The UI does not apply global color, gradient, scale, shadow, or spacing effects.

The export control selects an `ascii_N` slot and profile path. It previews the generated snippet and uses the existing `ProfileSnippet` or `UpdateProfile` functions. Direct updates keep the existing backup and atomic replacement behavior. The CLI remains available for non-interactive snippet output and direct updates.

## Dependencies and structure

Use Bubble Tea v1, Bubbles, and Lip Gloss, matching the major versions used by bit and sysc-go. Bubble Tea owns terminal input and alternate-screen handling; Bubbles supplies text inputs and file selection; Lip Gloss lays out panels and the preview. Keep the app in sysclet's root `main` package and reuse its TDF and config functions. Do not import bit's font renderer or effect model.

## README and showcase design

Follow bit's README hierarchy: centered wordmark and project summary, small badges and anchor links, then a full-width animated showcase before the feature and usage sections. Use sysclet's existing generated wordmark. Keep the README shorter than bit's because sysclet has no separate font library or general-purpose rendering API.

Document the TUI, keyboard controls, local TDF font setup, `render`, and sysc-greet profile export. Describe the native TDF colors accurately and retain the font-license note. Keep installation instructions aligned with the published `sysclet-git` package and Go install path.

Replace the CLI-only showcase with a screen recording of the TUI. Show separate generations for `SYSC`, `SEE`, `YOU`, `IN`, `SPACE`, and `COWBOY`, then show export to a sysc-greet slot. Capture at the testbench's native resolution and frame rate, trim pauses, and speed up the transitions if that makes the sequence clearer. Store the optimized result as an animated WebP asset in `assets/` and embed it near the top of the README.

## Acceptance criteria

- No-argument launch opens the TUI; `render`, `export`, and help remain usable.
- Text edits update the preview immediately, and the preview retains TDF SGR foreground/background colors.
- A user can select local TDF files, including named fonts in a multi-font TDF.
- TUI export can update a selected profile slot through the existing backup-safe path.
- The README presents sysclet's TUI, CLI, font setup, and export workflow in the approved bit-inspired hierarchy.
- The README showcase shows all six requested words and a sysc-greet export at high resolution.
- The AUR package builds the binary with its Go module dependencies and adds no runtime package dependency.

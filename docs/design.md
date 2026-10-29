# Architecture

sysclet generates ANSI headers from TheDraw TDF fonts. The root Go package owns the CLI, editor, font parser, bundled catalog, and export logic. `cmd/installer` is a separate user-local installer, launched by `install.sh`.

## Runtime paths

| File | Responsibility |
| --- | --- |
| `main.go` | Dispatch `tui`, `render`, `export`, `fonts`, and `version`; load bundled or local fonts. |
| `tdf.go` | Validate TDF records, decode CP437 glyph cells and colors, and render ANSI lines. |
| `tdf_catalog.go` | Read the embedded catalog and decompress only the selected font pack. |
| `tui.go` | Own editor state, input modes, live preview, font selection, and export interaction. |
| `export.go` | Generate ANSI and plain text, code samples, and output files for the selected export format; place sysc-walls and sysc-terminal art and update the sysc-walls config. |
| `config.go` | Format sysc-greet snippets and update profile slots. |
| `cmd/installer` | Show install progress and run `go install` into the current user's `~/.local/bin`. |

The rendering path is `TDF record → Font glyph rows → ANSI output`. Each record has slots for 94 printable ASCII characters, from `!` through `~`; some slots may be empty. A cell keeps its CP437 character and, for color fonts, its VGA attribute: the low four bits select the foreground and the next three select the background. The renderer applies the font's letter spacing, reserves four columns for a word space, maps VGA colors to ANSI, and resets SGR at the end of each line. TDF colors remain the default; the editor can override foreground and background colors and align the rendered block. Colors come from TDF attributes, not from visual glyphs such as `░`, `▒`, or `█`.

The bundled catalog maps font names to records in compressed packs. Local `.tdf` files use the same parser and can contain multiple named records. The parser bounds input size, record count, and decoded glyph data before returning fonts.

The `render` command writes terminal-ready ANSI output. The `export` command prints a sysc-greet profile snippet or updates a profile by default; `--format` selects ANSI, plain text, Go, JavaScript, Python, Rust, shell, sysc-walls, or sysc-terminal, the same formats the editor offers. sysc-walls and sysc-terminal draw each rune of their artwork as a cell in their own theme, so those targets write ANSI-stripped text under `~/.local/share` or `~/.config`, the only roots the apps read. The `fonts` command lists catalog names or the records in a local file. The export view puts keyboard-focused format tabs before the destination and optional sysc-greet slot. It detects a matching profile from compositor and desktop-session environment variables, then chooses the next available `ascii_N` slot by default. Run `sysclet --help` for the CLI syntax; the [README](../README.md) describes the interactive controls and file formats.

## File updates

Profile and generated-file updates inspect the final path component with `Lstat` and reject symlinks and non-regular files. When replacing an existing file, sysclet creates an exclusive backup, writes and syncs a temporary file in the same directory, compares the current contents with the bytes it read earlier, and renames the temporary file into place. The comparison can detect an earlier edit but does not prevent a concurrent change after the check. Updates preserve the existing permission bits. New profile and export files are owned by the user running sysclet and request mode `0644`, subject to the process umask.

The sysc-walls target also rewrites the `effect`, `theme`, `file`, and `cycle` lines of the `[animation]` section in `~/.config/sysc-walls/daemon.conf`, keeping every other line, with the same backup and compare-then-rename steps. Unlike profile updates it follows a symlinked config to its target, so a config managed from a dotfiles repository stays a symlink.

Updating a sysc-greet profile sets `ansi_style=true` and replaces the selected `ascii_N` block. Existing profile paths must be writable by the current user; system-owned profiles require running sysclet with elevated privileges.

## Installer and assets

`install.sh` requires Go 1.21 or newer (Go fetches the 1.26.10 toolchain `go.mod` declares), network access, and an interactive terminal. It refuses root execution; the installer writes to the invoking user's `~/.local/bin` and does not call a package manager. The catalog's Phosphor and sysc-Go BIT packs are rebuilt by `scripts/import_fonts.py`. Tagged releases are built by `.github/workflows/release.yml`; `packaging/aur` holds the `sysclet` and `sysclet-git` PKGBUILDs. The bundled TDF files are third-party assets with terms separate from sysclet's GPL-3.0-only code; see [THIRD-PARTY-NOTICES.md](../THIRD-PARTY-NOTICES.md).

`assets/sysclet-header.conf` holds the repository's ANSI header profile. The README wordmark uses that art on a transparent canvas with a moving beam. The profile stores art and colors. Select the **Beams** display effect in sysc-greet to animate it there.

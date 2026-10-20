# sysclet design

sysclet is a Go TUI and CLI for generating ANSI headers from TheDraw TDF fonts. It reads the glyph and color attributes stored in each font cell, then emits ANSI SGR output for terminals or a paste-ready sysc-greet profile entry.

## Scope

- Embed 3,481 named records from 36 TheDraw TDF packs and accept additional local TDF files.
- Search font names in the TUI; allow a name selection when a local file contains multiple records.
- Edit text in the TUI and render each keystroke immediately.
- Render to stdout with the TDF foreground and background colors preserved per cell.
- Allow TUI alignment and explicit foreground/background overrides; keep TDF colors as the default.
- Export an `ascii_N` entry with `ansi_style=true` and escaped `\\033` sequences for sysc-greet.
- By default, print the entry for copying. If the user supplies a profile path, update only the selected slot, save a backup, and replace the profile safely.
- Keep the TUI in the root `main` package and use Bubble Tea, Bubbles, and Lip Gloss for terminal input, local file selection, and layout.
- Do not infer colors from glyph characters.

sysclet does not implement background effects. AK-47 and other effects belong to sysc-greet.

The bundled font files are third-party assets with terms separate from the sysclet code; see `THIRD-PARTY-NOTICES.md`.

## TDF rendering

TheDraw font records hold up to 94 printable ASCII glyphs. A cell stores a CP437 byte and, for colored fonts, a color attribute. The low four attribute bits select the foreground; the next three select the background. Output maps those VGA color indexes to ANSI's standard and bright foreground codes and standard background codes. Fonts without per-cell color attributes use their stored default attribute.

Render each input character as rows of cells, use the font's spacing value between glyphs, reserve four columns for a word space, and reset SGR at each line end. Preserve the glyph's cell colors by default; only apply explicit TUI foreground or background overrides. Never assign colors based on characters such as `░`, `▒`, or `█`.

## CLI shape

```text
sysclet render --font FONT.tdf [--font-name NAME] TEXT
sysclet export --font FONT.tdf [--font-name NAME] --slot ascii_N [--config PROFILE] TEXT
```

`render` writes actual ESC bytes. `export` writes config text using readable `\\033` escapes. A profile update backs up the original before atomically replacing the selected slot and enabling ANSI style.

Running `sysclet` or `sysclet tui` opens the interactive editor. `sysclet render` and `sysclet export` remain available for scripts.

# sysclet design

sysclet is a small Go CLI for making ANSI colored text art from TheDraw TDF fonts. It reads the glyph and color attributes stored in each font cell, then emits ANSI SGR output for terminals or a paste ready sysc-greet profile entry.

## Scope

- Accept a TDF file and text, with an optional font name when the file contains multiple fonts.
- Render to stdout with the TDF foreground and background colors preserved per cell.
- Export an `ascii_N` entry with `ansi_style=true` and escaped `\\033` sequences for sysc-greet.
- By default, print the entry for copying. If the user supplies a profile path, update only the selected slot, save a backup, and replace the profile safely.
- Use Go's standard library only. Do not infer colors from glyph characters.

sysclet has no TUI or background effects. AK-47 and other effects belong to sysc-greet.

## TDF rendering

TheDraw font records hold up to 94 printable ASCII glyphs. A cell stores a CP437 byte and, for colored fonts, a color attribute. The low four attribute bits select the foreground; the next three select the background. Output maps those VGA color indexes to ANSI's standard and bright foreground codes and standard background codes. Fonts without per-cell color attributes use their stored default attribute.

Render each input character as rows of cells, use the font's spacing value between glyphs, reserve four columns for a word space, and reset SGR at each line end. Preserve the glyph's cell colors; never assign colors based on characters such as `░`, `▒`, or `█`.

## CLI shape

```text
sysclet render --font FONT.tdf [--font-name NAME] TEXT
sysclet export --font FONT.tdf [--font-name NAME] --slot ascii_N [--config PROFILE] TEXT
```

`render` writes actual ESC bytes. `export` writes config text using readable `\\033` escapes. A profile update backs up the original before atomically replacing the selected slot and enabling ANSI style.

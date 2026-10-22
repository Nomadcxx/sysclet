# Sysclet Export Redesign

## Audit

The export view currently opens with the path field focused, then requires two or three Tab presses before the format field. Formats appear as unboxed text and change only with arrow keys. The TXT exporter strips ANSI sequences without saying so, while the code formats preview generated source without showing how to run it. The sysc-greet path picker already exists, but the screen does not detect the active compositor. The default slot is `ascii_1`, so saving to a selected profile can replace its first variant.

Hallmark found no named visual anti-patterns that apply to this terminal UI: 0 critical, 0 major, 0 minor. The changes below address keyboard navigation, output guidance, target selection, and overwrite safety.

## Design

Export opens on a visible format tab. Tab and Shift+Tab move through each format tab, then the destination and the sysc-greet slot when applicable. Left and right also change formats while a tab has focus. The current selection uses a clear inverse/high-contrast treatment that preserves the existing cyan accent. The preview gets a height budget so it cannot push the save hint off a short terminal.

Add a raw ANSI format that writes the rendered escape sequences to `.ans`. Keep TXT plain and label that ANSI colors are removed. For executable formats, show the terminal command for running the generated file. For ANSI output, show `cat file.ans` and `less -R file.ans` as display options.

When sysc-greet export opens, resolve a profile from `SYSC_COMPOSITOR`, compositor-specific session variables, then desktop/session names. Map common aliases such as GNOME, Plasma, i3, and bspwm to sysc-greet's profile filenames. Keep the profile path editable and retain the `.conf` file picker for explicit selection.

For a selected profile, choose the next slot after the highest existing `ascii_N` by default. Keep that choice stable after save so another save updates the same slot; changing the profile chooses a new next slot. Editing the slot field opts into replacing that specific slot. Existing backup and atomic-write behavior remains in force.

Store the supplied `ansi_style=true` and `ascii_1` block verbatim in `assets/sysclet-header.conf`. Use it as the source for the repo wordmark, with a transparent canvas and a restrained sweeping beam. The ASCII profile format does not store sysc-greet's Beams display effect, so the profile remains portable and the README will explain where to select Beams in sysc-greet.

## Validation

Test format navigation, raw ANSI preservation, output hints, compositor mapping, automatic slot selection, explicit slot replacement, and preview height at a short terminal size. Compare the checked-in profile text byte-for-byte against its expected content. Verify that the wordmark is still an animated WebP and retains alpha.

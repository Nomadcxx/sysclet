# sysclet

<p align="center">
  <img src="assets/sysclet-wordmark.webp" alt="sysclet rendered with its own ANSI header engine" width="960">
</p>

<p align="center">Generate ANSI ASCII headers from TheDraw fonts.</p>

<p align="center">
  <img src="assets/sysclet-showcase.webp" alt="sysclet separately renders SYSC, SEE, YOU, IN, SPACE, and COWBOY, then exports COWBOY to sysc-greet">
</p>

sysclet reads each glyph's stored colors from a TheDraw `.tdf` font. It prints ANSI art or exports a colorized `ascii_N` profile slot for sysc-greet.

## Install

Arch Linux users can install the AUR package:

```sh
yay -S sysclet-git
```

With Go installed, build from the latest source:

```sh
go install github.com/Nomadcxx/sysclet@latest
```

sysclet has no runtime dependencies beyond the Go standard library.

## Get a TDF font

Download a font from [TAAG's TheDraw font sets](https://patorjk.com/software/taag/tdf-font-sets/). This example saves Bleach under your user data directory:

```sh
mkdir -p ~/.local/share/sysclet/fonts
curl -fL https://patorjk.com/software/taag/tdf-font-sets/tdf-bleach.tdf \
  -o ~/.local/share/sysclet/fonts/tdf-bleach.tdf
```

Each font has its own glyph coverage and license.

## Render text

```sh
sysclet render --font ~/.local/share/sysclet/fonts/tdf-bleach.tdf "SEE YOU IN SPACE COWBOY"
```

sysclet emits SGR color sequences to the terminal. It keeps per-cell foreground and background attributes when a font stores them, and uses the font's default attribute otherwise.

## Export to sysc-greet

Print a profile snippet for review and copying:

```sh
sysclet export --font ~/.local/share/sysclet/fonts/tdf-bleach.tdf --slot ascii_1 COWBOY
```

Update a profile directly:

```sh
sudo "$(command -v sysclet)" export --font ~/.local/share/sysclet/fonts/tdf-bleach.tdf \
  --slot ascii_1 --config /usr/share/sysc-greet/ascii_configs/sysc-compositor.conf COWBOY
```

The update enables `ansi_style=true`, replaces the selected slot, and saves the original beside the profile as `.sysclet-<timestamp>.bak`. sysc-greet maps the generated ANSI palette through the selected theme.

## Input rules

- Put options before the text. Quote phrases that contain spaces.
- Use `--` before text that begins with a dash.
- Use printable ASCII characters supported by the selected font. Some TDF fonts omit characters.
- Use `--font-name NAME` when a TDF file contains multiple fonts. Without it, sysclet selects the first font.

Run `sysclet --help`, `sysclet render --help`, or `sysclet export --help` for command options.

## License

sysclet is licensed under GPL-3.0-only. See [LICENSE](LICENSE). TDF fonts may use separate terms.

# sysclet

sysclet renders text with TheDraw `.tdf` fonts. It keeps the foreground and background colors stored in each glyph cell, writes ANSI art to a terminal, and exports `ascii_N` profile entries for sysc-greet.

## Install

```sh
go install github.com/Nomadcxx/sysclet@latest
```

Download a TDF font from [TAAG](https://patorjk.com/software/taag/). For example:

```sh
mkdir -p ~/.local/share/sysclet/fonts
curl -fL https://patorjk.com/software/taag/tdf-font-sets/tdf-bleach.tdf \
  -o ~/.local/share/sysclet/fonts/tdf-bleach.tdf
```

## Use

Render ANSI art to the terminal:

```sh
sysclet render --font ~/.local/share/sysclet/fonts/tdf-bleach.tdf SYSC
```

Print a sysc-greet profile snippet for copying:

```sh
sysclet export --font ~/.local/share/sysclet/fonts/tdf-bleach.tdf --slot ascii_1 SYSC
```

Update a profile in place:

```sh
sudo sysclet export --font ~/.local/share/sysclet/fonts/tdf-bleach.tdf --slot ascii_1 \
  --config /usr/share/sysc-greet/ascii_configs/sysc-compositor.conf SYSC
```

With `--config`, sysclet enables ANSI style, replaces the selected slot, and saves the previous file beside it with a `.sysclet-<timestamp>.bak` suffix. Review the generated art before updating a profile.

Use `--font-name NAME` when a TDF file contains more than one font. Without it, sysclet uses the first font. Pass printable ASCII text and spaces; quote text with spaces, for example `"HELLO WORLD"`. Put all flags before the text.

sysc-greet maps the generated standard ANSI colors to the active theme when `ansi_style=true`. sysclet handles header art; sysc-greet handles backgrounds and effects.

Run `sysclet --help`, `sysclet render --help`, or `sysclet export --help` for command options.

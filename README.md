<div align="center">

<img src="assets/sysclet-wordmark.webp" alt="SYSCLET rendered by sysclet with the bundled Bleach TDF font" width="72%" />

### sysclet · ANSI Header Studio and TheDraw Font Catalog

[![License](https://img.shields.io/badge/license-GPL--3.0--only-3434ff.svg)](LICENSE)
![Interface](https://img.shields.io/badge/interface-TUI%20%2B%20CLI-3434ff.svg)
[![Go](https://img.shields.io/badge/Go-1.20+-00c9e8.svg)](https://go.dev/)

[Features](#features) · [Quick Start](#quick-start) · [TUI](#tui) · [CLI](#cli) · [Font Catalog](#font-catalog) · [sysc-greet Export](#sysc-greet-export) · [License](#license)

<img src="assets/sysclet-showcase.webp" alt="sysclet generates SYSC, SEE, YOU, IN, SPACE, and COWBOY with TheDraw fonts, then exports a header to sysc-greet" width="100%" />

</div>

---

## Features

| Feature | What it does |
| --- | --- |
| **ANSI header editor** | Edit text in a full-screen terminal UI and see the TheDraw output update as you type. |
| **3,481 bundled font records** | Search and preview the bundled catalog without a network connection. Load other `.tdf` files from disk. |
| **Authored color and alignment** | Keep each TDF cell's foreground and background by default. Choose left, center, or right alignment, or override either ANSI color. |
| **sysc-greet export** | Preview a profile slot, then update it with a backup of the original file. |
| **Command line** | Render a header or print an escaped sysc-greet profile snippet without opening the TUI. |

The program embeds its font data and has no runtime package dependencies.

## Quick Start

Install from the AUR:

```sh
yay -S sysclet-git
```

Or install with Go:

```sh
go install github.com/Nomadcxx/sysclet@latest
```

Run `sysclet` to open the editor.

## TUI

The screen has a large ANSI preview, a control strip, and keyboard hints. The initial font is **Bleach**; the font control opens the bundled catalog.

| Key | Action |
| --- | --- |
| `←` / `→` | Move between text, font, alignment, colors, and export controls |
| `↑` / `↓` | Change the selected setting |
| `Enter` | Edit text or open the selected control |
| `Ctrl+O` in the font list | Browse for a local `.tdf` file |
| `e` | Open sysc-greet export |
| `Ctrl+O` in export | Browse for a sysc-greet profile |
| `Ctrl+S` in export | Update the selected profile slot and save a backup |
| `Esc` | Close the active editor or picker |
| `q` | Quit |

The `FG` and `BG` controls cycle through the 16-color ANSI palette and **TDF**, which restores the font's stored color for each cell. Alignment changes only the position of the rendered header.

To load local fonts, place them in `~/.local/share/sysclet/fonts` (or `$XDG_DATA_HOME/sysclet/fonts`) and choose **TDF FONT → Ctrl+O**. A file with multiple records opens a searchable font-name list.

## CLI

Render with a bundled font:

```sh
sysclet render --font-name Bleach SYSC
```

Render from a local file, including a named record in a multi-font file:

```sh
sysclet render --font ./fonts/my-fonts.tdf --font-name "Font Name" "SEE YOU"
```

Print a sysc-greet snippet for review:

```sh
sysclet export --font-name Bleach --slot ascii_1 COWBOY
```

Run `sysclet --help`, `sysclet render --help`, or `sysclet export --help` for options.

## sysc-greet Export

The TUI previews the selected `ascii_N` slot with its ANSI colors and writes escaped codes to the profile. Set a profile path and slot, then press `Ctrl+S` to update the profile. sysclet saves a timestamped backup before replacing the slot and enabling ANSI rendering.

The CLI can update a profile directly:

```sh
sudo "$(command -v sysclet)" export --font-name Bleach --slot ascii_1 \
  --config /usr/share/sysc-greet/ascii_configs/sysc-compositor.conf COWBOY
```

sysc-greet maps the ANSI colors through the selected theme.

## Font Catalog

sysclet bundles 3,481 named TDF records from 36 font packs distributed through TAAG's [TheDraw font sets](https://patorjk.com/software/taag/tdf-font-sets/). The TUI and CLI work offline. Individual records can have different authors and terms; see [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md) for source and license details. The GPL license for sysclet's code does not apply to those font files.

## License

sysclet's code is licensed under [GPL-3.0-only](LICENSE). Bundled font files are third-party assets with separate terms, documented in [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md).

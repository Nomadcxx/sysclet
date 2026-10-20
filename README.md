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
| **sysc-greet export** | Preview a profile slot, create a new profile, or update an existing one with a backup. |
| **Command line** | Render a header or print an escaped sysc-greet profile snippet without opening the TUI. |

The program embeds its font data. Clipboard copying uses the system clipboard utility when available; other features work without one.

## Quick Start

Install for your user on Linux distributions with Go and POSIX `sh`:

```sh
curl -fsSL https://raw.githubusercontent.com/Nomadcxx/sysclet/main/install.sh | sh
```

The installer shows the binary destination and streams Go's build output. It installs sysclet to `~/.local/bin/sysclet`; the TDF fonts are bundled in the binary. It needs Go 1.20+ and network access, and does not use `sudo` or a distro package manager. If `~/.local/bin` is not on your `PATH`, run `~/.local/bin/sysclet`.

You can install directly with Go instead:

```sh
mkdir -p "$HOME/.local/bin"
GOBIN="$HOME/.local/bin" go install github.com/Nomadcxx/sysclet@latest
```

Or install from the AUR:

```sh
yay -S sysclet-git
```

Run `sysclet` to open the editor.

## TUI

The screen has a large ANSI preview, a control strip, and keyboard hints. The active control is named above the strip and highlighted. The initial font is **Bleach**. The full font name appears above the controls; select **TDF FONT** and use `↑` / `↓` to cycle fonts. Press `Enter` on **TDF FONT** to search the bundled catalog or browse a local `.tdf` file.

| Key | Action |
| --- | --- |
| `Tab` / `Shift+Tab` | Move forward/backward through the controls |
| `←` / `→` | Move between text, font, alignment, colors, and export controls |
| `↑` / `↓` | Change the selected setting; cycle fonts when **TDF FONT** is selected |
| `Enter` | Edit text or open the selected control |
| `Ctrl+O` in the font list | Browse for a local `.tdf` file |
| `e` | Open export options, starting with sysc-greet |
| `Ctrl+O` / `Ctrl+B` in export | Browse for a file matching the selected export format |
| `Ctrl+S` in export | Save the selected format; existing files get a backup |
| `Esc` | Close the active editor or picker |
| `q` | Quit |

The `FG` and `BG` controls cycle through the 16-color ANSI palette and **TDF**, which restores the font's stored color for each cell. Alignment changes only the position of the rendered header.

In sysc-greet export, `F5` copies the plain profile snippet without the surrounding TUI. On Linux, install `wl-clipboard` for Wayland or `xclip` or `xsel` for X11. These are optional dependencies for `sysclet-git`.

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

The TUI starts with the sysc-greet format. Its snippet preview shows the selected `ascii_N` slot with ANSI colors. `Ctrl+S` updates an existing profile and saves a timestamped backup, or creates a new profile containing the ANSI setting and selected slot. `Ctrl+O` and `Ctrl+B` open the file picker from any export field; they browse existing files, while new paths can be typed directly.

The other export formats match BIT: **TXT**, **GO**, **JS**, **PY**, **RS**, and **SH**. Select a format in the export view, enter an output path, and press `Ctrl+S`. sysclet adds the format extension when needed and backs up an existing file before replacing it.

sysclet does not request elevated privileges. User-owned profile paths can be saved normally; root-owned system paths require running sysclet with `sudo`. Existing file modes are preserved when updated, and a new profile is created for the invoking user with mode `0644` limited by the system umask. Run the TUI as root only when editing a system-owned profile:

```sh
sudo "$(command -v sysclet)"
```

Run this command to update a system profile as root:

```sh
sudo "$(command -v sysclet)" export --font-name Bleach --slot ascii_1 \
  --config /usr/share/sysc-greet/ascii_configs/sysc-compositor.conf COWBOY
```

sysc-greet maps the ANSI colors through the selected theme.

## Font Catalog

sysclet bundles 3,481 named TDF records from 36 font packs distributed through TAAG's [TheDraw font sets](https://patorjk.com/software/taag/tdf-font-sets/). The TUI and CLI work offline. Individual records can have different authors and terms; see [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md) for source and license details. The GPL license for sysclet's code does not apply to those font files.

## License

sysclet's code is licensed under [GPL-3.0-only](LICENSE). Bundled font files are third-party assets with separate terms, documented in [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md).

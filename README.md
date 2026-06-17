# Futon

A fast, minimalist **TUI manga reader** written in Go.

Futon renders manga pages directly in your terminal using the Kitty Graphics Protocol — no external viewer needed. Search multiple manga sources, browse chapters, and read with smooth image rendering and Vim-like navigation.

## Features

- **Inline image rendering** via Kitty Graphics Protocol (Sixel fallback)
- **Multi-source support** — MangaDex, OTruyen (switch with `tab`)
- **Favorites & Reading history** — bookmark manga, resume where you left off
- **Offline image download** — save individual pages with `ctrl+d`
- **Chapter preloading** — seamless transition to next chapter
- **LRU image cache** (20 entries) — fast page flipping
- **Quick jump** — type a chapter number then `enter`
- **Language filter** — `/lang vi` or `/lang en` for MangaDex chapters
- **Vim-like navigation** — arrow keys, number jumps

## Prerequisites

A terminal that supports the **Kitty Graphics Protocol**:

| Terminal | Support |
|----------|---------|
| [Kitty](https://sw.kovidgoyal.net/kitty/) | Native |
| [WezTerm](https://wezfurlong.org/wezterm/) | Native |
| [Ghostty](https://ghostty.org/) | Native |
| Others | Sixel fallback |

## Installation

### From source

```bash
go install github.com/KabosuNeko/Futon@latest
```

### Pre-built binaries

Download the latest release for your platform from the [Releases](https://github.com/KabosuNeko/Futon/releases) page.

Builds available for:
- Linux (amd64, arm64)
- macOS (amd64, arm64)

## Usage

```bash
futon
```

### Keybindings

#### Search screen

| Key | Action |
|-----|--------|
| `q` / `ctrl+c` | Quit |
| `tab` | Switch manga source |
| `enter` | Search / open selected manga |
| `up` / `down` | Navigate results |
| `/fav` | Show favorites |
| `/his` | Show reading history |
| `/lang vi\|en` | Set chapter language (MangaDex) |

#### Favorites / History

| Key | Action |
|-----|--------|
| `enter` | Open manga |
| `d` | Remove entry |
| `esc` | Back to search |

#### Chapter list

| Key | Action |
|-----|--------|
| `up` / `down` | Navigate chapters |
| `ctrl+f` | Add/remove favorite |
| `enter` | Open selected chapter |
| `[number] + enter` | Jump to chapter |
| `esc` | Back to manga search |
| `q` / `ctrl+c` | Quit |

#### Reader

| Key | Action |
|-----|--------|
| `→` / `l` | Next page |
| `←` / `h` | Previous page |
| `ctrl+d` | Download current page |
| `esc` / `q` | Back to chapter list |
| `ctrl+c` | Quit |

## Data storage

| Data | Location |
|------|----------|
| Favorites | `~/.config/futon/favorites.json` |
| Reading history | `~/.config/futon/history.json` |
| Downloaded images | `~/Downloads/Futon_Downloads/` |

## Architecture

```
cmd/main.go          — entry point
internal/
  api/               — MangaProvider interface & HTTP clients
  models/            — shared domain types
  storage/           — JSON persistence (favorites, history)
  tui/               — Bubble Tea screens (search, chapters, reader)
  tui/imgrender/     — Kitty / Sixel renderer selection
```

## License

MIT

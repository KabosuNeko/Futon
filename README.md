# Futon

<p><br/></p>
<p align="center">
  <img src="https://github.com/user-attachments/assets/2b1cd5ba-eb66-4632-82d8-284f7c1e3780" alt="Futon Logo" style="width: 192px" />
</p>
<p><br/></p>

**Terminal manga reader** — written in Go, renders manga directly in your terminal via **Kitty Graphics Protocol** or **Sixel**. No image viewer needed. Search từ nhiều sources, browse chapters, đọc với Vim-style keys.

## Highlights

- **Render in terminal** — Kitty or Sixel, auto-detect
- **Multi-source** — OTruyen, MangaDex, TruyenQQ (tab to cycle, All mode searches everything)
- **Favorites & History** — bookmark truyện yêu thích, resume từ trang đã đọc
- **Save images** — `ctrl+d` to download current page
- **Preload next chapter** — seamless chapter transition, zero waiting
- **LRU image cache** — 20 rendered images cached, flip pages without re-render
- **Quick jump** — type chapter number + enter
- **Language filter** — `/lang vi` or `/lang en` for MangaDex
- **Vim-style navigation** — arrow keys, h/l, number jump

## Requirements

A terminal that speaks **Kitty** or **Sixel**:

| Terminal | Protocol |
|----------|----------|
| [Kitty](https://sw.kovidgoyal.net/kitty/) | Kitty (native) |
| [WezTerm](https://wezfurlong.org/wezterm/) | Kitty + Sixel |
| [Ghostty](https://ghostty.org/) | Kitty + Sixel |
| [foot](https://codeberg.org/dnkl/foot) | Sixel |
| [iTerm2](https://iterm2.com/) | Sixel |
| [Konsole](https://konsole.kde.org/) | Sixel |
| [mlterm](https://github.com/arakiken/mlterm) | Sixel |
| [XTerm](https://invisible-island.net/xterm/) | Sixel (compile with `--enable-sixel`) |

Kitty protocol is faster. If your terminal supports both, Futon picks Kitty.

## Install

### Auto (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/KabosuNeko/Futon/main/install.sh | bash
```

Uninstall:

```bash
curl -sSL https://raw.githubusercontent.com/KabosuNeko/Futon/main/install.sh | bash -s -- uninstall
```

### From source

```bash
go install github.com/KabosuNeko/Futon@latest
```

### Binary

Grab the latest from [Releases](https://github.com/KabosuNeko/Futon/releases).

Platforms:
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
| `ctrl+c` | Quit |
| `tab` | Cycle source (All → OTruyen → MangaDex → ... → All) |
| `enter` | Search / open selected manga |
| `↑` / `↓` | Navigate list |
| `/fav` | Show favorites |
| `/his` | Show history |
| `/lang vi\|en` | Set chapter language (MangaDex) |

#### Favorites / History

| Key | Action |
|-----|--------|
| `enter` | Open manga |
| `ctrl+d` | Remove from list |
| `esc` | Back to search |

#### Chapter list

| Key | Action |
|-----|--------|
| `↑` / `↓` | Browse chapters |
| `ctrl+f` | Toggle favorite |
| `enter` | Open chapter |
| `[number]` + `enter` | Jump to chapter |
| `esc` | Back to search |
| `ctrl+c` | Quit |

#### Reader

| Key | Action |
|-----|--------|
| `→` / `l` | Next page |
| `←` / `h` | Previous page |
| `ctrl+d` | Save current page |
| `esc` | Back to chapter list |
| `ctrl+c` | Quit |

## Data

| What | Where |
|------|-------|
| Favorites | `~/.config/futon/favorites.json` |
| Reading history | `~/.config/futon/history.json` |
| Downloaded images | `~/Downloads/Futon_Downloads/` |

## Architecture

```
cmd/main.go            — entry point
internal/
  api/                 — MangaProvider interface & HTTP clients
    provider.go        — interface + shared tea.Msg types
    source.go          — tea.Cmd wrappers (Search, GlobalSearch, Fetch*)
    otruyen.go         — OTruyen provider
    mangadex.go        — MangaDex provider
    truyenqq.go        — TruyenQQ provider
  models/              — shared types (manga, chapter)
  storage/             — JSON persistence (favorites, history)
  tui/                 — Bubble Tea screens
    app.go             — state machine: search → chapters → reader
    search*.go         — search screen (model, keys, cmd, view)
    chapter*.go        — chapter list (model, view)
    reader*.go         — reader (model, keys, msgs, cache, nav, view, download)
    flash.go           — flash message utility
    imgrender/         — Kitty / Sixel renderer
```

## License

MIT

# Futon

<p><br/></p>
<p align="center">
  <img src="https://github.com/user-attachments/assets/2b1cd5ba-eb66-4632-82d8-284f7c1e3780" alt="Futon Logo" style="width: 192px" />
</p>
<p><br/></p>

Một **terminal manga reader** viết bằng **Go** — render manga trực tiếp trong terminal qua **Kitty Graphics Protocol** hoặc **Sixel**, không cần mở app xem ảnh riêng. Search từ nhiều nguồn, browse chapters, đọc với Vim-style keys.

## Preview

<p><br/></p>
<p align="center">
  <img src="https://github.com/user-attachments/assets/d913a46c-7cb8-4738-ba55-6a046e0e0633" alt="Futon Preview" />
</p>
<p><br/></p>

## Highlights

- **Render ảnh trong terminal** — Kitty hoặc Sixel, auto-detect
- **Multi-source search** — OTruyen, MangaDex, TruyenQQ, FoxTruyen, BaoTangTruyen (chọn nguồn qua `/src`)
- **Favorites & History** — Bookmark truyện, resume từ trang đã đọc
- **Filter trong fav/his/src** — Gõ chữ để lọc danh sách real-time
- **Save ảnh** — `ctrl+s` để download trang hiện tại
- **Preload chapter** — Chuyển chapter mượt, zero waiting
- **LRU image cache** — 20 ảnh cached, flip page ko cần re-render
- **Quick jump** — Gõ số chapter + enter
- **Language filter** — `/lang vi` hoặc `/lang en` cho MangaDex
- **Vim-style navigation** — Arrow keys, h/l, number jump

## Yêu cầu

Terminal cần support **Kitty Graphics Protocol** hoặc **Sixel**:

| Terminal | Protocol |
|----------|----------|
| [Kitty](https://sw.kovidgoyal.net/kitty/) | Kitty (native) |
| [WezTerm](https://wezfurlong.org/wezterm/) | Kitty + Sixel |
| [Ghostty](https://ghostty.org/) | Kitty + Sixel |
| [foot](https://codeberg.org/dnkl/foot) | Sixel |
| [iTerm2](https://iterm2.com/) | Sixel |
| [Konsole](https://konsole.kde.org/) | Sixel |
| [mlterm](https://github.com/arakiken/mlterm) | Sixel |
| [XTerm](https://invisible-island.net/xterm/) | Sixel (compile với `--enable-sixel`) |

Kitty protocol nhanh hơn. Nếu terminal support cả hai, Futon tự động ưu tiên Kitty.

## Cài đặt

### Auto (recommended)

```bash
curl -sSL https://raw.githubusercontent.com/KabosuNeko/Futon/main/install.sh | bash
```

Gỡ cài đặt:

```bash
curl -sSL https://raw.githubusercontent.com/KabosuNeko/Futon/main/install.sh | bash -s -- uninstall
```

### Build từ source

```bash
go install github.com/KabosuNeko/Futon@latest
```

### Binary

Tải bản mới nhất từ [Releases](https://github.com/KabosuNeko/Futon/releases).

Hỗ trợ:
- Linux (amd64, arm64)
- macOS (amd64, arm64)

## Cách dùng

```bash
futon
```

### Keybindings

#### Search screen

| Key | Action |
|-----|--------|
| `ctrl+c` | Thoát |
| `enter` | Search / mở truyện đang chọn |
| `↑` / `↓` | Di chuyển list |
| `/fav` | Xem favorites |
| `/his` | Xem history |
| `/src` | Chọn nguồn (space để toggle) |
| `/lang vi\|en` | Set ngôn ngữ chapter (MangaDex) |

Khi ở màn hình `/fav`, `/his`, hoặc `/src`: gõ chữ để lọc danh sách.

#### Favorites / History

| Key | Action |
|-----|--------|
| `enter` | Mở truyện |
| `ctrl+d` | Xoá khỏi list |
| `esc` | Quay lại search |

#### Chapter list

| Key | Action |
|-----|--------|
| `↑` / `↓` | Browse chapters |
| `ctrl+f` | Toggle favorite |
| `enter` | Mở chapter |
| `[number]` + `enter` | Jump tới chapter |
| `esc` | Quay lại search |
| `ctrl+c` | Thoát |

#### Reader

| Key | Action |
|-----|--------|
| `→` / `l` | Trang tiếp |
| `←` / `h` | Trang trước |
| `ctrl+d` | Save trang hiện tại |
| `esc` | Về chapter list |
| `ctrl+c` | Thoát |

## Data

| Gì | Ở đâu |
|----|--------|
| Favorites + Sources | `~/.config/futon/userdata.json` |
| Reading history | `~/.config/futon/history.json` |
| Ảnh đã download | `~/Downloads/Futon_Downloads/` |

## Architecture

```
cmd/main.go            — entry point
internal/
  api/                 — MangaProvider interface & HTTP clients
    provider.go        — interface + shared helpers, tea.Msg types
    source.go          — tea.Cmd wrappers (Search, GlobalSearch, Fetch*)
    otruyen.go         — OTruyen provider
    mangadex.go        — MangaDex provider
    truyenqq.go        — TruyenQQ provider
    baotangtruyen.go   — BaoTangTruyen provider
    foxtruyen.go       — FoxTruyen provider
  models/              — shared types (manga, chapter)
  storage/             — local JSON persistence
    userdata.go        — UserData (favorites + sources merged)
    sources.go         — Load/SaveSources → delegates to userdata
    favorites.go       — Load/SaveFavorites → delegates to userdata
    history.go         — per-manga reading history
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

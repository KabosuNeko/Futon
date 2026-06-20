# Futon

<p><br/></p>
<p align="center">
  <img src="https://github.com/user-attachments/assets/2b1cd5ba-eb66-4632-82d8-284f7c1e3780" alt="Futon Logo" style="width: 192px" />
</p>
<p><br/></p>

Một **terminal manga reader** nhanh, gọn nhẹ viết bằng Go.

Futon render ảnh manga trực tiếp trong terminal của bạn qua **Kitty Graphics Protocol** hoặc **Sixel** — không cần phần mềm xem ảnh rời. Tìm kiếm truyện từ nhiều nguồn, duyệt chapter, và đọc với phím tắt kiểu Vim.

## Tính năng nổi bật

- **Render ảnh trong terminal** — Kitty Graphics Protocol hoặc Sixel
- **Đa nguồn truyện** — OTruyen, MangaDex, TruyenQQ, FoxTruyen, BaoTangTruyen (chuyển bằng `tab`, chế độ All để tìm tất cả)
- **Favorites & Lịch sử đọc** — đánh dấu truyện yêu thích, đọc tiếp từ trang đã dừng
- **Tải ảnh** — lưu trang bằng `ctrl+d`
- **Preload chapter kế tiếp** — chuyển chapter mượt mà, không chờ load
- **LRU image cache** — lật trang nhanh như chớp
- **Jump nhanh** — gõ số chapter rồi `enter`
- **Lọc ngôn ngữ** — `/lang vi` hoặc `/lang en` cho MangaDex
- **Phím tắt kiểu Vim** — phím mũi tên, nhảy số

## Yêu cầu

Terminal hỗ trợ **Kitty Graphics Protocol** hoặc **Sixel**:

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

> **Lưu ý:** Kitty graphics nhanh và mượt hơn Sixel trên cùng một terminal. Nếu terminal bạn hỗ trợ cả hai, Futon sẽ ưu tiên dùng Kitty.

## Hướng dẫn Cài đặt

### Tự động (khuyên dùng)

```bash
curl -sSL https://raw.githubusercontent.com/KabosuNeko/Futon/main/install.sh | bash
```

Script sẽ tự động phát hiện OS và architecture, tải bản release mới nhất từ GitHub Releases về và cài vào `/usr/local/bin/`.

Để gỡ cài đặt:

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

### Hệ thống Phím tắt

#### Màn hình tìm kiếm

| Phím | Chức năng |
|------|-----------|
| `ctrl+c` | Thoát |
| `tab` | Chuyển nguồn manga (All → OTruyen → MangaDex → ... → All) |
| `enter` | Tìm kiếm / mở truyện đang chọn |
| `lên` / `xuống` | Di chuyển danh sách |
| `/fav` | Xem danh sách yêu thích |
| `/his` | Xem lịch sử đọc |
| `/lang vi\|en` | Chọn ngôn ngữ chapter (MangaDex) |

#### Favorites / Lịch sử

| Phím | Chức năng |
|------|-----------|
| `enter` | Mở truyện |
| `ctrl+d` | Xoá khỏi danh sách |
| `esc` | Quay lại tìm kiếm |

#### Danh sách chapter

| Phím | Chức năng |
|------|-----------|
| `lên` / `xuống` | Duyệt chapter |
| `ctrl+f` | Thêm/xoá yêu thích |
| `enter` | Mở chapter |
| `[số] + enter` | Nhảy tới chapter |
| `esc` | Quay lại tìm kiếm |
| `ctrl+c` | Thoát |

#### Reader

| Phím | Chức năng |
|------|-----------|
| `→` / `l` | Trang tiếp |
| `←` / `h` | Trang trước |
| `ctrl+d` | Tải trang hiện tại |
| `esc` | Về danh sách chapter |
| `ctrl+c` | Thoát |

## Dữ liệu

| Dữ liệu | Đường dẫn |
|---------|-----------|
| Favorites | `~/.config/futon/favorites.json` |
| Lịch sử đọc | `~/.config/futon/history.json` |
| Ảnh đã tải | `~/Downloads/Futon_Downloads/` |

## Kiến trúc

```
cmd/main.go            — entry point
internal/
  api/                 — MangaProvider interface & HTTP clients
    provider.go        — interface definition + shared tea.Msg types
    source.go          — tea.Cmd wrappers (SearchCmd, GlobalSearchCmd, Fetch*Cmd)
    otruyen.go         — OTruyen provider
    mangadex.go        — MangaDex provider
    truyenqq.go        — TruyenQQ provider
    foxtruyen.go       — FoxTruyen provider
    baotangtruyen.go   — BaoTangTruyen provider
  models/              — shared data types (manga.go, chapter.go)
  storage/             — JSON persistence (favorites, history)
  tui/                 — Bubble Tea screens (search, chapters, reader)
    app.go             — router: search → chapters → reader
    search*.go         — search screen (model, keys, cmd, view)
    chapter*.go        — chapter list screen (model, view)
    reader*.go         — reader screen (model, keys, msgs, cache, navigation, view, download)
    flash.go           — flash message helper
    imgrender/         — Kitty / Sixel renderer selection
```

## Giấy phép

MIT

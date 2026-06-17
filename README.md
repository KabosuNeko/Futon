# Futon

Trình đọc manga trên terminal, nhanh và tối giản, viết bằng Go.

Futon render ảnh manga trực tiếp trong terminal nhờ Kitty Graphics Protocol — không cần phần mềm xem ảnh rời. Tìm kiếm truyện từ nhiều nguồn, duyệt chapter, và đọc với phím tắt kiểu Vim.

## Tính năng

- **Render ảnh trong terminal** — Kitty Graphics Protocol (dự phòng Sixel)
- **Nhiều nguồn truyện** — MangaDex, OTruyen (chuyển bằng `tab`)
- **Yêu thích & Lịch sử đọc** — đánh dấu truyện, đọc tiếp từ vị trí đã dừng
- **Tải ảnh ngoại tuyến** — lưu trang ảnh bằng `ctrl+d`
- **Preload chapter kế** — chuyển chapter mượt mà không chờ load
- **LRU image cache** (20 ảnh) — lật trang nhanh
- **Nhảy nhanh** — gõ số chapter rồi `enter`
- **Lọc ngôn ngữ** — `/lang vi` hoặc `/lang en` cho MangaDex
- **Phím tắt Vim-like** — phím mũi tên, nhảy số

## Yêu cầu

Terminal hỗ trợ **Kitty Graphics Protocol**:

| Terminal | Hỗ trợ |
|----------|--------|
| [Kitty](https://sw.kovidgoyal.net/kitty/) | Native |
| [WezTerm](https://wezfurlong.org/wezterm/) | Native |
| [Ghostty](https://ghostty.org/) | Native |
| Khác | Sixel dự phòng |

## Cài đặt

### Từ nguồn

```bash
go install github.com/KabosuNeko/Futon@latest
```

### Binary có sẵn

Tải bản mới nhất từ mục [Releases](https://github.com/KabosuNeko/Futon/releases).

Hỗ trợ:
- Linux (amd64, arm64)
- macOS (amd64, arm64)

## Sử dụng

```bash
futon
```

### Phím tắt

#### Màn hình tìm kiếm

| Phím | Chức năng |
|------|-----------|
| `q` / `ctrl+c` | Thoát |
| `tab` | Chuyển nguồn truyện |
| `enter` | Tìm kiếm / mở truyện đã chọn |
| `lên` / `xuống` | Di chuyển danh sách |
| `/fav` | Xem truyện yêu thích |
| `/his` | Xem lịch sử đọc |
| `/lang vi\|en` | Chọn ngôn ngữ chapter (MangaDex) |

#### Yêu thích / Lịch sử

| Phím | Chức năng |
|------|-----------|
| `enter` | Mở truyện |
| `d` | Xóa khỏi danh sách |
| `esc` | Quay lại tìm kiếm |

#### Danh sách chapter

| Phím | Chức năng |
|------|-----------|
| `lên` / `xuống` | Di chuyển chapter |
| `ctrl+f` | Thêm/xóa yêu thích |
| `enter` | Mở chapter |
| `[số] + enter` | Nhảy đến chapter |
| `esc` | Quay lại tìm kiếm |
| `q` / `ctrl+c` | Thoát |

#### Trình đọc

| Phím | Chức năng |
|------|-----------|
| `→` / `l` | Trang kế |
| `←` / `h` | Trang trước |
| `ctrl+d` | Tải trang hiện tại |
| `esc` / `q` | Quay lại danh sách chapter |
| `ctrl+c` | Thoát |

## Dữ liệu

| Dữ liệu | Đường dẫn |
|---------|-----------|
| Yêu thích | `~/.config/futon/favorites.json` |
| Lịch sử đọc | `~/.config/futon/history.json` |
| Ảnh đã tải | `~/Downloads/Futon_Downloads/` |

## Kiến trúc

```
cmd/main.go          — điểm vào
internal/
  api/               — interface MangaProvider & HTTP clients
  models/            — kiểu dữ liệu chung
  storage/           — JSON persistence (yêu thích, lịch sử)
  tui/               — màn hình Bubble Tea (tìm kiếm, chapter, đọc)
  tui/imgrender/     — chọn Kitty / Sixel renderer
```

## Giấy phép

MIT

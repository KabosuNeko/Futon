# Futon

<p align="center">
  <img src="https://github.com/user-attachments/assets/2b1cd5ba-eb66-4632-82d8-284f7c1e3780" alt="Futon Logo" style="width: 192px" />
</p>
<p align="center">
  <a href="https://github.com/KabosuNeko/Futon/releases"><img src="https://img.shields.io/github/v/release/KabosuNeko/Futon?color=d4a259&label=release" alt="GitHub release" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License" /></a>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/go-%3E%3D1.26-00ADD8.svg" alt="Go Version" /></a>
</p>

Một **terminal manga reader** viết bằng **Go** — render manga trực tiếp trong terminal qua **Kitty Graphics Protocol** hoặc **Sixel**, không cần mở app xem ảnh riêng. Search từ nhiều nguồn, browse chapters, đọc với Vim-style keys.

## Preview

<p align="center">
  <img src="https://github.com/user-attachments/assets/da70481c-36f1-4516-84f2-e647a2668a75" alt="Futon Preview" />
</p>

## Yêu cầu

Terminal cần support **Kitty Graphics Protocol** hoặc **Sixel**:

| Terminal | Protocol |
|----------|----------|
| [Kitty](https://sw.kovidgoyal.net/kitty/) | Kitty (native) |
| [WezTerm](https://wezfurlong.org/wezterm/) | Kitty + Sixel |
| [Ghostty](https://ghostty.org/) | Kitty |
| [foot](https://codeberg.org/dnkl/foot) | Sixel |
| [iTerm2](https://iterm2.com/) | Sixel |
| [Konsole](https://konsole.kde.org/) | Sixel |
| [mlterm](https://github.com/arakiken/mlterm) | Sixel |
| [XTerm](https://invisible-island.net/xterm/) | Sixel (compile với `--enable-sixel`) |

Kitty protocol nhanh hơn. Với Kitty, Ghostty và WezTerm, Futon tự dùng Kitty; terminal khác dùng Sixel. Có thể ép bằng `FUTON_RENDERER=kitty` hoặc `FUTON_RENDERER=sixel`.

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
go install github.com/KabosuNeko/Futon/cmd@latest
```

### Binary

Tải bản mới nhất từ [Releases](https://github.com/KabosuNeko/Futon/releases).

Hỗ trợ:
- Linux (amd64, arm64)
- macOS (amd64, arm64)

## Cách dùng

```bash
futon              # mở TUI
futon update       # kiểm tra & cài bản mới (không cần mở TUI)
```

### Keybindings

#### Search screen

| Key | Action |
|-----|--------|
| `ctrl+c` | Thoát |
| `enter` | Search / mở truyện đang chọn |
| `↑` / `↓` | Di chuyển list (xuống cuối feed/bộ lọc sẽ tự tải trang tiếp) |
| `shift+←` / `shift+→` | Đổi nguồn khi cùng một truyện có ở nhiều nguồn |
| `tab` | Chuyển tab (feed → yêu thích → lịch sử → nguồn → bộ lọc) |
| `shift+tab` | Chuyển tab ngược (bộ lọc → nguồn → lịch sử → yêu thích → feed) |
| `/fav` | Xem favorites |
| `/his` | Xem history |
| `/src` | Chọn nguồn (space để toggle) |
| `/feed` | Quay lại danh sách truyện mới cập nhật |
| `/filter` (`/f`) | Mở bộ lọc |
| `/update` | Kiểm tra bản mới (cài bằng `futon update` ngoài terminal) |
| `/lang vi\|en` | Set ngôn ngữ chapter (MangaDex), được lưu lại cho lần sau |

Khi ở màn hình `/fav`, `/his`, hoặc `/src`: gõ chữ để lọc danh sách.

#### Favorites / History

| Key | Action |
|-----|--------|
| `enter` | Mở truyện (từ Lịch sử: mở thẳng chương & trang đang đọc) |
| `ctrl+d` | Xoá khỏi list |
| `esc` | Quay lại search |

#### Chapter list

| Key | Action |
|-----|--------|
| `↑` / `↓` | Browse chapters |
| `g` / `home` | Về chapter đầu |
| `G` / `end` | Đến chapter cuối |
| `ctrl+f` | Thêm truyện vào Yêu thích |
| `ctrl+e` | Xuất chapter đang chọn ra CBZ |
| `enter` | Mở chapter |
| `[number]` + `enter` | Jump tới chapter |
| `esc` | Quay lại search |
| `ctrl+c` | Thoát |

#### Reader

| Key | Action |
|-----|--------|
| `→` / `l` | Trang tiếp |
| `←` / `h` | Trang trước |
| `j` / `↓` | Nhảy sang chapter tiếp theo (chế độ webtoon: `↓` cuộn trang) |
| `k` / `↑` | Về chapter trước (chế độ webtoon: `↑` cuộn trang) |
| `w` | Bật/tắt chế độ webtoon: vừa chiều ngang, cuộn bằng `↑`/`↓`/`space`/chuột (được nhớ) |
| `ctrl+d` | Save trang hiện tại |
| `ctrl+h` | Ẩn/hiện gợi ý phím tắt (trang dùng thêm 1 dòng) |
| `ctrl+e` | Xuất chapter hiện tại ra CBZ |
| `r` | Tải lại trang bị lỗi |
| `esc` | Về chapter list |
| `ctrl+c` | Thoát |

## Nguồn

| Nguồn | Ghi chú |
|-------|---------|
| MangaDex | API chính thức; một số nhà mạng/DNS chặn `api.mangadex.org` (đổi DNS nếu bị lỗi kết nối) |
| TruyenQQ | Tự chuyển sang mirror `truyenggvn.com` (TruyenGG) khi domain chính chết |
| BaoTangTruyen | |
| CuuTruyen | `cuutruyen.moe`, ảnh chất lượng cao (~1024px); có lẫn truyện 18+ |

Bật/tắt nguồn trong `/src`. Lịch sử và yêu thích cũ của TruyenGG được tự chuyển sang TruyenQQ.

## Data

| Gì | Ở đâu |
|----|--------|
| Favorites + Sources + `/lang` | `~/.config/futon/userdata.json` |
| Reading history | `~/.config/futon/history.json` |
| Ảnh đã download | `~/Downloads/Futon/` |
| CBZ đã xuất | `~/Downloads/Futon/` (mỗi chapter một file) |

## Architecture

Xem [`AGENTS.md`](AGENTS.md) để biết cấu trúc project và conventions.

## License

MIT

# SPEC — Futon

Terminal manga reader viết bằng **Go** + **Bubble Tea**, render ảnh trực tiếp trong terminal qua **Kitty Graphics Protocol** hoặc **Sixel**.

> Tài liệu này định nghĩa product requirements và scope boundaries của Futon.
> Mọi thay đổi hành vi phải được phản ánh vào đây trước khi merge.
> Tham chiếu: `AGENTS.md` (kiến trúc), `ROADMAP.md` (lộ trình), `TASKS.md` (việc hiện tại).

---

## 1. Tổng quan

Futon là ứng dụng TUI (terminal user interface) cho phép người dùng:

1. Tìm kiếm manga từ nhiều nguồn cùng lúc (multi-source search).
2. Duyệt danh sách chapter của một truyện.
3. Đọc manga ngay trong terminal — ảnh render trực tiếp, không cần app xem ảnh ngoài.
4. Quản lý favorites, lịch sử đọc, và tải ảnh về máy.

## 2. Goals

- **Đọc manga trong terminal** với trải nghiệm không thua kém web reader: render ảnh, flip trang mượt, chuyển chapter liên tục không chờ đợi.
- **Multi-source**: một truyện có thể tìm thấy từ nhiều nguồn; kết quả gộp lại và gắn nhãn nguồn.
- **Zero-friction resume**: mở lại truyện là về đúng chapter, đúng trang đã đọc.
- **Tự cập nhật**: bản phát hành mới được phát hiện và cài đặt ngay trong app.
- **Vim-style navigation** cho người quen thao tác bàn phím.

## 3. Non-Goals (Out of scope)

- Đọc manga dạng cuộn liên tục (infinite scroll) — chỉ đọc theo trang.
- Hỗ trợ Windows (build target chỉ Linux + macOS; code Windows chỉ để compile, không hoạt động).
- Quản lý thư viện truyện offline (download cả bộ truyện, đọc offline).
- Download nhiều trang cùng lúc theo yêu cầu thủ công (chỉ download 1 trang bằng `ctrl+d`).
- Tài khoản người dùng, đồng bộ cloud, follow/notification.
- Comment, rating, bình luận trong app.
- Rate limiting / retry / backoff cho các provider (không tồn tại ở interface hiện tại).

## 4. Yêu cầu chức năng

### 4.1 Tìm kiếm

- **FR-S1**: Gõ ≥ 3 ký tự ở search screen sẽ tự tìm kiếm sau **debounce 300ms**; dưới 3 ký tự không kích hoạt tìm kiếm.
- **FR-S2**: Tìm kiếm chạy **song song trên tất cả provider được bật** (1 goroutine/provider, đợi đủ bằng WaitGroup).
- **FR-S3**: Kết quả mỗi nguồn được gắn `Provider` và **đổi title thành `"Title (sourcename)"`** (tên nguồn chữ thường).
- **FR-S4**: Nếu một provider lỗi, kết quả các provider khác vẫn được trả về (partial success); lỗi gom thành 1 message nối bằng `"; "`.
- **FR-S5**: Nếu **không có nguồn nào được bật**, mọi thao tác tìm kiếm hiển thị lỗi "Chọn ít nhất một nguồn trong /src".
- **FR-S6**: Nhập bắt đầu bằng `/` sẽ reset kết quả cũ, không tìm kiếm (dành cho slash commands).
- **FR-S7**: Kết quả tìm kiếm cũ bị bỏ qua nếu query đã thay đổi (guard chống stale response).

#### Giới hạn theo provider
- **MangaDex**: search **phân trang** — mỗi request `limit=100`, loop theo offset tới khi đủ `total` (theo mẫu `FetchChapters`); không còn giới hạn 5 kết quả. Title ưu tiên `en` → `ja-ro` → ngôn ngữ bất kỳ, fallback "Không rõ tiêu đề".
- **OTruyen**: trả kết quả không có ảnh bìa (CoverURL rỗng).
- **TruyenQQ / FoxTruyen / BaoTangTruyen**: scrape HTML, cần browser UA (Chrome 127) do chặn bot.

### 4.2 Slash commands

- **FR-C1** `src`: mở checklist nguồn; `space` toggle bật/tắt từng provider; trạng thái **lưu ngay vào `userdata.json`**; `esc` đóng.
- **FR-C2** `fav`: mở danh sách favorites (load async); `enter` mở truyện; `ctrl+d` xóa favorite.
- **FR-C3** `his`: mở lịch sử đọc (load async); `enter` mở truyện resume đúng chapter; `ctrl+d` xóa entry.
- **FR-C4** `lang vi|en`: set ngôn ngữ chapter cho **MangaDex** (mặc định `vi`); cú pháp sai hiển thị "Dùng: /lang vi hoặc /lang en".
- **FR-C5**: Trong mode `/fav`, `/his`, `/src`, gõ chữ → **lọc real-time** (case-insensitive, substring) theo Title (fav), MangaTitle/ID (his), tên nguồn (src).

### 4.3 Favorites

- **FR-F1**: `ctrl+f` ở chapter list thêm manga vào favorites; hiển thị flash "Đã thêm ... vào Yêu thích" (2s).
- **FR-F2**: Dedupe theo MangaID — thêm trùng là no-op.
- **FR-F3**: Persist vào `~/.config/futon/userdata.json` (trường `favorites`).

### 4.4 Lịch sử đọc

- **FR-H1**: Mỗi lần flip trang (`→`/`l`, `←`/`h`) ghi history vào RAM cache (không block UI).
- **FR-H2**: Ghi disk **debounce 2s** kể từ lần save cuối (timer reset mỗi lần save).
- **FR-H3**: `esc` ở reader: flush history ngay (không chờ debounce).
- **FR-H4**: Xóa entry (`ctrl+d` ở `/his`) flush đồng bộ ngay lập tức.
- **FR-H8**: `ctrl+c` ở reader (hoặc từ router khi đang reader): save + flush history trước khi quit (chống mất vị trí trang cuối khi thoát trước debounce 2s).
- **FR-H5**: Chapter list khôi phục cursor về chapter đã đọc cuối (`restoreHistoryPosition`).
- **FR-H6**: Reader resume đúng trang khi mở chapter trùng lịch sử (via `StartPageIndex=-1`).
- **FR-H7**: History giữ lại Title/ChapterNumber cũ nếu lần ghi mới không cung cấp (chống mất thông tin).

### 4.5 Chapter list

- **FR-L1**: Hiển thị danh sách chapter của manga theo thứ tự từ chapter 1 (các nguồn HTML tự reverse newest-first về ascending).
- **FR-L2**: **Quick jump** — gõ số + `enter` nhảy tới đúng chapter (match theo `Number`); không match → flash "Không tìm thấy chapter X".
- **FR-L3**: `esc` khi đang nhập số xóa input buffer; `esc` khi không nhập quay về search.
- **FR-L4**: Provider HTML trả chapter **không có Number** → hiển thị fallback bằng Title.
- **FR-L5**: MangaDex feed chapter lọc theo ngôn ngữ đã set (`/lang`), pagination tự động 500 chapter/lần.

### 4.6 Reader

- **FR-R1**: Điều hướng trang: `→`/`l` trang tiếp, `←`/`h` trang trước. **Chỉ active ở reader** (không bị màn hình khác nuốt).
- **FR-R2**: **Chuyển chapter liên tục**: ở trang cuối bấm `→` sang chapter sau; ở trang đầu bấm `←` về chapter trước — không cần quay lại danh sách.
- **FR-R3**: Preload chapter: khi đến trang `total-3`, tự động fetch chapter kế + **tải trước 2 ảnh đầu**; nếu preload xong, chuyển chapter **không fetch lại, không chờ** (zero waiting).
- **FR-R4**: LRU image cache **20 ảnh render** (key = page index), evict trang cũ nhất; reset khi vào chapter mới. Pre-render tối đa 3 trang phía trước chưa có cache.
- **FR-R5**: Download trang hiện tại bằng `ctrl+d` → `~/Downloads/Futon_Downloads/`, tên file `{Title}_Ch{ChapterNumber}_Pg{page}.jpg`, sanitize ký tự đặc biệt, xử lý trùng tên bằng suffix `_1`...`_999`.
- **FR-R6**: Tải ảnh trong chapter: **tối đa 4 concurrent**, ưu tiên trang hiện tại → các trang phía trước → các trang phía sau.
- **FR-R7**: `esc` = save history + flush + clear screen + về chapter list (đảm bảo thứ tự bằng `tea.Sequence`).
- **FR-R8**: Khi đổi kích thước terminal → re-render trang hiện tại.

### 4.7 Image rendering

- **FR-G1**: Auto-detect renderer: `FUTON_RENDERER` env override (`kitty`/`sixel`) > Kitty (`TERM=xterm-kitty`, `KITTY_WINDOW_ID`, `TERM` prefix `st-`) > Sixel (fallback).
- **FR-G2**: Ảnh fit vào kích thước terminal, **chừa dòng cuối** cho footer/flash, giữ aspect ratio (resize Lanczos3).
- **FR-G3**: Dùng ANSI cursor positioning + clear screen (`\x1b[H\x1b[2J`); **không dùng `\n` scrolling** trong reader.
- **FR-G4**: Kitty protocol: PNG → base64 → chunk 2048 bytes.

### 4.8 Tự cập nhật

- **FR-U1**: Khi khởi động, check GitHub `releases/latest` (timeout 5s) ở background; version `dev` (build từ source) **không check**.
- **FR-U2**: Nếu có bản mới → banner `"[!] Đã có bản cập nhật {version}. Nhấn Ctrl+u để tự động cài đặt."` (phím thực tế: `ctrl+u`).
- **FR-U3**: `ctrl+u` → chạy `install.sh` từ GitHub main branch (qua `curl|bash`, có `sudo` bên trong), hiển thị "Đang cập nhật..."; xong bắt buộc thoát để áp dụng.
- **FR-U4**: So sánh phiên bản theo semver component-wise (bỏ prefix `v`).

### 4.9 Keybindings tổng hợp

| Màn hình | Phím | Hành động |
|---|---|---|
| Toàn cục | `ctrl+c` | Thoát app |
| Toàn cục | `ctrl+u` | Cài update (chỉ khi có bản mới, ở search) |
| Search | `enter` | Search / mở mục đang chọn / thực thi slash command |
| Search | `↑`/`↓` | Di chuyển danh sách |
| Search | `/fav`, `/his`, `/src`, `/lang vi\|en` | Mở tính năng |
| Fav/His | `enter` | Mở truyện |
| Fav/His | `ctrl+d` | Xóa khỏi danh sách |
| Fav/His/Src | `esc` | Quay về search |
| Chapter | `ctrl+f` | Thêm vào favorites |
| Chapter | `[số]`+`enter` | Jump tới chapter |
| Reader | `→`/`l`, `←`/`h` | Trang sau / trước |
| Reader | `ctrl+d` | Lưu ảnh trang hiện tại |
| Reader | `ctrl+c` | Save + flush history rồi thoát |

## 5. Yêu cầu phi chức năng

- **NFR-1**: Terminal phải hỗ trợ Kitty Graphics Protocol hoặc Sixel (xem bảng terminal tương thích trong `README.md`).
- **NFR-2**: UI không được block — mọi I/O mạng và disk nằm trong `tea.Cmd` / goroutine, không được chặn `Update`.
- **NFR-3**: Gõ phím phải phản hồi tức thì; network search không treo giao diện.
- **NFR-4**: Dữ liệu cục bộ an toàn — ghi file 0644, không ghi đè khi không cần thiết.
- **NFR-5**: Build chéo không cần CGO (`CGO_ENABLED=0`).
- **NFR-6**: Thoát bằng `ctrl+c` ở reader flush history ngay (save + flush đồng bộ) trước khi quit — không mất vị trí trang cuối.

## 6. Dữ liệu & Lưu trữ

| Gì | Đường dẫn | Định dạng |
|---|---|---|
| Favorites + source toggles | `~/.config/futon/userdata.json` | JSON: `sources[]`, `favorites[]` |
| Lịch sử đọc | `~/.config/futon/history.json` | JSON: map theo MangaID |
| Ảnh đã tải | `~/Downloads/Futon_Downloads/` | JPG |

- **Migration**: lần đầu chạy sau upgrade, nếu `userdata.json` chưa tồn tại, tự merge `favorites.json` + `sources.json` cũ vào `userdata.json` rồi xóa 2 file cũ (one-time).
- Source toggles lưu theo **tên provider**; mặc định tất cả bật.

## 7. Nền tảng

- **Build**: Go 1.26, `go build ./...`, `go test ./...`, `go test -race ./...`.
- **Release**: GoReleaser — Linux + macOS × amd64 + arm64; tag `v*`; inject version vào binary (`main.Version`).
- **Terminal hỗ trợ**: Kitty, WezTerm, Ghostty, foot, iTerm2, Konsole, mlterm, XTerm (sixel).

## 8. Ràng buộc kiến trúc

- `internal/api`: `MangaProvider` interface (`Name`, `Search`, `FetchChapters`, `FetchPages`) + các `tea.Cmd` wrapper. **Không** có context/retry/rate-limit ở interface.
- `internal/tui`: Bubble Tea, router `AppModel` (search → chapters → reader), message điều hướng `ViewMangaMsg` / `ViewChapterMsg` / `BackToSearchMsg` / `BackToChaptersMsg`.
- `internal/storage`: mọi persistence đi qua `userdata.go` (hợp nhất) hoặc `history.go` (debounced).
- Manga ID là **opaque string** — slug (OTruyen), UUID (MangaDex), URL (HTML providers) — không được giả định format.

## 9. Tiêu chí hoàn thành (Definition of Done cho mọi thay đổi)

1. Code build sạch: `go build ./...`.
2. Test pass: `go test ./...` và `go test -race ./...`.
3. Có regression test cho hành vi TUI/storage bị thay đổi.
4. `SPEC.md` / `README.md` cập nhật nếu hành vi user-facing đổi.
5. Không suppress type error; không bỏ test.

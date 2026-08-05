# TASKS — Futon

Trạng thái công việc hiện tại. Cập nhật file này khi bắt đầu/hoàn thành một task.
Legend: `[x]` = done, `[~]` = in progress, `[ ]` = pending/backlog.
Định nghĩa "done" chuẩn: `SPEC.md` §9 — build sạch, test pass (kể cả `-race`), regression test nếu đụng TUI/storage, cập nhật doc nếu đổi hành vi user-facing.

---

## Đang làm (In Progress)

_Phase 1 tech debt đã xử lý xong — dự án ở trạng thái stable. Xem backlog Phase 2 phía dưới._

---

## Task backlog (theo thứ tự ưu tiên)

### Phase 1 — Stability & Tech debt

| ID | Task | Ref | Ưu tiên | Trạng thái |
|----|------|-----|---------|------------|
| T-101 | Fix updater sai asset name trên macOS (`darwin` vs `macOS`) | `internal/updater/updater.go`, `.goreleaser.yaml` | P1 | [x] |
| T-102 | Flush history khi `ctrl+c` thoát reader (chống mất trang cuối) | `internal/storage/history.go`, `internal/tui/app.go`, `reader_keys.go` | P1 | [x] |
| T-103 | Sửa banner update "Nhấn 'U'" → đúng phím `ctrl+u` | `internal/tui/app.go` | P2 | [x] |
| T-104 | Thêm regression test cho `migrateOldData` (migration 1 lần) | `internal/storage/userdata.go` | P2 | [x] |
| T-105 | Cập nhật `chapterNumber` khi chuyển chapter trong reader (preload + `chapterNavCmd`) | `internal/tui/reader_navigation.go`, `reader_download.go` | P2 | [x] |
| T-106 | Verify checksum trước khi cài self-update (tuỳ chọn) | `install.sh` | P2 | [x] |

### Phase 2 — UX & tính năng

| ID | Task | Ref | Trạng thái |
|----|------|-----|------------|
| T-201 | MangaDex search: tăng limit 5 → configurable/paginate | `internal/api/mangadex.go` | [x] |
| T-202 | Hiển thị cover images trong search | `internal/tui/search_view.go`, providers | [ ] |
| T-203 | Retry/timeout cho provider calls | `internal/api/provider.go`, `source.go` | [ ] |
| T-204 | Chỉ báo loading/error theo từng provider trong search | `internal/api/source.go` (GlobalSearchMsg), `internal/tui/search.go` | [ ] |
| T-205 | Quick jump: hỗ trợ prefix/fuzzy thay vì match chính xác | `internal/tui/chapter.go` | [ ] |

---

## Đã hoàn thành (Done)

### Phase 2 — UX & tính năng

- [x] T-201: MangaDex search phân trang — `Search()` loop theo offset, mỗi request `limit=100`, dừng khi đủ `total`; thêm `Total` vào `MangaSearchResponse`; `mangadexBaseURL` thành package var để test httptest (`internal/api/mangadex_test.go`, 4 test).

### Phase 1 — Stability & Tech debt (mới hoàn thành)

- [x] T-101: `assetFileName` helper map `darwin`→`macOS` cho updater; thêm test darwin/linux.
- [x] T-102: `ctrl+c` khi đang ở reader giờ flush history (save + flush) trước khi quit — không mất trang cuối; fix ở cả `app.go` router và `reader_keys.go`; thêm test `quit_flush_test.go`.
- [x] T-103: banner update hiển thị "Nhấn Ctrl+u" thay vì "Nhấn 'U'" (đúng phím thật).
- [x] T-104: regression test `migrateOldData` — merge, skip khi userdata đã tồn tại, corrupt favorites, không file (`userdata_test.go`, 4 test).
- [x] T-105: `chapterNumber` được cập nhật khi chuyển chapter — `ViewChapterMsg` mang `AllChapterNumbers`, `chapterNavCmd` + `applyPreloadedChapter` set đúng số chapter; thêm test `chapter_number_test.go`.
- [x] T-106: `install.sh` verify sha256 checksum trước khi giải nén (tải `checksums.txt`, so khớp `sha256sum`/`shasum`, abort nếu lệch hoặc thiếu).

### Phase 0 — MVP

Lịch sử rút gọn theo git log (các nhóm chức năng đã ship):

- [x] Scaffold dự án, module path, GoReleaser config.
- [x] `h`/`l` keybinding reader + install script + version flag + self-update cơ chế.
- [x] Fix infinite render loop trong reader; đơn giản hoá guard clauses, gộp navigation logic trùng.
- [x] Bỏ phím `q`; chuyển `d` → `ctrl+d` (tránh conflict với input search); cập nhật footer hints.
- [x] Reuse `FlushHistory` trong `DeleteHistory` (bỏ duplicate flush logic).
- [x] Updater: validate HTTP status, download temp file, resolve asset URL từ API, semver compare.
- [x] **Bản stable đầu tiên phát hành** — auto update từ các bản sau.
- [x] Thêm nguồn **TruyenQQ** (rewrite cho truyenqqko.com, domain fallback probe).
- [x] Thêm nguồn **FoxTruyen**, **BaoTangTruyen**.
- [x] Thêm trường `Provider` vào `Manga` model + `GlobalSearchCmd` (search all).
- [x] Chế độ search All + GlobalSearch trong TUI; truyền `ProviderName` qua các screens.
- [x] Fix: màn hình search all bị tràn; `j`/`k` không gõ được trong tìm kiếm.
- [x] Fix: MangaDex gửi Placeholder thay vì ảnh (UA + bỏ Referer).
- [x] Hỗ trợ `st-graphics` (kitty protocol), chừa 1 dòng footer.
- [x] Renderer interface cleanup, debounce, flush logging, bounds guard, lang handler, UA const.
- [x] `/src` provider checklist, merge `userdata.json`, filter fav/his/src real-time.
- [x] Remap trigger update `U` → `ctrl+u` (để gõ được chữ U hoa trong search).
- [x] Bump Go 1.25 → 1.26, update deps, upgrade UA Chrome 127, `go fmt`.
- [x] Cleanup: bỏ comments thừa (slop), thêm comments quan trọng.
- [x] Dependabot + update GitHub Actions.

---

## Chú thích vận hành

- **Trước khi bắt đầu task mới**: đọc `SPEC.md` phần liên quan + `AGENTS.md`, tạo branch riêng.
- **Trước khi đóng task**: chạy `go build ./...`, `go test ./...`, `go test -race ./...`; nếu đụng UI, tự chạy thử render.
- **Nếu phát hiện hành vi sai mới**: thêm vào bảng Phase 1 ở trên trước (đừng sửa "trong lúc làm việc khác").
- **Nếu SPEC cũ**: cập nhật SPEC trước, code sau (spec-first).
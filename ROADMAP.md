# ROADMAP — Futon

Lộ trình phát triển theo giai đoạn. `TASKS.md` chứa chi tiết việc hiện tại; `SPEC.md` là đặc tả chuẩn.

Legend: `[x]` = done, `[~]` = in progress, `[ ]` = backlog.

---

## Phase 0 — MVP stable (HOÀN THÀNH)

Phiên bản nền tảng: đọc manga trong terminal, multi-source search, favorites/history, tự cập nhật.

- [x] **Render ảnh trong terminal** — Kitty (native) + Sixel (fallback), auto-detect (`FUTON_RENDERER` override).
- [x] **Multi-source search** — 5 provider (OTruyen, MangaDex, TruyenQQ, FoxTruyen, BaoTangTruyen) chạy song song, title chuẩn hoá `(source)`.
- [x] **Flow điều hướng** — search → chapter list → reader, điều hướng giữa chapter liên tục trong reader.
- [x] **Favorites** — thêm/xoá/lưu `userdata.json`, `ctrl+f`.
- [x] **Reading history** — resume đúng chapter/trang, debounce flush 2s, `/his`.
- [x] **`/src` provider checklist** — toggle, persist, lọc real-time.
- [x] **`/lang vi|en`** — filter ngôn ngữ chapter MangaDex.
- [x] **LRU image cache (20 ảnh)** + **preload chapter** — chuyển chapter zero-waiting.
- [x] **Save ảnh** — `ctrl+d` → `~/Downloads/Futon_Downloads/`, xử lý trùng tên.
- [x] **Self-update** — check GitHub releases, cài qua `install.sh`.
- [x] **Release infra** — GoReleaser (linux/darwin × amd64/arm64), tag `v*`, `install.sh`, checksums.

**Kết quả**: bản stable đầu tiên đã phát hành; auto-update hoạt động.

---

## Phase 1 — Stability & Tech debt (HOÀN THÀNH)

Dọn các điểm yếu đã biết trước khi thêm tính năng mới. Mục tiêu: không còn hành vi sai/sụp đổ trên bề mặt.

- [x] **Tech debt #1 — Updater hỏng trên macOS** (P1)
  - *Nguyên nhân*: GoReleaser đặt tên archive `futon_<ver>_macOS_arm64.tar.gz` nhưng `updater.go` tìm asset với `runtime.GOOS` = `darwin` → gõ nhầm tên file trên macOS.
  - *Giải pháp*: thêm helper `assetFileName(version, goos, goarch)` map `darwin`→`macOS`; dùng trong `CheckForUpdate`.
  - *Test*: `TestAssetFileNameDarwin`, `TestAssetFileNameLinux`.
  - **Reference**: `internal/updater/updater.go`, `.goreleaser.yaml`.

- [x] **Tech debt #2 — Banner update sai phím** (P2)
  - `"[!] Đã có bản cập nhật..."` ghi "Nhấn 'U'" nhưng key thực là `ctrl+u`.
  - *Giải pháp*: đổi banner → "Nhấn Ctrl+u"; test `TestUpdateBannerSaysCtrlU`; `SPEC.md` phần 9 đã cập nhật.
  - **Reference**: `internal/tui/app.go`.

- [x] **Tech debt #3 — Mất history khi thoát nhanh** (P1)
  - `ctrl+c` ở reader chỉ `tea.Quit`, không flush; nếu thoát trước debounce 2s, vị trí trang cuối mất.
  - *Giải pháp*: `ctrl+c` ở cả router `app.go` lẫn `reader_keys.go` giờ chạy `SaveHistoryCmd` + `FlushHistoryCmd` trước `tea.Quit` (guard khi chưa có manga/chapter).
  - *Test*: `TestCtrlCInReaderFlushesHistory`, `TestReaderKeysCtrlCFlushesHistory` (package tui).
  - **Reference**: `internal/storage/history.go`, `internal/tui/app.go`, `reader_keys.go`.

- [x] **Tech debt #4 — Thiếu test `migrateOldData`** (P2)
  - Migration 2 file cũ → `userdata.json` là logic nhạy cảm nhất nhưng không có regression test.
  - *Giải pháp*: `userdata_test.go` với 4 test — merge + xoá file cũ, skip khi userdata đã tồn tại, không file nào, corrupt favorites bị bỏ qua.
  - **Reference**: `internal/storage/userdata.go`.

- [x] **Tech debt #5 — `chapterNumber` stale sau chuyển chapter trong reader** (P2)
  - Chuyển chapter bằng preload / `chapterNavCmd` không cập nhật `chapterNumber` → ảnh save bằng `ctrl+d` sau đó bị nhãn chapter sai/cũ, history lưu số trống.
  - *Giải pháp*: `ViewChapterMsg` mang thêm `AllChapterNumbers`; reader lưu `allChapterNumbers`; `chapterNavCmd` và `applyPreloadedChapter` set đúng `chapterNumber` theo index (nil-safe).
  - *Test*: `chapter_number_test.go` — 6 test (cmd set number, out-of-range, nil, preload apply, giữ số cũ khi nil, router forward).
  - **Reference**: `internal/tui/app.go`, `reader_download.go`, `reader_navigation.go`, `reader_keys.go`, `chapter.go`.

- [x] **Tech debt #6 — Self-update không verify** (P2, bảo mật)
  - Cài qua `curl install.sh | bash` + `sudo`, không check checksum, không rollback.
  - *Giải pháp*: `install.sh` tải `checksums.txt`, verify sha256 (`sha256sum`/`shasum` fallback) trước khi giải nén; abort nếu thiếu/lệch; dọn `checksums.txt` khi cleanup. `bash -n` pass.
  - **Reference**: `install.sh`.

**Kết quả**: toàn bộ 6 tech debt Phase 1 đã đóng. `go build ./...`, `go test ./...`, `go test -race ./...` đều xanh (storage 11 test, tui 21 test, updater 8 test).

---

## Phase 2 — UX & tính năng (KẾ TIẾP)

Những cải thiện trải nghiệm không thay đổi kiến trúc nền.

- [x] **MangaDex search tăng limit** (`limit=5` → configurable/paginate) — search chỉ trả 5 kết quả đang quá ít.
  - *Giải pháp*: `Search()` paginate offset loop — mỗi request `limit=100`, loop tới khi đủ `total` (theo mẫu `FetchChapters`); thêm `Total` vào `MangaSearchResponse`; `mangadexBaseURL` thành package var để test bằng httptest. 4 test pagination (multi-page, single-page, stop-at-total, HTTP error).
  - **Reference**: `internal/api/mangadex.go`, `internal/api/mangadex_test.go`, `internal/models/manga.go`.
- [ ] **Cover images trong search** — hiện search không hiển thị ảnh bìa (OTruyen/MangaDex CoverURL rỗng).
- [ ] **Retry / timeout** cho provider — interface hiện không có; đỡ lỗi mạng nhất thời.
- [ ] **Chỉ báo loading cho từng provider** — biết nguồn nào đang chờ / đã lỗi trong multi-source search.
- [ ] **`/lang` áp cho provider khác** — hiện chỉ MangaDex; các site HTML có thể map ngôn ngữ.
- [ ] **Cải thiện quick jump** — hiện chỉ match chính xác `Number`; hỗ trợ fuzzy/prefix.

---

## Phase 3 — Tầm nhìn dài hạn (Backlog / IDEAS)

Chưa cam kết thời gian; khám phá hoặc chờ nhu cầu thực tế.

- [ ] **Offline mode / download cả chapter** — tải trước toàn bộ ảnh để đọc không mạng.
- [ ] **Extension/source plugin** — cho phép thêm nguồn không cần sửa code.
- [ ] **Themes / custom styling** — cấu hình màu sắc kitty/sixel.
- [ ] **Truyện yêu thích auto-check chapter mới** — badge "có chapter mới".
- [ ] **Config theo tuỳ chọn** — file config cho debounce time, cache size, download dir.
- [ ] **iTerm/kitty playback tối ưu** — giảm băng thông escape sequence cho màn hình lớn.

---

## Nguyên tắc quản lý quá trình

1. **Không bao giờ phá Phase 0** — change nào phải giữ regression tests xanh.
2. **Mỗi tính năng = 1 task** — vào `TASKS.md` trước khi code (chú thích reference file).
3. **Spec-first** — thay đổi hành vi user-facing phải cập nhật `SPEC.md` trước khi merge.
4. **Fix bug tối thiểu** — không refactor khi đang sửa bug (tránh dính scope creep).
5. **Ưu tiên mượt mà** — trải nghiệm reader (preload, cache, zero-wait) là giá trị cốt lõi.
6. **Bằng chứng** — mỗi task đóng với build + test + (nếu đụng UI) tự chạy kiểm tra hình ảnh.
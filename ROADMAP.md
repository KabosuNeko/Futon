# ROADMAP — Futon

Development roadmap by phase. `TASKS.md` holds the details of current work; `SPEC.md` is the canonical spec.

Legend: `[x]` = done, `[~]` = in progress, `[ ]` = backlog.

---

## Phase 0 — MVP stable (COMPLETE)

Foundation: read manga in the terminal, multi-source search, favorites/history, self-update.

- [x] **In-terminal image rendering** — Kitty (native) + Sixel (fallback), auto-detect (`FUTON_RENDERER` override).
- [x] **Multi-source search** — 5 providers (OTruyen, MangaDex, TruyenQQ, FoxTruyen, BaoTangTruyen) running concurrently, titles normalized as `(source)`.
- [x] **Navigation flow** — search → chapter list → reader, continuous chapter switching within the reader.
- [x] **Favorites** — add/remove/persist to `userdata.json`, `ctrl+f`.
- [x] **Reading history** — resume at the right chapter/page, 2s debounced flush, `/his`.
- [x] **`/src` provider checklist** — toggle, persist, real-time filtering.
- [x] **`/lang vi|en`** — MangaDex chapter language filter.
- [x] **LRU image cache (20 images)** + **chapter preload** — zero-waiting chapter switching.
- [x] **Save image** — `ctrl+d` → `~/Downloads/Futon_Downloads/`, duplicate-name handling.
- [x] **Self-update** — checks GitHub releases, installs via `install.sh`.
- [x] **Release infra** — GoReleaser (linux/darwin × amd64/arm64), `v*` tags, `install.sh`, checksums.

**Result**: first stable release shipped; auto-update works.

---

## Phase 1 — Stability & Tech debt (COMPLETE)

Clean up known weaknesses before adding new features. Goal: no more visible misbehavior or crashes on the surface.

- [x] **Tech debt #1 — Updater broken on macOS** (P1)
  - *Cause*: GoReleaser names the archive `futon_<ver>_macOS_arm64.tar.gz` but `updater.go` looks up assets with `runtime.GOOS` = `darwin` → wrong filename on macOS.
  - *Fix*: added `assetFileName(version, goos, goarch)` helper mapping `darwin`→`macOS`; used in `CheckForUpdate`.
  - *Test*: `TestAssetFileNameDarwin`, `TestAssetFileNameLinux`.
  - **Reference**: `internal/updater/updater.go`, `.goreleaser.yaml`.

- [x] **Tech debt #2 — Update banner shows wrong key** (P2)
  - `"[!] Đã có bản cập nhật..."` said "Nhấn 'U'" but the real key is `ctrl+u`.
  - *Fix*: banner changed to "Nhấn Ctrl+u"; test `TestUpdateBannerSaysCtrlU`; `SPEC.md` section 9 updated.
  - **Reference**: `internal/tui/app.go`.

- [x] **Tech debt #3 — History lost on quick exit** (P1)
  - `ctrl+c` in the reader only did `tea.Quit`, no flush; exiting before the 2s debounce lost the last page position.
  - *Fix*: `ctrl+c` in both the router `app.go` and `reader_keys.go` now runs `SaveHistoryCmd` + `FlushHistoryCmd` before `tea.Quit` (guarded when no manga/chapter is loaded).
  - *Test*: `TestCtrlCInReaderFlushesHistory`, `TestReaderKeysCtrlCFlushesHistory` (tui package).
  - **Reference**: `internal/storage/history.go`, `internal/tui/app.go`, `reader_keys.go`.

- [x] **Tech debt #4 — Missing `migrateOldData` tests** (P2)
  - Migrating the 2 old files → `userdata.json` is the most sensitive logic but had no regression tests.
  - *Fix*: `userdata_test.go` with 4 tests — merge + delete old files, skip when userdata already exists, no files at all, corrupt favorites ignored.
  - **Reference**: `internal/storage/userdata.go`.

- [x] **Tech debt #5 — `chapterNumber` stale after chapter switch in reader** (P2)
  - Switching chapters via preload / `chapterNavCmd` did not update `chapterNumber` → `ctrl+d` image saves afterward had wrong/stale chapter labels, and history saved an empty number.
  - *Fix*: `ViewChapterMsg` now carries `AllChapterNumbers`; the reader stores `allChapterNumbers`; `chapterNavCmd` and `applyPreloadedChapter` set the correct `chapterNumber` by index (nil-safe).
  - *Test*: `chapter_number_test.go` — 6 tests (cmd sets number, out-of-range, nil, preload apply, keeps old number on nil, router forwarding).
  - **Reference**: `internal/tui/app.go`, `reader_download.go`, `reader_navigation.go`, `reader_keys.go`, `chapter.go`.

- [x] **Tech debt #6 — Self-update does not verify** (P2, security)
  - Installs via `curl install.sh | bash` + `sudo`, no checksum check, no rollback.
  - *Fix*: `install.sh` downloads `checksums.txt`, verifies sha256 (`sha256sum`/`shasum` fallback) before extracting; aborts if missing/mismatched; cleans up `checksums.txt` on cleanup. `bash -n` passes.
  - **Reference**: `install.sh`.

**Result**: all 6 Phase 1 tech debts closed. `go build ./...`, `go test ./...`, `go test -race ./...` all green (storage 11 tests, tui 21 tests, updater 8 tests).

---

## Phase 2 — UX & features (NEXT)

Experience improvements that do not change the underlying architecture.

- [x] **MangaDex search limit increase** (`limit=5` → configurable/paginate) — 5 results was too few.
  - *Fix*: `Search()` paginates with an offset loop — each request `limit=100`, looping until `total` is reached (following the `FetchChapters` pattern); added `Total` to `MangaSearchResponse`; `mangadexBaseURL` became a package var for httptest testing. 4 pagination tests (multi-page, single-page, stop-at-total, HTTP error).
- [x] **Cover images & modern split UI in search** — full cover art support across all 5 providers + split-pane layout.
  - *Fix*: MangaDex eager loads cover art (`includes[]=cover_art` -> `256.jpg` CDN), OTruyen maps `thumb_url` via `APP_DOMAIN_CDN_IMAGE`, TruyenQQ/FoxTruyen/BaoTangTruyen support with Referer header; `FetchCoverBytes` helper in `provider.go`; `RenderInBox` scaling in `imgrender`; modern split-pane UI with preview pane in `search_view.go`, 150ms debounced cover loading, and LRU cover cache in `search.go`.
  - **Reference**: `internal/api/`, `internal/tui/search_view.go`, `internal/tui/search.go`, `internal/tui/imgrender/`.
- [x] **Retry / timeout for providers** — 10s timeout + 2 retries on network/5xx with incremental backoff (`provider.go`, `source.go`, all providers).
- [x] **Per-provider loading indicator** — searching shows provider names, partial failures show warning with counts (`source.go`, `search.go`, `search_view.go`).
- [ ] **`/lang` applied to other providers** — currently MangaDex only; HTML sites could map languages.
- [x] **Better quick jump** — fuzzy matching: exact → prefix → substring → title fallback (`chapter_view.go`).

---

## Phase 3 — Long-term vision (Backlog / IDEAS)

No committed timeline; to be explored or waiting for real demand.

- [ ] **Offline mode / whole-chapter download** — pre-download all images to read without a network.
- [ ] **Extension/source plugin** — add sources without touching code.
- [ ] **Themes / custom styling** — configurable kitty/sixel colors.
- [ ] **Auto-check favorites for new chapters** — "new chapter" badge.
- [ ] **User configuration** — config file for debounce time, cache size, download dir.
- [ ] **iTerm/kitty playback optimization** — reduce escape-sequence bandwidth for large screens.

---

## Process management principles

1. **Never break Phase 0** — any change must keep regression tests green.
2. **Each feature = 1 task** — goes into `TASKS.md` before coding (with file references).
3. **Spec-first** — user-facing behavior changes must update `SPEC.md` before merge.
4. **Minimal bug fixes** — no refactoring while fixing a bug (avoid scope creep).
5. **Smoothness first** — the reading experience (preload, cache, zero-wait) is core value.
6. **Evidence** — every task closes with build + test + (if UI-touching) a manual visual check.

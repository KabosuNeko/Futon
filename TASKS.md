# TASKS — Futon

Current work status. Update this file when starting/completing a task.
Legend: `[x]` = done, `[~]` = in progress, `[ ]` = pending/backlog.
Standard definition of "done": `SPEC.md` §9 — clean build, tests pass (including `-race`), regression test when TUI/storage is touched, docs updated if user-facing behavior changes.

---

## In Progress

_Phase 1 tech debt is fully resolved — the project is in a stable state. See the Phase 2 backlog below._

---

## Task backlog (by priority)

### Phase 1 — Stability & Tech debt

| ID | Task | Ref | Priority | Status |
|----|------|-----|----------|--------|
| T-101 | Fix updater wrong asset name on macOS (`darwin` vs `macOS`) | `internal/updater/updater.go`, `.goreleaser.yaml` | P1 | [x] |
| T-102 | Flush history when exiting the reader with `ctrl+c` (prevent lost last page) | `internal/storage/history.go`, `internal/tui/app.go`, `reader_keys.go` | P1 | [x] |
| T-103 | Fix update banner "Nhấn 'U'" → correct key `ctrl+u` | `internal/tui/app.go` | P2 | [x] |
| T-104 | Add regression tests for `migrateOldData` (one-time migration) | `internal/storage/userdata.go` | P2 | [x] |
| T-105 | Update `chapterNumber` when switching chapters in the reader (preload + `chapterNavCmd`) | `internal/tui/reader_navigation.go`, `reader_download.go` | P2 | [x] |
| T-106 | Verify checksum before installing self-update (optional) | `install.sh` | P2 | [x] |

### Phase 2 — UX & features

| ID | Task | Ref | Status |
|----|------|-----|--------|
| T-201 | MangaDex search: raise limit 5 → configurable/paginate | `internal/api/mangadex.go` | [x] |
| T-202 | Show cover images in search & modern split UI | `internal/tui/search_view.go`, `search.go`, `imgrender`, providers | [x] |
| T-203 | Retry/timeout for provider calls | `internal/api/provider.go`, `source.go`, `*truyen.go` | [x] |
| T-204 | Per-provider loading/error indicator in search | `internal/api/source.go` (GlobalSearchMsg), `internal/tui/search.go`, `search_view.go` | [x] |
| T-205 | Quick jump: prefix/fuzzy instead of exact match | `internal/tui/chapter_view.go` | [x] |

---

## Completed

### Phase 2 — UX & features

- [x] T-201: MangaDex search pagination — `Search()` loops by offset, each request `limit=100`, stops when `total` is reached; added `Total` to `MangaSearchResponse`; `mangadexBaseURL` became a package var for httptest testing (`internal/api/mangadex_test.go`, 4 tests).
- [x] T-202: Cover images & modern split UI — support cover URL parsing for MangaDex & OTruyen (all 5 sources now have CoverURL), `FetchCoverBytes` helper with provider-tailored referer/user-agent, `RenderInBox` scaling in `imgrender`, modern split-pane UI with preview pane in `search_view.go`, 150ms debounced cover loading, and LRU cover cache in `search.go`.
- [x] T-203: Provider retry/timeout — 10s timeout + 2 retries on network/5xx with 500ms incremental backoff; `ensureClient` helper; applied to `httpGet` + all `*Get` helpers and `New*` constructors (`provider.go`, `mangadex.go`, `otruyen.go`, `truyenqq.go`, `baotangtruyen.go`, `foxtruyen.go`).
- [x] T-204: Per-provider loading/error indicator — searching shows provider names ("Đang tìm trên X, Y..."); `GlobalSearchCmd` returns `ProviderCounts`/`ProviderErrors`; partial success shows results + warning line ("X lỗi - hiển thị N kết quả từ M/K nguồn"); full failure shows combined error.
- [x] T-205: Quick jump fuzzy — `jumpToChapter` now matches exact → prefix → substring on `Number`, then title fallback for title-only chapters (`chapter_view.go`).

### Phase 1 — Stability & Tech debt (recently completed)

- [x] T-101: `assetFileName` helper maps `darwin`→`macOS` for the updater; added darwin/linux tests.
- [x] T-102: `ctrl+c` in the reader now flushes history (save + flush) before quitting — no lost last page; fixed in both `app.go` router and `reader_keys.go`; added test `quit_flush_test.go`.
- [x] T-103: update banner shows "Nhấn Ctrl+u" instead of "Nhấn 'U'" (the real key).
- [x] T-104: regression tests for `migrateOldData` — merge, skip when userdata exists, corrupt favorites, no files (`userdata_test.go`, 4 tests).
- [x] T-105: `chapterNumber` updated when switching chapters — `ViewChapterMsg` carries `AllChapterNumbers`, `chapterNavCmd` + `applyPreloadedChapter` set the correct chapter number; added test `chapter_number_test.go`.
- [x] T-106: `install.sh` verifies sha256 checksum before extracting (downloads `checksums.txt`, matches with `sha256sum`/`shasum`, aborts on mismatch or missing file).

### Phase 0 — MVP

Condensed history per git log (feature groups already shipped):

- [x] Project scaffold, module path, GoReleaser config.
- [x] `h`/`l` reader keybindings + install script + version flag + self-update mechanism.
- [x] Fixed infinite render loop in the reader; simplified guard clauses, merged duplicate navigation logic.
- [x] Removed the `q` key; moved `d` → `ctrl+d` (avoid conflict with search input); updated footer hints.
- [x] Reuse `FlushHistory` in `DeleteHistory` (removed duplicate flush logic).
- [x] Updater: validate HTTP status, download temp file, resolve asset URL from API, semver compare.
- [x] **First stable release** — auto-updates from subsequent releases.
- [x] Added **TruyenQQ** source (rewritten for truyenqqko.com, domain fallback probe).
- [x] Added **FoxTruyen**, **BaoTangTruyen** sources.
- [x] Added `Provider` field to the `Manga` model + `GlobalSearchCmd` (search all).
- [x] Search-all mode + GlobalSearch in the TUI; `ProviderName` passed across screens.
- [x] Fix: search-all screen overflow; `j`/`k` untypeable during search.
- [x] Fix: MangaDex sent Placeholder instead of images (UA + removed Referer).
- [x] `st-graphics` support (kitty protocol), 1 footer row reserved.
- [x] Renderer interface cleanup, debounce, flush logging, bounds guard, lang handler, UA const.
- [x] `/src` provider checklist, merged `userdata.json`, real-time fav/his/src filtering.
- [x] Remapped update trigger `U` → `ctrl+u` (so uppercase U is typeable in search).
- [x] Bumped Go 1.25 → 1.26, updated deps, upgraded UA Chrome 127, `go fmt`.
- [x] Cleanup: removed redundant comments (slop), added important comments.
- [x] Dependabot + GitHub Actions updates.

---

## Operational notes

- **Before starting a new task**: read the relevant `SPEC.md` section + `AGENTS.md`, create a dedicated branch.
- **Before closing a task**: run `go build ./...`, `go test ./...`, `go test -race ./...`; if UI-touching, manually test rendering.
- **If new misbehavior is found**: add it to the Phase 1 table above first (do not fix "while working on something else").
- **If the SPEC is outdated**: update the SPEC first, then code (spec-first).

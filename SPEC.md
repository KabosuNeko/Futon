# SPEC — Futon

Terminal manga reader written in **Go** + **Bubble Tea**, rendering images directly in the terminal via **Kitty Graphics Protocol** or **Sixel**.

> This document defines Futon's product requirements and scope boundaries.
> Any behavior change must be reflected here before merge.
> See also: `AGENTS.md` (architecture), `ROADMAP.md` (roadmap), `TASKS.md` (current work).

---

## 1. Overview

Futon is a TUI (terminal user interface) application that lets users:

1. Search manga from multiple sources at once (multi-source search).
2. Browse the chapter list of a manga.
3. Read manga right in the terminal — images render inline, no external image viewer needed.
4. Manage favorites, reading history, and download images to disk.

## 2. Goals

- **Read manga in the terminal** with an experience on par with web readers: image rendering, smooth page flipping, continuous chapter switching without waiting.
- **Multi-source**: a manga can be found across multiple sources; results are merged and labeled with their source.
- **Zero-friction resume**: reopening a manga returns to the exact chapter and page you were reading.
- **Self-update**: new releases are detected and installed from within the app.
- **Vim-style navigation** for keyboard-centric users.

## 3. Non-Goals (Out of scope)

- Continuous-scroll manga reading (infinite scroll) — page-by-page reading only.
- Windows support (build targets are Linux + macOS only; Windows code merely compiles, it does not run).
- Offline library management (downloading full series, reading offline).
- Manual bulk page downloads (only 1 page per `ctrl+d`).
- User accounts, cloud sync, follow/notifications.
- Comments, ratings, in-app reviews.
- Rate limiting / retry / backoff for providers (does not exist in the current interface).

## 4. Functional Requirements

### 4.1 Search

- **FR-S1**: Typing ≥ 3 characters on the search screen triggers a search after a **300ms debounce**; fewer than 3 characters does not trigger a search.
- **FR-S2**: Search runs **concurrently across all enabled providers** (1 goroutine per provider, waits via WaitGroup).
- **FR-S3**: Each source's results carry a `Provider` and the title is rewritten to `"Title (sourcename)"` (lowercase source name).
- **FR-S4**: If one provider fails, results from the other providers are still returned (partial success); errors are joined into a single message with `"; "`.
- **FR-S5**: If **no source is enabled**, every search action shows the error "Chọn ít nhất một nguồn trong /src".
- **FR-S6**: Input starting with `/` resets previous results and does not search (reserved for slash commands).
- **FR-S7**: Stale search results are discarded if the query has changed (stale-response guard).

#### Per-provider limits
- **MangaDex**: search is **paginated** — each request uses `limit=100`, looping by offset until `total` is reached (following the `FetchChapters` pattern); no longer capped at 5 results. Title priority `en` → `ja-ro` → any language, fallback "Không rõ tiêu đề".
- **OTruyen**: returns results without cover images (empty CoverURL).
- **TruyenQQ / FoxTruyen / BaoTangTruyen**: HTML scraping, requires a browser UA (Chrome 127) due to bot detection.

### 4.2 Slash commands

- **FR-C1** `src`: opens the source checklist; `space` toggles each provider on/off; state is **persisted to `userdata.json` immediately**; `esc` closes it.
- **FR-C2** `fav`: opens the favorites list (loaded async); `enter` opens a manga; `ctrl+d` removes a favorite.
- **FR-C3** `his`: opens reading history (loaded async); `enter` opens a manga resuming at the right chapter; `ctrl+d` removes an entry.
- **FR-C4** `lang vi|en`: sets the chapter language for **MangaDex** (default `vi`); invalid syntax shows "Dùng: /lang vi hoặc /lang en".
- **FR-C5**: In `/fav`, `/his`, or `/src` mode, typing filters the list in real time (case-insensitive, substring) by Title (fav), MangaTitle/ID (his), source name (src).

### 4.3 Favorites

- **FR-F1**: `ctrl+f` on the chapter list adds a manga to favorites; shows flash "Đã thêm ... vào Yêu thích" (2s).
- **FR-F2**: Deduped by MangaID — adding a duplicate is a no-op.
- **FR-F3**: Persisted to `~/.config/futon/userdata.json` (`favorites` field).

### 4.4 Reading history

- **FR-H1**: Every page flip (`→`/`l`, `←`/`h`) writes history to the RAM cache (does not block the UI).
- **FR-H2**: Disk write is **debounced 2s** from the last save (timer resets on every save).
- **FR-H3**: `esc` in the reader: flushes history immediately (does not wait for debounce).
- **FR-H4**: Removing an entry (`ctrl+d` in `/his`) flushes synchronously right away.
- **FR-H8**: `ctrl+c` in the reader (or from the router while in the reader): saves + flushes history before quitting (prevents losing the last page position when exiting before the 2s debounce).
- **FR-H5**: The chapter list restores the cursor to the last-read chapter (`restoreHistoryPosition`).
- **FR-H6**: The reader resumes at the correct page when opening a chapter that matches history (via `StartPageIndex=-1`).
- **FR-H7**: History keeps the old Title/ChapterNumber if the new write does not provide them (prevents information loss).

### 4.5 Chapter list

- **FR-L1**: Shows the manga's chapter list ordered from chapter 1 (HTML sources reverse newest-first back to ascending).
- **FR-L2**: **Quick jump** — type a number + `enter` to jump to that chapter (matched by `Number`); no match → flash "Không tìm thấy chapter X".
- **FR-L3**: `esc` while typing a number clears the input buffer; `esc` with no input returns to search.
- **FR-L4**: HTML providers returning chapters **without a Number** fall back to displaying the Title.
- **FR-L5**: MangaDex chapter feed is filtered by the configured language (`/lang`), auto-paginated 500 chapters per request.

### 4.6 Reader

- **FR-R1**: Page navigation: `→`/`l` next page, `←`/`h` previous page. **Only active in the reader** (not swallowed by other screens).
- **FR-R2**: **Continuous chapter switching**: on the last page, `→` goes to the next chapter; on the first page, `←` goes to the previous chapter — no need to return to the list.
- **FR-R3**: Chapter preload: reaching page `total-3` automatically fetches the next chapter + **preloads the first 2 images**; if preload finished, switching chapters **does not refetch and does not wait** (zero waiting).
- **FR-R4**: LRU image cache of **20 rendered images** (key = page index), evicts the oldest page; reset when entering a new chapter. Pre-renders at most 3 pages ahead that are not yet cached.
- **FR-R5**: Download the current page with `ctrl+d` → `~/Downloads/Futon_Downloads/`, filename `{Title}_Ch{ChapterNumber}_Pg{page}.jpg`, sanitizes special characters, handles name collisions with `_1`...`_999` suffixes.
- **FR-R6**: Chapter image downloads: **max 4 concurrent**, prioritizing the current page → pages ahead → pages behind.
- **FR-R7**: `esc` = save history + flush + clear screen + return to chapter list (order guaranteed via `tea.Sequence`).
- **FR-R8**: Terminal resize → re-render the current page.

### 4.7 Image rendering

- **FR-G1**: Auto-detect renderer: `FUTON_RENDERER` env override (`kitty`/`sixel`) > Kitty (`TERM=xterm-kitty`, `KITTY_WINDOW_ID`, `TERM` prefix `st-`) > Sixel (fallback).
- **FR-G2**: Images fit the terminal size, **leaving the last row** for the footer/flash, preserving aspect ratio (Lanczos3 resize).
- **FR-G3**: Uses ANSI cursor positioning + screen clear (`\x1b[H\x1b[2J`); **no `\n` scrolling** in the reader.
- **FR-G4**: Kitty protocol: PNG → base64 → 2048-byte chunks.

### 4.8 Self-update

- **FR-U1**: On startup, checks GitHub `releases/latest` (5s timeout) in the background; `dev` version (built from source) **does not check**.
- **FR-U2**: If a new version exists → banner `"[!] Đã có bản cập nhật {version}. Nhấn Ctrl+u để tự động cài đặt."` (actual key: `ctrl+u`).
- **FR-U3**: `ctrl+u` → runs `install.sh` from the GitHub main branch (via `curl|bash`, includes `sudo`), shows "Đang cập nhật..."; then forces exit to apply.
- **FR-U4**: Version comparison is semver component-wise (strips the `v` prefix).

### 4.9 Keybinding summary

| Screen | Key | Action |
|---|---|---|
| Global | `ctrl+c` | Quit app |
| Global | `ctrl+u` | Install update (only when available, on search) |
| Search | `enter` | Search / open selected item / run slash command |
| Search | `↑`/`↓` | Move through list |
| Search | `/fav`, `/his`, `/src`, `/lang vi\|en` | Open feature |
| Fav/His | `enter` | Open manga |
| Fav/His | `ctrl+d` | Remove from list |
| Fav/His/Src | `esc` | Back to search |
| Chapter | `ctrl+f` | Add to favorites |
| Chapter | `[number]`+`enter` | Jump to chapter |
| Reader | `→`/`l`, `←`/`h` | Next / previous page |
| Reader | `ctrl+d` | Save current page image |
| Reader | `ctrl+c` | Save + flush history then quit |

## 5. Non-Functional Requirements

- **NFR-1**: The terminal must support Kitty Graphics Protocol or Sixel (see the terminal compatibility table in `README.md`).
- **NFR-2**: The UI must not block — all network and disk I/O lives in `tea.Cmd` / goroutines, never blocking `Update`.
- **NFR-3**: Key input must respond instantly; network search must not freeze the interface.
- **NFR-4**: Local data safety — write files with 0644, do not overwrite unnecessarily.
- **NFR-5**: Cross-compilation without CGO (`CGO_ENABLED=0`).
- **NFR-6**: Exiting with `ctrl+c` in the reader flushes history immediately (synchronous save + flush) before quitting — no lost last-page position.

## 6. Data & Storage

| What | Path | Format |
|---|---|---|
| Favorites + source toggles | `~/.config/futon/userdata.json` | JSON: `sources[]`, `favorites[]` |
| Reading history | `~/.config/futon/history.json` | JSON: map by MangaID |
| Downloaded images | `~/Downloads/Futon_Downloads/` | JPG |

- **Migration**: on first run after upgrade, if `userdata.json` does not exist, old `favorites.json` + `sources.json` are auto-merged into `userdata.json` and the 2 old files are deleted (one-time).
- Source toggles are stored by **provider name**; all enabled by default.

## 7. Platform

- **Build**: Go 1.26, `go build ./...`, `go test ./...`, `go test -race ./...`.
- **Release**: GoReleaser — Linux + macOS × amd64 + arm64; `v*` tags; version injected into the binary (`main.Version`).
- **Supported terminals**: Kitty, WezTerm, Ghostty, foot, iTerm2, Konsole, mlterm, XTerm (sixel).

## 8. Architecture Constraints

- `internal/api`: `MangaProvider` interface (`Name`, `Search`, `FetchChapters`, `FetchPages`) + `tea.Cmd` wrappers. **No** context/retry/rate-limit at the interface.
- `internal/tui`: Bubble Tea, `AppModel` router (search → chapters → reader), navigation messages `ViewMangaMsg` / `ViewChapterMsg` / `BackToSearchMsg` / `BackToChaptersMsg`.
- `internal/storage`: all persistence goes through `userdata.go` (merged) or `history.go` (debounced).
- Manga ID is an **opaque string** — slug (OTruyen), UUID (MangaDex), URL (HTML providers) — its format must not be assumed.

## 9. Definition of Done (for every change)

1. Clean build: `go build ./...`.
2. Tests pass: `go test ./...` and `go test -race ./...`.
3. Regression test for any changed TUI/storage behavior.
4. `SPEC.md` / `README.md` updated if user-facing behavior changes.
5. No type-error suppression; no removed tests.

# SPEC — Futon

Terminal manga reader written in **Go** + **Bubble Tea**, rendering images directly in the terminal via **Kitty Graphics Protocol** or **Sixel**.

> This document defines Futon's product requirements and scope boundaries.
> Any behavior change must be reflected here before merge.
> See also: `AGENTS.md` (architecture).

---

## 1. Overview

Futon is a terminal manga reader: multi-source search, chapter browsing, inline image reading via Kitty or Sixel, favorites/history, and page download. Usage lives in `README.md`.

## 2. Goals

- **Read manga in the terminal** with an experience on par with web readers: image rendering, smooth page flipping, continuous chapter switching without waiting.
- **Multi-source**: a manga can be found across multiple sources; results are merged and labeled with their source.
- **Zero-friction resume**: reopening a manga returns to the exact chapter and page you were reading.
- **Self-update**: new releases are detected and installed from within the app.
- **Vim-style navigation** for keyboard-centric users.

## 3. Non-Goals (Out of scope)

- Continuous-scroll manga reading (infinite scroll) — page-by-page reading only.
- Windows support (build targets are Linux + macOS only; Windows code merely compiles, it does not run).
- Offline library management (reading downloaded chapters inside Futon; exported CBZ files are read with other apps).
- Manual bulk page downloads (only 1 page per `ctrl+d`; CBZ export is per chapter or whole series, not arbitrary page selection).
- User accounts, cloud sync, follow/notifications.
- Comments, ratings, in-app reviews.

## 4. Functional Requirements

### 4.1 Search

- **FR-S1**: Typing ≥ 3 characters on the search screen triggers a search after a **300ms debounce**; fewer than 3 characters does not trigger a search.
- **FR-S2**: Search runs **concurrently across all enabled providers** (1 goroutine per provider); each source's results stream back as soon as it finishes, and a final snapshot marks completion. Starting a new search/feed/filter request **cancels** the in-flight one (its HTTP requests and retries stop).
- **FR-S3**: Each source's results carry a `Provider` and the title is rewritten to `"Title (sourcename)"` (lowercase source name).
- **FR-S4**: If one provider fails, results from the other providers are still returned (partial success); errors are joined into a single message with `"; "` and a per-provider warning line shows which sources failed while still displaying successful results.
- **FR-S5**: If **no source is enabled**, every search action shows the error "Chọn ít nhất một nguồn trong /src".
- **FR-S6**: Input starting with `/` resets previous results and does not search (reserved for slash commands).
- **FR-S7**: Stale search results are discarded if the query has changed (stale-response guard).
- **FR-S8**: Until the first results arrive, searches show the active providers (e.g. "Đang tìm trên MangaDex, TruyenQQ...") and the initial feed shows its loading text; partial results appear as each source completes; after completion a subtle warning lists the failed providers plus a healthy/total source count.
- **FR-S9**: Provider HTTP calls have a **10s timeout** and retry up to **2 times** on network errors or 5xx responses with incremental backoff.
- **FR-S10**: The latest feed and filter results are **paged**: moving the cursor onto the last item loads the next page and appends it below (titles already listed are skipped); an empty page ends paging. Search results are not paged.
- **FR-S12**: The result list shows **one row per title**: results whose titles match after normalization (source suffix removed, case folded, punctuation and spacing ignored) are grouped and list all their sources, the picked one first. `shift+←`/`shift+→` change the picked source (plain `←`/`→` keep editing the query); the preview shows it ("1/2"), `enter` opens it, and the choice survives more results streaming in. Untitled entries are never grouped.
- **FR-S11**: Provider errors are never masked: a failing filter request surfaces as that provider's error instead of silently falling back to the latest list, and a latest-updates page that parses to nothing is an error (the site markup changed).

#### Per-provider limits
- **MangaDex**: search is **paginated** — `limit=100` per request, at most **3 pages (300 results)**; a broad query would otherwise walk thousands of titles, trip the rate limit, and hit the API's `offset+limit ≤ 10000` cap. If a later page fails, the pages already fetched are kept. Search, latest and filter lists only include titles with chapters in the `/lang` language (`availableTranslatedLanguage[]`). Title priority `vi` → `en` → `ja-ro` → any language, fallback "Không rõ tiêu đề".
- **TruyenQQ / BaoTangTruyen / CuuTruyen**: HTML scraping, requires a browser UA (Chrome 127) due to bot detection.
- **TruyenQQ**: primary `truyenqqko.com`, mirror `truyenggvn.com` (TruyenGG: same catalog and paths), probed once on first use. Manga and chapter IDs are **paths without host or `.html`**, so they stay valid on either mirror and search/list results of the same manga share one ID; image downloads send the chapter's absolute URL on the current mirror as `Referer` (the CDN returns 403 without it).
- **CuuTruyen**: `cuutruyen.moe` (a copy of CuuTruyen with unscrambled, lightly compressed images, ~1024px). If the site shows its Livewire password gate, the provider submits the default password `5` and retries; a session cookie keeps it unlocked. IDs are paths; chapters are listed newest-first on the site and reversed.

#### Filter mapping
- Genre slugs per provider are kept in one table per provider, in the order of `api.FilterGenres`; a test enforces equal length.
- **TruyenQQ**: with a genre, `/the-loai/{slug}[/trang-N]` with `status` (0 ongoing, 2 completed) and `sort` (2 updated, 4 most viewed); without a genre, completed → `/truyen-hoan-thanh`, popular → `/top-thang`, rating → `/truyen-yeu-thich`, else latest. Paths never end in `.html` (TruyenQQ's `.html` redirect drops the query; the mirror accepts both).
- **CuuTruyen**: search uses `/tim-kiem?filter[name]=` (the site's `keyword` parameter returns HTTP 500). `/tim-kiem` combines `filter[accept_genres]`, `filter[status]` (site codes: 2 ongoing, 1 completed) and `sort` (`-updated_at`, `-views` for popular and rating, `name` for A–Z).
- A latest/filter page past the end returns an empty list, not an error; only an empty **first** latest page is an error.
- **BaoTangTruyen**: `/tim-truyen[/{slug}]?status=&sort=&page=` combines genre, status and sort (popular → top all-time, rating → most liked; no alphabetical sort).
- **Merged sources**: TruyenGG (and the earlier FoxTruyen) are folded into TruyenQQ: saved source toggles map to TruyenQQ, and on launch TruyenGG favorites/history are re-labelled TruyenQQ with their IDs converted to paths (idempotent; when two history entries collide the most recent wins). FoxTruyen entries are left as they are (different site).
- **Removed sources**: OTruyen was removed (its API returns 404 everywhere). Opening a favorite/history entry from a source that no longer exists shows "Nguồn X không còn được hỗ trợ." instead of opening it with another provider; entries without a provider still use the first source. A saved source list in which no source exists any more enables all sources.

### 4.2 Slash commands

- **FR-C1** `src`: opens the source checklist; `space` toggles each provider on/off; state is **persisted to `userdata.json` immediately** (in a background command; rapid toggles are written in order); `esc` closes it and reloads the feed with the new sources.
- **FR-C2** `fav`: opens the favorites list (loaded async); `enter` opens a manga; `ctrl+d` removes a favorite.
- **FR-C3** `his`: opens reading history (loaded async); `enter` loads the chapter list and goes **straight into the reader** at the saved chapter and page (staying on the list if that chapter no longer exists; `esc` from the reader returns to the list); `ctrl+d` removes an entry.
- **FR-C4** `lang vi|en`: sets the chapter language for **MangaDex** (default `vi`), persisted to `userdata.json` (`chapter_lang`) and restored on launch; invalid syntax shows "Dùng: /lang vi hoặc /lang en".
- **FR-C5**: In `/fav`, `/his`, or `/src` mode, typing filters the list in real time (case-insensitive, substring) by Title (fav), MangaTitle/ID (his), source name (src).

### 4.3 Favorites

- **FR-F1**: `ctrl+f` on the chapter list adds a manga to favorites; shows flash "Đã thêm ... vào Yêu thích" (2s).
- **FR-F2**: Deduped by (Provider, MangaID) — adding a duplicate is a no-op; the same ID on two providers is two favorites.
- **FR-F3**: Persisted to `~/.config/futon/userdata.json` (`favorites` field).

### 4.4 Reading history

- **FR-H1**: Every page flip (`→`/`l`, `←`/`h`) writes history to the RAM cache (does not block the UI).
- **FR-H2**: Disk write is **debounced 2s** from the last save (timer resets on every save).
- **FR-H3**: `esc` in the reader: flushes history immediately (does not wait for debounce).
- **FR-H4**: Removing an entry (`ctrl+d` in `/his`) flushes synchronously right away.
- **FR-H8**: `ctrl+c` in the reader (or from the router while in the reader): saves + flushes history before quitting (prevents losing the last page position when exiting before the 2s debounce). Every other exit also flushes once the program returns: quitting from another screen, `SIGTERM`, and `SIGHUP` (closing the terminal window), which is caught to quit cleanly.
- **FR-H10**: Several Futon windows share one history file: each flush re-reads it and keeps entries written by the other windows (the newer `updated_at` wins per manga); entries deleted or re-keyed in this window are not brought back. Flushes are serialized, and files are synced to disk before the atomic rename.
- **FR-H5**: The chapter list restores the cursor to the last-read chapter (`restoreHistoryPosition`).
- **FR-H6**: The reader resumes at the correct page when opening a chapter that matches history (via `StartPageIndex=-1`).
- **FR-H7**: History keeps the old Title/ChapterNumber if the new write does not provide them (prevents information loss).
- **FR-H9**: History is keyed by (Provider, MangaID). Entries written before the provider was recorded are still found by MangaID and are re-keyed on the next save.

### 4.5 Chapter list

- **FR-L1**: Shows the manga's chapter list ordered from chapter 1 (HTML sources reverse newest-first back to ascending).
- **FR-L2**: **Quick jump** — type a number + `enter` to jump to that chapter; matching is fuzzy in priority: exact `Number` → prefix `Number` → substring `Number` → title contains query; no match → flash "Không tìm thấy chapter X".
- **FR-L3**: `esc` while typing a number clears the input buffer; `esc` with no input returns to search.
- **FR-L4**: HTML providers returning chapters **without a Number** fall back to displaying the Title; that title is also the chapter label passed to the reader, history and file names.
- **FR-L5**: MangaDex chapter feed is filtered by the configured language (`/lang`), auto-paginated 500 chapters per request. External chapters (hosted on another site, no pages) are excluded, and when several groups upload the same chapter number only the first is listed.

### 4.6 Reader

- **FR-R1**: Page navigation: `→`/`l` next page, `←`/`h` previous page. **Only active in the reader** (not swallowed by other screens).
- **FR-R2**: **Continuous chapter switching**: on the last page, `→` goes to the next chapter; on the first page, `←` goes to the previous chapter — no need to return to the list.
- **FR-R10**: **Fast chapter jump**: `j` (and `↓` outside webtoon mode) switches to the next chapter from any page; `k` (and `↑`) to the previous one. Both open the target chapter at its first page and use the preloaded next chapter when available.
- **FR-R12**: **Webtoon mode** (`w` toggles, remembered in `userdata.json` as `reader_fit_width` and kept across chapters): pages are scaled to the terminal width, never beyond their native width, and shown through a window of terminal height. `↓`/`space`/wheel-down scroll half a screen; past the bottom they go to the next page (at its top); `↑`/wheel-up scroll back, and above the top go to the previous page **at its bottom**. `←`/`→` still change pages and `j`/`k` chapters. Each page is decoded and scaled once; a scroll only crops and encodes the visible window, and the previous window stays on screen until the new one is ready. The footer shows how far down the page the window is.
- **FR-R13**: The footer never wraps: when the terminal is too narrow, the least important hints are dropped whole (page number, navigation and `esc` are kept). Flash messages are truncated to one row.
- **FR-R3**: Chapter preload: being within 3 pages of the end (0-based index `>= total-3`, checked on every page change and when a chapter opens, so resumes and short chapters count) automatically fetches the next chapter + **preloads the first 2 images**; if preload finished, switching chapters **does not refetch and does not wait** (zero waiting).
- **FR-R4**: LRU image cache of **20 rendered images** (key = page index), evicts the oldest page; reset when entering a new chapter. Pre-renders at most 3 pages ahead that are not yet cached.
- **FR-R5**: Download the current page with `ctrl+d` → `~/Downloads/Futon/`, filename `{Title}_{Chapter}_Pg{page}.{ext}` where `ext` matches the image data (jpg/png/webp/gif), sanitizes special characters, handles name collisions with `_1`...`_999` suffixes.
- **FR-R6**: Chapter image downloads: **max 4 concurrent**, prioritizing the current page → pages ahead → pages behind. Each image retries up to **3 attempts** on network errors, 5xx and 429.
- **FR-R11**: A page that fails to download or decode does **not** end the chapter: that page shows "Lỗi trang N: … — [r] Thử lại", other pages and navigation keep working, and `r` downloads it again.
- **FR-R7**: `esc` = save history + flush + clear screen + return to chapter list (order guaranteed via `tea.Sequence`).
- **FR-R8**: Terminal resize → re-render the current page.
- **FR-R9**: `ctrl+h` toggles the reader footer/hint row. Hidden, the page is re-rendered to use the freed row; rendered cache is cleared so pages are not reused at the old size. The choice is kept across chapters and remembered in `userdata.json` (`reader_hide_footer`), like webtoon mode.

### 4.7 Image rendering

- **FR-G1**: Auto-detect renderer: `FUTON_RENDERER` env override (`kitty`/`sixel`) > Kitty (`TERM=xterm-kitty`, `KITTY_WINDOW_ID`, Ghostty via `TERM=xterm-ghostty` or `TERM_PROGRAM=ghostty`, WezTerm via `TERM_PROGRAM=WezTerm`) > Sixel (fallback).
- **FR-G2**: Images fit the terminal size, **leaving the last row** for the footer/flash, preserving aspect ratio (Catmull-Rom resize via `golang.org/x/image/draw`).
- **FR-G3**: Uses ANSI cursor positioning + screen clear (`\x1b[H\x1b[2J`); **no `\n` scrolling** in the reader.
- **FR-G4**: Kitty protocol: PNG (fastest compression level) → base64 → 2048-byte chunks.

### 4.8 Self-update

- **FR-U1**: On startup, checks GitHub `releases/latest` (5s timeout) in the background; `dev` version (built from source) **does not check**.
- **FR-U2**: If a new version exists → banner `"[!] Đã có bản cập nhật {version}. Thoát rồi chạy \`futon update\` để cài."`.
- **FR-U3**: Installing happens **only** from the CLI: `futon update` (also `futon --update`, `futon self-update`) runs outside the TUI, prints status and exits. It runs `install.sh` from the **release tag being installed** (`main` for dev builds), downloaded with `curl -f` to a `mktemp` file so an HTTP error aborts instead of running the error page. The TUI never runs the installer: it may prompt for `sudo`, which does not belong inside a full-screen app.
- **FR-U4**: `/update` in search only checks: outdated → systemMsg "Có bản {version}. Thoát rồi chạy \`futon update\` để cài."; up-to-date → "Đã là bản mới nhất."; failure → the error; dev builds → a hint to run `futon update`.
- **FR-U5**: Version comparison is semver component-wise (strips the `v` prefix).

### 4.9 Keybinding summary

The keybindings table in `README.md` is the user-facing source of truth; FR-R1/FR-R12/FR-H8 above are the normative requirements.
`tab` cycles search screens forward and `shift+tab` cycles them backward.

### 4.10 CBZ export

- **FR-E1**: `ctrl+e` in the chapter list or reader exports the focused/current chapter to `~/Downloads/Futon/{Title}_{Chapter}.cbz`; when a provider omits chapter numbers (HTML sources), the chapter title is used instead. `{Chapter}` is `Chap_{N}` for a bare chapter number and the chapter label as is otherwise ("Chương 12"), so names never repeat the word. When the chapter label starts with the manga title (some sources repeat it), that prefix is dropped. Names are kept under 200 bytes (file systems allow 255; Vietnamese letters take 2-3 bytes) by shortening the title, never the chapter part, so chapters cannot collide. Messages show paths with `~` for the home directory.
- **FR-E2**: Each CBZ contains `ComicInfo.xml` (original series/chapter titles, XML-escaped; only the file name is sanitized) and zero-padded page entries named `001.<ext>`, `002.<ext>`, ...
- **FR-E3**: A CBZ is only written when **every page** downloaded (pages download 4 at a time, with the FR-R6 retries); otherwise the export fails naming the page. The archive is written to a temp file and renamed, so a failed export leaves no partial `.cbz`.
- **FR-E4**: `ctrl+a` in the chapter list exports every chapter into `~/Downloads/Futon/{Title}/`, one CBZ per chapter, showing `{done}/{total}` progress as each chapter finishes; `esc` cancels it (after the chapter in progress). Chapters that share a label (e.g. MangaDex oneshots with neither number nor title) get `_2`, `_3`... so none overwrites another. Chapters that fail are skipped and named in the final flash with `{success}/{total}` (the first 5, then `+N`). Progress keeps updating while a chapter is being read.

## 5. Non-Functional Requirements

- **NFR-1**: The terminal must support Kitty Graphics Protocol or Sixel (see the terminal compatibility table in `README.md`).
- **NFR-2**: The UI must not block — all network and disk I/O lives in `tea.Cmd` / goroutines, never blocking `Update`.
- **NFR-3**: Key input must respond instantly; network search must not freeze the interface.
- **NFR-4**: Local data safety — write files with 0644, do not overwrite unnecessarily.
- **NFR-5**: Cross-compilation without CGO (`CGO_ENABLED=0`).
- **NFR-6**: Text from sources (titles, authors, genres, chapter names) has control characters removed before use, and the reader strips them again before writing raw to the terminal, so a crafted title cannot send terminal escape sequences (clipboard writes, screen changes). Images from sources are capped at 32 MB downloaded and 50 megapixels decoded, so a crafted page (a decompression bomb) fails like a broken page instead of exhausting memory.

## 6. Data & Storage

| What | Path | Format |
|---|---|---|
| Favorites + source toggles + chapter language | `~/.config/futon/userdata.json` | JSON: `sources[]`, `favorites[]`, `chapter_lang` |
| Reading history | `~/.config/futon/history.json` | JSON array of records, newest first (`provider` + `manga_id` key in memory) |
| Downloaded images | `~/Downloads/Futon/` | Original format (JPG/PNG/WebP/GIF) |

- **Migration**: on first run after upgrade, if `userdata.json` does not exist, old `favorites.json` + `sources.json` are auto-merged into `userdata.json` and the 2 old files are deleted (one-time).
- Source toggles are stored by **provider name**; all enabled by default.
- **Locations**: the config folder is `$XDG_CONFIG_HOME/futon` when that variable is set, unless `~/.config/futon` already exists (existing data is never orphaned). Downloads go to `{Downloads}/Futon`, where `{Downloads}` is `XDG_DOWNLOAD_DIR` (environment or `user-dirs.dirs`, e.g. `~/Tải xuống` on a Vietnamese desktop), else `~/Downloads`.

## 7. Platform

- **Language**: Go ≥ 1.26 (minimum in `go.mod`; releases are built with the latest stable Go); build/test commands live in `AGENTS.md`.
- **Release**: GoReleaser — Linux + macOS × amd64 + arm64; `v*` tags; version injected into the binary (`main.Version`).
- **Supported terminals**: Kitty, WezTerm, Ghostty, foot, iTerm2, Konsole, mlterm, XTerm (sixel).

## 8. Architecture Constraints

- Manga ID is an **opaque string** — UUID (MangaDex), URL path (HTML providers) — its format must not be assumed.
- Code structure, provider interface, and conventions live in `AGENTS.md`.

## 9. Definition of Done (for every change)

1. Clean build: `go build ./...`.
2. Tests pass: `go test ./...` and `go test -race ./...`. CI (`.github/workflows/ci.yml`) runs gofmt, `go vet`, `go test -race` and Linux/macOS builds on every push and PR; the release workflow runs vet + tests before GoReleaser, so a failing tag publishes nothing.
3. Regression test for any changed TUI/storage behavior.
4. `SPEC.md` / `README.md` updated if user-facing behavior changes.
5. No type-error suppression; no removed tests.

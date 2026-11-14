# AGENTS.md — Futon

Compact guidance for working in this repository.

## Documentation

Read only what the task needs; do not load docs speculatively.

- `SPEC.md` — product requirements, scope boundaries, keybindings, Definition of Done. Change behavior → update SPEC first (spec-first).
- `README.md` — user-facing usage, install, keybindings, terminal compatibility.

## Project

Futon is a terminal manga reader. It searches multiple manga sources, lists chapters, and renders pages inline using Kitty graphics or Sixel.

- Language: Go ≥ 1.26 (go.mod minimum; CI tests 1.26 and latest stable, releases build with stable)
- Module: `github.com/KabosuNeko/Futon`
- Entrypoint: `cmd/main.go` → `internal/tui.NewAppModel()`
- Build: `go build ./...`
- Run: `go run ./cmd/...`
- Test: `go test ./...`

## Architecture

```
cmd/main.go          # entrypoint: starts the Bubble Tea program
internal/
  models/            # domain structs only: Manga, Chapter
  api/               # manga sources; no UI dependencies
    provider.go      # MangaProvider interface, FilterOptions, FilterGenres, shared parsing helpers
    http.go          # httpGet/httpDo: timeout, retry with backoff, context cancellation
    aggregate.go     # SearchAll/LatestAll/FilterAll: fan out over providers, stream Snapshots on a channel
    images.go        # FetchCover, LookupCover, DownloadPage, DownloadChapterPages; per-provider image headers
    mangadex.go      # MangaDex provider + its API response types
    truyenqq.go      # TruyenQQ provider (TruyenGG is its fallback mirror)
    baotangtruyen.go # BaoTangTruyen provider
    cuutruyen.go     # CuuTruyen (cuutruyen.moe) provider, incl. password gate
  storage/           # local JSON persistence; no UI dependencies
    userdata.go      # favorites, source toggles, /lang; migrateOldData
    history.go       # per-manga reading history with debounced flush
    migrate.go       # MigrateEntries: rename merged sources / rewrite saved IDs
  export/            # CBZ packaging only (no network): ComicInfo.xml, filenames, image format
    cbz.go
  updater/           # GitHub release check + install.sh invocation
    updater.go
  tui/               # Bubble Tea screens; the only package that knows about tea.Cmd/tea.Msg
    app.go           # router: search → chapters → reader; source merges/migration at startup
    provider_cmds.go # tea wrappers around api: result stream (resultsMsg), chapters, pages
    storage_cmds.go  # tea wrappers around storage (history save/flush/delete, favorites/history loads)
    search.go        # SearchModel, constructor, Init, Update dispatch, list filtering
    search_keys.go   # keyboard + mouse input, tab switching (switchView), slash commands
    search_results.go # latest/filter/search requests, paging, merge, debounce
    search_sources.go # source toggles and their persistence
    search_cover.go  # preview cover: cache, debounce, fetch, out-of-band paint
    search_layout.go # pane sizes, viewport, list rows for mouse hit-testing
    search_view.go   # search screen rendering
    chapter.go       # ChapterListModel, Init, Update
    chapter_export.go # ctrl+e chapter / ctrl+a whole-series CBZ export (streamed progress, cancel)
    chapter_view.go  # chapter list view, viewport, history restore, quick jump
    reader.go        # ReaderModel, New, Init, Update dispatcher
    reader_keys.go   # reader key handling (navigation, retry, save, export, quit)
    reader_msgs.go   # reader message handling (images, downloads, render, preload)
    reader_cache.go  # LRU image cache helpers
    reader_navigation.go  # chapter/page navigation and preload state helpers
    reader_view.go   # reader view, image layout and the width-fitting footer
    reader_webtoon.go # webtoon (fit-width) mode: scaled page, scroll window
    reader_download.go    # image download/render/save commands
    flash.go         # flash message helper
    *_test.go        # tests, named after the file they cover
    imgrender/       # Kitty / Sixel renderer selection
```

Dependency direction: `tui` → `api`, `storage`, `export`, `updater`, `imgrender`; `api` → `models`. Keep `api`, `storage` and `export` free of Bubble Tea: they return plain data (or a channel), and `tui` wraps them in commands.

Navigation between screens uses custom `tea.Msg` types defined in `internal/tui/app.go`: `ViewMangaMsg`, `ViewChapterMsg`, `BackToSearchMsg`, `BackToChaptersMsg`.

## Providers

- `/src` opens a checklist of providers with `[X]`/`[ ]` toggles.
- Arrow keys move cursor, `space` toggles the focused provider, `esc` closes the list.
- All providers are checked by default (searches all sources via `api.SearchAll`).
- Only checked providers are searched; if none are checked, an error is shown.
- Toggle state is persisted to `userdata.json` and restored on next launch.
- Typing in `/src` mode filters the provider list (case-insensitive substring).
- Provider interface: `Name`, `Search`, `FetchLatest`, `Filter`, `FetchChapters`, `FetchPages`; every method takes a `context.Context` first.
- `fanOut` in `aggregate.go` uses `sync.WaitGroup` + `sync.Mutex` for concurrent requests; the search screen cancels the previous request's context when it starts a new one.
- Genre slugs live in one table per provider, ordered like `api.FilterGenres` (test-enforced). Never fall back to another list on a provider error; return the error.
- Titles are standardized to `Name (source)` format in all search modes.
- Text from sources is untrusted: `api.CleanText` strips control characters where it enters (`fanOut` for manga lists, `fetchChaptersCmd` for chapters) and where the reader writes raw to the terminal (flash, centered messages). A title must never reach `tea.Raw` unfiltered: an ESC in it becomes a terminal command (e.g. OSC 52 rewrites the clipboard).

## Runtime State

- Favorites + source toggles + `/lang`: `~/.config/futon/userdata.json` (read-modify-write guarded by a mutex)
- Reading history (last chapter + page per manga, keyed by provider + manga ID): `~/.config/futon/history.json`
- Downloaded images and CBZ: `{Downloads}/Futon/` (`XDG_DOWNLOAD_DIR` or `user-dirs.dirs`, else `~/Downloads`); the config folder honours `XDG_CONFIG_HOME` unless `~/.config/futon` already exists. Tests isolate both through `HOME` and unset the XDG variables in `TestMain`.
- History flushes are serialized and merge entries written by other Futon windows; `cmd/main.go` flushes after the program exits (also on `SIGTERM`/`SIGHUP`).
- File names come from `export.FileStem` (title prefix dropped from chapter labels, kept under 200 bytes).
- Old `favorites.json` and `sources.json` are auto-migrated to `userdata.json` on first launch.

## Image Rendering

`imgrender.New()` auto-detects the terminal:

- `FUTON_RENDERER=kitty|sixel` overrides detection.
- Kitty protocol for kitty (`TERM=xterm-kitty` or `KITTY_WINDOW_ID`), Ghostty (`TERM=xterm-ghostty` or `TERM_PROGRAM=ghostty`) and WezTerm (`TERM_PROGRAM=WezTerm`).
- Sixel otherwise.

Every Kitty graphics command carries `q=2` so the terminal sends no replies into the input stream. The reader uses ANSI cursor positioning and explicit clear sequences (`\x1b_Ga=d,q=2;\x1b\\` and `\x1b[H\x1b[2J`). Avoid introducing normal `\n` scrolling in reader output.

## Image Cache

`ReaderModel` keeps an in-memory LRU cache of rendered images (`imageCache`) keyed by page index, capped at 20 entries. It is reset on every new chapter to avoid memory leaks.

## External APIs

- MangaDex: `api.mangadex.org`
- TruyenQQ: `truyenqqko.com` (fallback mirror `truyenggvn.com`, a.k.a. TruyenGG)
- BaoTangTruyen: `baotangtruyen.vip`
- CuuTruyen: `cuutruyen.moe`
- API requests default to `User-Agent: Futon-App/1.0`; the HTML providers send a full browser UA to bypass bot detection.

## Image Downloads

- MangaDex image requests use `User-Agent: Futon-App/1.0 (https://github.com/KabosuNeko/Futon)` and **no** `Referer` header to avoid placeholder responses.
- Providers declare image headers by implementing the optional `coverReferer`, `pageReferer(chapterID)` and `imageUserAgent` methods (see `images.go`); never switch on provider names. They also implement `lookupCover(ctx, mangaID)`, which finds the cover of a favorite/history entry saved without one (`api.LookupCover`). Chapter IDs may be paths, so the page Referer is built from the current mirror.
- Reader downloads and CBZ export share `api.DownloadPage` / `api.DownloadChapterPages`, so both paths send the same headers and the same retry policy (3 attempts on network errors, 5xx, 429).

## Concurrency

- Keep UI concurrency inside `tea.Cmd` functions; do not block `Update`.
- History writes are debounced (2s) and flushed via `tea.Cmd`; `DeleteHistory` delegates to `FlushHistory` for persistence.

## UI Conventions

- Slash commands on the search screen: `/fav`, `/his`, `/feed`, `/filter` (`/f`), `/lang vi|en`, `/src`, `/update` (check only; installing is `futon update` on the CLI).
- `tab` cycles feed → favorites → history → sources → filters and back; `shift+tab` cycles backward.
- `esc` in `/fav`, `/his`, or `/src` returns to the search screen.
- `ctrl+d` in `/fav` removes a favorite; `ctrl+d` in `/his` removes a history entry.
- `ctrl+h` in the reader toggles the footer/hint row.
- `ctrl+e` exports the focused/current chapter to CBZ; `ctrl+a` in the chapter list exports every chapter (`esc` cancels it).
- `r` in the reader retries a page that failed to load.
- Search-screen tabs are one `searchView` enum (`m.view`); switch tabs through `switchView`, not by setting fields by hand.
- Views must not do I/O: screens get their size from the router (`WindowSizeMsg`, propagated to newly created screens) and cached state; mouse rows come from the same layout code that renders the view.
- `g`/`home` and `G`/`end` jump to the first/last chapter.
- `h`/`l` for page navigation are **only** active in Reader mode (not caught by other screens).
- `shift+↓`/`shift+↑` (or `j`/`k`) in Reader mode jump to the next/previous chapter in both modes; `w` toggles webtoon mode, where plain `↑`/`↓`/`space` scroll.
- `q` is **not** a keybinding anywhere — use `ctrl+c` to quit, `esc` to go back.
- The search screen shows the focused manga's cover in the preview pane (painted out-of-band with `tea.Raw`, see `search_cover.go`); there are no covers in the list itself.
- When in `/fav`, `/his`, or `/src`, typing filters the list in real-time (case-insensitive substring match).
- UI text is mixed Vietnamese and English; preserve existing wording unless asked to change it.

## Testing

- Add regression tests for observable behavior before refactoring TUI/storage code.
- Run `go test ./...` and `go test -race ./...` before declaring changes complete.
- Live provider export smoke test (opt-in, hits the network): `FUTON_LIVE_EXPORT=1 go test ./internal/export -run TestLiveProviderExports -v -timeout 10m`.

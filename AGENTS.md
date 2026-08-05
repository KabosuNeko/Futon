# AGENTS.md — Futon

Compact guidance for working in this repository.

## Documentation

Read only what the task needs; do not load docs speculatively.

- `SPEC.md` — product requirements, scope boundaries, keybindings, Definition of Done. Change behavior → update SPEC first (spec-first).
- `ROADMAP.md` — phases, tech debt, backlog. Tech-debt items are prioritized (P1 first).
- `TASKS.md` — current work status: in-progress, pending backlog (T-XXX), and what shipped.
- `README.md` — user-facing usage, install, keybindings, terminal compatibility.

## Project

Futon is a terminal manga reader. It searches multiple manga sources, lists chapters, and renders pages inline using Kitty graphics or Sixel.

- Language: Go 1.26.5
- Module: `github.com/KabosuNeko/Futon`
- Entrypoint: `cmd/main.go` → `internal/tui.NewAppModel()`
- Build: `go build ./...`
- Run: `go run ./cmd/...`
- Test: `go test ./...`

## Architecture

```
cmd/main.go          # entrypoint: starts the Bubble Tea program
internal/
  api/               # provider interface and HTTP implementations
    provider.go      # MangaProvider interface + shared helpers, tea.Msg types
    source.go        # tea.Cmd wrappers: SearchCmd, GlobalSearchCmd, FetchChaptersCmd, FetchPagesCmd
    mangadex.go      # MangaDex provider
    otruyen.go       # OTruyen provider
    truyenqq.go      # TruyenQQ provider
    baotangtruyen.go # BaoTangTruyen provider
    foxtruyen.go     # FoxTruyen provider
  models/            # response and domain structs
    chapter.go
    manga.go
  storage/           # local JSON persistence
    userdata.go      # UserData struct (favorites + sources merged), LoadUserData, SaveUserData, migrateOldData
    sources.go       # LoadSources/SaveSources → delegates to userdata
    favorites.go     # Load/Save/Add/RemoveFavorite → delegates to userdata
    history.go       # per-manga reading history with debounced flush
    *_test.go        # storage regression tests
  tui/               # Bubble Tea screens (split by responsibility)
    app.go           # router: search → chapters → reader
    search.go        # SearchModel, New, Init, Update dispatcher
    search_keys.go   # key handling for search/chapter list
    search_cmd.go    # command helpers: debounce, load favorites/history, box color
    search_view.go   # search/chapter list rendering helpers
    chapter.go       # ChapterListModel, New, Init, Update
    chapter_view.go  # chapter list view, viewport, history restore, quick jump
    reader.go        # ReaderModel, New, Init, Update dispatcher
    reader_keys.go   # reader key handling (navigation, save, quit)
    reader_msgs.go   # reader message handling (images, downloads, render, preload)
    reader_cache.go  # LRU image cache helpers
    reader_navigation.go  # chapter/page navigation and preload state helpers
    reader_view.go   # reader view and image layout helpers
    reader_download.go    # image download/render/save commands
    flash.go         # flash message helper
    *_test.go        # TUI regression tests
    imgrender/       # Kitty / Sixel renderer selection
```

Navigation between screens uses custom `tea.Msg` types defined in `internal/tui/app.go`: `ViewMangaMsg`, `ViewChapterMsg`, `BackToSearchMsg`, `BackToChaptersMsg`.

## Providers

- `/src` opens a checklist of providers with `[X]`/`[ ]` toggles.
- Arrow keys move cursor, `space` toggles the focused provider, `esc` closes the list.
- All providers are checked by default (searches all sources via `GlobalSearchCmd`).
- Only checked providers are searched; if none are checked, an error is shown.
- Toggle state is persisted to `userdata.json` and restored on next launch.
- Typing in `/src` mode filters the provider list (case-insensitive substring).
- Provider interface: `Name`, `Search`, `FetchChapters`, `FetchPages`.
- `GlobalSearchCmd` in `source.go` uses `sync.WaitGroup` + `sync.Mutex` for concurrent searches.
- Titles are standardized to `Name (source)` format in all search modes.

## Runtime State

- Favorites + source toggles: `~/.config/futon/userdata.json`
- Reading history (last chapter + page per manga): `~/.config/futon/history.json`
- Downloaded images: `~/Downloads/Futon_Downloads/`
- Old `favorites.json` and `sources.json` are auto-migrated to `userdata.json` on first launch.

## Image Rendering

`imgrender.New()` auto-detects the terminal:

- Kitty protocol when `TERM=xterm-kitty` or `KITTY_WINDOW_ID` is set.
- Sixel otherwise.

The reader uses ANSI cursor positioning and explicit clear sequences (`\x1b_Ga=d;\x1b\\` and `\x1b[H\x1b[2J`). Avoid introducing normal `\n` scrolling in reader output.

## Image Cache

`ReaderModel` keeps an in-memory LRU cache of rendered images (`imageCache`) keyed by page index, capped at 20 entries. It is reset on every new chapter to avoid memory leaks.

## External APIs

- MangaDex: `api.mangadex.org`
- OTruyen: `otruyenapi.com`
- API requests default to `User-Agent: Futon-App/1.0`; TruyenQQ sends a full browser UA to bypass bot detection.

## Image Downloads

- MangaDex image requests use `User-Agent: Futon-App/1.0 (https://github.com/KabosuNeko/Futon)` and **no** `Referer` header to avoid placeholder responses.
- Other providers may send a chapter-scoped `Referer` when required by their CDN.
- Download URLs are logged for debugging.

## Concurrency

- Keep UI concurrency inside `tea.Cmd` functions; do not block `Update`.
- History writes are debounced (2s) and flushed via `tea.Cmd`; `DeleteHistory` delegates to `FlushHistory` for persistence.

## UI Conventions

- Slash commands on the search screen: `/fav`, `/his`, `/lang vi|en`, `/src`.
- `esc` in `/fav`, `/his`, or `/src` returns to the search screen.
- `ctrl+d` in `/fav` removes a favorite; `ctrl+d` in `/his` removes a history entry.
- `h`/`l` for page navigation are **only** active in Reader mode (not caught by other screens).
- `q` is **not** a keybinding anywhere — use `ctrl+c` to quit, `esc` to go back.
- The search screen does **not** show cover images.
- When in `/fav`, `/his`, or `/src`, typing filters the list in real-time (case-insensitive substring match).
- UI text is mixed Vietnamese and English; preserve existing wording unless asked to change it.

## Testing

- Add regression tests for observable behavior before refactoring TUI/storage code.
- Run `go test ./...` and `go test -race ./...` before declaring changes complete.

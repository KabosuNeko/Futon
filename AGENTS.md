# AGENTS.md — Futon

Compact guidance for working in this repository.

## Project

Futon is a terminal manga reader. It searches multiple manga sources, lists chapters, and renders pages inline using Kitty graphics or Sixel.

- Language: Go 1.24.2
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
    provider.go      # MangaProvider interface + shared tea.Msg types
    source.go        # tea.Cmd wrappers: SearchCmd, FetchChaptersCmd, FetchPagesCmd
    mangadex.go      # MangaDex provider
    otruyen.go       # OTruyen provider (default)
  models/            # response and domain structs
    chapter.go
    manga.go
  storage/           # local JSON persistence
    favorites.go     # ~/.config/futon/favorites.json
    history.go       # ~/.config/futon/history.json
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

- Default provider: **OTruyen** (`api.NewOTruyenProvider()`).
- `tab` cycles through available providers (currently OTruyen, MangaDex).
- The footer shows the active source name.
- Provider interface: `Name`, `Search`, `FetchChapters`, `FetchPages`.

## Runtime State

- Favorites: `~/.config/futon/favorites.json`
- Reading history (last chapter + page per manga): `~/.config/futon/history.json`
- Downloaded images: `~/Downloads/Futon_Downloads/`

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
- All HTTP requests set `User-Agent: Futon-App/1.0`.

## Concurrency

- Keep UI concurrency inside `tea.Cmd` functions; do not block `Update`.
- History writes are debounced (2s) and flushed via `tea.Cmd`; `DeleteHistory` flushes immediately.

## UI Conventions

- Slash commands on the search screen: `/fav`, `/his`, `/lang vi|en`.
- `esc` in `/fav` or `/his` returns to the search screen.
- `d` in `/fav` removes a favorite; `d` in `/his` removes a history entry.
- The search screen does **not** show cover images.
- UI text is mixed Vietnamese and English; preserve existing wording unless asked to change it.

## Testing

- Add regression tests for observable behavior before refactoring TUI/storage code.
- Run `go test ./...` and `go test -race ./...` before declaring changes complete.

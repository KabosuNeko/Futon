package tui

import (
	"strings"
	"time"

	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/storage"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type searchTriggerMsg struct {
	query string
}

type favoritesLoadedMsg struct {
	favorites []storage.FavoriteManga
	err       error
}

type historyLoadedMsg struct {
	history []storage.ReadHistory
	err     error
}

func debounceSearch(query string, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return searchTriggerMsg{query: query}
	})
}

func loadFavoritesCmd() tea.Cmd {
	return func() tea.Msg {
		favs, err := storage.LoadFavorites()
		return favoritesLoadedMsg{favorites: favs, err: err}
	}
}

func loadHistoryCmd() tea.Cmd {
	return func() tea.Msg {
		all, err := storage.LoadAllHistory()
		return historyLoadedMsg{history: all, err: err}
	}
}

func boxColor(val string) lipgloss.Color {
	if strings.HasPrefix(strings.TrimSpace(val), "/") {
		return "5"
	}
	return "6"
}

type coverDebounceMsg struct {
	mangaID  string
	coverURL string
	provider string
}

type coverRenderedMsg struct {
	mangaID  string
	coverURL string
	rendered imgrender.RenderedImage
	err      error
}

func debounceCover(mangaID, coverURL, provider string, delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return coverDebounceMsg{
			mangaID:  mangaID,
			coverURL: coverURL,
			provider: provider,
		}
	})
}

func fetchAndRenderCoverCmd(renderer imgrender.Renderer, mangaID, coverURL, provider string, cols, rows int) tea.Cmd {
	return func() tea.Msg {
		if coverURL == "" || renderer == nil {
			return coverRenderedMsg{mangaID: mangaID, coverURL: coverURL, err: nil}
		}
		data, err := api.FetchCoverBytes(coverURL, provider)
		if err != nil {
			return coverRenderedMsg{mangaID: mangaID, coverURL: coverURL, err: err}
		}
		rendered, err := renderer.RenderInBox(data, cols, rows)
		if err != nil {
			return coverRenderedMsg{mangaID: mangaID, coverURL: coverURL, err: err}
		}
		return coverRenderedMsg{
			mangaID:  mangaID,
			coverURL: coverURL,
			rendered: rendered,
		}
	}
}


package tui

import (
	"strings"
	"time"

	"github.com/KabosuNeko/Futon/internal/storage"
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
		return "39"
	}
	return "205"
}

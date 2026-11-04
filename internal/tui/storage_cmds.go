package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/storage"
)

// Storage calls wrapped as commands so disk I/O never runs inside Update.
// Errors are dropped: history is best-effort and a later save retries.

func saveHistoryCmd(mangaID, mangaTitle, provider, chapterID, chapterNumber string, pageIndex int) tea.Cmd {
	return func() tea.Msg {
		_ = storage.SaveHistory(mangaID, mangaTitle, provider, chapterID, chapterNumber, pageIndex)
		return nil
	}
}

func flushHistoryCmd() tea.Cmd {
	return func() tea.Msg {
		_ = storage.FlushHistory()
		return nil
	}
}

func deleteHistoryCmd(mangaID, provider string) tea.Cmd {
	return func() tea.Msg {
		_ = storage.DeleteHistory(mangaID, provider)
		return nil
	}
}

type favoritesLoadedMsg struct {
	favorites []storage.FavoriteManga
	err       error
}

type historyLoadedMsg struct {
	history []storage.ReadHistory
	err     error
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

// storageSavedMsg reports the result of a background favorites/settings write.
type storageSavedMsg struct {
	err error
}

func saveReaderFitWidthCmd(on bool) tea.Cmd {
	return func() tea.Msg {
		return storageSavedMsg{err: storage.SaveReaderFitWidth(on)}
	}
}

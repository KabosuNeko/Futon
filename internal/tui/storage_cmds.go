package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/storage"
)

// Storage calls wrapped as commands so disk I/O never runs inside Update.
// Errors are dropped: history is best-effort and a later save retries.

func saveHistoryCmd(mangaID, mangaTitle, provider, coverURL, chapterID, chapterNumber string, pageIndex int) tea.Cmd {
	return func() tea.Msg {
		_ = storage.SaveHistory(mangaID, mangaTitle, provider, coverURL, chapterID, chapterNumber, pageIndex)
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

// setFavoriteCoverCmd fills in the cover of a favorite saved without one.
func setFavoriteCoverCmd(mangaID, provider, coverURL string) tea.Cmd {
	if coverURL == "" {
		return nil
	}
	return func() tea.Msg {
		return storageSavedMsg{err: storage.SetFavoriteCover(mangaID, provider, coverURL)}
	}
}

// saveFoundCoverCmd keeps a looked-up cover on the favorite and the history
// entry of that manga, whichever exist.
func saveFoundCoverCmd(mangaID, provider, coverURL string) tea.Cmd {
	return func() tea.Msg {
		err := storage.SetFavoriteCover(mangaID, provider, coverURL)
		if herr := storage.SetHistoryCover(mangaID, provider, coverURL); err == nil {
			err = herr
		}
		return storageSavedMsg{err: err}
	}
}

// storageSavedMsg reports the result of a background favorites/settings write.
type storageSavedMsg struct {
	err error
}

// saveUserDataCmd saves a change to the user's settings.
func saveUserDataCmd(update func(*storage.UserData)) tea.Cmd {
	return func() tea.Msg {
		return storageSavedMsg{err: storage.UpdateUserData(update)}
	}
}

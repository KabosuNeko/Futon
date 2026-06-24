package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/storage"
)

func (m SearchModel) handleKeyMsg(msg tea.KeyMsg) (SearchModel, tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit, true

	case "tab":
		m.providerIdx++
		if m.providerIdx >= len(m.providers) {
			m.providerIdx = -1
		}

		if m.providerIdx >= 0 {
			if p, ok := m.providers[m.providerIdx].(*api.MangaDexProvider); ok {
				p.SetLang(m.chapterLang)
			}
		}

		m.showingFavorites = false
		m.showingHistory = false
		m.favorites = nil
		m.history = nil
		m.results = nil
		m.cursor = 0
		m.viewportStart = 0
		m.err = nil

		if len(strings.TrimSpace(m.currentQuery)) >= 3 {
			m.isSearching = true
			if m.providerIdx < 0 {
				return m, api.GlobalSearchCmd(m.providers, m.currentQuery), true
			}
			return m, api.SearchCmd(m.CurrentProvider(), m.currentQuery), true
		}
		return m, nil, true

	case "esc":
		if m.showingHistory || m.showingFavorites {
			m.showingHistory = false
			m.showingFavorites = false
			m.history = nil
			m.favorites = nil
			m.cursor = 0
			m.viewportStart = 0
			return m, nil, true
		}

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.adjustViewport()
		}
		return m, nil, true

	case "down", "j":
		switch {
		case m.showingFavorites && m.cursor < len(m.favorites)-1:
			m.cursor++
			m.adjustViewport()
		case m.showingHistory && m.cursor < len(m.history)-1:
			m.cursor++
			m.adjustViewport()
		case !m.showingFavorites && !m.showingHistory && m.cursor < len(m.results)-1:
			m.cursor++
			m.adjustViewport()
		}
		return m, nil, true

	case "ctrl+d":
		if m.showingFavorites && len(m.favorites) > 0 && m.cursor < len(m.favorites) {
			fav := m.favorites[m.cursor]
			m.favorites = append(m.favorites[:m.cursor], m.favorites[m.cursor+1:]...)
			if m.cursor >= len(m.favorites) && m.cursor > 0 {
				m.cursor--
			}
			m.adjustViewport()
			m.flashMsg = fmt.Sprintf("Đã xóa \"%s\" khỏi Yêu thích", fav.Title)
			return m, tea.Batch(
				func() tea.Msg { return favoriteSavedMsg{err: storage.RemoveFavorite(fav.MangaID)} },
				clearFlashAfter(2*time.Second),
			), true
		}
		if m.showingHistory && len(m.history) > 0 && m.cursor < len(m.history) {
			h := m.history[m.cursor]
			m.history = append(m.history[:m.cursor], m.history[m.cursor+1:]...)
			if m.cursor >= len(m.history) && m.cursor > 0 {
				m.cursor--
			}
			m.adjustViewport()
			return m, storage.DeleteHistoryCmd(h.MangaID), true
		}
		return m, nil, true

	case "enter":
		val := strings.TrimSpace(m.input.Value())

		if val == "/fav" {
			m.showingFavorites = true
			m.loadingFavorites = true
			m.results = nil
			m.currentQuery = ""
			m.cursor = 0
			m.viewportStart = 0
			m.input.SetValue("")
			return m, loadFavoritesCmd(), true
		}

		if strings.HasPrefix(val, "/lang") {
			parts := strings.Fields(val)
			if len(parts) >= 2 && (parts[1] == "vi" || parts[1] == "en") {
				m.chapterLang = parts[1]
				if p, ok := m.CurrentProvider().(*api.MangaDexProvider); ok {
					p.SetLang(parts[1])
				}
				m.systemMsg = "Đã cài đặt ngôn ngữ chapter mặc định: " + parts[1]
			} else {
				m.systemMsg = "Dùng: /lang vi hoặc /lang en"
			}
			m.input.SetValue("")
			return m, nil, true
		}

		if val == "/his" {
			m.showingHistory = true
			m.loadingHistory = true
			m.results = nil
			m.favorites = nil
			m.showingFavorites = false
			m.currentQuery = ""
			m.cursor = 0
			m.viewportStart = 0
			m.input.SetValue("")
			return m, loadHistoryCmd(), true
		}

		if m.showingFavorites && len(m.favorites) > 0 && m.cursor < len(m.favorites) {
			fav := m.favorites[m.cursor]
			m.showingFavorites = false
			m.favorites = nil
			m.cursor = 0
			m.viewportStart = 0
			providerName := fav.Provider
			if providerName == "" && len(m.providers) > 0 {
				providerName = m.providers[0].Name()
			}
			return m, func() tea.Msg {
				return ViewMangaMsg{MangaID: fav.MangaID, Title: fav.Title, ProviderName: providerName}
			}, true
		}

		if m.showingHistory && len(m.history) > 0 && m.cursor < len(m.history) {
			h := m.history[m.cursor]
			m.showingHistory = false
			m.history = nil
			m.cursor = 0
			m.viewportStart = 0
			title := h.MangaTitle
			if title == "" {
				title = h.MangaID
			}
			providerName := h.Provider
			if providerName == "" && len(m.providers) > 0 {
				providerName = m.providers[0].Name()
			}
			return m, func() tea.Msg {
				return ViewMangaMsg{MangaID: h.MangaID, Title: title, ProviderName: providerName}
			}, true
		}

		if len(m.results) > 0 && m.cursor < len(m.results) {
			manga := m.results[m.cursor]
			providerName := manga.Provider
			if providerName == "" {
				if p := m.CurrentProvider(); p != nil {
					providerName = p.Name()
				} else if len(m.providers) > 0 {
					providerName = m.providers[0].Name()
				}
			}
			return m, func() tea.Msg {
				return ViewMangaMsg{MangaID: manga.ID, Title: manga.Title, ProviderName: providerName}
			}, true
		}

		if val == "" {
			return m, nil, true
		}
		m.showingFavorites = false
		m.currentQuery = val
		m.results = nil
		m.cursor = 0
		m.viewportStart = 0
		m.err = nil
		m.isSearching = true
		if m.providerIdx < 0 {
			return m, api.GlobalSearchCmd(m.providers, val), true
		}
		return m, api.SearchCmd(m.CurrentProvider(), val), true
	}
	return m, nil, false
}

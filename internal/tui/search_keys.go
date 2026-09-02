package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/storage"
	tea "github.com/charmbracelet/bubbletea"
)

func (m SearchModel) selectCurrentItem() (SearchModel, tea.Cmd, bool) {
	if m.showingFavorites {
		indices := m.filteredFavIndices()
		if len(indices) > 0 && m.cursor < len(indices) {
			fav := m.favorites[indices[m.cursor]]
			m.showingFavorites = false
			m.favorites = nil
			m.cursor = 0
			m.viewportStart = 0
			m.filterQuery = ""
			m.input.SetValue("")
			m.input.Placeholder = "Nhập tên manga cần tìm..."
			providerName := fav.Provider
			if providerName == "" && len(m.providers) > 0 {
				providerName = m.providers[0].Name()
			}
			return m, func() tea.Msg {
				return ViewMangaMsg{MangaID: fav.MangaID, Title: fav.Title, ProviderName: providerName}
			}, true
		}
		return m, nil, true
	}

	if m.showingHistory {
		indices := m.filteredHistoryIndices()
		if len(indices) > 0 && m.cursor < len(indices) {
			h := m.history[indices[m.cursor]]
			m.showingHistory = false
			m.history = nil
			m.cursor = 0
			m.viewportStart = 0
			m.filterQuery = ""
			m.input.SetValue("")
			m.input.Placeholder = "Nhập tên manga cần tìm..."
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
		return m, nil, true
	}

	if len(m.results) > 0 && m.cursor < len(m.results) {
		manga := m.results[m.cursor]
		providerName := manga.Provider
		if providerName == "" {
			active := m.activeProviders()
			if len(active) > 0 {
				providerName = active[0].Name()
			}
		}
		return m, func() tea.Msg {
			return ViewMangaMsg{MangaID: manga.ID, Title: manga.Title, ProviderName: providerName}
		}, true
	}

	return m, nil, false
}

func (m SearchModel) handleKeyMsg(msg tea.KeyMsg) (SearchModel, tea.Cmd, bool) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit, true

	case "esc":
		if m.showingFilters {
			m.showingFilters = false
			return m, nil, true
		}
		if m.showingSources {
			m.showingSources = false
			m.sourceCursor = 0
			m.filterQuery = ""
			m.input.SetValue("")
			m.input.Placeholder = "Nhập tên manga cần tìm..."
			return m, nil, true
		}
		if m.showingHistory || m.showingFavorites {
			m.showingHistory = false
			m.showingFavorites = false
			m.showingFeed = true
			m.history = nil
			m.favorites = nil
			m.cursor = 0
			m.viewportStart = 0
			m.filterQuery = ""
			m.input.SetValue("")
			m.input.Placeholder = "Nhập tên manga cần tìm..."
			active := m.activeProviders()
			if len(active) == 0 {
				active = m.providers
			}
			return m, api.GlobalLatestCmd(active, 1), true
		}

	case "tab":
		m.filterQuery = ""
		m.input.SetValue("")
		m.cursor = 0
		m.viewportStart = 0
		if m.showingFeed && !m.showingFavorites && !m.showingHistory && !m.showingSources && !m.showingFilters {
			// Switch to Favorites
			m.showingFeed = false
			m.showingFavorites = true
			m.loadingFavorites = true
			m.input.Placeholder = "Lọc truyện yêu thích..."
			return m, loadFavoritesCmd(), true
		} else if m.showingFavorites {
			// Switch to History
			m.showingFavorites = false
			m.showingHistory = true
			m.loadingHistory = true
			m.input.Placeholder = "Lọc lịch sử đọc..."
			return m, loadHistoryCmd(), true
		} else if m.showingHistory {
			// Switch to Sources
			m.showingHistory = false
			m.showingSources = true
			m.sourceCursor = 0
			m.input.Placeholder = "Lọc nguồn..."
			return m, nil, true
		} else if m.showingSources {
			// Switch to Filters
			m.showingSources = false
			m.showingFilters = true
			m.filterCursor = 0
			return m, nil, true
		} else {
			// Switch to Feed
			m.showingFilters = false
			m.showingSources = false
			m.showingFavorites = false
			m.showingHistory = false
			m.showingFeed = true
			m.isSearching = true
			m.input.Placeholder = "Nhập tên manga cần tìm..."
			active := m.activeProviders()
			if len(active) == 0 {
				active = m.providers
			}
			return m, api.GlobalLatestCmd(active, 1), true
		}

	case "left":
		if m.showingFilters {
			switch m.filterCursor {
			case 0:
				m.filterStatus = (m.filterStatus - 1 + len(filterStatusOptions)) % len(filterStatusOptions)
			case 1:
				m.filterSort = (m.filterSort - 1 + len(filterSortOptions)) % len(filterSortOptions)
			case 2:
				m.filterGenre = (m.filterGenre - 1 + len(filterGenreOptions)) % len(filterGenreOptions)
			}
			return m, nil, true
		}

	case "right":
		if m.showingFilters {
			switch m.filterCursor {
			case 0:
				m.filterStatus = (m.filterStatus + 1) % len(filterStatusOptions)
			case 1:
				m.filterSort = (m.filterSort + 1) % len(filterSortOptions)
			case 2:
				m.filterGenre = (m.filterGenre + 1) % len(filterGenreOptions)
			}
			return m, nil, true
		}

	case "up":
		if m.showingFilters {
			if m.filterCursor > 0 {
				m.filterCursor--
			}
			return m, nil, true
		}
		if m.showingSources {
			if m.sourceCursor > 0 {
				m.sourceCursor--
			}
			return m, nil, true
		} else if m.cursor > 0 {
			m.cursor--
			m.adjustViewport()
			coverCmd := m.updateFocusedCover()
			return m, coverCmd, true
		}
		return m, nil, true

	case "down":
		if m.showingFilters {
			if m.filterCursor < 4 {
				m.filterCursor++
			}
			return m, nil, true
		}
		if m.showingSources {
			indices := m.filteredProviderIndices()
			if m.sourceCursor < len(indices)-1 {
				m.sourceCursor++
			}
			return m, nil, true
		}
		moved := false
		switch {
		case m.showingFavorites && m.cursor < len(m.filteredFavIndices())-1:
			m.cursor++
			m.adjustViewport()
			moved = true
		case m.showingHistory && m.cursor < len(m.filteredHistoryIndices())-1:
			m.cursor++
			m.adjustViewport()
			moved = true
		case !m.showingFavorites && !m.showingHistory && m.cursor < len(m.results)-1:
			m.cursor++
			m.adjustViewport()
			moved = true
		}
		if moved {
			coverCmd := m.updateFocusedCover()
			return m, coverCmd, true
		}
		return m, nil, true

	case " ":
		if m.showingSources {
			indices := m.filteredProviderIndices()
			if m.sourceCursor < len(indices) {
				m.providerToggles[indices[m.sourceCursor]] = !m.providerToggles[indices[m.sourceCursor]]
				_ = storage.SaveSources(m.activeProviderNames())
			}
			return m, nil, true
		}
		return m, nil, false

	case "ctrl+d":
		if m.showingFavorites {
			indices := m.filteredFavIndices()
			if len(indices) > 0 && m.cursor < len(indices) {
				actualIdx := indices[m.cursor]
				fav := m.favorites[actualIdx]
				m.favorites = append(m.favorites[:actualIdx], m.favorites[actualIdx+1:]...)
				newIndices := m.filteredFavIndices()
				if m.cursor >= len(newIndices) && m.cursor > 0 {
					m.cursor--
				}
				m.adjustViewport()
				m.flashMsg = fmt.Sprintf("Đã xóa \"%s\" khỏi Yêu thích", fav.Title)
				return m, tea.Batch(
					func() tea.Msg { return favoriteSavedMsg{err: storage.RemoveFavorite(fav.MangaID)} },
					clearFlashAfter(2*time.Second),
				), true
			}
		}
		if m.showingHistory {
			indices := m.filteredHistoryIndices()
			if len(indices) > 0 && m.cursor < len(indices) {
				actualIdx := indices[m.cursor]
				h := m.history[actualIdx]
				m.history = append(m.history[:actualIdx], m.history[actualIdx+1:]...)
				newIndices := m.filteredHistoryIndices()
				if m.cursor >= len(newIndices) && m.cursor > 0 {
					m.cursor--
				}
				m.adjustViewport()
				return m, storage.DeleteHistoryCmd(h.MangaID), true
			}
		}
		return m, nil, true

	case "enter":
		if m.showingFilters {
			if m.filterCursor == 4 {
				// Reset
				m.filterStatus = 0
				m.filterSort = 0
				m.filterGenre = 0
			}
			m.showingFilters = false
			m.isSearching = true
			m.results = nil
			m.cursor = 0
			m.viewportStart = 0
			active := m.activeProviders()
			if len(active) == 0 {
				active = m.providers
			}
			opts := api.FilterOptions{
				Status: m.filterStatus,
				Sort:   m.filterSort,
				Genre:  m.filterGenre,
				Page:   1,
			}
			return m, api.GlobalFilterCmd(active, opts), true
		}

		val := strings.TrimSpace(m.input.Value())

		if val == "/src" {
			m.showingSources = true
			m.sourceCursor = 0
			m.showingFavorites = false
			m.showingHistory = false
			m.showingFilters = false
			m.results = nil
			m.favorites = nil
			m.history = nil
			m.currentQuery = ""
			m.cursor = 0
			m.viewportStart = 0
			m.filterQuery = ""
			m.input.SetValue("")
			m.input.Placeholder = "Lọc nguồn..."
			return m, nil, true
		}

		if val == "/filter" || val == "/f" {
			m.showingFilters = true
			m.filterCursor = 0
			m.input.SetValue("")
			return m, nil, true
		}

		if val == "/feed" {
			m.showingFavorites = false
			m.showingHistory = false
			m.showingSources = false
			m.showingFilters = false
			m.showingFeed = true
			m.results = nil
			m.cursor = 0
			m.viewportStart = 0
			m.isSearching = true
			m.input.SetValue("")
			m.input.Placeholder = "Nhập tên manga cần tìm..."
			active := m.activeProviders()
			if len(active) == 0 {
				active = m.providers
			}
			return m, api.GlobalLatestCmd(active, 1), true
		}

		if val == "/fav" {
			m.showingFavorites = true
			m.loadingFavorites = true
			m.showingSources = false
			m.showingFilters = false
			m.showingFeed = false
			m.results = nil
			m.currentQuery = ""
			m.cursor = 0
			m.viewportStart = 0
			m.filterQuery = ""
			m.input.SetValue("")
			m.input.Placeholder = "Lọc truyện yêu thích..."
			return m, loadFavoritesCmd(), true
		}

		if strings.HasPrefix(val, "/lang") {
			parts := strings.Fields(val)
			if len(parts) < 2 || (parts[1] != "vi" && parts[1] != "en") {
				m.systemMsg = "Dùng: /lang vi hoặc /lang en"
				m.input.SetValue("")
				return m, nil, true
			}
			m.chapterLanguage = parts[1]
			for _, p := range m.providers {
				if md, ok := p.(*api.MangaDexProvider); ok {
					md.SetLang(parts[1])
				}
			}
			m.systemMsg = "Đã cài đặt ngôn ngữ chapter mặc định: " + parts[1]
			m.input.SetValue("")
			return m, nil, true
		}

		if val == "/his" {
			m.showingHistory = true
			m.loadingHistory = true
			m.showingSources = false
			m.showingFilters = false
			m.showingFeed = false
			m.results = nil
			m.favorites = nil
			m.showingFavorites = false
			m.currentQuery = ""
			m.cursor = 0
			m.viewportStart = 0
			m.filterQuery = ""
			m.input.SetValue("")
			m.input.Placeholder = "Lọc lịch sử đọc..."
			return m, loadHistoryCmd(), true
		}

		if val == "/update" {
			m.input.SetValue("")
			return m, func() tea.Msg { return RequestUpdateMsg{} }, true
		}

		if newM, cmd, handled := m.selectCurrentItem(); handled {
			return newM, cmd, true
		}

		if val == "" {
			return m, nil, true
		}
		m.showingFavorites = false
		m.showingSources = false
		m.showingFilters = false
		m.showingFeed = false
		m.currentQuery = val
		m.results = nil
		m.cursor = 0
		m.viewportStart = 0
		m.err = nil
		m.isSearching = true
		active := m.activeProviders()
		if len(active) == 0 {
			m.err = fmt.Errorf("Chọn ít nhất một nguồn trong /src")
			m.isSearching = false
			return m, nil, true
		}
		return m, api.GlobalSearchCmd(active, val), true
	}
	return m, nil, false
}

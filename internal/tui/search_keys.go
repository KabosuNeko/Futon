package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/storage"
)

func (m SearchModel) selectCurrentItem() (SearchModel, tea.Cmd, bool) {
	if m.view == viewFavorites {
		indices := m.filteredFavIndices()
		if len(indices) > 0 && m.cursor < len(indices) {
			fav := m.favorites[indices[m.cursor]]
			m.resetView(viewFeed)
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

	if m.view == viewHistory {
		indices := m.filteredHistoryIndices()
		if len(indices) > 0 && m.cursor < len(indices) {
			h := m.history[indices[m.cursor]]
			m.resetView(viewFeed)
			title := h.MangaTitle
			if title == "" {
				title = h.MangaID
			}
			providerName := h.Provider
			if providerName == "" && len(m.providers) > 0 {
				providerName = m.providers[0].Name()
			}
			return m, func() tea.Msg {
				return ViewMangaMsg{MangaID: h.MangaID, Title: title, ProviderName: providerName, ResumeChapterID: h.ChapterID}
			}, true
		}
		return m, nil, true
	}

	if m.view == viewFeed {
		manga, ok := m.focusedManga()
		if !ok {
			return m, nil, false
		}
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

// resetView switches to tab v and clears the per-tab state (input, filter,
// cursor) without loading anything.
func (m *SearchModel) resetView(v searchView) {
	m.view = v
	m.filterQuery = ""
	m.input.SetValue("")
	m.cursor = 0
	m.viewportStart = 0
	m.sourceCursor = 0
	m.favorites = nil
	m.history = nil
	switch v {
	case viewFavorites:
		m.input.Placeholder = "Lọc truyện yêu thích..."
	case viewHistory:
		m.input.Placeholder = "Lọc lịch sử đọc..."
	case viewSources:
		m.input.Placeholder = "Lọc nguồn..."
	default:
		m.input.Placeholder = searchPlaceholder
	}
}

// switchView opens tab v and starts whatever load it needs. Entering the
// feed tab reloads the latest list, which also picks up source changes.
func (m SearchModel) switchView(v searchView) (SearchModel, tea.Cmd) {
	m.resetView(v)
	m.err = nil // an error belongs to the tab that produced it
	switch v {
	case viewFavorites:
		m.loadingFavorites = true
		return m, loadFavoritesCmd()
	case viewHistory:
		m.loadingHistory = true
		return m, loadHistoryCmd()
	case viewFilters:
		m.filterCursor = 0
		return m, nil
	case viewSources:
		return m, nil
	default:
		m.currentQuery = ""
		return m, m.requestResults(resultsLatest, 1, false)
	}
}

// cycleView moves through the search tabs; delta=1 is tab, delta=-1 is shift+tab.
func (m SearchModel) cycleView(delta int) (SearchModel, tea.Cmd) {
	next := (int(m.view) + delta + int(viewCount)) % int(viewCount)
	return m.switchView(searchView(next))
}

// slashCommands maps the slash commands that just open a tab.
var slashCommands = map[string]searchView{
	"/feed":   viewFeed,
	"/fav":    viewFavorites,
	"/his":    viewHistory,
	"/src":    viewSources,
	"/filter": viewFilters,
	"/f":      viewFilters,
}

func (m SearchModel) handleKeyMsg(msg tea.KeyMsg) (SearchModel, tea.Cmd, bool) {
	switch msg.String() {
	case "esc":
		switch m.view {
		case viewFilters:
			// Close the modal over the current results without reloading;
			// typing the /f command cleared them, so reload in that case.
			if len(m.results) > 0 {
				m.view = viewFeed
				return m, nil, true
			}
			m, cmd := m.switchView(viewFeed)
			return m, cmd, true
		case viewSources, viewFavorites, viewHistory:
			m, cmd := m.switchView(viewFeed)
			return m, cmd, true
		}

	case "tab":
		m, cmd := m.cycleView(1)
		return m, cmd, true

	case "shift+tab":
		m, cmd := m.cycleView(-1)
		return m, cmd, true

	case "shift+left", "shift+right":
		// Same title on several sources: pick which one to open.
		if m.view == viewFeed {
			delta := 1
			if msg.String() == "shift+left" {
				delta = -1
			}
			if m.cycleVariant(delta) {
				return m, m.updateFocusedCover(), true
			}
			return m, nil, true
		}

	case "left", "right":
		if m.view == viewFilters {
			delta := 1
			if msg.String() == "left" {
				delta = -1
			}
			m.cycleFilter(delta)
			return m, nil, true
		}

	case "up":
		m, cmd := m.moveSelection(-1)
		return m, cmd, true

	case "down":
		m, cmd := m.moveSelection(1)
		return m, cmd, true

	case "space":
		if m.view == viewSources {
			return m, m.toggleCurrentSource(), true
		}
		return m, nil, false

	case "ctrl+d":
		return m.deleteFocusedEntry()

	case "enter":
		return m.handleEnter()
	}
	return m, nil, false
}

// deleteFocusedEntry removes the focused favorite or history entry.
func (m SearchModel) deleteFocusedEntry() (SearchModel, tea.Cmd, bool) {
	switch m.view {
	case viewFavorites:
		indices := m.filteredFavIndices()
		if len(indices) == 0 || m.cursor >= len(indices) {
			return m, nil, true
		}
		actualIdx := indices[m.cursor]
		fav := m.favorites[actualIdx]
		m.favorites = append(m.favorites[:actualIdx], m.favorites[actualIdx+1:]...)
		m.clampCursorAfterDelete()
		m.flashMsg = fmt.Sprintf("Đã xóa \"%s\" khỏi Yêu thích", fav.Title)
		return m, tea.Batch(
			func() tea.Msg { return storageSavedMsg{err: storage.RemoveFavorite(fav.MangaID, fav.Provider)} },
			clearFlashAfter(2*time.Second),
		), true

	case viewHistory:
		indices := m.filteredHistoryIndices()
		if len(indices) == 0 || m.cursor >= len(indices) {
			return m, nil, true
		}
		actualIdx := indices[m.cursor]
		h := m.history[actualIdx]
		m.history = append(m.history[:actualIdx], m.history[actualIdx+1:]...)
		m.clampCursorAfterDelete()
		return m, deleteHistoryCmd(h.MangaID, h.Provider), true
	}
	return m, nil, true
}

func (m *SearchModel) clampCursorAfterDelete() {
	if m.cursor >= m.currentListLen() && m.cursor > 0 {
		m.cursor--
	}
	m.adjustViewport()
}

func (m SearchModel) handleEnter() (SearchModel, tea.Cmd, bool) {
	if m.view == viewFilters {
		if m.filterCursor == filterRowReset {
			m.filterStatus = 0
			m.filterSort = 0
			m.filterGenre = 0
		}
		m.view = viewFeed
		return m, m.requestResults(resultsFilter, 1, false), true
	}

	val := strings.TrimSpace(m.input.Value())

	if v, ok := slashCommands[val]; ok {
		m, cmd := m.switchView(v)
		return m, cmd, true
	}

	if strings.HasPrefix(val, "/lang") {
		parts := strings.Fields(val)
		m.input.SetValue("")
		if len(parts) < 2 || (parts[1] != "vi" && parts[1] != "en") {
			m.systemMsg = "Dùng: /lang vi hoặc /lang en"
			return m, nil, true
		}
		m.setChapterLanguage(parts[1])
		m.systemMsg = "Đã cài đặt ngôn ngữ chapter mặc định: " + parts[1]
		lang := parts[1]
		return m, saveUserDataCmd(func(ud *storage.UserData) { ud.ChapterLang = lang }), true
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
	m.view = viewFeed
	m.currentQuery = val
	return m, m.requestResults(resultsSearch, 1, false), true
}

// setChapterLanguage applies the MangaDex chapter language.
func (m *SearchModel) setChapterLanguage(lang string) {
	for _, p := range m.providers {
		if md, ok := p.(*api.MangaDexProvider); ok {
			md.SetLang(lang)
		}
	}
}

// cycleFilter rotates the value of the filter row under the cursor.
func (m *SearchModel) cycleFilter(delta int) {
	cycle := func(value, n int) int { return (value + delta + n) % n }
	switch m.filterCursor {
	case filterRowStatus:
		m.filterStatus = cycle(m.filterStatus, len(filterStatusOptions))
	case filterRowSort:
		m.filterSort = cycle(m.filterSort, len(filterSortOptions))
	case filterRowGenre:
		m.filterGenre = cycle(m.filterGenre, len(filterGenreOptions))
	}
}

func (m SearchModel) handleMouseMsg(msg tea.MouseMsg) (SearchModel, tea.Cmd, bool) {
	mouse := msg.Mouse()
	switch mouse.Button {
	case tea.MouseWheelUp:
		m, cmd := m.moveSelection(-1)
		return m, cmd, true

	case tea.MouseWheelDown:
		m, cmd := m.moveSelection(1)
		return m, cmd, true

	case tea.MouseLeft:
		if _, ok := msg.(tea.MouseClickMsg); !ok {
			return m, nil, false
		}

		itemIdx := m.viewportStart + (mouse.Y - m.listTopRow())
		if itemIdx >= 0 && itemIdx < m.currentListLen() {
			if m.view == viewSources {
				m.sourceCursor = itemIdx
				return m, m.toggleCurrentSource(), true
			}

			if itemIdx == m.cursor {
				return m.selectCurrentItem()
			}
			m.cursor = itemIdx
			m.adjustViewport()
			return m, m.updateFocusedCover(), true
		}
	}
	return m, nil, false
}

// moveSelection moves the cursor of whatever list the current view shows.
// Reaching the end of the latest/filter list fetches the next page.
func (m SearchModel) moveSelection(delta int) (SearchModel, tea.Cmd) {
	switch m.view {
	case viewFilters:
		m.filterCursor = max(0, min(filterRowCount-1, m.filterCursor+delta))
		return m, nil
	case viewSources:
		m.sourceCursor = max(0, min(m.currentListLen()-1, m.sourceCursor+delta))
		return m, nil
	}
	if !m.moveCursor(delta) {
		return m, nil
	}
	return m, tea.Batch(m.updateFocusedCover(), m.maybeLoadMore())
}

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/storage"
)

const searchUIOffset = 9

type SearchModel struct {
	input            textinput.Model
	width            int
	height           int
	results          []models.Manga
	favorites        []storage.FavoriteManga
	history          []storage.ReadHistory
	showingFavorites bool
	showingHistory   bool
	cursor           int
	viewportStart    int
	isSearching      bool
	loadingFavorites bool
	loadingHistory   bool
	err              error
	currentQuery     string
	flashMsg         string

	filterQuery    string
	chapterLanguage string
	systemMsg       string

	providers       []api.MangaProvider
	providerToggles []bool
	showingSources  bool
	sourceCursor    int
}

func NewSearchModel(providers []api.MangaProvider) SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Nhập tên manga cần tìm..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	toggles := loadSourceToggles(providers)

	return SearchModel{
		input:           ti,
		width:           80,
		height:          24,
		chapterLanguage: "vi",
		providers:       providers,
		providerToggles: toggles,
	}
}

func (m SearchModel) activeProviders() []api.MangaProvider {
	var active []api.MangaProvider
	for i, p := range m.providers {
		if m.providerToggles[i] {
			active = append(active, p)
		}
	}
	return active
}

func loadSourceToggles(providers []api.MangaProvider) []bool {
	toggles := make([]bool, len(providers))
	for i := range toggles {
		toggles[i] = true
	}

	saved, err := storage.LoadSources()
	if err != nil || len(saved) == 0 || len(saved) > len(providers) {
		return toggles
	}

	for i := range toggles {
		toggles[i] = false
	}
	savedSet := make(map[string]bool, len(saved))
	for _, name := range saved {
		savedSet[name] = true
	}
	for i, p := range providers {
		if savedSet[p.Name()] {
			toggles[i] = true
		}
	}
	return toggles
}

func (m SearchModel) activeProviderNames() []string {
	var names []string
	for i, p := range m.providers {
		if m.providerToggles[i] {
			names = append(names, p.Name())
		}
	}
	return names
}

func (m SearchModel) filteredFavIndices() []int {
	if m.filterQuery == "" || m.showingSources {
		indices := make([]int, len(m.favorites))
		for i := range m.favorites {
			indices[i] = i
		}
		return indices
	}
	var indices []int
	q := strings.ToLower(m.filterQuery)
	for i, fav := range m.favorites {
		if strings.Contains(strings.ToLower(fav.Title), q) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m SearchModel) filteredHistoryIndices() []int {
	if m.filterQuery == "" || m.showingSources {
		indices := make([]int, len(m.history))
		for i := range m.history {
			indices[i] = i
		}
		return indices
	}
	var indices []int
	q := strings.ToLower(m.filterQuery)
	for i, h := range m.history {
		title := strings.ToLower(h.MangaTitle)
		if title == "" {
			title = strings.ToLower(h.MangaID)
		}
		if strings.Contains(title, q) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m SearchModel) filteredProviderIndices() []int {
	if m.filterQuery == "" || !m.showingSources {
		indices := make([]int, len(m.providers))
		for i := range m.providers {
			indices[i] = i
		}
		return indices
	}
	var indices []int
	q := strings.ToLower(m.filterQuery)
	for i, p := range m.providers {
		if strings.Contains(strings.ToLower(p.Name()), q) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m SearchModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if newM, cmd, handled := m.handleKeyMsg(msg); handled {
			return newM, cmd
		}

	case favoritesLoadedMsg:
		m.loadingFavorites = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.favorites = msg.favorites
		m.cursor = 0
		m.viewportStart = 0
		return m, nil

	case historyLoadedMsg:
		m.loadingHistory = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.history = msg.history
		m.cursor = 0
		m.viewportStart = 0
		return m, nil

	case searchTriggerMsg:
		if msg.query == m.currentQuery && len(strings.TrimSpace(msg.query)) >= 3 {
			active := m.activeProviders()
			if len(active) == 0 {
				m.err = fmt.Errorf("Chọn ít nhất một nguồn trong /src")
				return m, nil
			}
			m.isSearching = true
			return m, tea.Batch(cmd, api.GlobalSearchCmd(active, msg.query))
		}
		return m, nil

	case api.MangaSearchResultMsg:
		m.isSearching = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		if len(strings.TrimSpace(m.currentQuery)) < 3 {
			return m, nil
		}
		m.results = msg.Manga
		m.cursor = 0
		m.viewportStart = 0
		m.err = nil
		return m, nil

	case clearFlashMsg:
		m.flashMsg = ""
		return m, nil

	case favoriteSavedMsg:
		if msg.err != nil {
			m.flashMsg = fmt.Sprintf("Lỗi: %v", msg.err)
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustViewport()
	}

	oldVal := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	newVal := m.input.Value()

	if oldVal != newVal {
		trimmed := strings.TrimSpace(newVal)
		m.systemMsg = ""

		if m.showingFavorites || m.showingHistory || m.showingSources {
			m.filterQuery = trimmed
			m.cursor = 0
			m.viewportStart = 0
			if m.showingSources {
				m.sourceCursor = 0
			}
			return m, cmd
		}

		if strings.HasPrefix(trimmed, "/") {
			m.currentQuery = ""
			m.results = nil
			m.cursor = 0
			m.viewportStart = 0
			m.isSearching = false
			m.err = nil
			return m, cmd
		}

		m.currentQuery = trimmed
		if len(trimmed) >= 3 {
			// 300ms debounce — fast enough to feel responsive, slow enough to not DDoS APIs.
			return m, tea.Batch(cmd, debounceSearch(trimmed, 300*time.Millisecond))
		}
		m.results = nil
		m.cursor = 0
		m.viewportStart = 0
		m.isSearching = false
		m.err = nil
	}

	return m, cmd
}

func (m SearchModel) listVisibleItems() int {
	h := m.height
	visible := h - searchUIOffset
	if visible < 1 {
		visible = 1
	}
	return visible
}

func (m *SearchModel) adjustViewport() {
	visible := m.listVisibleItems()
	var total int
	switch {
	case m.showingFavorites:
		total = len(m.filteredFavIndices())
	case m.showingHistory:
		total = len(m.filteredHistoryIndices())
	default:
		total = len(m.results)
	}

	if m.cursor < m.viewportStart {
		m.viewportStart = m.cursor
	}
	if m.cursor >= m.viewportStart+visible {
		m.viewportStart = m.cursor - visible + 1
	}
	if m.viewportStart < 0 {
		m.viewportStart = 0
	}
	if m.viewportStart >= total && total > 0 {
		m.viewportStart = total - 1
	}
}

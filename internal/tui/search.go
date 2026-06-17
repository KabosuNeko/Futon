package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/futon/internal/api"
	"github.com/user/futon/internal/models"
	"github.com/user/futon/internal/storage"
)

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
	isSearching      bool
	loadingFavorites bool
	loadingHistory   bool
	err              error
	currentQuery     string
	flashMsg         string

	chapterLang string
	systemMsg   string

	providers   []api.MangaProvider
	providerIdx int
}

func NewSearchModel(providers []api.MangaProvider) SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Nhập tên manga cần tìm..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40

	idx := 0
	if len(providers) == 0 {
		idx = -1
	}

	return SearchModel{
		input:       ti,
		width:       80,
		height:      24,
		chapterLang: "vi",
		providers:   providers,
		providerIdx: idx,
	}
}

func (m SearchModel) CurrentProvider() api.MangaProvider {
	if m.providerIdx < 0 || m.providerIdx >= len(m.providers) {
		return nil
	}
	return m.providers[m.providerIdx]
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
		return m, nil

	case historyLoadedMsg:
		m.loadingHistory = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.history = msg.history
		m.cursor = 0
		return m, nil

	case searchTriggerMsg:
		if msg.query == m.currentQuery && len(strings.TrimSpace(msg.query)) >= 3 {
			m.isSearching = true
			return m, tea.Batch(cmd, api.SearchCmd(m.CurrentProvider(), msg.query))
		}
		return m, nil

	case api.MangaSearchResultMsg:
		m.isSearching = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.results = msg.Manga
		m.cursor = 0
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
	}

	oldVal := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	newVal := m.input.Value()

	if oldVal != newVal {
		trimmed := strings.TrimSpace(newVal)
		m.systemMsg = ""

		if strings.HasPrefix(trimmed, "/") {
			m.currentQuery = ""
			m.results = nil
			m.cursor = 0
			m.isSearching = false
			m.err = nil
			return m, cmd
		}

		m.currentQuery = trimmed
		if len(trimmed) >= 3 {
			return m, tea.Batch(cmd, debounceSearch(trimmed, 300*time.Millisecond))
		}
		m.results = nil
		m.cursor = 0
		m.isSearching = false
		m.err = nil
	}

	return m, cmd
}

package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/storage"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

const searchUIOffset = 9

var filterStatusOptions = []string{"Tất cả", "Đang tiến hành", "Đã hoàn thành"}
var filterSortOptions = []string{"Mới cập nhật", "Độ phổ biến", "Đánh giá cao", "Tên (A-Z)"}
var filterGenreOptions = []string{"Tất cả", "Action", "Adventure", "Comedy", "Drama", "Fantasy", "Isekai", "Romance", "Shounen", "Webtoons / Manhwa"}

type SearchModel struct {
	input            textinput.Model
	width            int
	height           int
	results          []models.Manga
	favorites        []storage.FavoriteManga
	history          []storage.ReadHistory
	showingFavorites bool
	showingHistory   bool
	showingFeed      bool
	showingFilters   bool
	filterStatus     int
	filterSort       int
	filterGenre      int
	filterCursor     int
	cursor           int
	viewportStart    int
	isSearching      bool
	loadingFavorites bool
	loadingHistory   bool
	err              error
	currentQuery     string
	flashMsg         string

	searchingProviders []string
	providerCounts     map[string]int
	providerErrors     map[string]string

	filterQuery     string
	chapterLanguage string
	systemMsg       string

	providers       []api.MangaProvider
	providerToggles []bool
	showingSources  bool
	sourceCursor    int

	renderer       imgrender.Renderer
	coverCache     map[string]imgrender.RenderedImage
	currentCover   *imgrender.RenderedImage
	currentCoverID string
	coverLoading   bool
	coverGen       int
	coverKey       coverPaintKey
}

func NewSearchModel(providers []api.MangaProvider) SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Nhập tên manga cần tìm..."
	ti.Focus()
	ti.CharLimit = 156
	ti.SetWidth(40)

	toggles := loadSourceToggles(providers)

	return SearchModel{
		input:           ti,
		width:           80,
		height:          24,
		chapterLanguage: "vi",
		providers:       providers,
		providerToggles: toggles,
		showingFeed:     true,
		isSearching:     false,
		renderer:        imgrender.New(),
		coverCache:      make(map[string]imgrender.RenderedImage),
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

// filteredIndices returns the indices of n items whose key contains the
// current filter query, or all indices when the query is empty.
func (m SearchModel) filteredIndices(n int, key func(int) string) []int {
	if m.filterQuery == "" {
		indices := make([]int, n)
		for i := range indices {
			indices[i] = i
		}
		return indices
	}
	var indices []int
	q := strings.ToLower(m.filterQuery)
	for i := 0; i < n; i++ {
		if strings.Contains(strings.ToLower(key(i)), q) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m SearchModel) filteredFavIndices() []int {
	return m.filteredIndices(len(m.favorites), func(i int) string { return m.favorites[i].Title })
}

func (m SearchModel) filteredHistoryIndices() []int {
	return m.filteredIndices(len(m.history), func(i int) string {
		if m.history[i].MangaTitle != "" {
			return m.history[i].MangaTitle
		}
		return m.history[i].MangaID
	})
}

func (m SearchModel) filteredProviderIndices() []int {
	return m.filteredIndices(len(m.providers), func(i int) string { return m.providers[i].Name() })
}

func (m SearchModel) Init() tea.Cmd {
	active := m.activeProviders()
	if len(active) == 0 {
		active = m.providers
	}
	return tea.Batch(
		textinput.Blink,
		api.GlobalLatestCmd(active, 1),
	)
}

func (m SearchModel) handleMouseMsg(msg tea.MouseMsg) (SearchModel, tea.Cmd, bool) {
	mouse := msg.Mouse()
	switch mouse.Button {
	case tea.MouseWheelUp:
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
		}
		if m.moveCursor(-1) {
			return m, m.updateFocusedCover(), true
		}
		return m, nil, true

	case tea.MouseWheelDown:
		if m.showingFilters {
			if m.filterCursor < 4 {
				m.filterCursor++
			}
			return m, nil, true
		}
		if m.showingSources {
			if m.sourceCursor < m.currentListLen()-1 {
				m.sourceCursor++
			}
			return m, nil, true
		}
		if m.moveCursor(1) {
			return m, m.updateFocusedCover(), true
		}
		return m, nil, true

	case tea.MouseLeft:
		if _, ok := msg.(tea.MouseClickMsg); !ok {
			return m, nil, false
		}

		itemIdx := m.viewportStart + (mouse.Y - searchUIOffset)
		if itemIdx >= 0 && itemIdx < m.currentListLen() {
			if m.showingSources {
				m.sourceCursor = itemIdx
				m.toggleCurrentSource()
				return m, nil, true
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

// coverPaintDelay lets the renderer flush the view that reserves the preview
// area before the cover is drawn. Raw output is flushed before the renderer's
// diff within a tick, so an immediate draw can land before the reserving pane
// (and a resize forces a full renderer erase that would wipe it). Deferring one
// frame puts the draw strictly after the render.
const coverPaintDelay = 30 * time.Millisecond

type coverPaintMsg struct{ key coverPaintKey }

// Update wraps message handling with the out-of-band cover paint: whenever the
// cover image or the pane geometry changes, a deferred tea.Raw command clears
// and redraws the preview cover.
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newM, cmd := m.update(msg)
	if key := newM.coverPaintState(); key != m.coverKey {
		newM.coverKey = key
		cmd = tea.Batch(cmd, tea.Tick(coverPaintDelay, func(time.Time) tea.Msg {
			return coverPaintMsg{key: key}
		}))
	}
	return newM, cmd
}

// coverPaintState computes the cover paint key for the current state.
func (m SearchModel) coverPaintState() coverPaintKey {
	_, key := m.buildContent()
	return key
}

// repaintCoverCmd forces the preview cover to be painted again. The reader
// deletes every terminal image when it exits, so a cover on screen before must
// be redrawn when the search screen comes back.
func (m *SearchModel) repaintCoverCmd() tea.Cmd {
	key := m.coverPaintState()
	m.coverKey = key
	return tea.Tick(coverPaintDelay, func(time.Time) tea.Msg {
		return coverPaintMsg{key: key}
	})
}

// setCover records the rendered preview cover and bumps coverGen so the
// out-of-band cover paint is re-emitted.
func (m *SearchModel) setCover(img *imgrender.RenderedImage) {
	if m.currentCover == img {
		return
	}
	m.currentCover = img
	m.coverGen++
}

func (m SearchModel) update(msg tea.Msg) (SearchModel, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if newM, cmd, handled := m.handleMouseMsg(msg); handled {
			return newM, cmd
		}

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
				m.err = errNoSource
				return m, nil
			}
			m.isSearching = true
			m.err = nil
			m.searchingProviders = m.activeProviderNames()
			m.providerCounts = nil
			m.providerErrors = nil
			return m, tea.Batch(cmd, api.GlobalSearchCmd(active, msg.query))
		}
		return m, nil

	case api.MangaSearchResultMsg:
		m.isSearching = false
		m.searchingProviders = nil
		m.providerCounts = msg.ProviderCounts
		m.providerErrors = msg.ProviderErrors
		if !m.showingFeed && len(strings.TrimSpace(m.currentQuery)) < 3 {
			return m, nil
		}
		if len(msg.Manga) == 0 && msg.Err != nil {
			m.results = nil
			m.err = msg.Err
			m.setCover(nil)
			m.currentCoverID = ""
			m.coverLoading = false
			return m, nil
		}
		m.results = msg.Manga
		m.cursor = 0
		m.viewportStart = 0
		m.err = nil
		return m, m.updateFocusedCover()

	case coverDebounceMsg:
		focused, ok := m.focusedManga()
		if ok && focused.ID == msg.mangaID && focused.CoverURL == msg.coverURL {
			boxW, boxH := m.previewBoxSize()
			return m, fetchAndRenderCoverCmd(m.renderer, msg.mangaID, msg.coverURL, focused.Provider, boxW, boxH)
		}
		return m, nil

	case coverRenderedMsg:
		if msg.err == nil {
			if m.coverCache == nil {
				m.coverCache = make(map[string]imgrender.RenderedImage)
			}
			if len(m.coverCache) >= 50 {
				m.coverCache = make(map[string]imgrender.RenderedImage)
			}
			m.coverCache[msg.coverURL] = msg.rendered
			focused, ok := m.focusedManga()
			if ok && focused.ID == msg.mangaID && focused.CoverURL == msg.coverURL {
				m.setCover(&msg.rendered)
				m.coverLoading = false
			}
		} else {
			focused, ok := m.focusedManga()
			if ok && focused.ID == msg.mangaID && focused.CoverURL == msg.coverURL {
				m.setCover(nil)
				m.coverLoading = false
			}
		}
		return m, nil

	case coverPaintMsg:
		if msg.key == m.coverKey {
			return m, tea.Raw(m.coverSequence(msg.key))
		}
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
		coverCmd := m.updateFocusedCover()
		return m, coverCmd
	}

	oldVal := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	newVal := m.input.Value()
	if strings.Contains(newVal, "_Gi=") || strings.Contains(newVal, ";OK") {
		cleaned := strings.ReplaceAll(newVal, "_Gi=1;OK", "")
		cleaned = strings.ReplaceAll(cleaned, "_Gi=", "")
		cleaned = strings.ReplaceAll(cleaned, ";OK", "")
		cleaned = strings.ReplaceAll(cleaned, "_G", "")
		cleaned = strings.ReplaceAll(cleaned, "\\le", "")
		if cleaned != newVal {
			m.input.SetValue(cleaned)
			newVal = cleaned
		}
	}

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
			m.resetSearchResults()
			return m, cmd
		}

		m.currentQuery = trimmed
		if len(trimmed) >= 3 {
			return m, tea.Batch(cmd, debounceSearch(trimmed, 300*time.Millisecond))
		}
		m.resetSearchResults()
	}

	return m, cmd
}

func (m SearchModel) listVisibleItems() int {
	h := m.height
	boxH := h - 8
	if boxH > 22 {
		boxH = 22
	}
	if boxH < 6 {
		boxH = 6
	}
	visible := boxH - 4
	if visible < 1 {
		visible = 1
	}
	return visible
}

func (m SearchModel) currentListLen() int {
	switch {
	case m.showingFavorites:
		return len(m.filteredFavIndices())
	case m.showingHistory:
		return len(m.filteredHistoryIndices())
	case m.showingSources:
		return len(m.filteredProviderIndices())
	default:
		return len(m.results)
	}
}

func (m *SearchModel) adjustViewport() {
	visible := m.listVisibleItems()
	total := m.currentListLen()

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

// moveCursor shifts the list cursor by delta, clamped to the active list.
// It reports whether the cursor moved.
func (m *SearchModel) moveCursor(delta int) bool {
	next := m.cursor + delta
	if next < 0 || next >= m.currentListLen() {
		return false
	}
	m.cursor = next
	m.adjustViewport()
	return true
}

// resetSearchResults clears result-bound state when the query changes.
func (m *SearchModel) resetSearchResults() {
	m.results = nil
	m.cursor = 0
	m.viewportStart = 0
	m.isSearching = false
	m.err = nil
	m.providerCounts = nil
	m.providerErrors = nil
	m.searchingProviders = nil
}

// toggleCurrentSource flips the checkbox under the source cursor and persists it.
func (m *SearchModel) toggleCurrentSource() {
	indices := m.filteredProviderIndices()
	if m.sourceCursor < len(indices) {
		idx := indices[m.sourceCursor]
		m.providerToggles[idx] = !m.providerToggles[idx]
		_ = storage.SaveSources(m.activeProviderNames())
	}
}

func (m SearchModel) focusedManga() (manga models.Manga, ok bool) {
	if m.showingSources {
		return models.Manga{}, false
	}
	if m.showingFavorites {
		indices := m.filteredFavIndices()
		if len(indices) > 0 && m.cursor < len(indices) {
			fav := m.favorites[indices[m.cursor]]
			return models.Manga{
				ID:       fav.MangaID,
				Title:    fav.Title,
				Provider: fav.Provider,
			}, true
		}
		return models.Manga{}, false
	}
	if m.showingHistory {
		indices := m.filteredHistoryIndices()
		if len(indices) > 0 && m.cursor < len(indices) {
			h := m.history[indices[m.cursor]]
			t := h.MangaTitle
			if t == "" {
				t = h.MangaID
			}
			return models.Manga{
				ID:       h.MangaID,
				Title:    t,
				Provider: h.Provider,
			}, true
		}
		return models.Manga{}, false
	}
	if len(m.results) > 0 && m.cursor < len(m.results) {
		res := m.results[m.cursor]
		return res, true
	}
	return models.Manga{}, false
}

func (m *SearchModel) updateFocusedCover() tea.Cmd {
	focused, ok := m.focusedManga()
	if !ok || focused.CoverURL == "" {
		m.setCover(nil)
		m.currentCoverID = ""
		m.coverLoading = false
		return nil
	}
	if m.currentCoverID == focused.ID && (m.currentCover != nil || m.coverLoading) {
		return nil
	}
	m.currentCoverID = focused.ID
	if img, cached := m.coverCache[focused.CoverURL]; cached {
		m.setCover(&img)
		m.coverLoading = false
		return nil
	}
	m.setCover(nil)
	m.coverLoading = true
	return debounceCover(focused.ID, focused.CoverURL, 150*time.Millisecond)
}

func (m SearchModel) previewPaneWidth() int {
	if m.width < 80 {
		return 0
	}
	w := 42
	if m.width >= 120 {
		w = 50
	}
	if m.width >= 150 {
		w = 56
	}
	if w > m.width-50 {
		w = m.width - 50
	}
	return w
}

func (m SearchModel) listPaneWidth() int {
	if m.width < 80 {
		return m.width - 4
	}
	w := 62
	if m.width >= 120 {
		w = 72
	}
	if m.width >= 150 {
		w = 80
	}
	maxW := m.width - m.previewPaneWidth() - 8
	if w > maxW {
		w = maxW
	}
	if w < 36 {
		w = 36
	}
	return w
}

func (m SearchModel) previewBoxSize() (cols, rows int) {
	if m.width < 80 {
		return 0, 0
	}
	previewW := m.previewPaneWidth()
	cols = previewW - 8
	boxH := m.listVisibleItems() + 4
	rows = boxH - 11
	if cols < 12 {
		cols = 12
	}
	if rows < 4 {
		rows = 4
	}
	if rows > 14 {
		rows = 14
	}
	return cols, rows
}

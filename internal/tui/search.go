package tui

import (
	"context"
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

var filterStatusOptions = []string{"Tất cả", "Đang tiến hành", "Đã hoàn thành"}
var filterSortOptions = []string{"Mới cập nhật", "Độ phổ biến", "Đánh giá cao", "Tên (A-Z)"}
var filterGenreOptions = api.FilterGenres

// Rows of the filter modal, top to bottom.
const (
	filterRowStatus = iota
	filterRowSort
	filterRowGenre
	filterRowApply
	filterRowReset
	filterRowCount
)

// searchView is the tab shown on the search screen.
type searchView int

const (
	viewFeed searchView = iota
	viewFavorites
	viewHistory
	viewSources
	viewFilters
	viewCount
)

const searchPlaceholder = "Nhập tên manga cần tìm..."

type SearchModel struct {
	input            textinput.Model
	width            int
	height           int
	results          []models.Manga
	favorites        []storage.FavoriteManga
	history          []storage.ReadHistory
	view             searchView
	listMode         resultMode
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

	filterQuery string
	systemMsg   string

	// pendingSeq identifies the newest result request; snapshots from older
	// requests are ignored (stale-response guard).
	pendingSeq int
	// cancelRequest aborts the in-flight result request when it is replaced.
	cancelRequest context.CancelFunc

	// Paging for the latest/filter lists: page is the last page requested,
	// appendBase holds the earlier pages while the next one streams in.
	page        int
	hasMore     bool
	loadingMore bool
	appendBase  []models.Manga

	providers       []api.MangaProvider
	providerToggles []bool
	sourceCursor    int
	sourceSaveSeq   int

	renderer   imgrender.Renderer
	coverCache map[string]imgrender.RenderedImage
	// coverLookups holds the fav/his entries whose missing cover was looked
	// up this session, so a source without one is asked only once.
	coverLookups   map[string]bool
	currentCover   *imgrender.RenderedImage
	currentCoverID string
	coverLoading   bool
	coverGen       int
	coverKey       coverPaintKey
}

func NewSearchModel(providers []api.MangaProvider) SearchModel {
	ti := textinput.New()
	ti.Placeholder = searchPlaceholder
	ti.Focus()
	ti.CharLimit = 156
	ti.SetWidth(40)

	toggles := loadSourceToggles(providers)

	return SearchModel{
		input:           ti,
		width:           80,
		height:          24,
		providers:       providers,
		providerToggles: toggles,
		page:            1,
		renderer:        imgrender.New(),
		coverCache:      make(map[string]imgrender.RenderedImage),
	}
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
		return tea.Batch(
			textinput.Blink,
			func() tea.Msg { return searchInitMsg{err: errNoSource} },
		)
	}
	return tea.Batch(
		textinput.Blink,
		func() tea.Msg { return searchInitMsg{} },
		searchRequestCmd(0, latestAllCmd(context.Background(), active, 1)),
	)
}

// searchInitMsg marks the initial feed load as in progress once the model is
// running; the constructor stays quiet so tests can build a SearchModel without
// a pending search.
type searchInitMsg struct {
	err error
}

// Update wraps message handling with the out-of-band cover paint: whenever the
// cover image or the pane geometry changes, a deferred tea.Raw command clears
// and redraws the preview cover.
func (m SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newM, cmd := m.update(msg)
	if key := newM.coverPaintState(); key != m.coverKey {
		// Sixel images are plain pixels in cells that the renderer thinks are
		// blank, so they stay until erased: wipe the old area before the new
		// frame paints (Kitty images go with the delete in the same sequence).
		if old := m.coverKey; old.isSplit && !sameCoverArea(old, key) {
			old.draw = false
			cmd = tea.Batch(tea.Raw(m.coverSequence(old)), cmd)
		}
		newM.coverKey = key
		cmd = tea.Batch(cmd, tea.Tick(coverPaintDelay, func(time.Time) tea.Msg {
			return coverPaintMsg{key: key}
		}))
	}
	return newM, cmd
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
		return m, m.updateFocusedCover()

	case historyLoadedMsg:
		m.loadingHistory = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.history = msg.history
		m.cursor = 0
		m.viewportStart = 0
		return m, m.updateFocusedCover()

	case searchInitMsg:
		if msg.err != nil {
			m.err = msg.err
			m.isSearching = false
			return m, nil
		}
		m.isSearching = true
		m.searchingProviders = m.activeProviderNames()
		return m, nil

	case searchTriggerMsg:
		if msg.query == m.currentQuery && len(strings.TrimSpace(msg.query)) >= 3 {
			return m, m.requestResults(resultsSearch, 1, false)
		}
		return m, nil

	case resultsMsg:
		if msg.RequestID != m.pendingSeq {
			return m, nil
		}
		m.providerCounts = msg.ProviderCounts
		m.providerErrors = msg.ProviderErrors
		appending := m.loadingMore

		if msg.Stream != nil {
			next := searchRequestCmd(msg.RequestID, msg.Stream.Next())
			if len(msg.Manga) == 0 {
				return m, next
			}
			m.results = m.mergeResults(msg.Manga)
			if !appending {
				m.cursor = 0
				m.viewportStart = 0
			}
			m.err = nil
			return m, tea.Batch(next, m.updateFocusedCover())
		}

		m.isSearching = false
		m.loadingMore = false
		m.searchingProviders = nil
		m.cancelRequest = nil
		// Paged lists end when a page brings nothing new.
		m.hasMore = m.listMode != resultsSearch && len(m.mergeResults(msg.Manga)) > len(m.appendBase)
		if appending {
			m.results = m.mergeResults(msg.Manga)
			m.appendBase = nil
			if msg.Err != nil {
				// Every source failed: ask for this page again on the next try.
				m.page--
				m.hasMore = true
				m.flashMsg = fmt.Sprintf("Lỗi tải thêm: %v", msg.Err)
				return m, clearFlashAfter(3 * time.Second)
			}
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

	case coverFoundMsg:
		return m.applyFoundCover(msg)

	case coverDebounceMsg:
		focused, ok := m.focusedManga()
		if ok && focused.ID == msg.mangaID && focused.CoverURL == msg.coverURL {
			if msg.coverURL == "" {
				return m, m.lookupCover(focused)
			}
			boxW, boxH := m.previewBoxSize()
			return m, fetchAndRenderCoverCmd(m.renderer, m.providerByName(focused.Provider), msg.mangaID, msg.coverURL, boxW, boxH)
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
				// A saved cover URL can go stale (re-uploaded covers): ask again.
				if m.canLookupCover(focused) {
					return m, m.lookupCover(focused)
				}
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

	case storageSavedMsg:
		if msg.err != nil {
			m.flashMsg = fmt.Sprintf("Lỗi: %v", msg.err)
			return m, clearFlashAfter(3 * time.Second)
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

	if oldVal != newVal {
		trimmed := strings.TrimSpace(newVal)
		m.systemMsg = ""

		if m.view == viewFavorites || m.view == viewHistory || m.view == viewSources {
			m.filterQuery = trimmed
			m.cursor = 0
			m.viewportStart = 0
			m.sourceCursor = 0
			return m, tea.Batch(cmd, m.updateFocusedCover())
		}

		// Any change makes in-flight results belong to an older request.
		m.cancelInFlight()
		m.pendingSeq++
		m.loadingMore = false
		m.appendBase = nil

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

func (m SearchModel) focusedManga() (manga models.Manga, ok bool) {
	switch m.view {
	case viewSources:
		return models.Manga{}, false
	case viewFavorites:
		indices := m.filteredFavIndices()
		if len(indices) > 0 && m.cursor < len(indices) {
			fav := m.favorites[indices[m.cursor]]
			return models.Manga{
				ID:       fav.MangaID,
				Title:    fav.Title,
				Provider: fav.Provider,
				CoverURL: fav.CoverURL,
			}, true
		}
		return models.Manga{}, false
	case viewHistory:
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
				CoverURL: h.CoverURL,
			}, true
		}
		return models.Manga{}, false
	}
	if m.cursor >= 0 && m.cursor < len(m.results) {
		return m.results[m.cursor], true
	}
	return models.Manga{}, false
}

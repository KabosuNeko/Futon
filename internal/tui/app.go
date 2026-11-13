package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/storage"
	"github.com/KabosuNeko/Futon/internal/updater"
)

type ViewMangaMsg struct {
	MangaID      string
	Title        string
	ProviderName string
	// ResumeChapterID opens this chapter in the reader as soon as the
	// chapter list loads (used when continuing from history).
	ResumeChapterID string
}

type BackToSearchMsg struct{}

type ViewChapterMsg struct {
	MangaID           string
	MangaTitle        string
	ChapterID         string
	ChapterNumber     string
	AllChapterIDs     []string
	AllChapterNumbers []string
	ChapterIndex      int
	StartPageIndex    int
}

type BackToChaptersMsg struct{}

type UpdateAvailableMsg struct {
	Version string
}

// RequestUpdateMsg asks for a manual update check (/update). Installing is
// left to the `futon update` CLI: it may prompt for sudo, which does not
// belong inside the TUI.
type RequestUpdateMsg struct{}

// chaptersReadyMsg switches the app to the chapter list after the search
// cover has been cleared from the terminal.
type chaptersReadyMsg struct{}

type UpdateCheckedMsg struct {
	Available bool
	Version   string
	Err       error
}

type appState int

const (
	stateSearch appState = iota
	stateChapters
	stateReader
)

type AppModel struct {
	state           appState
	search          SearchModel
	chapter         ChapterListModel
	reader          ReaderModel
	providers       []api.MangaProvider
	currentProvider api.MangaProvider

	appVersion      string
	updateAvailable bool
	updateVersion   string

	// Last known terminal size, handed to screens created after the resize.
	width, height int
	// Saved reader choices (webtoon mode, hidden hints) for the first reader.
	readerFitWidth   bool
	readerHideFooter bool
}

func NewAppModel(version string) AppModel {
	providers := []api.MangaProvider{
		api.NewMangaDexProvider(),
		api.NewTruyenQQProvider(),
		api.NewBaoTangTruyenProvider(),
		api.NewCuuTruyenProvider(),
	}

	// Fold entries of merged sources into their successor before anything
	// reads them; failing here only means old entries stay as they were.
	_ = storage.MigrateEntries(migrateLegacyEntry)

	search := NewSearchModel(providers)
	// Restore saved choices; an unreadable file just keeps the defaults.
	ud, err := storage.LoadUserData()
	if err != nil {
		ud = &storage.UserData{}
	}
	if ud.ChapterLang != "" {
		search.setChapterLanguage(ud.ChapterLang)
	}

	return AppModel{
		state:            stateSearch,
		search:           search,
		providers:        providers,
		currentProvider:  providers[0],
		appVersion:       version,
		readerFitWidth:   ud.ReaderFitWidth,
		readerHideFooter: ud.ReaderHideFooter,
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.search.Init(), checkForUpdateCmd(m.appVersion))
}

func checkForUpdateCmd(currentVersion string) tea.Cmd {
	return func() tea.Msg {
		available, version, err := updater.CheckForUpdate(currentVersion)
		if err != nil || !available {
			return nil
		}
		return UpdateAvailableMsg{Version: version}
	}
}

func checkUpdateForManualCmd(currentVersion string) tea.Cmd {
	return func() tea.Msg {
		available, version, err := updater.CheckForUpdate(currentVersion)
		if err != nil {
			return UpdateCheckedMsg{Err: err}
		}
		if !available {
			return UpdateCheckedMsg{Available: false}
		}
		return UpdateCheckedMsg{Available: true, Version: version}
	}
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		sm, sc := m.search.Update(msg)
		m.search = sm.(SearchModel)
		cm, cc := m.chapter.Update(msg)
		m.chapter = cm.(ChapterListModel)
		rm, rc := m.reader.Update(msg)
		m.reader = rm.(ReaderModel)
		return m, tea.Batch(sc, cc, rc)

	case ViewMangaMsg:
		provider, ok := m.findProvider(msg.ProviderName)
		if !ok {
			// A favorite/history entry from a source that was removed (e.g.
			// OTruyen): its ID means nothing to the remaining providers.
			m.search.systemMsg = fmt.Sprintf("Nguồn %s không còn được hỗ trợ.", msg.ProviderName)
			return m, nil
		}
		m.currentProvider = provider
		m.chapter = NewChapterListModel(msg.MangaID, msg.Title, m.currentProvider)
		m.chapter.setSize(m.width, m.height)
		m.chapter.resumeChapterID = msg.ResumeChapterID
		// Clear the preview cover while the search view is still current, then
		// switch state: a clear emitted after the switch would sit on top of the
		// freshly rendered chapter list.
		return m, tea.Sequence(
			m.search.clearCoverCmd(),
			func() tea.Msg { return chaptersReadyMsg{} },
		)

	case chaptersReadyMsg:
		m.state = stateChapters
		return m, m.chapter.Init()

	case BackToSearchMsg:
		m.state = stateSearch
		return m, m.search.repaintCoverCmd()

	case ViewChapterMsg:
		// Keep the reader choices across chapters: the previous reader holds
		// the latest ones (w or ctrl+h may have been pressed since launch).
		if m.reader.renderer != nil {
			m.readerFitWidth = m.reader.fitWidth
			m.readerHideFooter = m.reader.hideFooter
		}
		m.state = stateReader
		m.reader = NewReaderModel(msg.MangaID, msg.MangaTitle, msg.ChapterID, msg.ChapterNumber, msg.AllChapterIDs, msg.ChapterIndex, msg.StartPageIndex, m.currentProvider)
		m.reader.allChapterNumbers = msg.AllChapterNumbers
		m.reader.fitWidth = m.readerFitWidth
		m.reader.hideFooter = m.readerHideFooter
		if m.width > 0 && m.height > 0 {
			m.reader.width, m.reader.height = m.width, m.height
		}
		return m, m.reader.Init()

	case batchExportMsg:
		// Export progress belongs to the chapter list even while reading.
		cm, cmd := m.chapter.Update(msg)
		m.chapter = cm.(ChapterListModel)
		return m, cmd

	case BackToChaptersMsg:
		m.state = stateChapters
		m.chapter.reloadHistory() // the reader just saved a new position
		return m, nil

	case UpdateAvailableMsg:
		m.updateAvailable = true
		m.updateVersion = msg.Version
		return m, nil

	case RequestUpdateMsg:
		if m.state != stateSearch {
			return m, nil
		}
		if m.appVersion == "dev" {
			m.search.systemMsg = "Bản dev (build từ source): chạy `futon update` để cài bản release mới nhất."
			return m, nil
		}
		m.search.systemMsg = "Đang kiểm tra cập nhật..."
		return m, checkUpdateForManualCmd(m.appVersion)

	case UpdateCheckedMsg:
		if msg.Err != nil {
			m.search.systemMsg = fmt.Sprintf("Kiểm tra cập nhật lỗi: %v", msg.Err)
			return m, nil
		}
		if !msg.Available {
			m.search.systemMsg = "Đã là bản mới nhất."
			return m, nil
		}
		m.updateAvailable = true
		m.updateVersion = msg.Version
		m.search.systemMsg = fmt.Sprintf("Có bản %s. Thoát rồi chạy `futon update` để cài.", msg.Version)
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// The reader owns its quit path (save + flush history first).
			if m.state == stateReader {
				rm, cmd := m.reader.Update(msg)
				m.reader = rm.(ReaderModel)
				return m, cmd
			}
			return m, tea.Quit
		}
	}

	var newModel tea.Model
	var cmd tea.Cmd
	switch m.state {
	case stateSearch:
		newModel, cmd = m.search.Update(msg)
		m.search = newModel.(SearchModel)
		return m, cmd
	case stateChapters:
		newModel, cmd = m.chapter.Update(msg)
		m.chapter = newModel.(ChapterListModel)
		return m, cmd
	case stateReader:
		newModel, cmd = m.reader.Update(msg)
		m.reader = newModel.(ReaderModel)
		return m, cmd
	default:
		return m, nil
	}
}

// mergedSources maps source names that were folded into another source.
// TruyenGG is a mirror of TruyenQQ (same catalog and paths); FoxTruyen's
// domain was parked and its users had been moved to TruyenGG.
var mergedSources = map[string]string{
	"TruyenGG":  "TruyenQQ",
	"FoxTruyen": "TruyenQQ",
}

// migrateLegacyEntry moves TruyenGG entries to TruyenQQ and turns the
// absolute, mirror-specific URLs that TruyenQQ-family entries used to store
// into the mirror-independent path IDs. FoxTruyen entries are left alone:
// its IDs belong to a different site.
func migrateLegacyEntry(provider, mangaID, chapterID string) (string, string, string) {
	switch provider {
	case "TruyenGG", "TruyenQQ":
		return "TruyenQQ", api.TruyenQQPath(mangaID), api.TruyenQQPath(chapterID)
	}
	return provider, mangaID, chapterID
}

// findProvider resolves a provider by name. An empty name (entries saved
// before the provider was recorded) falls back to the first provider; an
// unknown name is a source that no longer exists and is reported as missing.
func (m *AppModel) findProvider(name string) (api.MangaProvider, bool) {
	if successor, merged := mergedSources[name]; merged {
		name = successor
	}
	for _, p := range m.providers {
		if p.Name() == name {
			return p, true
		}
	}
	if name == "" && len(m.providers) > 0 {
		return m.providers[0], true
	}
	return nil, false
}

func (m AppModel) View() tea.View {
	var updateBanner string
	if m.updateAvailable && m.state == stateSearch {
		updateBanner = fmt.Sprintf("[!] Đã có bản cập nhật %s. Thoát rồi chạy `futon update` để cài.", m.updateVersion)
	}

	view := ""
	switch m.state {
	case stateSearch:
		view = m.search.View().Content
	case stateChapters:
		view = m.chapter.View().Content
	case stateReader:
		view = m.reader.View().Content
	default:
		view = "Unknown state"
	}

	if updateBanner != "" {
		view = view + "\n" + updateBanner
	}

	return appView(view)
}

// appView wraps rendered content with the terminal features every futon screen needs.
func appView(content string) tea.View {
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

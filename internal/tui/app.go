package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/api"
)

type ViewMangaMsg struct {
	MangaID string
	Title   string
}

type BackToSearchMsg struct{}

type ViewChapterMsg struct {
	MangaID        string
	MangaTitle     string
	ChapterID      string
	ChapterNumber  string
	AllChapterIDs  []string // toàn bộ chapter ID để auto-next
	ChapterIndex   int      // vị trí chapter hiện tại
	StartPageIndex int      // -1: không chỉ định (dùng lịch sử hoặc 0); -2: trang cuối; >=0: trang cụ thể
}

type BackToChaptersMsg struct{}

type appState int

const (
	stateSearch appState = iota
	stateChapters
	stateReader
)

// AppModel đóng vai trò Router, điều phối giữa các màn hình con.
type AppModel struct {
	state           appState
	search          SearchModel
	chapter         ChapterListModel
	reader          ReaderModel
	providers       []api.MangaProvider
	currentProvider api.MangaProvider
}

// NewAppModel tạo AppModel với màn hình tìm kiếm mặc định.
func NewAppModel() AppModel {
	providers := []api.MangaProvider{
		api.NewOTruyenProvider(),
		api.NewMangaDexProvider(),
	}

	return AppModel{
		state:           stateSearch,
		search:          NewSearchModel(providers),
		providers:       providers,
		currentProvider: providers[0],
	}
}

func (m AppModel) Init() tea.Cmd {
	return m.search.Init()
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		var searchCmd, chapterCmd, readerCmd tea.Cmd
		var sm, cm, rm tea.Model
		sm, searchCmd = m.search.Update(msg)
		m.search = sm.(SearchModel)
		cm, chapterCmd = m.chapter.Update(msg)
		m.chapter = cm.(ChapterListModel)
		rm, readerCmd = m.reader.Update(msg)
		m.reader = rm.(ReaderModel)
		m.syncProvider()
		return m, tea.Batch(searchCmd, chapterCmd, readerCmd)

	case ViewMangaMsg:
		m.state = stateChapters
		m.chapter = NewChapterListModel(msg.MangaID, msg.Title, m.currentProvider)
		return m, m.chapter.Init()

	case BackToSearchMsg:
		m.state = stateSearch
		return m, nil

	case ViewChapterMsg:
		m.state = stateReader
		m.reader = NewReaderModel(msg.MangaID, msg.MangaTitle, msg.ChapterID, msg.ChapterNumber, msg.AllChapterIDs, msg.ChapterIndex, msg.StartPageIndex, m.currentProvider)
		return m, m.reader.Init()

	case BackToChaptersMsg:
		m.state = stateChapters
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	switch m.state {
	case stateSearch:
		var cmd tea.Cmd
		var newModel tea.Model
		newModel, cmd = m.search.Update(msg)
		m.search = newModel.(SearchModel)
		m.syncProvider()
		return m, cmd
	case stateChapters:
		var cmd tea.Cmd
		var newModel tea.Model
		newModel, cmd = m.chapter.Update(msg)
		m.chapter = newModel.(ChapterListModel)
		return m, cmd
	case stateReader:
		var cmd tea.Cmd
		var newModel tea.Model
		newModel, cmd = m.reader.Update(msg)
		m.reader = newModel.(ReaderModel)
		return m, cmd
	default:
		return m, nil
	}
}

func (m *AppModel) syncProvider() {
	if p := m.search.CurrentProvider(); p != nil {
		m.currentProvider = p
	}
}

func (m AppModel) View() string {
	switch m.state {
	case stateSearch:
		return m.search.View()
	case stateChapters:
		return m.chapter.View()
	case stateReader:
		return m.reader.View()
	default:
		return "Unknown state"
	}
}

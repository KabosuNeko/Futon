package tui

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/updater"
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
	AllChapterIDs  []string
	ChapterIndex   int
	StartPageIndex int
}

type BackToChaptersMsg struct{}

type UpdateAvailableMsg struct {
	Version string
	URL     string
}

type UpdateReadyMsg struct {
	Err error
}

type appState int

const (
	stateSearch appState = iota
	stateChapters
	stateReader
	stateUpdating
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
	updateURL       string
	updateError     error
	updateSuccess   bool
}

func NewAppModel(version string) AppModel {
	providers := []api.MangaProvider{
		api.NewOTruyenProvider(),
		api.NewMangaDexProvider(),
		api.NewTruyenQQProvider(),
	}

	return AppModel{
		state:           stateSearch,
		search:          NewSearchModel(providers),
		providers:       providers,
		currentProvider: providers[0],
		appVersion:      version,
	}
}

func (m AppModel) Init() tea.Cmd {
	return tea.Batch(m.search.Init(), checkForUpdateCmd(m.appVersion))
}

func checkForUpdateCmd(currentVersion string) tea.Cmd {
	return func() tea.Msg {
		available, version, url, err := updater.CheckForUpdate(currentVersion)
		if err != nil || !available {
			return nil
		}
		return UpdateAvailableMsg{Version: version, URL: url}
	}
}

func runInstallScriptCmd() tea.Cmd {
	cmdStr := "curl -sSL https://raw.githubusercontent.com/KabosuNeko/Futon/main/install.sh -o /tmp/futon_install.sh && bash /tmp/futon_install.sh && rm /tmp/futon_install.sh"
	c := exec.Command("bash", "-c", cmdStr)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return UpdateReadyMsg{Err: err}
	})
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		sm, sc := m.search.Update(msg)
		m.search = sm.(SearchModel)
		cm, cc := m.chapter.Update(msg)
		m.chapter = cm.(ChapterListModel)
		rm, rc := m.reader.Update(msg)
		m.reader = rm.(ReaderModel)
		m.syncProvider()
		return m, tea.Batch(sc, cc, rc)

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

	case UpdateAvailableMsg:
		m.updateAvailable = true
		m.updateVersion = msg.Version
		m.updateURL = msg.URL
		return m, nil

	case UpdateReadyMsg:
		if msg.Err != nil {
			m.updateError = msg.Err
		}
		m.updateSuccess = true
		m.updateAvailable = false
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "U":
			if m.updateAvailable && m.state == stateSearch {
				m.state = stateUpdating
				return m, runInstallScriptCmd()
			}
			return m, nil
		}
	}

	if m.state == stateUpdating {
		return m, nil
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
	if m.state == stateUpdating {
		return "Đang cập nhật...\n"
	}

	var updateBanner string
	if m.updateAvailable && m.state == stateSearch {
		updateBanner = fmt.Sprintf("[!] Đã có bản cập nhật %s. Nhấn 'U' để tự động cài đặt.", m.updateVersion)
	}

	view := ""
	switch m.state {
	case stateSearch:
		view = m.search.View()
	case stateChapters:
		view = m.chapter.View()
	case stateReader:
		view = m.reader.View()
	default:
		view = "Unknown state"
	}

	if updateBanner != "" {
		view = view + "\n" + updateBanner
	}

	if m.updateSuccess {
		view = view + "\nCập nhật thành công! Vui lòng thoát (Ctrl+C) và mở lại futon."
	}

	return view
}

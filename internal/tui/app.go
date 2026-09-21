package tui

import (
	"fmt"
	"os"

	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/storage"
	"github.com/KabosuNeko/Futon/internal/updater"
	tea "github.com/charmbracelet/bubbletea"
)

type ViewMangaMsg struct {
	MangaID      string
	Title        string
	ProviderName string
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

type UpdateReadyMsg struct {
	Err error
}

type RequestUpdateMsg struct{}

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
	updateSuccess   bool
}

func NewAppModel(version string) AppModel {
	providers := []api.MangaProvider{
		api.NewOTruyenProvider(),
		api.NewMangaDexProvider(),
		api.NewTruyenQQProvider(),
		api.NewFoxTruyenProvider(),
		api.NewBaoTangTruyenProvider(),
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

func runInstallScriptCmd() tea.Cmd {
	c := updater.InstallScriptCommand()
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
		return m, tea.Batch(sc, cc, rc)

	case ViewMangaMsg:
		m.state = stateChapters
		m.currentProvider = m.findProvider(msg.ProviderName)
		m.chapter = NewChapterListModel(msg.MangaID, msg.Title, m.currentProvider)
		return m, m.chapter.Init()

	case BackToSearchMsg:
		m.state = stateSearch
		return m, nil

	case ViewChapterMsg:
		m.state = stateReader
		m.reader = NewReaderModel(msg.MangaID, msg.MangaTitle, msg.ChapterID, msg.ChapterNumber, msg.AllChapterIDs, msg.ChapterIndex, msg.StartPageIndex, m.currentProvider)
		m.reader.allChapterNumbers = msg.AllChapterNumbers
		return m, m.reader.Init()

	case BackToChaptersMsg:
		m.state = stateChapters
		return m, nil

	case UpdateAvailableMsg:
		m.updateAvailable = true
		m.updateVersion = msg.Version
		return m, nil

	case RequestUpdateMsg:
		if m.state != stateSearch {
			return m, nil
		}
		if m.updateAvailable {
			m.state = stateUpdating
			return m, runInstallScriptCmd()
		}
		if m.appVersion == "dev" {
			m.state = stateUpdating
			return m, runInstallScriptCmd()
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
		m.state = stateUpdating
		return m, runInstallScriptCmd()

	case UpdateReadyMsg:
		m.updateSuccess = true
		m.updateAvailable = false
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.state == stateReader && m.reader.mangaID != "" && m.reader.chapterID != "" {
				providerName := ""
				if m.reader.provider != nil {
					providerName = m.reader.provider.Name()
				}
				return m, tea.Sequence(
					storage.SaveHistoryCmd(m.reader.mangaID, m.reader.mangaTitle, providerName, m.reader.chapterID, m.reader.chapterNumber, m.reader.currentIdx),
					storage.FlushHistoryCmd(),
					tea.Quit,
				)
			}
			return m, tea.Quit
		case "ctrl+u":
			if m.state == stateSearch {
				return m.Update(RequestUpdateMsg{})
			}
			return m, nil
		}
	}

	if m.state == stateUpdating {
		return m, nil
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

func (m *AppModel) findProvider(name string) api.MangaProvider {
	for _, p := range m.providers {
		if p.Name() == name {
			return p
		}
	}
	if len(m.providers) > 0 {
		return m.providers[0]
	}
	return nil
}

func (m AppModel) View() string {
	if m.state == stateUpdating {
		return "Đang cập nhật...\n"
	}

	var updateBanner string
	if m.updateAvailable && m.state == stateSearch {
		updateBanner = fmt.Sprintf("[!] Đã có bản cập nhật %s. Nhấn Ctrl+u để tự động cài đặt.", m.updateVersion)
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

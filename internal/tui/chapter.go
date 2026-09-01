package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unicode"

	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/export"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/storage"
	tea "github.com/charmbracelet/bubbletea"
)

type ChapterListModel struct {
	mangaID       string
	mangaTitle    string
	provider      api.MangaProvider
	chapters      []models.Chapter
	cursor        int
	viewportStart int
	inputBuffer   string
	loading       bool
	flashMsg      string
	err           error
	width         int
	height        int
}

type favoriteSavedMsg struct {
	err error
}

type cbzExportedMsg struct {
	path string
	err  error
}

type batchExportProgressMsg struct {
	current int
	total   int
	err     error
	done    bool
	destDir string
}

func exportAllChaptersCmd(provider api.MangaProvider, mangaTitle string, chapters []models.Chapter) tea.Cmd {
	return func() tea.Msg {
		cleanTitle := export.SanitizeFilename(mangaTitle)
		baseDir, err := export.GetDefaultExportDir()
		if err != nil {
			baseDir = "."
		}
		mangaDir := filepath.Join(baseDir, cleanTitle)
		if err := os.MkdirAll(mangaDir, 0755); err != nil {
			return batchExportProgressMsg{err: fmt.Errorf("tạo thư mục truyện: %w", err), done: true}
		}

		successCount := 0
		for _, ch := range chapters {
			urls, err := provider.FetchPages(ch.ID)
			if err != nil || len(urls) == 0 {
				continue
			}
			_, err = export.ExportChapterURLsToCBZ(cleanTitle, ch.Number, urls, "", "", mangaDir)
			if err == nil {
				successCount++
			}
		}

		return batchExportProgressMsg{
			done:    true,
			destDir: mangaDir,
			total:   len(chapters),
			current: successCount,
		}
	}
}

func exportChapterCmd(provider api.MangaProvider, chapterID, mangaTitle, chapterNumber string) tea.Cmd {
	return func() tea.Msg {
		urls, err := provider.FetchPages(chapterID)
		if err != nil {
			return cbzExportedMsg{err: fmt.Errorf("lấy link ảnh: %w", err)}
		}
		path, err := export.ExportChapterURLsToCBZ(mangaTitle, chapterNumber, urls, "", "", "")
		return cbzExportedMsg{path: path, err: err}
	}
}

func NewChapterListModel(mangaID, mangaTitle string, provider api.MangaProvider) ChapterListModel {
	return ChapterListModel{
		mangaID:    mangaID,
		mangaTitle: mangaTitle,
		provider:   provider,
		loading:    true,
		width:      80,
		height:     24,
	}
}

func (m ChapterListModel) Init() tea.Cmd {
	return api.FetchChaptersCmd(m.provider, m.mangaID)
}

func (m ChapterListModel) selectCurrentChapter() (ChapterListModel, tea.Cmd) {
	if len(m.chapters) > 0 && m.cursor >= 0 && m.cursor < len(m.chapters) {
		ch := m.chapters[m.cursor]
		allIDs := make([]string, len(m.chapters))
		allNumbers := make([]string, len(m.chapters))
		for i, c := range m.chapters {
			allIDs[i] = c.ID
			allNumbers[i] = c.Number
		}
		return m, func() tea.Msg {
			return ViewChapterMsg{
				MangaID:           m.mangaID,
				MangaTitle:        m.mangaTitle,
				ChapterID:         ch.ID,
				ChapterNumber:     ch.Number,
				AllChapterIDs:     allIDs,
				AllChapterNumbers: allNumbers,
				ChapterIndex:      m.cursor,
				StartPageIndex:    -1,
			}
		}
	}
	return m, nil
}

func (m ChapterListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.cursor > 0 {
				m.cursor--
				m.adjustViewport()
			}
			return m, nil
		case tea.MouseButtonWheelDown:
			if m.cursor < len(m.chapters)-1 {
				m.cursor++
				m.adjustViewport()
			}
			return m, nil
		case tea.MouseButtonLeft:
			if msg.Action != tea.MouseActionPress {
				return m, nil
			}
			// Header/margin offset in chapter list card is ~6 lines
			itemIdx := m.viewportStart + (msg.Y - 6)
			if itemIdx >= 0 && itemIdx < len(m.chapters) {
				if itemIdx == m.cursor {
					return m.selectCurrentChapter()
				}
				m.cursor = itemIdx
				m.adjustViewport()
			}
			return m, nil
		}

	case tea.KeyMsg:
		s := msg.String()

		switch s {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			if m.inputBuffer != "" {
				m.inputBuffer = ""
				return m, nil
			}
			return m, func() tea.Msg { return BackToSearchMsg{} }

		case "ctrl+f":
			if m.mangaID == "" {
				return m, nil
			}
			m.flashMsg = fmt.Sprintf("Đã thêm \"%s\" vào Yêu thích", m.mangaTitle)
			return m, tea.Batch(
				func() tea.Msg {
					err := storage.AddFavorite(storage.FavoriteManga{
						MangaID:  m.mangaID,
						Title:    m.mangaTitle,
						Provider: m.provider.Name(),
					})
					return favoriteSavedMsg{err: err}
				},
				clearFlashAfter(2*time.Second),
			)

		case "ctrl+e":
			if len(m.chapters) > 0 && m.cursor >= 0 && m.cursor < len(m.chapters) {
				ch := m.chapters[m.cursor]
				m.flashMsg = fmt.Sprintf("Đang xuất CBZ Chapter %s...", ch.Number)
				return m, exportChapterCmd(m.provider, ch.ID, m.mangaTitle, ch.Number)
			}
			return m, nil

		case "ctrl+a":
			if len(m.chapters) > 0 {
				m.flashMsg = fmt.Sprintf("Đang tải & xuất toàn bộ %d Chapters ra CBZ...", len(m.chapters))
				return m, exportAllChaptersCmd(m.provider, m.mangaTitle, m.chapters)
			}
			return m, nil

		case "g", "home":
			m.cursor = 0
			m.adjustViewport()
			return m, nil

		case "G", "end":
			if len(m.chapters) > 0 {
				m.cursor = len(m.chapters) - 1
				m.adjustViewport()
			}
			return m, nil

		case "up":
			if m.cursor > 0 {
				m.cursor--
				m.adjustViewport()
			}
			return m, nil

		case "down":
			if m.cursor < len(m.chapters)-1 {
				m.cursor++
				m.adjustViewport()
			}
			return m, nil

		case "enter":
			if m.inputBuffer != "" {
				if !m.jumpToChapter(m.inputBuffer) {
					m.flashMsg = fmt.Sprintf("Không tìm thấy chapter %s", m.inputBuffer)
					m.inputBuffer = ""
					return m, clearFlashAfter(2 * time.Second)
				}
				m.inputBuffer = ""
				return m, nil
			}

			return m.selectCurrentChapter()

		case "backspace":
			if m.inputBuffer != "" {
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			}
			return m, nil
		}

		if len(s) == 1 {
			r := rune(s[0])
			if unicode.IsDigit(r) || r == '.' {
				m.inputBuffer += s
				return m, nil
			}
		}

	case cbzExportedMsg:
		if msg.err != nil {
			m.flashMsg = fmt.Sprintf("Lỗi xuất CBZ: %v", msg.err)
		} else {
			m.flashMsg = fmt.Sprintf("Đã xuất CBZ: %s", msg.path)
		}
		return m, clearFlashAfter(3 * time.Second)

	case batchExportProgressMsg:
		if msg.err != nil {
			m.flashMsg = fmt.Sprintf("Lỗi xuất toàn bộ CBZ: %v", msg.err)
		} else {
			m.flashMsg = fmt.Sprintf("Đã xuất %d/%d chapters vào %s", msg.current, msg.total, msg.destDir)
		}
		return m, clearFlashAfter(5 * time.Second)

	case favoriteSavedMsg:
		if msg.err != nil {
			m.flashMsg = fmt.Sprintf("Lỗi lưu yêu thích: %v", msg.err)
		}
		return m, nil

	case clearFlashMsg:
		m.flashMsg = ""
		return m, nil

	case api.ChapterListMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.chapters = msg.Chapters
		m.cursor = 0
		m.viewportStart = 0
		m.err = nil
		m.restoreHistoryPosition()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustViewport()
	}

	return m, nil
}

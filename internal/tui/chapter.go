package tui

import (
	"context"
	"fmt"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/export"
	"github.com/KabosuNeko/Futon/internal/models"
	"github.com/KabosuNeko/Futon/internal/storage"
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
	// history is the reading position for this manga, loaded with the
	// chapter list and refreshed when the reader returns, so View stays pure.
	history *storage.ReadHistory
	// resumeChapterID, when set, opens that chapter once the list loads.
	resumeChapterID string
	// cancelExport stops a running ctrl+a export; nil when none is running.
	cancelExport context.CancelFunc
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
	return fetchChaptersCmd(m.provider, m.mangaID)
}

func (m ChapterListModel) selectCurrentChapter() (ChapterListModel, tea.Cmd) {
	if len(m.chapters) > 0 && m.cursor >= 0 && m.cursor < len(m.chapters) {
		ch := m.chapters[m.cursor]
		allIDs := make([]string, len(m.chapters))
		allNumbers := make([]string, len(m.chapters))
		for i, c := range m.chapters {
			allIDs[i] = c.ID
			// HTML sources have no chapter numbers; the label keeps the
			// reader, history and exports readable for them.
			allNumbers[i] = chapterLabel(c)
		}
		return m, func() tea.Msg {
			return ViewChapterMsg{
				MangaID:           m.mangaID,
				MangaTitle:        m.mangaTitle,
				ChapterID:         ch.ID,
				ChapterNumber:     chapterLabel(ch),
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
		mouse := msg.Mouse()
		switch mouse.Button {
		case tea.MouseWheelUp:
			if m.cursor > 0 {
				m.cursor--
				m.adjustViewport()
			}
			return m, nil
		case tea.MouseWheelDown:
			if m.cursor < len(m.chapters)-1 {
				m.cursor++
				m.adjustViewport()
			}
			return m, nil
		case tea.MouseLeft:
			if _, ok := msg.(tea.MouseClickMsg); !ok {
				return m, nil
			}
			top := m.listTopRow()
			if top == 0 {
				return m, nil // only the too-small notice is on screen
			}
			itemIdx := m.viewportStart + (mouse.Y - top)
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
		case "esc":
			if m.inputBuffer != "" {
				m.inputBuffer = ""
				return m, nil
			}
			if m.cancelExport != nil {
				m.cancelExport()
				m.cancelExport = nil
				m.flashMsg = "Đang hủy xuất CBZ..."
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
					return storageSavedMsg{err: err}
				},
				clearFlashAfter(2*time.Second),
			)

		case "ctrl+e":
			if len(m.chapters) > 0 && m.cursor >= 0 && m.cursor < len(m.chapters) {
				ch := m.chapters[m.cursor]
				m.flashMsg = fmt.Sprintf("Đang xuất CBZ Chapter %s...", chapterLabel(ch))
				return m, exportChapterCmd(m.provider, ch.ID, m.mangaTitle, chapterLabel(ch))
			}
			return m, nil

		case "ctrl+a":
			if m.cancelExport != nil || len(m.chapters) == 0 {
				return m, nil
			}
			ctx, cancel := context.WithCancel(context.Background())
			m.cancelExport = cancel
			m.flashMsg = fmt.Sprintf("Đang xuất 0/%d chapters ra CBZ... [esc] Hủy", len(m.chapters))
			return m, exportAllChaptersCmd(ctx, m.mangaID, m.provider, m.mangaTitle, m.chapters)

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

	case batchExportMsg:
		return m.handleBatchExport(msg)

	case cbzExportedMsg:
		if msg.err != nil {
			m.flashMsg = fmt.Sprintf("Lỗi xuất CBZ: %v", msg.err)
		} else {
			m.flashMsg = fmt.Sprintf("Đã xuất CBZ: %s", export.ShortPath(msg.path))
		}
		return m, clearFlashAfter(3 * time.Second)

	case storageSavedMsg:
		if msg.err != nil {
			m.flashMsg = fmt.Sprintf("Lỗi lưu yêu thích: %v", msg.err)
		}
		return m, nil

	case clearFlashMsg:
		m.flashMsg = ""
		return m, nil

	case chaptersLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.chapters = msg.Chapters
		m.cursor = 0
		m.viewportStart = 0
		m.err = nil
		m.reloadHistory()
		m.restoreHistoryPosition()
		// Continuing from history: go straight to the reader when the saved
		// chapter still exists (the reader resumes the saved page itself).
		resume := m.resumeChapterID
		m.resumeChapterID = ""
		if resume != "" && m.cursor < len(m.chapters) && m.chapters[m.cursor].ID == resume {
			return m.selectCurrentChapter()
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustViewport()
	}

	return m, nil
}

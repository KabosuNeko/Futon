package tui

import (
	"fmt"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/api"
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
}

type favoriteSavedMsg struct {
	err error
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

func (m ChapterListModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		s := msg.String()

		switch s {
		case "ctrl+c", "q":
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
						MangaID: m.mangaID,
						Title:   m.mangaTitle,
					})
					return favoriteSavedMsg{err: err}
				},
				clearFlashAfter(2*time.Second),
			)

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

			if len(m.chapters) > 0 && m.cursor >= 0 && m.cursor < len(m.chapters) {
				ch := m.chapters[m.cursor]
				allIDs := make([]string, len(m.chapters))
				for i, c := range m.chapters {
					allIDs[i] = c.ID
				}
				return m, func() tea.Msg {
					return ViewChapterMsg{
						MangaID:        m.mangaID,
						MangaTitle:     m.mangaTitle,
						ChapterID:      ch.ID,
						ChapterNumber:  ch.Number,
						AllChapterIDs:  allIDs,
						ChapterIndex:   m.cursor,
						StartPageIndex: -1,
					}
				}
			}
			return m, nil

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

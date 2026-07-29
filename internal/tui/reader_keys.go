package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/storage"
)

const preloadTriggerOffset = 3

func (m ReaderModel) loadCurrentPage() (ReaderModel, []tea.Cmd) {
	if _, ok := m.getCached(m.currentIdx); ok {
		m.isLoading = false
		return m, nil
	}
	m.isLoading = true
	if m.currentIdx >= 0 && m.currentIdx < len(m.imageData) && len(m.imageData[m.currentIdx]) > 0 {
		return m, []tea.Cmd{renderPage(m.renderer, m.imageData[m.currentIdx], m.currentIdx)}
	}
	return m, nil
}

func (m ReaderModel) handleKeyMsg(msg tea.KeyMsg) (ReaderModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		return m, tea.Sequence(
			storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.provider.Name(), m.chapterID, m.chapterNumber, m.currentIdx),
			storage.FlushHistoryCmd(),
			clearScreenCmd(),
			func() tea.Msg { return BackToChaptersMsg{} },
		)

	case "ctrl+d":
		if m.step == stepRead && m.validCurrentImage() {
			m.flashMsg = "Đang lưu ảnh..."
			return m, tea.Batch(
				saveImageCmd(m.imageData[m.currentIdx], m.mangaTitle, m.chapterNumber, m.currentIdx+1),
				clearFlashAfter(2*time.Second),
			)
		}
		return m, nil

	case "right", "l":
		if m.step != stepRead || m.isLoading {
			return m, nil
		}
		m.clampCurrentIndex()
		if m.currentIdx < m.total-1 {
			m.currentIdx++

			var cmds []tea.Cmd
			m, pageCmds := m.loadCurrentPage()
			cmds = append(cmds, pageCmds...)
			cmds = append(cmds, storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.provider.Name(), m.chapterID, m.chapterNumber, m.currentIdx))
			if m.currentIdx == m.total-preloadTriggerOffset && !m.isPreloadingNext && m.hasNextChapter() {
				m.isPreloadingNext = true
				nextID := m.allChapterIDs[m.chapterIndex+1]
				cmds = append(cmds, preloadNextChapter(nextID, m.provider))
			}
			return m, tea.Batch(cmds...)
		} else if m.hasNextChapter() {
			nextID := m.allChapterIDs[m.chapterIndex+1]
			saveCmd := storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.provider.Name(), m.chapterID, m.chapterNumber, m.currentIdx)

			if m.preloadedChapID == nextID && len(m.preloadedURLs) > 0 {
				m.applyPreloadedChapter(nextID)
				return m, tea.Sequence(
					saveCmd,
					storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.provider.Name(), m.chapterID, m.chapterNumber, 0),
					clearGraphicsCmd(),
					func() tea.Msg { return preloadTransitionReadyMsg{} },
				)
			}

			m.clearPreloaded()
			m.step = stepLoadingNext
			return m, tea.Sequence(
				saveCmd,
				clearScreenCmd(),
				chapterNavCmd(nextID, m.mangaID, m.mangaTitle, m.allChapterIDs, m.chapterIndex+1, 0),
			)
		}
		return m, nil

	case "left", "h":
		if m.step != stepRead || m.isLoading {
			return m, nil
		}
		m.clampCurrentIndex()
		if m.currentIdx > 0 {
			m.currentIdx--

			var cmds []tea.Cmd
			m, pageCmds := m.loadCurrentPage()
			cmds = append(cmds, pageCmds...)
			cmds = append(cmds, storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.provider.Name(), m.chapterID, m.chapterNumber, m.currentIdx))
			return m, tea.Batch(cmds...)
		} else if m.hasPreviousChapter() {
			prevID := m.allChapterIDs[m.chapterIndex-1]
			saveCmd := storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.provider.Name(), m.chapterID, m.chapterNumber, m.currentIdx)

			m.clearPreloaded()
			m.step = stepLoadingNext
			return m, tea.Sequence(
				saveCmd,
				clearScreenCmd(),
				chapterNavCmd(prevID, m.mangaID, m.mangaTitle, m.allChapterIDs, m.chapterIndex-1, -2),
			)
		}
		return m, nil
	}
	return m, nil
}

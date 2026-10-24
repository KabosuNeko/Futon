package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/export"
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
		return m, []tea.Cmd{renderPage(m.renderer, m.imageData[m.currentIdx], m.currentIdx, m.chapterID, m.hideFooter)}
	}
	return m, nil
}

func (m ReaderModel) providerName() string {
	if m.provider != nil {
		return m.provider.Name()
	}
	return ""
}

func exportReaderCBZCmd(mangaTitle, chapterNumber string, images [][]byte, urls []string, providerName, chapterURL string) tea.Cmd {
	return func() tea.Msg {
		hasAll := true
		for _, img := range images {
			if len(img) == 0 {
				hasAll = false
				break
			}
		}
		if hasAll && len(images) > 0 {
			path, err := export.ExportImagesToCBZ(mangaTitle, chapterNumber, images, "")
			return cbzExportedMsg{path: path, err: err}
		}
		path, err := export.ExportChapterURLsToCBZ(mangaTitle, chapterNumber, urls, "", providerName, chapterURL)
		return cbzExportedMsg{path: path, err: err}
	}
}

func (m ReaderModel) handleNextPage() (ReaderModel, tea.Cmd) {
	if m.step != stepRead || m.isLoading {
		return m, nil
	}
	m.clampCurrentIndex()
	if m.currentIdx < m.total-1 {
		m.currentIdx++

		m, cmds := m.loadCurrentPage()
		cmds = append(cmds, storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.providerName(), m.chapterID, m.chapterNumber, m.currentIdx))
		if m.currentIdx == m.total-preloadTriggerOffset && !m.isPreloadingNext && m.hasNextChapter() {
			m.isPreloadingNext = true
			nextID := m.allChapterIDs[m.chapterIndex+1]
			cmds = append(cmds, preloadNextChapter(nextID, m.provider))
		}
		return m, tea.Batch(cmds...)
	} else if m.hasNextChapter() {
		return m.jumpChapter(1, 0)
	}
	return m, nil
}

func (m ReaderModel) handlePrevPage() (ReaderModel, tea.Cmd) {
	if m.step != stepRead || m.isLoading {
		return m, nil
	}
	m.clampCurrentIndex()
	if m.currentIdx > 0 {
		m.currentIdx--

		m, cmds := m.loadCurrentPage()
		cmds = append(cmds, storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.providerName(), m.chapterID, m.chapterNumber, m.currentIdx))
		return m, tea.Batch(cmds...)
	} else if m.hasPreviousChapter() {
		return m.jumpChapter(-1, -2)
	}
	return m, nil
}

// jumpChapter switches chapters immediately. startPage: 0 = first page,
// -2 = last page. The next chapter is applied without a fetch when preloaded.
func (m ReaderModel) jumpChapter(delta, startPage int) (ReaderModel, tea.Cmd) {
	if m.step != stepRead {
		return m, nil
	}
	m.clampCurrentIndex()
	target := m.chapterIndex + delta
	if target < 0 || target >= len(m.allChapterIDs) {
		return m, nil
	}

	saveCmd := storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.providerName(), m.chapterID, m.chapterNumber, m.currentIdx)
	targetID := m.allChapterIDs[target]

	if delta > 0 && m.preloadedChapID == targetID && len(m.preloadedURLs) > 0 {
		m.applyPreloadedChapter(targetID)
		return m, tea.Sequence(
			saveCmd,
			storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.providerName(), m.chapterID, m.chapterNumber, 0),
			clearGraphicsCmd(),
			func() tea.Msg { return preloadTransitionReadyMsg{} },
		)
	}

	m.clearPreloaded()
	m.step = stepLoadingNext
	return m, tea.Sequence(
		saveCmd,
		clearScreenCmd(),
		chapterNavCmd(targetID, m.mangaID, m.mangaTitle, m.allChapterIDs, m.allChapterNumbers, target, startPage),
	)
}

func (m ReaderModel) handleMouseMsg(msg tea.MouseMsg) (ReaderModel, tea.Cmd) {
	if m.step != stepRead {
		return m, nil
	}
	mouse := msg.Mouse()
	switch mouse.Button {
	case tea.MouseWheelDown:
		return m.handleNextPage()
	case tea.MouseWheelUp:
		return m.handlePrevPage()
	case tea.MouseLeft:
		if _, ok := msg.(tea.MouseClickMsg); !ok {
			return m, nil
		}
		if mouse.X > m.width/2 {
			return m.handleNextPage()
		}
		return m.handlePrevPage()
	}
	return m, nil
}

// exportLabel names the reader export; HTML providers have no chapter numbers,
// so fall back to the chapter ID instead of the shared "Untitled" filename.
func (m ReaderModel) exportLabel() string {
	if m.chapterNumber != "" {
		return m.chapterNumber
	}
	return m.chapterID
}

func (m ReaderModel) handleKeyMsg(msg tea.KeyMsg) (ReaderModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.mangaID == "" || m.chapterID == "" {
			return m, tea.Quit
		}
		return m, tea.Sequence(
			storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.providerName(), m.chapterID, m.chapterNumber, m.currentIdx),
			storage.FlushHistoryCmd(),
			tea.Quit,
		)

	case "esc":
		return m, tea.Sequence(
			storage.SaveHistoryCmd(m.mangaID, m.mangaTitle, m.providerName(), m.chapterID, m.chapterNumber, m.currentIdx),
			storage.FlushHistoryCmd(),
			clearScreenCmd(),
			func() tea.Msg { return BackToChaptersMsg{} },
		)

	case "ctrl+e":
		if m.step == stepRead && m.total > 0 {
			m.flashMsg = fmt.Sprintf("Đang xuất CBZ Chapter %s...", m.exportLabel())
			return m, exportReaderCBZCmd(m.mangaTitle, m.exportLabel(), m.imageData, m.urls, m.providerName(), m.chapterID)
		}
		return m, nil

	case "ctrl+h":
		if m.step != stepRead || !m.validCurrentImage() {
			return m, nil
		}
		m.hideFooter = !m.hideFooter
		m.resetImageCache()
		m.isLoading = true
		return m, renderPage(m.renderer, m.imageData[m.currentIdx], m.currentIdx, m.chapterID, m.hideFooter)

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
		return m.handleNextPage()

	case "left", "h":
		return m.handlePrevPage()

	case "j", "down":
		return m.jumpChapter(1, 0)

	case "k", "up":
		return m.jumpChapter(-1, 0)
	}
	return m, nil
}

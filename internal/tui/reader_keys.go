package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/export"
	"github.com/KabosuNeko/Futon/internal/storage"
)

const preloadTriggerOffset = 3

func (m ReaderModel) loadCurrentPage() (ReaderModel, []tea.Cmd) {
	if _, ok := m.getCached(m.currentIdx); ok {
		m.isLoading = false
		return m, nil
	}
	if _, failed := m.failed[m.currentIdx]; failed {
		m.isLoading = false // show the page error instead of waiting
		return m, nil
	}
	m.isLoading = true
	if m.currentIdx >= 0 && m.currentIdx < len(m.imageData) && len(m.imageData[m.currentIdx]) > 0 {
		return m, []tea.Cmd{m.renderCmd(m.currentIdx)}
	}
	return m, nil
}

// saveHistoryCmd records page as the reading position of the open chapter.
func (m ReaderModel) saveHistoryCmd(page int) tea.Cmd {
	return saveHistoryCmd(m.mangaID, m.mangaTitle, m.providerName(), m.coverURL, m.chapterID, m.chapterNumber, page)
}

func (m ReaderModel) providerName() string {
	if m.provider != nil {
		return m.provider.Name()
	}
	return ""
}

// exportReaderCBZCmd exports the open chapter, reusing the pages already
// downloaded when all of them are, and downloading the chapter otherwise.
func exportReaderCBZCmd(provider api.MangaProvider, chapterID, mangaTitle, label string, images [][]byte, urls []string) tea.Cmd {
	return func() tea.Msg {
		complete := len(images) > 0
		for _, img := range images {
			if len(img) == 0 {
				complete = false
				break
			}
		}
		if !complete {
			var err error
			if images, err = api.DownloadChapterPages(context.Background(), provider, chapterID, urls); err != nil {
				return cbzExportedMsg{err: err}
			}
		}
		path, err := export.ExportImagesToCBZ(mangaTitle, label, images, "")
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
		m.scrollRow = 0

		m, cmds := m.loadCurrentPage()
		cmds = append(cmds, m.saveHistoryCmd(m.currentIdx))
		cmds = append(cmds, m.maybePreloadNext())
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
		m.scrollRow = 0

		m, cmds := m.loadCurrentPage()
		cmds = append(cmds, m.saveHistoryCmd(m.currentIdx))
		return m, tea.Batch(cmds...)
	} else if m.hasPreviousChapter() {
		return m.jumpChapter(-1, -2)
	}
	return m, nil
}

// maybePreloadNext starts fetching the next chapter once the reader is within
// preloadTriggerOffset pages of the end. It is checked on every page change
// and when a chapter opens, so short chapters and resumes near the end
// (where the trigger page is skipped) still preload.
func (m *ReaderModel) maybePreloadNext() tea.Cmd {
	if m.isPreloadingNext || !m.hasNextChapter() || m.preloadedChapID != "" || m.total == 0 {
		return nil
	}
	if m.currentIdx < m.total-preloadTriggerOffset {
		return nil
	}
	nextID := m.allChapterIDs[m.chapterIndex+1]
	if m.preloadTried == nextID {
		return nil
	}
	m.isPreloadingNext = true
	m.preloadTried = nextID
	return preloadNextChapter(nextID, m.provider)
}

// retryCurrentPage downloads a failed current page again.
func (m ReaderModel) retryCurrentPage() (ReaderModel, tea.Cmd) {
	if _, failed := m.failed[m.currentIdx]; !failed || m.currentIdx >= len(m.urls) {
		return m, nil
	}
	delete(m.failed, m.currentIdx)
	m.imageData[m.currentIdx] = nil
	m.downloading[m.currentIdx] = struct{}{}
	m.isLoading = true
	return m, downloadOne(m.provider, m.chapterID, m.urls[m.currentIdx], m.currentIdx)
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

	saveCmd := m.saveHistoryCmd(m.currentIdx)
	targetID := m.allChapterIDs[target]

	if delta > 0 && m.preloadedChapID == targetID && len(m.preloadedURLs) > 0 {
		m.applyPreloadedChapter(targetID)
		return m, tea.Sequence(
			saveCmd,
			m.saveHistoryCmd(0),
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
		if m.fitWidth {
			return m.scrollPage(1)
		}
		return m.handleNextPage()
	case tea.MouseWheelUp:
		if m.fitWidth {
			return m.scrollPage(-1)
		}
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

func (m ReaderModel) handleKeyMsg(msg tea.KeyMsg) (ReaderModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.mangaID == "" || m.chapterID == "" {
			return m, tea.Quit
		}
		return m, tea.Sequence(
			m.saveHistoryCmd(m.currentIdx),
			flushHistoryCmd(),
			tea.Quit,
		)

	case "esc":
		return m, tea.Sequence(
			m.saveHistoryCmd(m.currentIdx),
			flushHistoryCmd(),
			clearScreenCmd(),
			func() tea.Msg { return BackToChaptersMsg{} },
		)

	case "ctrl+e":
		if m.step == stepRead && m.total > 0 {
			m.flashMsg = fmt.Sprintf("Đang xuất CBZ Chapter %s...", m.chapterNumber)
			return m, exportReaderCBZCmd(m.provider, m.chapterID, m.mangaTitle, m.chapterNumber, m.imageData, m.urls)
		}
		return m, nil

	case "ctrl+h":
		if m.step != stepRead || !m.validCurrentImage() {
			return m, nil
		}
		m.hideFooter = !m.hideFooter
		m.resetImageCache()
		m.isLoading = true
		hidden := m.hideFooter
		return m, tea.Batch(m.renderCmd(m.currentIdx), saveUserDataCmd(func(ud *storage.UserData) { ud.ReaderHideFooter = hidden }))

	case "ctrl+d":
		if m.step == stepRead && m.validCurrentImage() {
			m.flashMsg = "Đang lưu ảnh..."
			return m, tea.Batch(
				saveImageCmd(m.imageData[m.currentIdx], m.mangaTitle, m.chapterNumber, m.currentIdx+1),
				clearFlashAfter(2*time.Second),
			)
		}
		return m, nil

	case "r":
		if m.step == stepRead {
			return m.retryCurrentPage()
		}
		return m, nil

	case "right", "l":
		return m.handleNextPage()

	case "left", "h":
		return m.handlePrevPage()

	case "w":
		return m.toggleFitWidth()

	case "down", "space":
		// Plain arrows only scroll (webtoon mode); shift+arrows switch chapters.
		if m.fitWidth {
			return m.scrollPage(1)
		}
		return m, nil

	case "up":
		if m.fitWidth {
			return m.scrollPage(-1)
		}
		return m, nil

	case "shift+down", "j":
		return m.jumpChapter(1, 0)

	case "shift+up", "k":
		return m.jumpChapter(-1, 0)
	}
	return m, nil
}

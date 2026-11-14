package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/export"
	"github.com/KabosuNeko/Futon/internal/storage"
)

func (m ReaderModel) handleChapterImages(msg pagesLoadedMsg) (ReaderModel, tea.Cmd) {
	if msg.Err != nil {
		m.step = stepError
		m.err = msg.Err
		return m, nil
	}
	if len(msg.URLs) == 0 {
		m.step = stepError
		m.err = fmt.Errorf("không có ảnh nào trong chapter này")
		return m, nil
	}
	m.urls = msg.URLs
	m.total = len(msg.URLs)
	m.imageData = make([][]byte, m.total)
	m.resetImageCache()
	m.downloaded = 0
	m.downloadPos = 0
	m.downloading = make(map[int]struct{})
	m.failed = make(map[int]error)
	m.currentIdx = 0

	// Resolve starting page: -2 = last page, >=0 = specific, default = history bookmark.
	switch {
	case m.startPage == -2 && m.total > 0:
		m.currentIdx = m.total - 1
	case m.startPage >= 0 && m.startPage < m.total:
		m.currentIdx = m.startPage
	default:
		if history, ok := storage.GetHistory(m.mangaID, m.providerName()); ok && history.ChapterID == m.chapterID {
			if history.PageIndex > 0 && history.PageIndex < m.total {
				m.currentIdx = history.PageIndex
			}
		}
	}

	m.downloadOrder = m.buildDownloadOrder(m.currentIdx)
	m.step = stepDownload
	return m, tea.Batch(append(m.scheduleDownloads(), m.maybePreloadNext())...)
}

func (m ReaderModel) handleDownloadProgress(msg downloadProgressMsg) (ReaderModel, tea.Cmd) {
	// A download from the chapter we just left must not fill this chapter's pages.
	if msg.chapterID != m.chapterID {
		return m, nil
	}
	if msg.index < 0 || msg.index >= len(m.imageData) {
		return m, nil
	}
	if msg.err != nil {
		// One bad page must not end the chapter: mark it and keep going.
		delete(m.downloading, msg.index)
		m.markFailed(msg.index, msg.err)
		return m, tea.Batch(m.scheduleDownloads()...)
	}
	if msg.data != nil {
		m.imageData[msg.index] = msg.data
	}
	m.downloaded++
	delete(m.downloading, msg.index)

	var cmds []tea.Cmd
	if msg.index == m.currentIdx && m.validCurrentImage() {
		cmds = append(cmds, m.startReading())
	}
	cmds = append(cmds, m.scheduleDownloads()...)
	if len(cmds) == 0 && m.step != stepRead && m.validCurrentImage() {
		return m, m.startReading()
	}
	return m, tea.Batch(cmds...)
}

// startReading transitions to the read step and renders the current page.
func (m *ReaderModel) startReading() tea.Cmd {
	m.step = stepRead
	m.isLoading = true
	return m.renderCmd(m.currentIdx)
}

func (m ReaderModel) handleRenderDone(msg renderDoneMsg) (ReaderModel, tea.Cmd) {
	// A render begun before a resize or a mode toggle has the old size or mode.
	if msg.chapterID != m.chapterID || msg.gen != m.renderGen {
		return m, nil
	}
	if msg.err != nil {
		// Undecodable page (e.g. an unsupported format): same as a failed download.
		m.markFailed(msg.index, msg.err)
		return m, nil
	}
	if msg.windowed {
		return m.handleWindowRendered(msg)
	}
	m.setCached(msg.index, msg.img)
	m.clampCurrentIndex()
	if msg.index == m.currentIdx {
		m.isLoading = false
	}

	cmds := m.scheduleDownloads()
	if nextIdx := m.nextRenderIndex(); nextIdx >= 0 && nextIdx < len(m.imageData) {
		cmds = append(cmds, m.renderCmd(nextIdx))
	}
	if len(cmds) == 0 {
		return m, nil
	}
	return m, tea.Batch(cmds...)
}

func (m ReaderModel) handlePreloadComplete(msg PreloadCompleteMsg) (ReaderModel, tea.Cmd) {
	m.isPreloadingNext = false
	if m.hasNextChapter() && msg.ChapID == m.allChapterIDs[m.chapterIndex+1] && len(msg.URLs) > 0 {
		m.preloadedChapID = msg.ChapID
		m.preloadedURLs = msg.URLs
		m.preloadedImages = msg.Images
	}
	return m, nil
}

// markFailed records a page error. If it is the page on screen, stop waiting
// for it so the error is shown and navigation keeps working.
func (m *ReaderModel) markFailed(idx int, err error) {
	m.failed[idx] = err
	if idx == m.currentIdx {
		m.step = stepRead
		m.isLoading = false
	}
}

func (m ReaderModel) handlePreloadTransitionReady(_ preloadTransitionReadyMsg) (ReaderModel, tea.Cmd) {
	cmds := append(m.scheduleDownloads(), m.maybePreloadNext())
	if m.step == stepRead && m.validCurrentImage() {
		cmds = append(cmds, m.renderCmd(m.currentIdx))
	}
	return m, tea.Batch(cmds...)
}

func (m ReaderModel) handleImageSaved(msg imageSavedMsg) (ReaderModel, tea.Cmd) {
	if msg.err != nil {
		m.flashMsg = fmt.Sprintf("Lỗi lưu ảnh: %v", msg.err)
	} else {
		m.flashMsg = fmt.Sprintf("Đã lưu ảnh: %s", export.ShortPath(msg.path))
	}
	return m, nil
}

func (m ReaderModel) handleWindowSize(msg tea.WindowSizeMsg) (ReaderModel, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.resetImageCache() // pages are rendered for the terminal size
	if m.step == stepRead && m.validCurrentImage() {
		m.isLoading = true
		return m, m.renderCmd(m.currentIdx)
	}
	return m, nil
}

package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/user/futon/internal/api"
	"github.com/user/futon/internal/storage"
	"github.com/user/futon/internal/tui/imgrender"
)

func (m ReaderModel) handleChapterImages(msg api.ChapterImagesMsg) (ReaderModel, tea.Cmd) {
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
	m.imageCache = make(map[int]imgrender.RenderedImage)
	m.cacheOrder = nil
	m.downloaded = 0
	m.downloadPos = 0
	m.downloading = make(map[int]struct{})
	m.currentIdx = 0

	switch {
	case m.startPage == -2 && m.total > 0:
		m.currentIdx = m.total - 1
	case m.startPage >= 0 && m.startPage < m.total:
		m.currentIdx = m.startPage
	default:
		if history, ok := storage.GetHistory(m.mangaID); ok && history.ChapterID == m.chapterID {
			if history.PageIndex > 0 && history.PageIndex < m.total {
				m.currentIdx = history.PageIndex
			}
		}
	}

	m.downloadOrder = m.buildDownloadOrder(m.currentIdx)
	m.step = stepDownload
	return m, tea.Batch(m.scheduleDownloads()...)
}

func (m ReaderModel) handleDownloadProgress(msg downloadProgressMsg) (ReaderModel, tea.Cmd) {
	if msg.err != nil {
		m.step = stepError
		m.err = msg.err
		return m, nil
	}
	if msg.index < 0 || msg.index >= len(m.imageData) {
		return m, nil
	}
	if msg.data != nil {
		m.imageData[msg.index] = msg.data
	}
	m.downloaded++
	delete(m.downloading, msg.index)

	var cmds []tea.Cmd
	if msg.index == m.currentIdx && m.validCurrentImage() {
		m.step = stepRead
		m.isLoading = true
		cmds = append(cmds, renderPage(m.renderer, m.imageData[m.currentIdx], m.currentIdx, m.width))
	}
	cmds = append(cmds, m.scheduleDownloads()...)
	if len(cmds) == 0 && m.validCurrentImage() {
		m.step = stepRead
		m.isLoading = true
		return m, renderPage(m.renderer, m.imageData[m.currentIdx], m.currentIdx, m.width)
	}
	return m, tea.Batch(cmds...)
}

func (m ReaderModel) handleRenderDone(msg renderDoneMsg) (ReaderModel, tea.Cmd) {
	if msg.err != nil {
		m.step = stepError
		m.err = msg.err
		m.isLoading = false
		return m, nil
	}
	m.setCached(msg.index, msg.img)
	m.clampCurrentIndex()
	if msg.index == m.currentIdx {
		m.isLoading = false
	}

	var cmds []tea.Cmd
	cmds = append(cmds, m.scheduleDownloads()...)
	if nextIdx := m.nextRenderIndex(); nextIdx >= 0 && nextIdx < len(m.imageData) {
		cmds = append(cmds, renderPage(m.renderer, m.imageData[nextIdx], nextIdx, m.width))
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

func (m ReaderModel) handlePreloadTransitionReady(_ preloadTransitionReadyMsg) (ReaderModel, tea.Cmd) {
	var cmds []tea.Cmd
	if m.step == stepRead && m.validCurrentImage() {
		cmds = append(cmds, renderPage(m.renderer, m.imageData[m.currentIdx], m.currentIdx, m.width))
	}
	cmds = append(cmds, m.scheduleDownloads()...)
	return m, tea.Batch(cmds...)
}

func (m ReaderModel) handleImageSaved(msg imageSavedMsg) (ReaderModel, tea.Cmd) {
	if msg.err != nil {
		m.flashMsg = fmt.Sprintf("Lỗi lưu ảnh: %v", msg.err)
	} else {
		m.flashMsg = fmt.Sprintf("Đã lưu ảnh: %s", msg.path)
	}
	return m, nil
}

func (m ReaderModel) handleWindowSize(msg tea.WindowSizeMsg) (ReaderModel, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	if m.step == stepRead && m.validCurrentImage() {
		m.isLoading = true
		return m, renderPage(m.renderer, m.imageData[m.currentIdx], m.currentIdx, m.width)
	}
	return m, nil
}

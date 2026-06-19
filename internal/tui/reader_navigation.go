package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

func (m ReaderModel) hasNextChapter() bool {
	return m.chapterIndex+1 < len(m.allChapterIDs)
}

func (m ReaderModel) hasPreviousChapter() bool {
	return m.chapterIndex > 0
}

func (m *ReaderModel) clampCurrentIndex() {
	if m.total <= 0 {
		m.currentIdx = 0
		return
	}
	if m.currentIdx < 0 {
		m.currentIdx = 0
	} else if m.currentIdx >= m.total {
		m.currentIdx = m.total - 1
	}
}

func (m ReaderModel) validCurrentImage() bool {
	return m.currentIdx >= 0 && m.currentIdx < len(m.imageData) && len(m.imageData[m.currentIdx]) > 0
}

const maxConcurrentDownloads = 4

func (m *ReaderModel) scheduleDownloads() []tea.Cmd {
	var cmds []tea.Cmd
	for len(m.downloading) < maxConcurrentDownloads && m.downloadPos < len(m.downloadOrder) {
		idx := m.downloadOrder[m.downloadPos]
		m.downloadPos++
		if idx >= len(m.urls) || idx >= len(m.imageData) {
			continue
		}
		if len(m.imageData[idx]) > 0 {
			continue
		}
		if _, ok := m.downloading[idx]; ok {
			continue
		}
		m.downloading[idx] = struct{}{}
		cmds = append(cmds, downloadOne(m.urls[idx], idx))
	}
	return cmds
}

func (m ReaderModel) buildDownloadOrder(startIdx int) []int {
	order := make([]int, 0, m.total)
	order = append(order, startIdx)
	for i := startIdx + 1; i < m.total; i++ {
		order = append(order, i)
	}
	for i := 0; i < startIdx; i++ {
		order = append(order, i)
	}
	return order
}

// preloadPageLimit controls how many pages ahead of the current page are
// pre-rendered into the image cache. A small window prevents the infinite
// render loop that occurs when nextRenderIndex scans the entire chapter
// and re-renders cache-evicted pages endlessly.
const preloadPageLimit = 3

func (m ReaderModel) nextRenderIndex() int {
	start := m.currentIdx + 1
	end := m.currentIdx + preloadPageLimit + 1
	if end > m.total {
		end = m.total
	}
	for i := start; i < end && i < len(m.imageData); i++ {
		if _, cached := m.imageCache[i]; !cached && len(m.imageData[i]) > 0 {
			return i
		}
	}
	return -1
}

func (m *ReaderModel) applyPreloadedChapter(nextID string) {
	m.chapterID = nextID
	m.chapterIndex++
	m.urls = m.preloadedURLs
	m.total = len(m.preloadedURLs)
	m.imageData = make([][]byte, m.total)
	m.imageCache = make(map[int]imgrender.RenderedImage)
	m.cacheOrder = nil
	m.downloading = make(map[int]struct{})
	for i, data := range m.preloadedImages {
		if i < m.total {
			m.imageData[i] = data
		}
	}
	m.downloaded = len(m.preloadedImages)
	m.currentIdx = 0
	m.downloadPos = m.downloaded
	m.downloadOrder = m.buildDownloadOrder(0)
	if m.downloaded > 0 {
		m.step = stepRead
	} else {
		m.step = stepDownload
	}
	m.clearPreloaded()
}

func (m *ReaderModel) clearPreloaded() {
	m.preloadedChapID = ""
	m.preloadedURLs = nil
	m.preloadedImages = nil
	m.isPreloadingNext = false
}

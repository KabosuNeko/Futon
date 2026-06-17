package tui

import "github.com/KabosuNeko/Futon/internal/tui/imgrender"

const maxImageCache = 20

func (m *ReaderModel) setCached(idx int, img imgrender.RenderedImage) {
	m.imageCache[idx] = img

	m.cacheOrder = filterInt(m.cacheOrder, idx)
	m.cacheOrder = append(m.cacheOrder, idx)

	for len(m.cacheOrder) > maxImageCache {
		oldest := m.cacheOrder[0]
		m.cacheOrder = m.cacheOrder[1:]
		delete(m.imageCache, oldest)
	}
}

func (m *ReaderModel) getCached(idx int) (imgrender.RenderedImage, bool) {
	img, ok := m.imageCache[idx]
	if !ok {
		return imgrender.RenderedImage{}, false
	}
	m.cacheOrder = filterInt(m.cacheOrder, idx)
	m.cacheOrder = append(m.cacheOrder, idx)
	return img, true
}

func filterInt(slice []int, target int) []int {
	result := make([]int, 0, len(slice))
	for _, v := range slice {
		if v != target {
			result = append(result, v)
		}
	}
	return result
}

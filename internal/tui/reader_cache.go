package tui

import (
	"slices"

	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

// Cap LRU to 20 rendered images — enough for smooth scrolling, won't OOM your laptop.
const maxImageCache = 20

func (m *ReaderModel) setCached(idx int, img imgrender.RenderedImage) {
	m.imageCache[idx] = img

	m.cacheOrder = slices.DeleteFunc(m.cacheOrder, func(v int) bool { return v == idx })
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
	m.cacheOrder = slices.DeleteFunc(m.cacheOrder, func(v int) bool { return v == idx })
	m.cacheOrder = append(m.cacheOrder, idx)
	return img, true
}

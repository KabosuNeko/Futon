package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/tui/imgrender"
)

type downloadProgressMsg struct {
	index int
	err   error
	data  []byte
}

type renderDoneMsg struct {
	index int
	img   imgrender.RenderedImage
	err   error
}

type PreloadCompleteMsg struct {
	ChapID string
	URLs   []string
	Images [][]byte
}

type preloadTransitionReadyMsg struct{}

type imageSavedMsg struct {
	path string
	err  error
}

type ReaderModel struct {
	mangaID           string
	mangaTitle        string
	chapterNumber     string
	chapterID         string
	allChapterIDs     []string
	allChapterNumbers []string
	chapterIndex      int
	provider          api.MangaProvider
	urls              []string
	imageData         [][]byte
	imageCache        map[int]imgrender.RenderedImage
	cacheOrder        []int
	currentIdx        int
	startPage         int
	downloadOrder     []int
	downloadPos       int
	downloading       map[int]struct{}
	renderer          imgrender.Renderer
	downloaded        int
	total             int
	step              readerStep
	isLoading         bool
	isPreloadingNext  bool
	preloadedChapID   string
	preloadedURLs     []string
	preloadedImages   [][]byte
	flashMsg          string
	err               error
	width             int
	height            int
	lastFrame         string
}

type readerStep int

const (
	// Lifecycle: FetchURLs → Download → Read → (LoadingNext → back to Read) → Error.
	stepFetchURLs readerStep = iota
	stepDownload
	stepRead
	stepLoadingNext
	stepError
)

func NewReaderModel(mangaID, mangaTitle, chapterID, chapterNumber string, allChapterIDs []string, chapterIndex, startPage int, provider api.MangaProvider) ReaderModel {
	return ReaderModel{
		mangaID:       mangaID,
		mangaTitle:    mangaTitle,
		chapterID:     chapterID,
		chapterNumber: chapterNumber,
		allChapterIDs: allChapterIDs,
		chapterIndex:  chapterIndex,
		startPage:     startPage,
		provider:      provider,
		renderer:      imgrender.New(),
		imageCache:    make(map[int]imgrender.RenderedImage),
		downloading:   make(map[int]struct{}),
		step:          stepFetchURLs,
		width:         80,
		height:        24,
	}
}

// readerFirstFrameDelay gives the renderer one frame to paint the reader's
// blank view before the first raw frame is submitted. The first flush after
// switching away from a non-blank screen erases the whole screen; a frame
// submitted in that same tick would be wiped. Later frames are safe because
// the renderer settles once the blank view repeats.
const readerFirstFrameDelay = 30 * time.Millisecond

type readerFirstFrameMsg struct{}

func (m ReaderModel) Init() tea.Cmd {
	return tea.Batch(
		api.FetchPagesCmd(m.provider, m.chapterID),
		tea.Tick(readerFirstFrameDelay, func(time.Time) tea.Msg { return readerFirstFrameMsg{} }),
	)
}

// Update wraps message handling with the reader's out-of-band frame painting:
// whenever the frame content changes, a tea.Raw command repaints the whole
// frame (see View in reader_view.go for why).
func (m ReaderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newM, cmd := m.update(msg)
	if frame := newM.frameContent(); frame != newM.lastFrame {
		newM.lastFrame = frame
		cmd = tea.Batch(cmd, tea.Raw(frame))
	}
	return newM, cmd
}

func (m ReaderModel) update(msg tea.Msg) (ReaderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		return m.handleMouseMsg(msg)
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case api.ChapterImagesMsg:
		return m.handleChapterImages(msg)
	case downloadProgressMsg:
		return m.handleDownloadProgress(msg)
	case renderDoneMsg:
		return m.handleRenderDone(msg)
	case PreloadCompleteMsg:
		return m.handlePreloadComplete(msg)
	case preloadTransitionReadyMsg:
		return m.handlePreloadTransitionReady(msg)
	case imageSavedMsg:
		return m.handleImageSaved(msg)
	case cbzExportedMsg:
		if msg.err != nil {
			m.flashMsg = fmt.Sprintf("Lỗi xuất CBZ: %v", msg.err)
		} else {
			m.flashMsg = fmt.Sprintf("Đã xuất CBZ: %s", msg.path)
		}
		return m, clearFlashAfter(3 * time.Second)
	case clearFlashMsg:
		m.flashMsg = ""
		return m, nil
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	case readerFirstFrameMsg:
		// Force a repaint: a frame emitted before this tick may have been
		// erased by the renderer's transition flush.
		m.lastFrame = m.frameContent()
		return m, tea.Raw(m.lastFrame)
	}
	return m, nil
}

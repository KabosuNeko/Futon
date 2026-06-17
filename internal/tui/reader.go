package tui

import (
	tea "github.com/charmbracelet/bubbletea"
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

type clearDoneMsg struct{}

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
	mangaID          string
	mangaTitle       string
	chapterNumber    string
	chapterID        string
	allChapterIDs    []string
	chapterIndex     int
	provider         api.MangaProvider
	urls             []string
	imageData        [][]byte
	imageCache       map[int]imgrender.RenderedImage
	cacheOrder       []int
	currentIdx       int
	startPage        int
	downloadOrder    []int
	downloadPos      int
	downloading      map[int]struct{}
	renderer         imgrender.Renderer
	downloaded       int
	total            int
	step             readerStep
	isLoading        bool
	isPreloadingNext bool
	preloadedChapID  string
	preloadedURLs    []string
	preloadedImages  [][]byte
	flashMsg         string
	err              error
	width            int
	height           int
}

type readerStep int

const (
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

func (m ReaderModel) Init() tea.Cmd {
	return api.FetchPagesCmd(m.provider, m.chapterID)
}

func (m ReaderModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	case api.ChapterImagesMsg:
		return m.handleChapterImages(msg)
	case downloadProgressMsg:
		return m.handleDownloadProgress(msg)
	case renderDoneMsg:
		return m.handleRenderDone(msg)
	case clearDoneMsg:
		return m, nil
	case PreloadCompleteMsg:
		return m.handlePreloadComplete(msg)
	case preloadTransitionReadyMsg:
		return m.handlePreloadTransitionReady(msg)
	case imageSavedMsg:
		return m.handleImageSaved(msg)
	case clearFlashMsg:
		m.flashMsg = ""
		return m, nil
	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)
	}
	return m, nil
}

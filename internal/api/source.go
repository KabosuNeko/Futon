package api

import tea "github.com/charmbracelet/bubbletea"

func SearchCmd(p MangaProvider, query string) tea.Cmd {
	return func() tea.Msg {
		manga, err := p.Search(query)
		return MangaSearchResultMsg{Manga: manga, Err: err}
	}
}

func FetchChaptersCmd(p MangaProvider, mangaID string) tea.Cmd {
	return func() tea.Msg {
		chapters, err := p.FetchChapters(mangaID)
		return ChapterListMsg{Chapters: chapters, Err: err}
	}
}

func FetchPagesCmd(p MangaProvider, chapterID string) tea.Cmd {
	return func() tea.Msg {
		urls, err := p.FetchPages(chapterID)
		return ChapterImagesMsg{URLs: urls, Err: err}
	}
}

package api

import (
	"fmt"
	"strings"
	"sync"

	"github.com/KabosuNeko/Futon/internal/models"
	tea "github.com/charmbracelet/bubbletea"
)

func SearchCmd(p MangaProvider, query string) tea.Cmd {
	return func() tea.Msg {
		manga, err := p.Search(query)
		return MangaSearchResultMsg{Manga: manga, Err: err}
	}
}

func GlobalSearchCmd(providers []MangaProvider, query string) tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		var wg sync.WaitGroup
		allResults := make([]models.Manga, 0)
		var errs []string
		counts := make(map[string]int)
		perr := make(map[string]string)

		for _, p := range providers {
			wg.Add(1)
			go func(provider MangaProvider) {
				defer wg.Done()
				results, err := provider.Search(query)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					errs = append(errs, fmt.Sprintf("%s: %v", provider.Name(), err))
					perr[provider.Name()] = err.Error()
					counts[provider.Name()] = 0
					return
				}
				for i := range results {
					results[i].Provider = provider.Name()
				}
				counts[provider.Name()] = len(results)
				allResults = append(allResults, results...)
			}(p)
		}

		wg.Wait()

		var combinedErr error
		if len(errs) > 0 {
			combinedErr = fmt.Errorf("%s", strings.Join(errs, "; "))
		}
		return MangaSearchResultMsg{Manga: allResults, Err: combinedErr, ProviderCounts: counts, ProviderErrors: perr}
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

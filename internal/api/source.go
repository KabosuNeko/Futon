package api

import (
	"fmt"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/models"
)

func globalCmd(providers []MangaProvider, fetch func(MangaProvider) ([]models.Manga, error)) tea.Cmd {
	return func() tea.Msg {
		var mu sync.Mutex
		var wg sync.WaitGroup
		allResults := make([]models.Manga, 0)
		counts := make(map[string]int)
		perr := make(map[string]string)

		for _, p := range providers {
			wg.Add(1)
			go func(provider MangaProvider) {
				defer wg.Done()
				results, err := fetch(provider)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
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
		if len(perr) > 0 && len(allResults) == 0 {
			errs := make([]string, 0, len(perr))
			for _, p := range providers {
				if msg, ok := perr[p.Name()]; ok {
					errs = append(errs, fmt.Sprintf("%s: %s", p.Name(), msg))
				}
			}
			combinedErr = fmt.Errorf("%s", strings.Join(errs, "; "))
		}
		return MangaSearchResultMsg{Manga: allResults, Err: combinedErr, ProviderCounts: counts, ProviderErrors: perr}
	}
}

func GlobalSearchCmd(providers []MangaProvider, query string) tea.Cmd {
	return globalCmd(providers, func(p MangaProvider) ([]models.Manga, error) { return p.Search(query) })
}

func GlobalLatestCmd(providers []MangaProvider, page int) tea.Cmd {
	return globalCmd(providers, func(p MangaProvider) ([]models.Manga, error) { return p.FetchLatest(page) })
}

func GlobalFilterCmd(providers []MangaProvider, opts FilterOptions) tea.Cmd {
	return globalCmd(providers, func(p MangaProvider) ([]models.Manga, error) { return p.Filter(opts) })
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

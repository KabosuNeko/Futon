package api

import (
	"fmt"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/models"
)

// SearchStream delivers one snapshot per provider completion, then a final
// snapshot with a nil MangaSearchResultMsg.Stream.
type SearchStream struct {
	ch chan MangaSearchResultMsg
}

// Next waits for the next snapshot of the stream.
func (s *SearchStream) Next() tea.Cmd {
	return func() tea.Msg { return <-s.ch }
}

// globalCmd fans fetch out over the providers and streams a cumulative snapshot
// as each one finishes, so callers can show partial results instead of waiting
// for the slowest (or a dead) provider.
func globalCmd(providers []MangaProvider, fetch func(MangaProvider) ([]models.Manga, error)) tea.Cmd {
	return func() tea.Msg {
		if len(providers) == 0 {
			return MangaSearchResultMsg{}
		}
		ch := make(chan MangaSearchResultMsg, len(providers)+1)
		stream := &SearchStream{ch: ch}

		var mu sync.Mutex
		var wg sync.WaitGroup
		allResults := make([]models.Manga, 0)
		counts := make(map[string]int)
		perr := make(map[string]string)

		snapshot := func() MangaSearchResultMsg {
			manga := make([]models.Manga, len(allResults))
			copy(manga, allResults)
			c := make(map[string]int, len(counts))
			for k, v := range counts {
				c[k] = v
			}
			e := make(map[string]string, len(perr))
			for k, v := range perr {
				e[k] = v
			}
			return MangaSearchResultMsg{Manga: manga, ProviderCounts: c, ProviderErrors: e, Stream: stream}
		}

		for _, p := range providers {
			wg.Add(1)
			go func(provider MangaProvider) {
				defer wg.Done()
				results, err := fetch(provider)
				mu.Lock()
				if err != nil {
					perr[provider.Name()] = err.Error()
					counts[provider.Name()] = 0
				} else {
					for i := range results {
						results[i].Provider = provider.Name()
					}
					counts[provider.Name()] = len(results)
					allResults = append(allResults, results...)
				}
				msg := snapshot()
				mu.Unlock()
				ch <- msg
			}(p)
		}

		go func() {
			wg.Wait()
			mu.Lock()
			msg := snapshot()
			msg.Stream = nil
			if len(perr) > 0 && len(allResults) == 0 {
				errs := make([]string, 0, len(perr))
				for _, p := range providers {
					if e, ok := perr[p.Name()]; ok {
						errs = append(errs, fmt.Sprintf("%s: %s", p.Name(), e))
					}
				}
				msg.Err = fmt.Errorf("%s", strings.Join(errs, "; "))
			}
			mu.Unlock()
			ch <- msg
			close(ch)
		}()

		return <-ch
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

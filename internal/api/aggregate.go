package api

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/KabosuNeko/Futon/internal/models"
)

// Snapshot is the cumulative result of a request fanned out over several
// providers, as of the latest provider to finish.
type Snapshot struct {
	Manga          []models.Manga
	ProviderCounts map[string]int
	ProviderErrors map[string]string
	// Err is set on the final snapshot when every provider failed.
	Err error
	// Final marks the last snapshot; the channel is closed after it.
	Final bool
}

// SearchAll searches every provider concurrently. See fanOut.
func SearchAll(ctx context.Context, providers []MangaProvider, query string) <-chan Snapshot {
	return fanOut(ctx, providers, func(ctx context.Context, p MangaProvider) ([]models.Manga, error) { return p.Search(ctx, query) })
}

// LatestAll fetches a page of each provider's latest updates concurrently.
func LatestAll(ctx context.Context, providers []MangaProvider, page int) <-chan Snapshot {
	return fanOut(ctx, providers, func(ctx context.Context, p MangaProvider) ([]models.Manga, error) { return p.FetchLatest(ctx, page) })
}

// FilterAll runs a filter on every provider concurrently.
func FilterAll(ctx context.Context, providers []MangaProvider, opts FilterOptions) <-chan Snapshot {
	return fanOut(ctx, providers, func(ctx context.Context, p MangaProvider) ([]models.Manga, error) { return p.Filter(ctx, opts) })
}

// fanOut runs fetch on every provider at once and sends a cumulative snapshot
// as each one finishes, so callers can show partial results instead of
// waiting for the slowest (or a dead) provider, then a Final snapshot.
// Results are labelled with their provider. Cancelling ctx aborts the
// in-flight requests; the stream still ends with a Final snapshot. The
// channel is buffered, so an abandoned stream never blocks the workers.
func fanOut(ctx context.Context, providers []MangaProvider, fetch func(context.Context, MangaProvider) ([]models.Manga, error)) <-chan Snapshot {
	out := make(chan Snapshot, len(providers)+1)

	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		all    []models.Manga
		counts = make(map[string]int)
		errs   = make(map[string]string)
	)
	snapshot := func() Snapshot {
		return Snapshot{
			Manga:          append([]models.Manga(nil), all...),
			ProviderCounts: maps.Clone(counts),
			ProviderErrors: maps.Clone(errs),
		}
	}

	for _, p := range providers {
		wg.Add(1)
		go func(p MangaProvider) {
			defer wg.Done()
			results, err := fetch(ctx, p)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs[p.Name()] = err.Error()
				counts[p.Name()] = 0
			} else {
				for i := range results {
					results[i].Provider = p.Name()
				}
				counts[p.Name()] = len(results)
				all = append(all, results...)
			}
			out <- snapshot()
		}(p)
	}

	go func() {
		wg.Wait()
		mu.Lock()
		final := snapshot()
		final.Final = true
		if len(errs) > 0 && len(all) == 0 {
			var msgs []string
			for _, p := range providers {
				if e, ok := errs[p.Name()]; ok {
					msgs = append(msgs, fmt.Sprintf("%s: %s", p.Name(), e))
				}
			}
			final.Err = fmt.Errorf("%s", strings.Join(msgs, "; "))
		}
		mu.Unlock()
		out <- final
		close(out)
	}()
	return out
}

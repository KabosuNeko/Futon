package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KabosuNeko/Futon/internal/models"
)

type fakeProvider struct {
	name  string
	delay time.Duration
	manga []models.Manga
	err   error
}

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) Search(ctx context.Context, _ string) ([]models.Manga, error) {
	return f.fetch(ctx)
}
func (f fakeProvider) FetchLatest(ctx context.Context, _ int) ([]models.Manga, error) {
	return f.fetch(ctx)
}
func (f fakeProvider) Filter(ctx context.Context, _ FilterOptions) ([]models.Manga, error) {
	return f.fetch(ctx)
}
func (f fakeProvider) FetchChapters(context.Context, string) ([]models.Chapter, error) {
	return nil, nil
}
func (f fakeProvider) FetchPages(context.Context, string) ([]string, error) { return nil, nil }

func (f fakeProvider) fetch(ctx context.Context) ([]models.Manga, error) {
	if err := sleepCtx(ctx, f.delay); err != nil {
		return nil, err
	}
	return f.manga, f.err
}

// drain collects every snapshot of a stream.
func drain(ch <-chan Snapshot) []Snapshot {
	var out []Snapshot
	for s := range ch {
		out = append(out, s)
	}
	return out
}

func TestFanOutStreamsSnapshots(t *testing.T) {
	fast := fakeProvider{name: "Fast", manga: []models.Manga{{ID: "1", Title: "A"}}}
	dead := fakeProvider{name: "Dead", delay: 100 * time.Millisecond, err: errors.New("boom")}
	slow := fakeProvider{name: "Slow", delay: 200 * time.Millisecond, manga: []models.Manga{{ID: "2", Title: "B"}}}

	snaps := drain(SearchAll(context.Background(), []MangaProvider{slow, dead, fast}, "q"))
	if len(snaps) != 4 {
		t.Fatalf("snapshots = %d, want one per provider + final", len(snaps))
	}
	first := snaps[0]
	if first.Final || len(first.Manga) != 1 || first.Manga[0].Provider != "Fast" {
		t.Fatalf("first snapshot = %+v, want only the Fast result", first)
	}

	final := snaps[len(snaps)-1]
	if !final.Final || len(final.Manga) != 2 {
		t.Fatalf("final snapshot = %+v, want Final with 2 results", final)
	}
	if final.ProviderCounts["Slow"] != 1 || final.ProviderCounts["Fast"] != 1 || final.ProviderCounts["Dead"] != 0 {
		t.Errorf("final counts = %v", final.ProviderCounts)
	}
	if final.ProviderErrors["Dead"] != "boom" {
		t.Errorf("final errors = %v", final.ProviderErrors)
	}
}

func TestFanOutAllFailuresCombinedError(t *testing.T) {
	snaps := drain(SearchAll(context.Background(), []MangaProvider{
		fakeProvider{name: "A", err: errors.New("x")},
		fakeProvider{name: "B", err: errors.New("y")},
	}, "q"))
	final := snaps[len(snaps)-1]
	if final.Err == nil || final.Err.Error() != "A: x; B: y" {
		t.Errorf("combined error = %v, want A: x; B: y", final.Err)
	}
}

func TestFanOutNoProvidersEndsImmediately(t *testing.T) {
	snaps := drain(LatestAll(context.Background(), nil, 1))
	if len(snaps) != 1 || !snaps[0].Final || snaps[0].Err != nil {
		t.Errorf("snapshots = %+v, want a single empty final snapshot", snaps)
	}
}

func TestFanOutCancelStopsSlowProviders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	slow := fakeProvider{name: "Slow", delay: 10 * time.Second, manga: []models.Manga{{ID: "1"}}}

	start := time.Now()
	cancel()
	snaps := drain(SearchAll(ctx, []MangaProvider{slow}, "x"))
	if time.Since(start) > 2*time.Second {
		t.Fatalf("cancelled search took %v, want it to stop promptly", time.Since(start))
	}
	if final := snaps[len(snaps)-1]; len(final.Manga) != 0 {
		t.Errorf("cancelled search returned %d results, want 0", len(final.Manga))
	}
}

func TestFanOutStripsTerminalEscapesFromTitles(t *testing.T) {
	evil := "Evil\x1b]52;c;cm0gLXJmIH4=\aTitle"
	p := fakeProvider{name: "P", manga: []models.Manga{{ID: "1", Title: evil, Author: "A\x1b[2J", Genres: []string{"G\x07"}}}}
	snaps := drain(SearchAll(context.Background(), []MangaProvider{p}, "q"))
	got := snaps[len(snaps)-1].Manga[0]
	if got.Title != "Evil]52;c;cm0gLXJmIH4=Title" || got.Author != "A[2J" || got.Genres[0] != "G" {
		t.Errorf("control characters survived: %q %q %q", got.Title, got.Author, got.Genres)
	}
}

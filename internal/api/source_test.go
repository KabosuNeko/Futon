package api

import (
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
func (f fakeProvider) Search(string) ([]models.Manga, error) {
	return f.fetch()
}
func (f fakeProvider) FetchLatest(int) ([]models.Manga, error) {
	return f.fetch()
}
func (f fakeProvider) Filter(FilterOptions) ([]models.Manga, error) {
	return f.fetch()
}
func (f fakeProvider) FetchChapters(string) ([]models.Chapter, error) { return nil, nil }
func (f fakeProvider) FetchPages(string) ([]string, error)            { return nil, nil }

func (f fakeProvider) fetch() ([]models.Manga, error) {
	time.Sleep(f.delay)
	return f.manga, f.err
}

func TestGlobalCmdStreamsSnapshots(t *testing.T) {
	fast := fakeProvider{name: "Fast", manga: []models.Manga{{ID: "1", Title: "A"}}}
	dead := fakeProvider{name: "Dead", delay: 100 * time.Millisecond, err: errors.New("boom")}
	slow := fakeProvider{name: "Slow", delay: 200 * time.Millisecond, manga: []models.Manga{{ID: "2", Title: "B"}}}

	first := GlobalSearchCmd([]MangaProvider{slow, dead, fast}, "q")().(MangaSearchResultMsg)
	if first.Stream == nil {
		t.Fatal("first snapshot must be a stream snapshot")
	}
	if len(first.Manga) != 1 || first.Manga[0].Provider != "Fast" {
		t.Fatalf("first snapshot = %+v, want only the Fast result", first.Manga)
	}

	var final MangaSearchResultMsg
	for msg := first; ; {
		if msg.Stream == nil {
			final = msg
			break
		}
		next, ok := msg.Stream.Next()().(MangaSearchResultMsg)
		if !ok {
			t.Fatal("stream yielded a non-search message")
		}
		msg = next
	}

	if len(final.Manga) != 2 {
		t.Fatalf("final snapshot = %d results, want 2", len(final.Manga))
	}
	if final.ProviderCounts["Slow"] != 1 || final.ProviderCounts["Fast"] != 1 || final.ProviderCounts["Dead"] != 0 {
		t.Errorf("final counts = %v", final.ProviderCounts)
	}
	if final.ProviderErrors["Dead"] != "boom" {
		t.Errorf("final errors = %v", final.ProviderErrors)
	}
}

func TestGlobalCmdAllFailuresCombinedError(t *testing.T) {
	cmd := GlobalSearchCmd([]MangaProvider{
		fakeProvider{name: "A", err: errors.New("x")},
		fakeProvider{name: "B", err: errors.New("y")},
	}, "q")

	msg := cmd().(MangaSearchResultMsg)
	for msg.Stream != nil {
		msg = msg.Stream.Next()().(MangaSearchResultMsg)
	}
	if msg.Err == nil || msg.Err.Error() != "A: x; B: y" {
		t.Errorf("combined error = %v, want A: x; B: y", msg.Err)
	}
}

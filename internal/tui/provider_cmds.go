package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/models"
)

// resultsMsg carries one snapshot of a multi-provider result request.
type resultsMsg struct {
	Manga          []models.Manga
	Err            error
	ProviderCounts map[string]int
	ProviderErrors map[string]string
	// RequestID lets the search screen discard snapshots of a superseded request.
	RequestID int
	// Stream is non-nil while more snapshots are coming; the final one has nil.
	Stream *resultStream
}

// resultStream turns an api snapshot channel into a chain of commands.
type resultStream struct {
	snapshots <-chan api.Snapshot
}

// Next waits for the following snapshot.
func (s *resultStream) Next() tea.Cmd {
	return func() tea.Msg {
		snap, ok := <-s.snapshots
		if !ok {
			return resultsMsg{} // closed early: treat as an empty final snapshot
		}
		msg := resultsMsg{
			Manga:          snap.Manga,
			Err:            snap.Err,
			ProviderCounts: snap.ProviderCounts,
			ProviderErrors: snap.ProviderErrors,
		}
		if !snap.Final {
			msg.Stream = s
		}
		return msg
	}
}

// streamCmd starts a fan-out inside the command, so Update never blocks, and
// yields its first snapshot.
func streamCmd(start func() <-chan api.Snapshot) tea.Cmd {
	return func() tea.Msg {
		return (&resultStream{snapshots: start()}).Next()()
	}
}

func searchAllCmd(ctx context.Context, providers []api.MangaProvider, query string) tea.Cmd {
	return streamCmd(func() <-chan api.Snapshot { return api.SearchAll(ctx, providers, query) })
}

func latestAllCmd(ctx context.Context, providers []api.MangaProvider, page int) tea.Cmd {
	return streamCmd(func() <-chan api.Snapshot { return api.LatestAll(ctx, providers, page) })
}

func filterAllCmd(ctx context.Context, providers []api.MangaProvider, opts api.FilterOptions) tea.Cmd {
	return streamCmd(func() <-chan api.Snapshot { return api.FilterAll(ctx, providers, opts) })
}

type chaptersLoadedMsg struct {
	Chapters []models.Chapter
	Err      error
}

func fetchChaptersCmd(p api.MangaProvider, mangaID string) tea.Cmd {
	return func() tea.Msg {
		chapters, err := p.FetchChapters(context.Background(), mangaID)
		for i := range chapters {
			chapters[i].Title = api.CleanText(chapters[i].Title)
			chapters[i].Number = api.CleanText(chapters[i].Number)
		}
		return chaptersLoadedMsg{Chapters: chapters, Err: err}
	}
}

type pagesLoadedMsg struct {
	URLs []string
	Err  error
}

func fetchPagesCmd(p api.MangaProvider, chapterID string) tea.Cmd {
	return func() tea.Msg {
		urls, err := p.FetchPages(context.Background(), chapterID)
		return pagesLoadedMsg{URLs: urls, Err: err}
	}
}

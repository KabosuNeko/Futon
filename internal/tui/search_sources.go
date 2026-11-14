package tui

import (
	"errors"
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/KabosuNeko/Futon/internal/api"
	"github.com/KabosuNeko/Futon/internal/storage"
)

func (m SearchModel) activeProviders() []api.MangaProvider {
	var active []api.MangaProvider
	for i, p := range m.providers {
		if m.providerToggles[i] {
			active = append(active, p)
		}
	}
	return active
}

func loadSourceToggles(providers []api.MangaProvider) []bool {
	toggles := make([]bool, len(providers))
	for i := range toggles {
		toggles[i] = true
	}

	saved, err := storage.LoadSources()
	if err != nil || len(saved) == 0 {
		return toggles
	}

	for i := range toggles {
		toggles[i] = false
	}
	savedSet := make(map[string]bool, len(saved))
	for _, name := range saved {
		if successor, merged := mergedSources[name]; merged {
			name = successor
		}
		savedSet[name] = true
	}
	matched := false
	for i, p := range providers {
		if savedSet[p.Name()] {
			toggles[i] = true
			matched = true
		}
	}
	if !matched {
		// Every saved source was removed (e.g. only OTruyen was enabled):
		// start with all sources rather than none.
		for i := range toggles {
			toggles[i] = true
		}
	}
	return toggles
}

// providerByName finds a configured provider (nil if none matches).
func (m SearchModel) providerByName(name string) api.MangaProvider {
	for _, p := range m.providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}

func (m SearchModel) activeProviderNames() []string {
	var names []string
	for i, p := range m.providers {
		if m.providerToggles[i] {
			names = append(names, p.Name())
		}
	}
	return names
}

// activeOrError returns the enabled providers, or records errNoSource when the
// user has switched every source off.
func (m *SearchModel) activeOrError() ([]api.MangaProvider, bool) {
	active := m.activeProviders()
	if len(active) == 0 {
		m.err = errNoSource
		m.isSearching = false
		return nil, false
	}
	return active, true
}

// toggleCurrentSource flips the checkbox under the source cursor and returns
// the command that persists it.
func (m *SearchModel) toggleCurrentSource() tea.Cmd {
	indices := m.filteredProviderIndices()
	if m.sourceCursor >= len(indices) {
		return nil
	}
	idx := indices[m.sourceCursor]
	m.providerToggles[idx] = !m.providerToggles[idx]
	m.sourceSaveSeq++
	return saveSourcesCmd(m.sourceSaveSeq, m.activeProviderNames())
}

// sourceSaves orders source-toggle writes: commands run concurrently, so a
// slower, older snapshot must not overwrite a newer one.
var sourceSaves struct {
	sync.Mutex
	latest int
}

func saveSourcesCmd(seq int, names []string) tea.Cmd {
	return func() tea.Msg {
		sourceSaves.Lock()
		defer sourceSaves.Unlock()
		if seq <= sourceSaves.latest {
			return nil
		}
		sourceSaves.latest = seq
		return storageSavedMsg{err: storage.SaveSources(names)}
	}
}

//lint:ignore ST1005 Vietnamese UI message is intentionally capitalized.
var errNoSource = errors.New("Chọn ít nhất một nguồn trong /src")

package storage

// EntryFix maps a saved entry to its current provider name and IDs. It returns
// its inputs unchanged for entries that need no migration; chapterID is empty
// for favorites.
type EntryFix func(provider, mangaID, chapterID string) (newProvider, newMangaID, newChapterID string)

// MigrateEntries rewrites favorites and reading history with fix, e.g. after
// a source is merged into another. It is idempotent and only writes files
// that changed. When two history entries collapse onto one key, the most
// recently read wins.
func MigrateEntries(fix EntryFix) error {
	if err := migrateFavorites(fix); err != nil {
		return err
	}
	return migrateHistory(fix)
}

func migrateFavorites(fix EntryFix) error {
	userdataMu.Lock()
	defer userdataMu.Unlock()

	ud, err := LoadUserData()
	if err != nil {
		return err
	}
	changed := false
	seen := make(map[string]bool, len(ud.Favorites))
	kept := ud.Favorites[:0]
	for _, f := range ud.Favorites {
		p, id, _ := fix(f.Provider, f.MangaID, "")
		if p != f.Provider || id != f.MangaID {
			f.Provider, f.MangaID = p, id
			changed = true
		}
		if key := historyKey(f.MangaID, f.Provider); seen[key] {
			changed = true
			continue
		} else {
			seen[key] = true
		}
		kept = append(kept, f)
	}
	if !changed {
		return nil
	}
	ud.Favorites = kept
	return SaveUserData(ud)
}

func migrateHistory(fix EntryFix) error {
	historyMu.Lock()
	if err := loadHistory(); err != nil {
		historyMu.Unlock()
		return err
	}
	changed := false
	migrated := make(map[string]ReadHistory, len(historyCache))
	for oldKey, e := range historyCache {
		p, id, ch := fix(e.Provider, e.MangaID, e.ChapterID)
		if p != e.Provider || id != e.MangaID || ch != e.ChapterID {
			e.Provider, e.MangaID, e.ChapterID = p, id, ch
			changed = true
			if historyKey(id, p) != oldKey {
				historyDeleted[oldKey] = true // the file still has it under the old key
			}
		}
		key := historyKey(e.MangaID, e.Provider)
		if old, ok := migrated[key]; ok {
			changed = true
			if old.UpdatedAt >= e.UpdatedAt {
				continue
			}
		}
		migrated[key] = e
	}
	if changed {
		historyCache = migrated
	}
	historyMu.Unlock()

	if !changed {
		return nil
	}
	return FlushHistory()
}

package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type UserData struct {
	Sources   []string        `json:"sources,omitempty"`
	Favorites []FavoriteManga `json:"favorites,omitempty"`
	// ChapterLang is the MangaDex chapter language chosen with /lang.
	ChapterLang string `json:"chapter_lang,omitempty"`
	// ReaderFitWidth remembers the reader's webtoon (fit-width) mode.
	ReaderFitWidth bool `json:"reader_fit_width,omitempty"`
}

// userdataMu serializes read-modify-write cycles: favorites and source
// toggles are saved from concurrent tea.Cmds, and an unguarded update could
// overwrite the other's change.
var userdataMu sync.Mutex

type FavoriteManga struct {
	MangaID  string `json:"manga_id"`
	Title    string `json:"title"`
	Provider string `json:"provider,omitempty"`
}

func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("lấy thư mục home: %w", err)
	}
	dir := filepath.Join(home, ".config", "futon")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("tạo thư mục cấu hình: %w", err)
	}
	return dir, nil
}

func userdataPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "userdata.json"), nil
}

func LoadUserData() (*UserData, error) {
	path, err := userdataPath()
	if err != nil {
		return nil, err
	}

	if err := migrateOldData(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &UserData{}, nil
		}
		return nil, fmt.Errorf("đọc file userdata: %w", err)
	}

	var ud UserData
	if err := json.Unmarshal(data, &ud); err != nil {
		return nil, fmt.Errorf("parse userdata JSON: %w", err)
	}
	return &ud, nil
}

func updateUserData(update func(*UserData)) error {
	userdataMu.Lock()
	defer userdataMu.Unlock()

	ud, err := LoadUserData()
	if err != nil {
		return err
	}
	update(ud)
	return SaveUserData(ud)
}

func SaveUserData(ud *UserData) error {
	path, err := userdataPath()
	if err != nil {
		return err
	}

	if err := writeJSON(path, ud); err != nil {
		return fmt.Errorf("ghi file userdata: %w", err)
	}
	return nil
}

func LoadSources() ([]string, error) {
	ud, err := LoadUserData()
	if err != nil {
		return nil, err
	}
	return ud.Sources, nil
}

func SaveSources(names []string) error {
	return updateUserData(func(ud *UserData) { ud.Sources = names })
}

// LoadChapterLang returns the saved MangaDex chapter language, or "" if unset.
func LoadChapterLang() (string, error) {
	ud, err := LoadUserData()
	if err != nil {
		return "", err
	}
	return ud.ChapterLang, nil
}

func SaveChapterLang(lang string) error {
	return updateUserData(func(ud *UserData) { ud.ChapterLang = lang })
}

// LoadReaderFitWidth reports whether the reader opens in fit-width mode.
func LoadReaderFitWidth() (bool, error) {
	ud, err := LoadUserData()
	if err != nil {
		return false, err
	}
	return ud.ReaderFitWidth, nil
}

func SaveReaderFitWidth(on bool) error {
	return updateUserData(func(ud *UserData) { ud.ReaderFitWidth = on })
}

func LoadFavorites() ([]FavoriteManga, error) {
	ud, err := LoadUserData()
	if err != nil {
		return nil, err
	}
	return ud.Favorites, nil
}

// AddFavorite stores a manga once per (provider, ID): IDs are opaque strings
// per source, so the same ID on two sources is two different manga.
func AddFavorite(manga FavoriteManga) error {
	return updateUserData(func(ud *UserData) {
		for _, f := range ud.Favorites {
			if f.MangaID == manga.MangaID && f.Provider == manga.Provider {
				return
			}
		}
		ud.Favorites = append(ud.Favorites, manga)
	})
}

func RemoveFavorite(mangaID, provider string) error {
	return updateUserData(func(ud *UserData) {
		for i, f := range ud.Favorites {
			if f.MangaID == mangaID && f.Provider == provider {
				ud.Favorites = append(ud.Favorites[:i], ud.Favorites[i+1:]...)
				return
			}
		}
	})
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	// Write to a sibling temp file and rename: a crash mid-write must not leave
	// favorites/history truncated.
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	// Force the data to disk before the rename makes it the real file;
	// otherwise a power cut can leave an empty file (APFS does not order these).
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// migrateOldData — because past me thought one file per feature was a good idea.
func migrateOldData() error {
	path, err := userdataPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return nil
	}

	oldDir, err := ConfigDir()
	if err != nil {
		return err
	}

	var ud UserData

	oldFavPath := filepath.Join(oldDir, "favorites.json")
	if data, err := os.ReadFile(oldFavPath); err == nil {
		var favs []FavoriteManga
		if json.Unmarshal(data, &favs) == nil {
			ud.Favorites = favs
		}
	}

	oldSrcPath := filepath.Join(oldDir, "sources.json")
	if data, err := os.ReadFile(oldSrcPath); err == nil {
		var srcs []string
		if json.Unmarshal(data, &srcs) == nil {
			ud.Sources = srcs
		}
	}

	if ud.Favorites == nil && ud.Sources == nil {
		return nil
	}
	if ud.Favorites == nil {
		ud.Favorites = []FavoriteManga{}
	}

	if err := SaveUserData(&ud); err != nil {
		return err
	}

	os.Remove(oldFavPath)
	os.Remove(oldSrcPath)

	return nil
}

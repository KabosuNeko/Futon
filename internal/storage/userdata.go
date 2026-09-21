package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type UserData struct {
	Sources   []string        `json:"sources,omitempty"`
	Favorites []FavoriteManga `json:"favorites,omitempty"`
}

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

func LoadFavorites() ([]FavoriteManga, error) {
	ud, err := LoadUserData()
	if err != nil {
		return nil, err
	}
	return ud.Favorites, nil
}

func AddFavorite(manga FavoriteManga) error {
	return updateUserData(func(ud *UserData) {
		for _, f := range ud.Favorites {
			if f.MangaID == manga.MangaID {
				return
			}
		}
		ud.Favorites = append(ud.Favorites, manga)
	})
}

func RemoveFavorite(mangaID string) error {
	return updateUserData(func(ud *UserData) {
		for i, f := range ud.Favorites {
			if f.MangaID == mangaID {
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
	return os.WriteFile(path, data, 0o644)
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

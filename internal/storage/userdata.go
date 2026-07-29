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
			return nil, nil
		}
		return nil, fmt.Errorf("đọc file userdata: %w", err)
	}

	var ud UserData
	if err := json.Unmarshal(data, &ud); err != nil {
		return nil, fmt.Errorf("parse userdata JSON: %w", err)
	}
	return &ud, nil
}

func SaveUserData(ud *UserData) error {
	path, err := userdataPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(ud, "", "  ")
	if err != nil {
		return fmt.Errorf("encode userdata JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("ghi file userdata: %w", err)
	}
	return nil
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

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	oldDir := filepath.Join(home, ".config", "futon")

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

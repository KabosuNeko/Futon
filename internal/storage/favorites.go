package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type FavoriteManga struct {
	MangaID string `json:"manga_id"`
	Title   string `json:"title"`
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

func favoritesPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "favorites.json"), nil
}

func LoadFavorites() ([]FavoriteManga, error) {
	path, err := favoritesPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []FavoriteManga{}, nil
		}
		return nil, fmt.Errorf("đọc file favorites: %w", err)
	}

	var favorites []FavoriteManga
	if err := json.Unmarshal(data, &favorites); err != nil {
		return nil, fmt.Errorf("parse favorites JSON: %w", err)
	}
	return favorites, nil
}

func SaveFavorites(favorites []FavoriteManga) error {
	path, err := favoritesPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(favorites, "", "  ")
	if err != nil {
		return fmt.Errorf("encode favorites JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("ghi file favorites: %w", err)
	}
	return nil
}

func AddFavorite(manga FavoriteManga) error {
	favorites, err := LoadFavorites()
	if err != nil {
		return err
	}

	for _, f := range favorites {
		if f.MangaID == manga.MangaID {
			return nil
		}
	}

	favorites = append(favorites, manga)
	return SaveFavorites(favorites)
}

func RemoveFavorite(mangaID string) error {
	favorites, err := LoadFavorites()
	if err != nil {
		return err
	}

	for i, f := range favorites {
		if f.MangaID == mangaID {
			favorites = append(favorites[:i], favorites[i+1:]...)
			return SaveFavorites(favorites)
		}
	}
	return nil
}

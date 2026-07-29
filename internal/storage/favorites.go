package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

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

func LoadFavorites() ([]FavoriteManga, error) {
	ud, err := LoadUserData()
	if err != nil {
		return nil, err
	}
	if ud == nil {
		return []FavoriteManga{}, nil
	}
	return ud.Favorites, nil
}

func SaveFavorites(favorites []FavoriteManga) error {
	ud, err := LoadUserData()
	if err != nil {
		return err
	}
	if ud == nil {
		ud = &UserData{Favorites: favorites}
	} else {
		ud.Favorites = favorites
	}
	return SaveUserData(ud)
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

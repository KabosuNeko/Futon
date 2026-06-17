package api

import "github.com/user/futon/internal/models"

// MangaSearchResultMsg được gửi khi tìm kiếm manga hoàn tất.
type MangaSearchResultMsg struct {
	Manga []models.Manga
	Err   error
}

// ChapterListMsg được gửi khi danh sách chapter đã được tải.
type ChapterListMsg struct {
	Chapters []models.Chapter
	Err      error
}

// ChapterImagesMsg được gửi khi danh sách URL ảnh chapter đã được tải.
type ChapterImagesMsg struct {
	URLs []string
	Err  error
}

// MangaProvider là interface chung cho các nguồn truyện.
type MangaProvider interface {
	Name() string
	Search(keyword string) ([]models.Manga, error)
	FetchChapters(mangaID string) ([]models.Chapter, error)
	FetchPages(chapterID string) ([]string, error)
}

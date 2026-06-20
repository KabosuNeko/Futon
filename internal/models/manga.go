package models

type Manga struct {
	ID        string
	Title     string
	CoverURL  string
}

type MangaSearchResponse struct {
	Data []MangaData `json:"data"`
}

type MangaData struct {
	ID         string          `json:"id"`
	Attributes MangaAttributes `json:"attributes"`
}

type MangaAttributes struct {
	Title map[string]string `json:"title"`
}

func (d MangaData) ToManga() Manga {
	return Manga{
		ID:    d.ID,
		Title: pickTitle(d.Attributes.Title),
	}
}

func pickTitle(titles map[string]string) string {
	if t, ok := titles["en"]; ok {
		return t
	}
	if t, ok := titles["ja-ro"]; ok {
		return t
	}
	for _, t := range titles {
		return t
	}
	return "Không rõ tiêu đề"
}

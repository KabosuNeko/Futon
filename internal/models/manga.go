package models

import "fmt"

type Manga struct {
	ID       string
	Title    string
	CoverURL string
	Provider string
	Author   string
	Year     string
	Status   string
	Genres   []string
}

type MangaSearchResponse struct {
	Data  []MangaData `json:"data"`
	Total int         `json:"total"`
}

type MangaData struct {
	ID            string              `json:"id"`
	Attributes    MangaAttributes     `json:"attributes"`
	Relationships []MangaRelationship `json:"relationships"`
}

type MangaRelationship struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	Attributes *MangaRelAttributes `json:"attributes"`
}

type MangaRelAttributes struct {
	FileName string `json:"fileName"`
	Name     string `json:"name"`
}

type MangaTag struct {
	Attributes struct {
		Name map[string]string `json:"name"`
	} `json:"attributes"`
}

type MangaAttributes struct {
	Title  map[string]string `json:"title"`
	Year   *int              `json:"year"`
	Status string            `json:"status"`
	Tags   []MangaTag        `json:"tags"`
}

func (d MangaData) ToManga() Manga {
	var coverURL string
	var author string
	for _, rel := range d.Relationships {
		if rel.Type == "cover_art" && rel.Attributes != nil && rel.Attributes.FileName != "" {
			coverURL = "https://uploads.mangadex.org/covers/" + d.ID + "/" + rel.Attributes.FileName + ".256.jpg"
		}
		if (rel.Type == "author" || rel.Type == "artist") && rel.Attributes != nil && rel.Attributes.Name != "" {
			if author == "" {
				author = rel.Attributes.Name
			}
		}
	}

	var year string
	if d.Attributes.Year != nil && *d.Attributes.Year > 0 {
		year = fmt.Sprintf("%d", *d.Attributes.Year)
	}

	var genres []string
	for _, tag := range d.Attributes.Tags {
		if name, ok := tag.Attributes.Name["en"]; ok && name != "" {
			genres = append(genres, name)
		}
	}

	return Manga{
		ID:       d.ID,
		Title:    pickTitle(d.Attributes.Title),
		CoverURL: coverURL,
		Author:   author,
		Year:     year,
		Status:   d.Attributes.Status,
		Genres:   genres,
	}
}

func pickTitle(titles map[string]string) string {
	if t, ok := titles["vi"]; ok && t != "" {
		return t
	}
	if t, ok := titles["en"]; ok && t != "" {
		return t
	}
	if t, ok := titles["ja-ro"]; ok && t != "" {
		return t
	}
	for _, t := range titles {
		if t != "" {
			return t
		}
	}
	return "Không rõ tiêu đề"
}

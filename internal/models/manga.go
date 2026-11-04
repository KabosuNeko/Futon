package models

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

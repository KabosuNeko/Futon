package models

type Chapter struct {
	ID     string
	Number string
	Title  string
}

type ChapterFeedResponse struct {
	Data   []ChapterData `json:"data"`
	Limit  int           `json:"limit"`
	Offset int           `json:"offset"`
	Total  int           `json:"total"`
}

type ChapterData struct {
	ID         string            `json:"id"`
	Attributes ChapterAttributes `json:"attributes"`
}

type ChapterAttributes struct {
	Chapter string `json:"chapter"`
	Title   string `json:"title"`
}

func (d ChapterData) ToChapter() Chapter {
	return Chapter{
		ID:     d.ID,
		Number: d.Attributes.Chapter,
		Title:  d.Attributes.Title,
	}
}

type AtHomeResponse struct {
	BaseURL string        `json:"baseUrl"`
	Chapter AtHomeChapter `json:"chapter"`
}

type AtHomeChapter struct {
	Hash string   `json:"hash"`
	Data []string `json:"data"`
}

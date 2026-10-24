package models

import "testing"

func TestPickTitlePriority(t *testing.T) {
	cases := []struct {
		name   string
		titles map[string]string
		want   string
	}{
		{"vi wins", map[string]string{"vi": "Vi", "en": "En", "ja-ro": "Ja"}, "Vi"},
		{"en next", map[string]string{"en": "En", "ja-ro": "Ja"}, "En"},
		{"ja-ro next", map[string]string{"ja-ro": "Ja"}, "Ja"},
		{"any language fallback", map[string]string{"fr": "Fr"}, "Fr"},
		{"empty", map[string]string{}, "Không rõ tiêu đề"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pickTitle(tc.titles); got != tc.want {
				t.Errorf("pickTitle(%v) = %q, want %q", tc.titles, got, tc.want)
			}
		})
	}
}

func TestMangaDataToManga(t *testing.T) {
	year := 1997
	action := MangaTag{}
	action.Attributes.Name = map[string]string{"en": "Action"}
	vietnamese := MangaTag{}
	vietnamese.Attributes.Name = map[string]string{"vi": "Hành động"}

	data := MangaData{
		ID: "manga-1",
		Attributes: MangaAttributes{
			Title:  map[string]string{"en": "One Piece"},
			Year:   &year,
			Status: "ongoing",
			Tags:   []MangaTag{action, vietnamese},
		},
		Relationships: []MangaRelationship{
			{Type: "cover_art", Attributes: &MangaRelAttributes{FileName: "cover.jpg"}},
			{Type: "author", Attributes: &MangaRelAttributes{Name: "Oda"}},
			{Type: "artist", Attributes: &MangaRelAttributes{Name: "Oda"}},
		},
	}

	manga := data.ToManga()
	if manga.ID != "manga-1" || manga.Title != "One Piece" {
		t.Errorf("unexpected identity: %+v", manga)
	}
	if manga.CoverURL != "https://uploads.mangadex.org/covers/manga-1/cover.jpg.256.jpg" {
		t.Errorf("CoverURL = %q", manga.CoverURL)
	}
	if manga.Author != "Oda" || manga.Year != "1997" || manga.Status != "ongoing" {
		t.Errorf("unexpected metadata: %+v", manga)
	}
	if len(manga.Genres) != 1 || manga.Genres[0] != "Action" {
		t.Errorf("Genres = %v, want [Action] (English tags only)", manga.Genres)
	}
}

package api

import "testing"

func TestMangaDexTitlePriority(t *testing.T) {
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
			if got := mangadexTitle(tc.titles); got != tc.want {
				t.Errorf("mangadexTitle(%v) = %q, want %q", tc.titles, got, tc.want)
			}
		})
	}
}

func TestMangaDexMangaToManga(t *testing.T) {
	year := 1997
	action := mangadexTag{}
	action.Attributes.Name = map[string]string{"en": "Action"}
	vietnamese := mangadexTag{}
	vietnamese.Attributes.Name = map[string]string{"vi": "Hành động"}

	data := mangadexManga{
		ID: "manga-1",
		Attributes: mangadexMangaAttributes{
			Title:  map[string]string{"en": "One Piece"},
			Year:   &year,
			Status: "ongoing",
			Tags:   []mangadexTag{action, vietnamese},
		},
		Relationships: []mangadexRelationship{
			{Type: "cover_art", Attributes: &mangadexRelAttributes{FileName: "cover.jpg"}},
			{Type: "author", Attributes: &mangadexRelAttributes{Name: "Oda"}},
			{Type: "artist", Attributes: &mangadexRelAttributes{Name: "Oda"}},
		},
	}

	manga := data.toManga()
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

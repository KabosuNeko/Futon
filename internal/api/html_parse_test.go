package api

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func testDocument(t *testing.T, body string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		t.Fatalf("parse test HTML: %v", err)
	}
	return doc
}

func TestParseBaoTangList(t *testing.T) {
	doc := testDocument(t, `
		<div class="items">
			<div class="row">
				<div class="item">
					<div class="image"><a href="/truyen/naruto"><img data-original="/covers/naruto.jpg"></a></div>
					<figcaption><h3><a href="/truyen/naruto">Naruto</a></h3></figcaption>
				</div>
			</div>
		</div>`)

	mangas := parseBaoTangList(doc, "div.items > div.row > div.item")
	if len(mangas) != 1 {
		t.Fatalf("mangas = %d, want 1", len(mangas))
	}
	if mangas[0].ID != "/truyen/naruto" || mangas[0].Title != "Naruto" || mangas[0].CoverURL != "/covers/naruto.jpg" {
		t.Errorf("unexpected manga: %+v", mangas[0])
	}
}

func TestParseTruyenQQList(t *testing.T) {
	doc := testDocument(t, `
		<ul class="list_grid">
			<li>
				<h3><a href="/truyen/bleach">Bleach</a></h3>
				<img src="/covers/bleach.jpg">
			</li>
		</ul>`)

	mangas := parseTruyenHTMLList(doc)
	if len(mangas) != 1 {
		t.Fatalf("mangas = %d, want 1", len(mangas))
	}
	if mangas[0].ID != "/truyen/bleach" || mangas[0].Title != "Bleach" || mangas[0].CoverURL != "/covers/bleach.jpg" {
		t.Errorf("unexpected manga: %+v", mangas[0])
	}
}

func TestImageSrcPrefersLazyAttributes(t *testing.T) {
	doc := testDocument(t, `<img src="/fallback.jpg" data-original="/original.jpg">`)
	src, ok := imageSrc(doc.Find("img"))
	if !ok || src != "/original.jpg" {
		t.Errorf("imageSrc = %q, %v; want /original.jpg, true", src, ok)
	}
}

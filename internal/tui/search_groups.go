package tui

import (
	"fmt"
	"maps"
	"strings"
	"unicode"

	"github.com/KabosuNeko/Futon/internal/models"
)

// resultRow is one line of the result list: the same title as found on one
// or more sources. Rows are derived from m.results on demand, so streaming,
// paging and merging keep working on the flat list.
type resultRow struct {
	key      string
	variants []models.Manga
}

// titleKey normalizes a title so that the same manga on different sources
// lands on one row: the source suffix is dropped, case is folded and any
// punctuation or spacing difference is ignored.
func titleKey(m models.Manga) string {
	t := strings.ToLower(cleanTitle(m.Title, m.Provider))
	var b strings.Builder
	space := false
	for _, r := range t {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			b.WriteRune(r)
			space = false
		} else {
			space = true
		}
	}
	return b.String() // "" for untitled entries, which are never merged
}

// rowCache remembers the grouping of one results slice. Results are only ever
// replaced by a new slice or grown, so its first element and length identify
// it. Grouping 3000 results took ~75% of a search-screen frame, and a frame
// asks for the rows several times.
type rowCache struct {
	first *models.Manga
	n     int
	rows  []resultRow
}

// resultRows groups m.results by title, in first-seen order.
func (m SearchModel) resultRows() []resultRow {
	if len(m.results) == 0 {
		return nil
	}
	if c := m.rowCache; c != nil && c.first == &m.results[0] && c.n == len(m.results) {
		return c.rows
	}
	rows := groupRows(m.results)
	if m.rowCache != nil {
		*m.rowCache = rowCache{first: &m.results[0], n: len(m.results), rows: rows}
	}
	return rows
}

func groupRows(results []models.Manga) []resultRow {
	var rows []resultRow
	index := make(map[string]int, len(results))
	for n, r := range results {
		key := titleKey(r)
		if key == "" {
			key = fmt.Sprintf("\x00%d", n)
		}
		if i, ok := index[key]; ok {
			rows[i].variants = append(rows[i].variants, r)
			continue
		}
		index[key] = len(rows)
		rows = append(rows, resultRow{key: key, variants: []models.Manga{r}})
	}
	return rows
}

// selectedVariant is the row's manga on the source the user picked
// (the first source by default).
func (m SearchModel) selectedVariant(row resultRow) (models.Manga, int) {
	if provider, ok := m.variantSel[row.key]; ok {
		for i, v := range row.variants {
			if v.Provider == provider {
				return v, i
			}
		}
	}
	return row.variants[0], 0
}

// focusedRow returns the result row under the cursor.
func (m SearchModel) focusedRow() (resultRow, bool) {
	rows := m.resultRows()
	if m.cursor < 0 || m.cursor >= len(rows) {
		return resultRow{}, false
	}
	return rows[m.cursor], true
}

// cycleVariant switches the focused row to its next/previous source.
func (m *SearchModel) cycleVariant(delta int) bool {
	row, ok := m.focusedRow()
	if !ok || len(row.variants) < 2 {
		return false
	}
	_, cur := m.selectedVariant(row)
	next := (cur + delta + len(row.variants)) % len(row.variants)
	// Copy on write: models are values, and a shared map would leak the
	// change into earlier copies.
	sel := maps.Clone(m.variantSel)
	if sel == nil {
		sel = make(map[string]string)
	}
	sel[row.key] = row.variants[next].Provider
	m.variantSel = sel
	return true
}

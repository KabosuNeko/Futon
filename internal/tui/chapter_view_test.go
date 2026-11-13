package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/KabosuNeko/Futon/internal/models"
)

func TestChapterListFitsAnEightyColumnTerminal(t *testing.T) {
	m := NewChapterListModel("m", "Blue Lock", nil)
	m.setSize(80, 24)
	m.loading = false
	for i := 1; i <= 40; i++ {
		m.chapters = append(m.chapters, models.Chapter{ID: string(rune('a' + i)), Number: "1", Title: "Ước mơ"})
	}

	lines := strings.Split(m.View().Content, "\n")
	for _, line := range lines {
		if w := lipgloss.Width(line); w > 80 {
			t.Fatalf("a %d-column line on an 80-column terminal: %q", w, line)
		}
	}
	if len(lines) > 24 {
		t.Errorf("view is %d rows on a 24-row terminal", len(lines))
	}
	if !strings.Contains(m.View().Content, "Quay lại") {
		t.Error("the esc hint was lost")
	}
}

func TestChapterListFitsOrSaysTooSmall(t *testing.T) {
	for rows := 6; rows <= 24; rows++ {
		m := NewChapterListModel("m", "Blue Lock", nil)
		m.setSize(80, rows)
		m.loading = false
		for i := 0; i < 40; i++ {
			m.chapters = append(m.chapters, models.Chapter{ID: string(rune('a' + i)), Number: "1"})
		}
		if h := len(strings.Split(m.View().Content, "\n")); h > rows {
			t.Errorf("chapter list at 80x%d: %d rows", rows, h)
		}
		if rows >= 12 && strings.Contains(m.View().Content, "quá nhỏ") {
			t.Errorf("chapter list at 80x%d shows the notice but fits", rows)
		}
	}
}

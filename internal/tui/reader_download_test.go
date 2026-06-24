package tui

import "testing"

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Hello/World", "Hello-World"},
		{"a:b*c?d\"e<f>g|h\\i", "a-b-c-d-e-f-g-h-i"},
		{"", ""},
		{"normal", "normal"},
		{"Chap: 1/2?", "Chap- 1-2-"},
	}

	for _, c := range cases {
		got := sanitizeFilename(c.input)
		if got != c.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

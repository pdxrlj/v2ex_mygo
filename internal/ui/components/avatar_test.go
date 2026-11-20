package components

import "testing"

// TestResolveImageURL checks that Markdown picture addresses become absolute
// against the site.
func TestResolveImageURL(t *testing.T) {
	cases := map[string]string{
		"https://cdn.example.com/a.webp":    "https://cdn.example.com/a.webp",
		"//cdn.example.com/a.webp":          "https://cdn.example.com/a.webp",
		"/static/a.png":                     "https://www.v2ex.com/static/a.png",
		"static/a.png":                      "https://www.v2ex.com/static/a.png",
		"  https://cdn.example.com/a.png  ": "https://cdn.example.com/a.png",
	}
	for raw, want := range cases {
		if got := resolveImageURL(raw); got != want {
			t.Errorf("resolveImageURL(%q) = %q, want %q", raw, got, want)
		}
	}
}

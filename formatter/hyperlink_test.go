package formatter

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kthesword/github-trending-repos/models"
)

const (
	linkStart = "\x1b]8;;"
	linkEnd   = "\x1b]8;;\x1b\\"
)

func TestRenderLinksRepositoryNamesWhenColorEnabled(t *testing.T) {
	var buf bytes.Buffer
	repos := []models.Repository{{
		FullName: "acme/rocket",
		HTMLURL:  "https://github.com/acme/rocket",
	}}
	if err := NewTable(&buf, WithColor(true)).Render(testMeta, repos); err != nil {
		t.Fatal(err)
	}

	want := linkStart + "https://github.com/acme/rocket\x1b\\" + "acme/rocket" + linkEnd
	if !strings.Contains(buf.String(), want) {
		t.Errorf("output missing hyperlinked name %q\n%q", want, buf.String())
	}
}

func TestRenderNoLinksWhenColorDisabled(t *testing.T) {
	out := render(t, testMeta, []models.Repository{{
		FullName: "acme/rocket",
		HTMLURL:  "https://github.com/acme/rocket",
	}})
	if strings.Contains(out, "\x1b") {
		t.Errorf("plain output must not contain escape sequences:\n%q", out)
	}
}

func TestSafeURL(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://github.com/a/b", "https://github.com/a/b"},
		{"  http://example.com/x  ", "http://example.com/x"},
		{"", ""},
		{"javascript:alert(1)", ""},
		{"ftp://example.com", ""},
		{"https://evil.com/\x1b\\\x1b]0;pwned\x07", ""},
		{"https://evil.com/a b", ""},
	}
	for _, tt := range tests {
		if got := safeURL(tt.in); got != tt.want {
			t.Errorf("safeURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestHyperlinkEmptyURLReturnsText(t *testing.T) {
	if got := hyperlink("", "name"); got != "name" {
		t.Errorf("hyperlink(\"\", name) = %q, want plain text", got)
	}
}

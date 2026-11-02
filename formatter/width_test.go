package formatter

import (
	"strings"
	"testing"

	"github.com/kthesword/github-trending-repos/models"
	"github.com/mattn/go-runewidth"
)

// Wide (CJK) characters occupy two terminal cells. If widths are counted in
// runes, descriptions overflow the intended width and wrap, corrupting the table.
func TestTruncateCountsDisplayCells(t *testing.T) {
	got := truncate("红果短剧的非官方匿名SDK提供多种语言支持", 10)
	if w := runewidth.StringWidth(got); w > 10 {
		t.Errorf("truncate() = %q has width %d, want <= 10", got, w)
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("truncate() = %q, want trailing ellipsis", got)
	}
}

func TestRenderRowsWithWideCharsStayWithinWidth(t *testing.T) {
	out := render(t, testMeta, []models.Repository{
		{FullName: "a/one", Description: strings.Repeat("中文描述", 30), Language: "Go"},
		{FullName: "b/two", Description: strings.Repeat("ascii ", 30), Language: "Go"},
	}, WithMaxDescriptionWidth(40))

	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, "a/one") && !strings.Contains(line, "b/two") {
			continue
		}
		// indent + rank + name + stars + language + gaps + 40-cell description
		if w := runewidth.StringWidth(line); w > 80 {
			t.Errorf("row is %d cells wide, would wrap in a typical terminal: %q", w, line)
		}
	}
}

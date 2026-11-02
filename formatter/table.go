// Package formatter renders repository data for display in a terminal.
package formatter

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/fatih/color"
	"github.com/kthesword/github-trending-repos/models"
)

const (
	// DefaultMaxDescriptionWidth is the default number of characters shown
	// for a repository description before it is truncated.
	DefaultMaxDescriptionWidth = 60

	indent      = "  "
	columnGap   = "   "
	placeholder = "-"
	ellipsis    = "…"
)

// Meta carries contextual information displayed around the table.
type Meta struct {
	Period     string    // human-readable window, e.g. "past 7 days"
	Since      time.Time // start of the window
	TotalCount int       // total matches reported by GitHub
	Incomplete bool      // GitHub reported incomplete results
}

// Table renders repositories as an aligned, optionally colorized table.
//
// Column widths are computed on the plain text *before* colors are applied,
// so ANSI escape sequences never break alignment (a common pitfall when
// combining text/tabwriter with colored output).
type Table struct {
	w            io.Writer
	useColor     bool
	maxDescWidth int
}

// Option customizes a Table.
type Option func(*Table)

// WithColor forces colorized output on or off. By default color is enabled
// only when fatih/color detects a capable terminal (and NO_COLOR is unset).
func WithColor(enabled bool) Option {
	return func(t *Table) { t.useColor = enabled }
}

// WithMaxDescriptionWidth sets the maximum description length in characters.
// Non-positive values are ignored.
func WithMaxDescriptionWidth(n int) Option {
	return func(t *Table) {
		if n > 0 {
			t.maxDescWidth = n
		}
	}
}

// NewTable creates a Table that writes to w.
func NewTable(w io.Writer, opts ...Option) *Table {
	t := &Table{
		w:            w,
		useColor:     !color.NoColor,
		maxDescWidth: DefaultMaxDescriptionWidth,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// row holds the sanitized plain-text cells for one repository.
type row struct {
	rank, name, stars, language, description string
	url                                      string // repository page; empty means "don't link"
	noLanguage, noDescription                bool
}

// widths holds the display width of each padded column. The description is
// the last column and is never padded, so it needs no width.
type widths struct {
	rank, name, stars, language int
}

// Render writes the full report: title, table, and summary footer.
func (t *Table) Render(meta Meta, repos []models.Repository) error {
	bw := bufio.NewWriter(t.w)
	st := newStyles(t.useColor)

	if len(repos) == 0 {
		fmt.Fprintln(bw, st.warn.Sprintf("No repositories found that were created in the %s.", meta.Period))
		return bw.Flush()
	}

	header := row{rank: "#", name: "REPOSITORY", stars: "STARS", language: "LANGUAGE", description: "DESCRIPTION"}
	rows := t.buildRows(repos)
	w := measure(append([]row{header}, rows...))

	// Title.
	fmt.Fprintf(bw, "\n%s%s  %s\n\n",
		indent,
		st.title.Sprint("Trending on GitHub"),
		st.subtle.Sprintf("· %s (since %s)", meta.Period, meta.Since.Local().Format("Jan 2, 2006 15:04")),
	)

	// Header + separator.
	writeRow(bw, w, header, rowStyle{rank: st.header, name: st.header, stars: st.header, language: st.header, description: st.header})
	sepWidth := w.rank + w.name + w.stars + w.language + 4*utf8.RuneCountInString(columnGap) + longestDescription(rows, header)
	fmt.Fprintln(bw, indent+st.subtle.Sprint(strings.Repeat("─", sepWidth)))

	// Body.
	for _, r := range rows {
		rs := rowStyle{rank: st.rank, name: st.name, stars: st.stars, language: st.language}
		if r.noLanguage {
			rs.language = st.subtle
		}
		if r.noDescription {
			rs.description = st.subtle
		}
		writeRow(bw, w, r, rs)
	}

	// Footer.
	fmt.Fprintf(bw, "\n%s%s\n", indent, st.subtle.Sprintf(
		"Showing %d of %s matching repositories, ranked by stars.",
		len(rows), formatCount(meta.TotalCount)))
	if meta.Incomplete {
		fmt.Fprintf(bw, "%s%s\n", indent,
			st.warn.Sprint("Note: GitHub reported incomplete results; some repositories may be missing."))
	}
	fmt.Fprintln(bw)

	return bw.Flush()
}

// buildRows converts repositories into sanitized, display-ready rows.
func (t *Table) buildRows(repos []models.Repository) []row {
	rows := make([]row, len(repos))
	for i, repo := range repos {
		name := sanitize(repo.FullName)
		if name == "" {
			name = sanitize(repo.Name)
		}

		r := row{
			rank:        strconv.Itoa(i + 1),
			name:        name,
			stars:       formatCount(repo.StargazersCount),
			language:    sanitize(repo.Language),
			description: truncate(sanitize(repo.Description), t.maxDescWidth),
		}
		// Hyperlinks are escape sequences, so only emit them when we're
		// writing to a terminal (the same condition that enables color).
		if t.useColor {
			r.url = safeURL(repo.HTMLURL)
		}
		if r.language == "" {
			r.language, r.noLanguage = placeholder, true
		}
		if r.description == "" {
			r.description, r.noDescription = placeholder, true
		}
		rows[i] = r
	}
	return rows
}

// rowStyle selects the color for each cell. A nil style means "plain text".
type rowStyle struct {
	rank, name, stars, language, description *color.Color
}

// writeRow pads each cell to its column width and only then colors it.
// The repository name is wrapped in a hyperlink when the row has a URL; its
// padding is kept outside the link so alignment and underlines stay tidy.
func writeRow(w io.Writer, cw widths, r row, rs rowStyle) {
	name := paint(rs.name, hyperlink(r.url, r.name)) +
		strings.Repeat(" ", max(0, cw.name-displayWidth(r.name)))

	fmt.Fprint(w,
		indent,
		paint(rs.rank, padLeft(r.rank, cw.rank)), columnGap,
		name, columnGap,
		paint(rs.stars, padLeft(r.stars, cw.stars)), columnGap,
		paint(rs.language, padRight(r.language, cw.language)), columnGap,
		paint(rs.description, r.description), "\n",
	)
}

// hyperlink wraps text in an OSC 8 terminal hyperlink so it is clickable in
// terminals that support it (Windows Terminal, iTerm2, VS Code, GNOME
// Terminal, kitty, ...). Terminals without support simply show the text.
// An empty url returns text unchanged.
func hyperlink(url, text string) string {
	if url == "" {
		return text
	}
	return "\x1b]8;;" + url + "\x1b\\" + text + "\x1b]8;;\x1b\\"
}

// safeURL returns raw only if it is a well-formed http(s) URL free of control
// characters. The value comes from the API response, so it must not be able
// to terminate the hyperlink sequence early and inject other escapes.
func safeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "https://") && !strings.HasPrefix(raw, "http://") {
		return ""
	}
	for _, r := range raw {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return ""
		}
	}
	return raw
}

// measure computes the widest cell in every padded column.
func measure(rows []row) widths {
	var w widths
	for _, r := range rows {
		w.rank = max(w.rank, displayWidth(r.rank))
		w.name = max(w.name, displayWidth(r.name))
		w.stars = max(w.stars, displayWidth(r.stars))
		w.language = max(w.language, displayWidth(r.language))
	}
	return w
}

func longestDescription(rows []row, header row) int {
	n := displayWidth(header.description)
	for _, r := range rows {
		n = max(n, displayWidth(r.description))
	}
	return n
}

// styles is the color palette used by the table.
type styles struct {
	title, subtle, header, rank, name, stars, language, warn *color.Color
}

func newStyles(enabled bool) styles {
	mk := func(attrs ...color.Attribute) *color.Color {
		c := color.New(attrs...)
		if enabled {
			c.EnableColor()
		} else {
			c.DisableColor()
		}
		return c
	}
	return styles{
		title:    mk(color.FgHiMagenta, color.Bold),
		subtle:   mk(color.FgHiBlack),
		header:   mk(color.FgHiWhite, color.Bold),
		rank:     mk(color.FgHiBlack),
		name:     mk(color.FgHiCyan, color.Bold),
		stars:    mk(color.FgHiYellow),
		language: mk(color.FgHiGreen),
		warn:     mk(color.FgYellow),
	}
}

func paint(c *color.Color, s string) string {
	if c == nil {
		return s
	}
	return c.Sprint(s)
}

// sanitize makes untrusted API text safe for single-line terminal output:
// control characters (including ANSI escape sequences that a malicious
// description could use to manipulate the terminal) are removed, and runs
// of whitespace such as tabs and newlines are collapsed into single spaces.
func sanitize(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// truncate shortens s to at most maxRunes characters, ending with an
// ellipsis when text was removed.
func truncate(s string, maxRunes int) string {
	if maxRunes <= 0 || utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	if maxRunes == 1 {
		return ellipsis
	}
	return strings.TrimRight(string(runes[:maxRunes-1]), " ") + ellipsis
}

// formatCount renders an integer with thousands separators (e.g. 12,345).
func formatCount(n int) string {
	s := strconv.Itoa(n)
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}
	if len(s) <= 3 {
		return sign + s
	}

	var b strings.Builder
	b.WriteString(sign)
	head := len(s) % 3
	if head > 0 {
		b.WriteString(s[:head])
	}
	for i := head; i < len(s); i += 3 {
		if b.Len() > len(sign) {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func displayWidth(s string) int { return utf8.RuneCountInString(s) }

func padRight(s string, width int) string {
	if gap := width - displayWidth(s); gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	return s
}

func padLeft(s string, width int) string {
	if gap := width - displayWidth(s); gap > 0 {
		return strings.Repeat(" ", gap) + s
	}
	return s
}

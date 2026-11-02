package models

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// maxLanguageLen bounds user input; the longest GitHub language names are
// well under this.
const maxLanguageLen = 50

// ParseLanguage validates and normalizes a --language value.
//
// An empty (or all-whitespace) value is valid and means "any language". Names
// are not checked against a fixed list, because GitHub adds languages over
// time; unknown names simply match nothing. Matching on GitHub's side is
// case-insensitive, so the value is passed through unchanged apart from
// trimming and collapsing internal whitespace (e.g. "Jupyter  Notebook").
func ParseLanguage(s string) (string, error) {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "", nil
	}
	if len([]rune(s)) > maxLanguageLen {
		return "", fmt.Errorf("invalid language: must be at most %d characters", maxLanguageLen)
	}
	for _, r := range s {
		// Quotes would break out of the quoted search qualifier we build.
		if r == '"' || unicode.IsControl(r) {
			return "", errors.New(`invalid language: must not contain quotes or control characters`)
		}
	}
	return s, nil
}

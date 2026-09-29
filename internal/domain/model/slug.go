package model

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Slug normalizes a wizard name into the path segment used by load_config ids.
//
// It trims, lowercases, strips accents, turns spaces and "-" into "_", and
// keeps only [a-z0-9_]. An empty result is rejected.
func Slug(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = stripAccents(value)

	var builder strings.Builder
	builder.Grow(len(value))

	for _, symbol := range value {
		switch {
		case symbol == '-' || unicode.IsSpace(symbol):
			builder.WriteByte('_')
		case symbol == '_' || (symbol >= 'a' && symbol <= 'z') || (symbol >= '0' && symbol <= '9'):
			builder.WriteRune(symbol)
		}
	}

	slug := builder.String()
	if slug == "" {
		return "", ErrEmptySlug
	}

	return slug, nil
}

func stripAccents(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))

	for _, symbol := range norm.NFD.String(value) {
		if unicode.Is(unicode.Mn, symbol) {
			continue
		}

		builder.WriteRune(symbol)
	}

	return builder.String()
}

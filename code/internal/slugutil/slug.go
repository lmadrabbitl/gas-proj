package slugutil

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

const maxSlugLength = 50

var nonAlphaNumericPattern = regexp.MustCompile(`[^a-z0-9]+`)

func GenerateUnique(name string, fallback string, existingCodes map[string]struct{}) string {
	base := normalize(name, fallback)
	if _, exists := existingCodes[base]; !exists {
		return base
	}

	for suffix := 2; ; suffix++ {
		suffixLabel := fmt.Sprintf("-%d", suffix)
		candidate := truncate(base, maxSlugLength-len(suffixLabel)) + suffixLabel
		if _, exists := existingCodes[candidate]; !exists {
			return candidate
		}
	}
}

func normalize(name string, fallback string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fallback
	}

	decomposed := norm.NFD.String(trimmed)
	builder := strings.Builder{}
	builder.Grow(len(decomposed))
	for _, r := range decomposed {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		builder.WriteRune(unicode.ToLower(r))
	}

	normalized := nonAlphaNumericPattern.ReplaceAllString(builder.String(), "-")
	normalized = strings.Trim(normalized, "-")
	normalized = truncate(normalized, maxSlugLength)
	if normalized == "" {
		return fallback
	}
	return normalized
}

func truncate(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	return strings.TrimRight(value[:limit], "-")
}

package utils

import (
	"regexp"
	"strings"
	"unicode"
)

type PANParsed struct {
	PAN        string
	Name       string
	FatherName string
	DOB        string
	RawText    string
}

func ParsePANText(raw string) PANParsed {
	t := strings.ToUpper(raw)

	// PAN Regex
	panRegex := regexp.MustCompile(`[A-Z]{5}[0-9]{4}[A-Z]`)
	pan := panRegex.FindString(t)

	// DOB Regex
	dobRegex := regexp.MustCompile(`(0[1-9]|[12][0-9]|3[01])[/-](0[1-9]|1[0-2])[/-][0-9]{4}`)
	dob := dobRegex.FindString(t)

	lines := cleanLines(t)

	name, father := extractNames(lines)

	return PANParsed{
		PAN:        pan,
		Name:       name,
		FatherName: father,
		DOB:        dob,
		RawText:    t,
	}
}

func cleanLines(t string) []string {
	lines := strings.Split(t, "\n")
	out := []string{}

	for _, l := range lines {
		l = strings.TrimSpace(l)
		if len(l) < 3 {
			continue
		}
		if strings.Contains(l, "INCOME") ||
			strings.Contains(l, "GOVT") ||
			strings.Contains(l, "TAX") ||
			strings.Contains(l, "DEPARTMENT") {
			continue
		}
		out = append(out, l)
	}
	return out
}

func isNameLike(s string) bool {
	for _, c := range s {
		if unicode.IsDigit(c) {
			return false
		}
	}
	return true
}

func extractNames(lines []string) (string, string) {
	var name, father string

	for i := 0; i < len(lines); i++ {
		l := lines[i]

		// Label-based detection (OCR variations handled)
		if (strings.Contains(l, "NAME") || strings.Contains(l, "/NAME")) &&
			!strings.Contains(l, "FATHER") && i+1 < len(lines) {

			candidate := strings.TrimSpace(lines[i+1])
			if isNameLike(candidate) {
				name = candidate
			}
		}

		if strings.Contains(l, "FATHER") && i+1 < len(lines) {
			candidate := strings.TrimSpace(lines[i+1])
			if isNameLike(candidate) {
				father = candidate
			}
		}
	}

	// Fallback logic — BUT must not override correct values
	if name == "" {
		for _, l := range lines {
			if isNameLike(l) && !strings.Contains(l, "FATHER") && len(strings.Fields(l)) >= 1 {
				name = l
				break
			}
		}
	}

	if father == "" {
		for _, l := range lines {
			if isNameLike(l) && strings.Contains(l, "KUMAR") { // heuristic improvement
				father = l
				break
			}
		}
	}

	return name, father
}

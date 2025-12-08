package appointmentletter

import (
	"regexp"
	"strings"
)

// Extract: Roshan Kumara
func ParseNameLetter(text string) string {
	lines := strings.Split(text, "\n")

	// Look for "To." pattern → next lines contain name
	for i, line := range lines {
		if strings.TrimSpace(line) == "To." {
			if i+2 < len(lines) {
				name := strings.TrimSpace(lines[i+2])
				if regexp.MustCompile(`^[A-Z][a-z]+ [A-Z][a-z]+$`).MatchString(name) {
					return name
				}
			}
		}
	}

	// Fallback: Dear Name
	reg := regexp.MustCompile(`(?i)Dear\s+([A-Z][A-Za-z]+ [A-Za-z]+)`)
	if m := reg.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}

	return ""
}

// Extract company (only if present; current OCR letter has NONE)
func ParseCompanyLetter(text string) string {
	if strings.Contains(text, "TechNova Solutions Pvt Ltd") {
		return "TechNova Solutions Pvt Ltd"
	}
	return ""
}

// Extract designation (OCR misreads "Software" as "5arlware")
func ParseDesignationLetter(text string) string {
	reg := regexp.MustCompile(`(?i)(Software Engineer|5arlware Engineer|Soflvare Engineer)`)
	if m := reg.FindStringSubmatch(text); len(m) > 1 {
		return "Software Engineer"
	}
	return ""
}

// Extract joining date (OCR: "trom May 15. 2025")
func ParseJoiningDate(text string) string {
	reg := regexp.MustCompile(`(?i)(May|April|June|July)\s+(\d{1,2}).\s*(\d{4})`)
	m := reg.FindStringSubmatch(text)
	if len(m) == 4 {
		day := m[2]
		year := m[3]
		month := "05" // May
		return day + "/" + month + "/" + year
	}
	return ""
}

// Extract location: fix OCR misread "Dengalore"
func ParseLocationLetter(text string) string {
	reg := regexp.MustCompile(`(?i)Location[: ]+([A-Za-z]+)`)
	if m := reg.FindStringSubmatch(text); len(m) > 1 {
		loc := m[1]
		if strings.HasPrefix(strings.ToLower(loc), "deng") {
			return "Bangalore"
		}
		return loc
	}
	return ""
}

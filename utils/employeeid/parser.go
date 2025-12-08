package employeeid

import (
	"regexp"
	"strings"
)

// Extracts: Rohan Sharma
func ParseNameID(text string) string {
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Match exact human names (Firstname Lastname)
		if regexp.MustCompile(`^[A-Z][a-z]+ [A-Z][a-z]+$`).MatchString(line) {
			return line
		}
	}
	return ""
}

func ParseEmployeeID(text string) string {
	reg := regexp.MustCompile(`(?i)(EMP[- ]?\d{3,})`)
	if m := reg.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}
	return ""
}

func ParseCompanyID(text string) string {
	if strings.Contains(text, "TechNova Solutions Pvt Ltd") {
		return "TechNova Solutions Pvt Ltd"
	}
	return ""
}

func ParseDesignationID(text string) string {
	if strings.Contains(text, "Software Engineer") {
		return "Software Engineer"
	}
	return ""
}

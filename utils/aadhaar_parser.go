package utils

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/Aashish23092/ocr-income-verification/dto"
)

// ParseAadhaarFromText parses Aadhaar data from raw OCR text of a UIDAI letter.
// It is tuned for the layout you provided (UIDAI letter with Aadhaar number,
// VID, disclaimer text, etc.)
func ParseAadhaarFromText(text string) dto.AadhaarExtractResponse {
	lines := normalizeLines(text)

	dob, dobIdx := extractDOBLineBased(lines)
	name := extractNameNearDOB(lines, dobIdx)
	gender := extractGenderNearDOB(lines, dobIdx)
	address := extractAddressBlock(lines)
	aadhaarLast4 := extractAadhaarLast4(text)

	return dto.AadhaarExtractResponse{
		Name:         name,
		DOB:          dob,
		Gender:       gender,
		Address:      address,
		AadhaarLast4: aadhaarLast4,
		Source:       "ocr",
	}
}

// normalizeLines cleans and splits OCR text into lines
func normalizeLines(text string) []string {
	text = strings.ReplaceAll(text, "\r", "")
	// DO NOT collapse '\n' here; we need line structure
	rawLines := strings.Split(text, "\n")

	lines := make([]string, 0, len(rawLines))
	for _, l := range rawLines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

// ---------------- DOB ----------------

func extractDOBLineBased(lines []string) (string, int) {
	// Primary: match "DOB: 23/09/2004"
	reDOB := regexp.MustCompile(`(?i)dob\s*[:\-]?\s*([0-9]{2}[/-][0-9]{2}[/-][0-9]{4})`)

	for i, line := range lines {
		if m := reDOB.FindStringSubmatch(line); len(m) > 1 {
			return m[1], i
		}
	}

	// Fallback: look for any DD/MM/YYYY in all lines
	reDate := regexp.MustCompile(`\b([0-9]{2}[/-][0-9]{2}[/-][0-9]{4})\b`)
	for i, line := range lines {
		if m := reDate.FindStringSubmatch(line); len(m) > 1 {
			return m[1], i
		}
	}

	return "", -1
}

// ---------------- Name ----------------

// extractNameNearDOB takes the line just above the DOB line and cleans it into a name.
func extractNameNearDOB(lines []string, dobIdx int) string {
	if dobIdx <= 0 || dobIdx >= len(lines) {
		return ""
	}

	// Look up to 3 lines above DOB to find a good candidate
	for i := dobIdx - 1; i >= 0 && dobIdx-i <= 3; i-- {
		candidateLine := strings.TrimSpace(lines[i])
		if candidateLine == "" {
			continue
		}
		name := cleanNameFromLine(candidateLine)
		if isLikelyPersonName(name) {
			return name
		}
	}

	// Fallback: scan all lines for a likely person name that is close
	// to DOB line (heuristic)
	if dobIdx > 0 {
		start := max(0, dobIdx-5)
		end := minimize(len(lines), dobIdx+5)
		for i := start; i < end; i++ {
			name := cleanNameFromLine(lines[i])
			if isLikelyPersonName(name) {
				return name
			}
		}
	}

	return ""
}

// cleanNameFromLine strips noise and returns the first 2–3 alphabetic words.
func cleanNameFromLine(line string) string {
	// Keep only letters and spaces
	line = regexp.MustCompile(`[^A-Za-z\s]+`).ReplaceAllString(line, " ")
	line = strings.TrimSpace(line)
	line = regexp.MustCompile(`\s+`).ReplaceAllString(line, " ")
	if line == "" {
		return ""
	}

	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ""
	}

	// Take up to 3 words, usually "Ashish Rawat" or "First Middle Last"
	maxWords := 3
	if len(parts) < maxWords {
		maxWords = len(parts)
	}
	parts = parts[:maxWords]

	// Title-case each word
	for i, p := range parts {
		parts[i] = strings.Title(strings.ToLower(p))
	}
	return strings.Join(parts, " ")
}

// isLikelyPersonName runs a few sanity checks to avoid picking
// "Government of India", "Unique Identification Authority", etc.
func isLikelyPersonName(name string) bool {
	if name == "" {
		return false
	}

	words := strings.Fields(name)
	if len(words) < 2 || len(words) > 4 {
		return false
	}

	// Reject known non-person words
	lower := strings.ToLower(name)
	badTokens := []string{
		"government", "india", "authority", "unique",
		"identification", "aadhaar", "address", "pin", "code",
	}
	for _, t := range badTokens {
		if strings.Contains(lower, t) {
			return false
		}
	}

	// Must have mostly letters
	letterCount := 0
	for _, r := range name {
		if unicode.IsLetter(r) {
			letterCount++
		}
	}
	if letterCount < 4 {
		return false
	}

	// Each word at least 2 chars
	for _, w := range words {
		if len(w) < 2 {
			return false
		}
	}

	return true
}

// ---------------- Gender ----------------

func extractGenderNearDOB(lines []string, dobIdx int) string {
	// Search in a small window around DOB to avoid picking from disclaimer
	start := 0
	if dobIdx > 0 {
		start = max(0, dobIdx-2)
	}
	end := minimize(len(lines), dobIdx+5)

	for i := start; i < end; i++ {
		lower := strings.ToLower(lines[i])
		if strings.Contains(lower, "female") {
			return "Female"
		}
		if strings.Contains(lower, "male") {
			return "Male"
		}
		// Hindi
		if strings.Contains(lower, "महिला") {
			return "Female"
		}
		if strings.Contains(lower, "पुरुष") {
			return "Male"
		}
	}

	// No clear gender
	return ""
}

// ---------------- Aadhaar last 4 ----------------

func extractAadhaarLast4(text string) string {
	// Prefer a 12-digit Aadhaar number (3 groups of 4 digits)
	// e.g., "6260 7951 8316"
	reAadhaar := regexp.MustCompile(`\b(\d{4})\s+(\d{4})\s+(\d{4})\b`)
	if m := reAadhaar.FindStringSubmatch(text); len(m) == 4 {
		return m[3]
	}

	// Fallback: last 4 digits anywhere, but avoid obviously being part of VID
	re4 := regexp.MustCompile(`\b(\d{4})\b`)
	all := re4.FindAllStringSubmatch(text, -1)
	if len(all) == 0 {
		return ""
	}
	// Last occurrence as fallback
	return all[len(all)-1][1]
}

// ---------------- Address ----------------

// extractAddressBlock reads lines starting from the line that contains "Address"
// and collects a few subsequent lines, stopping before disclaimer text.
func extractAddressBlock(lines []string) string {
	startIdx := -1
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), "address") {
			startIdx = i
			break
		}
	}
	if startIdx == -1 {
		// Fallback: start from S/O, C/O line
		for i, line := range lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "s/o") ||
				strings.Contains(lower, "d/o") ||
				strings.Contains(lower, "c/o") ||
				strings.Contains(lower, "w/o") {
				startIdx = i
				break
			}
		}
	}

	if startIdx == -1 {
		return ""
	}

	var addrLines []string

	// First line: text after "Address:"
	addrFirst := lines[startIdx]
	if strings.Contains(strings.ToLower(addrFirst), "address") {
		re := regexp.MustCompile(`(?i)address\s*[:\-]?\s*(.+)`)
		if m := re.FindStringSubmatch(addrFirst); len(m) > 1 {
			cl := cleanAddressLine(m[1])
			if cl != "" {
				addrLines = append(addrLines, cl)
			}
		}
	}

	// Collect next few lines until disclaimer starts
	for i := startIdx + 1; i < len(lines) && len(addrLines) < 6; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)

		// Stop when disclaimer / non-address section starts
		if strings.Contains(lower, "aadhaar is proof") ||
			strings.Contains(lower, "aadhaar is proof of identity") ||
			strings.Contains(lower, "it should be used with verification") ||
			strings.Contains(lower, "authentication") {
			break
		}

		cl := cleanAddressLine(line)
		if cl != "" {
			addrLines = append(addrLines, cl)
		}
	}

	if len(addrLines) == 0 {
		return ""
	}

	// Deduplicate and join
	seen := make(map[string]bool)
	final := make([]string, 0, len(addrLines))
	for _, l := range addrLines {
		if !seen[l] {
			seen[l] = true
			final = append(final, l)
		}
	}

	return strings.Join(final, ", ")
}

// cleanAddressLine trims leading noise and compresses spaces.
// It is intentionally permissive because OCR address lines are noisy.
func cleanAddressLine(line string) string {
	// Remove leading garbage like "7 1] §", ": i = a :]", etc.
	line = regexp.MustCompile(`^[^A-Za-z0-9]+`).ReplaceAllString(line, "")
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	// Collapse multiple spaces and commas
	line = regexp.MustCompile(`\s+`).ReplaceAllString(line, " ")
	line = regexp.MustCompile(`\s*,\s*`).ReplaceAllString(line, ", ")

	// Filter out lines that clearly look like generic info
	lower := strings.ToLower(line)
	genericTokens := []string{
		"aadhaar is proof", "date of birth", "it should be used",
		"authentication", "online", "offline xml", "unique and secure",
	}
	for _, t := range genericTokens {
		if strings.Contains(lower, t) {
			return ""
		}
	}

	// Require at least some letters or digits
	letterOrDigit := 0
	for _, r := range line {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			letterOrDigit++
		}
	}
	if letterOrDigit < 4 {
		return ""
	}

	return line
}

// ---------------- small helpers ----------------

func minimize(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

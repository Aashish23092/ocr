package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Aashish23092/ocr-income-verification/dto"
)

// ParseSalarySlip extracts structured data from salary slip OCR text
func ParseSalarySlip(ocrText string) dto.SalarySlipData {
	return dto.SalarySlipData{
		PayMonth:      extractMonth(ocrText),
		NetSalary:     extractSalaryAmount(ocrText),
		AccountNumber: extractAccountNumber(ocrText),
		EmployeeName:  extractEmployeeName(ocrText),
	}
}

// ParseBankStatement extracts structured data from bank statement OCR text
func ParseBankStatement(ocrText string) dto.BankStatementData {
	return dto.BankStatementData{
		AccountNumber:     extractAccountNumber(ocrText),
		AccountHolderName: extractAccountHolderName(ocrText),
		Transactions:      extractSalaryCredits(ocrText),
	}
}

// extractMonth extracts month information from text
func extractMonth(text string) string {
	months := []string{
		"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
		"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	}

	textLower := strings.ToLower(text)
	for _, month := range months {
		if strings.Contains(textLower, strings.ToLower(month)) {
			// Try to extract year as well
			yearRegex := regexp.MustCompile(`(?i)` + month + `[\s\-,]*(\d{4})`)
			if matches := yearRegex.FindStringSubmatch(text); len(matches) > 1 {
				return month + " " + matches[1]
			}
			return month
		}
	}

	// Try date format MM/YYYY or MM-YYYY
	dateRegex := regexp.MustCompile(`(\d{1,2})[/-](\d{4})`)
	if matches := dateRegex.FindStringSubmatch(text); len(matches) > 2 {
		return matches[1] + "/" + matches[2]
	}

	return "Unknown"
}

// extractSalaryAmount extracts salary amount from text
func extractSalaryAmount(text string) float64 {
	// Patterns for salary amounts
	patterns := []string{
		`(?i)net\s*(?:pay|salary|amount|payment)[\s:]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)total\s*(?:pay|salary|amount)[\s:]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)salary[\s:]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)gross\s*(?:pay|salary)[\s:]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			amountStr := strings.ReplaceAll(matches[1], ",", "")
			if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
				return amount
			}
		}
	}

	return 0.0
}

// extractAccountNumber extracts bank account number from text
func extractAccountNumber(text string) string {
	cleaned := strings.ReplaceAll(text, "—", "-")
	cleaned = strings.ReplaceAll(cleaned, ":", " ")
	cleaned = strings.ReplaceAll(cleaned, "|", " ")
	cleaned = strings.ToLower(cleaned)

	// 1️⃣ PRIORITY: Explicit account number labels (HIGHEST accuracy)
	explicitPatterns := []string{
		`account\s*no[\s\-]*([0-9]{9,18})`,
		`accountnumber[\s\-]*([0-9]{9,18})`,
		`a/c\s*no[\s\-]*([0-9]{9,18})`,
		`ac\s*no[\s\-]*([0-9]{9,18})`,
		`acc\s*no[\s\-]*([0-9]{9,18})`,
	}

	for _, p := range explicitPatterns {
		re := regexp.MustCompile(p)
		if matches := re.FindStringSubmatch(cleaned); len(matches) > 1 {
			return matches[1]
		}
	}

	// 2️⃣ Masked formats like XXXXXXXX6323
	masked := regexp.MustCompile(`x{4,}[0-9]{3,6}`)
	if m := masked.FindString(cleaned); m != "" {
		// Extract last digits only
		digits := regexp.MustCompile(`[0-9]+`).FindString(m)
		if len(digits) >= 4 {
			return digits
		}
	}

	// 3️⃣ Fallback: 9–18 digit numbers, but EXCLUDE cust id, cif, etc
	fallback := regexp.MustCompile(`([0-9]{9,18})`)
	candidates := fallback.FindAllString(cleaned, -1)

	for _, c := range candidates {
		// Reject typical non-account number fields
		if strings.Contains(cleaned, "cust id "+c) ||
			strings.Contains(cleaned, "customer id "+c) ||
			strings.Contains(cleaned, "cif "+c) ||
			strings.Contains(cleaned, "custid "+c) {
			continue
		}

		// Bank accounts are usually ≥ 10 digits, but not 8 digits
		if len(c) >= 10 {
			return c
		}
	}

	return ""
}

// extractEmployeeName extracts employee name from salary slip
func extractEmployeeName(text string) string {
	lines := strings.Split(text, "\n")

	for i, line := range lines {
		lower := strings.ToLower(line)

		if strings.Contains(lower, "name") && strings.Contains(line, ":") {

			// 1) Try previous line first:
			if i > 0 {
				candidate := strings.TrimSpace(lines[i-1])
				candidate = cleanName(candidate)
				if isCleanName(candidate) {
					return candidate
				}
			}

			// 2) Fallback: extract name after "Name:"
			name := extractNameAfterLabel(line)
			name = cleanName(name)
			if isCleanName(name) {
				return name
			}
		}
	}

	return ""
}

func cleanName(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Fields(s)

	stopWords := map[string]bool{
		"opening": true,
		"state":   true,
		"branch":  true,
		"bank":    true,
		"acc":     true,
		"account": true,
		"salary":  true,
		"amount":  true,
		"credit":  true,
		"no":      true,
	}

	clean := []string{}
	for _, p := range parts {
		l := strings.ToLower(p)
		if stopWords[l] {
			break // stop reading further noise
		}
		clean = append(clean, p)
		if len(clean) == 2 { // cap at 2 words: First + Last
			break
		}
	}

	return strings.Join(clean, " ")
}

func isCleanName(s string) bool {
	if s == "" {
		return false
	}
	parts := strings.Fields(s)
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if !regexp.MustCompile(`^[A-Za-z]+$`).MatchString(p) {
			return false
		}
	}
	return true
}

func extractNameAfterLabel(line string) string {
	re := regexp.MustCompile(`(?i)name\s*:\s*([A-Za-z ]+)`)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// extractAccountHolderName extracts account holder name from bank statement
func extractAccountHolderName(text string) string {
	// 1. Try label-based patterns first (your original logic)
	patterns := []string{
		`(?i)account\s*holder[\s:]*([A-Z][a-zA-Z\s\.]+)`,
		`(?i)customer\s*name[\s:]*([A-Z][a-zA-Z\s\.]+)`,
		`(?i)name[\s:]*([A-Z][a-zA-Z\s\.]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			name := cleanName(matches[1])
			if validName(name) {
				return name
			}
		}
	}

	// 2. Fallback: Detect standalone prefix-based names (HDFC, SBI, ICICI)
	prefixRegex := regexp.MustCompile(`(?m)(?i)\b(MR|MRS|MS|SHRI|SMT)\.?\s+[A-Z][A-Z\s]{2,50}`)
	match := prefixRegex.FindString(text)
	if match != "" {
		// remove prefix and clean
		parts := strings.Fields(match)
		if len(parts) >= 2 {
			name := strings.Join(parts[1:], " ")
			name = cleanName(name)
			if validName(name) {
				return name
			}
		}
	}

	return ""
}

// helpers

func validName(n string) bool {
	return len(n) > 2 && len(n) < 50
}

// extractSalaryCredits extracts salary credit transactions from bank statement
func extractSalaryCredits(text string) []dto.BankTransaction {
	var credits []dto.BankTransaction

	// Pattern for transaction lines with date, description, and amount
	// Example: "15/01/2024 SALARY CREDIT 50000.00"
	pattern := `(\d{1,2}[/-]\d{1,2}[/-]\d{2,4})\s+([A-Z\s]+(?:SALARY|SAL|CREDIT)[\w\s]*)\s+([0-9,]+\.?\d*)`
	re := regexp.MustCompile(pattern)

	matches := re.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) > 3 {
			amountStr := strings.ReplaceAll(match[3], ",", "")
			if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
				date, _ := parseDate(match[1])
				credits = append(credits, dto.BankTransaction{
					Date:        date,
					Amount:      amount,
					Description: strings.TrimSpace(match[2]),
					IsCredit:    true,
				})
			}
		}
	}

	// Alternative pattern for credit entries
	if len(credits) == 0 {
		pattern2 := `(?i)(salary|sal)\s+(?:credit|cr)\s+([0-9,]+\.?\d*)`
		re2 := regexp.MustCompile(pattern2)
		matches2 := re2.FindAllStringSubmatch(text, -1)
		for _, match := range matches2 {
			if len(match) > 2 {
				amountStr := strings.ReplaceAll(match[2], ",", "")
				if amount, err := strconv.ParseFloat(amountStr, 64); err == nil {
					credits = append(credits, dto.BankTransaction{
						Amount:      amount,
						Description: "Salary Credit",
						IsCredit:    true,
					})
				}
			}
		}
	}

	return credits
}

func parseDate(dateStr string) (time.Time, error) {
	formats := []string{"02/01/2006", "02-01-2006", "2006-01-02"}
	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown date format")
}

// NormalizeString normalizes string for comparison (lowercase, remove spaces)
func NormalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ".", "")
	return s
}

// CompareNames compares two names for matching
func CompareNames(name1, name2 string) bool {
	if name1 == "" || name2 == "" {
		return false
	}

	norm1 := NormalizeString(name1)
	norm2 := NormalizeString(name2)

	// Exact match
	if norm1 == norm2 {
		return true
	}

	// Partial match (one contains the other)
	if strings.Contains(norm1, norm2) || strings.Contains(norm2, norm1) {
		return true
	}

	// Check if all words in shorter name appear in longer name
	words1 := strings.Fields(strings.ToLower(name1))
	words2 := strings.Fields(strings.ToLower(name2))

	if len(words1) > len(words2) {
		words1, words2 = words2, words1
	}

	matchCount := 0
	for _, w1 := range words1 {
		for _, w2 := range words2 {
			if strings.Contains(w2, w1) || strings.Contains(w1, w2) {
				matchCount++
				break
			}
		}
	}

	// If at least 50% of words match
	return float64(matchCount)/float64(len(words1)) >= 0.5
}

// CalculateNameSimilarity calculates the similarity between two names using Levenshtein distance
// Returns a score between 0.0 and 1.0
func CalculateNameSimilarity(name1, name2 string) float64 {
	s1 := NormalizeString(name1)
	s2 := NormalizeString(name2)

	if s1 == "" && s2 == "" {
		return 1.0
	}
	if s1 == "" || s2 == "" {
		return 0.0
	}

	dist := levenshteinDistance(s1, s2)
	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - float64(dist)/float64(maxLen)
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(s1, s2 string) int {
	r1 := []rune(s1)
	r2 := []rune(s2)
	n, m := len(r1), len(r2)

	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}

	matrix := make([][]int, n+1)
	for i := range matrix {
		matrix[i] = make([]int, m+1)
	}

	for i := 0; i <= n; i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= m; j++ {
		matrix[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[n][m]
}

func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

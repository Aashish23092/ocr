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

// ParseITR extracts structured data from ITR (Income Tax Return) OCR text
// =========================
//  ITR PARSER (ITR-V / ITR-1 / ITR-3 / ITR-4)
// =========================

// -----------------------
// Parse ITR (ITR-V, ITR-1, ITR-3, ITR-4)
// -----------------------
func ParseITR(ocrText string) dto.ITRResult {
	lines := splitAndTrimLines(ocrText)

	res := dto.ITRResult{
		RawText: ocrText,
	}

	// -----------------------
	// 1. PAN (OCR missing in sample)
	// -----------------------
	res.PAN = extractPAN(ocrText)

	// -----------------------
	// 2. Assessment Year
	// -----------------------
	res.AssessmentYear = extractAssessmentYearFromLines(lines)
	if res.AssessmentYear == "" {
		res.AssessmentYear = extractAssessmentYear(ocrText)
	}

	// -----------------------
	// 3. NAME (fix: ignore section headers)
	// -----------------------
	res.Name = extractNameSmart(lines)

	// -----------------------
	// 4. TOTAL INCOME
	// -----------------------
	if v := extractNumberUnderLabelSmart(lines, "Total Income"); v > 0 {
		res.TotalIncome = v
	} else {
		res.TotalIncome = extractTotalIncome(ocrText)
	}

	// -----------------------
	// 5. TAX PAID
	// -----------------------
	if v := extractNumberUnderLabelSmart(lines, "Taxes Paid"); v > 0 {
		res.TaxPaid = v
	} else {
		res.TaxPaid = extractTaxPaid(ocrText)
	}

	// -----------------------
	// 6. REFUND AMOUNT (fix row label issue)
	// -----------------------
	res.RefundAmount = extractRefundSmart(lines)

	// -----------------------
	// 7. FILING DATE
	// -----------------------
	res.FilingDate = extractITRFilingDate(lines)

	return res
}

// ----- ITR helpers -----

func splitAndTrimLines(text string) []string {
	raw := strings.Split(text, "\n")
	out := make([]string, 0, len(raw))
	for _, l := range raw {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func cleanLabel(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, ":", ""))
}

// PAN format ABCDE1234F
func extractPAN(text string) string {
	panPattern := regexp.MustCompile(`\b([A-Z]{5}[0-9]{4}[A-Z])\b`)
	if matches := panPattern.FindStringSubmatch(text); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func extractAssessmentYearFromLines(lines []string) string {
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), "assessment year") {
			for j := 1; j <= 3 && i+j < len(lines); j++ {
				cand := cleanLabel(lines[i+j])
				if regexp.MustCompile(`^\d{4}-\d{2,4}$`).MatchString(cand) {
					return cand
				}
			}
		}
	}
	return ""
}

func extractITRNameFromLines(lines []string) string {
	for i, line := range lines {
		if strings.EqualFold(cleanLabel(line), "Name") {
			for j := 1; j <= 3 && i+j < len(lines); j++ {
				cand := cleanLabel(lines[i+j])
				if cand == "" {
					continue
				}
				lower := strings.ToLower(cand)
				if lower == "address" || lower == "status" ||
					strings.Contains(lower, "individual") ||
					strings.Contains(lower, "huf") ||
					strings.Contains(lower, "company") {
					continue
				}
				if regexp.MustCompile(`^[A-Za-z]`).MatchString(cand) {
					return cand
				}
			}
		}
	}
	return ""
}

// Generic regex-based ITR name extractor (for other layouts)
func extractITRName(text string) string {
	patterns := []string{
		`(?i)name\s*of\s*(?:the\s*)?(?:assessee|taxpayer)[:\s]*([A-Z][a-zA-Z\s\.]{2,50})`,
		`(?i)assessee\s*name[:\s]*([A-Z][a-zA-Z\s\.]{2,50})`,
		`(?i)taxpayer\s*name[:\s]*([A-Z][a-zA-Z\s\.]{2,50})`,
		`(?i)name[:\s]*([A-Z][a-zA-Z\s\.]{2,50})`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			name := strings.TrimSpace(matches[1])
			name = regexp.MustCompile(`[^a-zA-Z\s]+$`).ReplaceAllString(name, "")
			name = strings.TrimSpace(name)
			if len(name) > 2 && len(name) < 50 {
				return name
			}
		}
	}
	return ""
}

func extractAssessmentYear(text string) string {
	patterns := []string{
		`(?i)assessment\s*year[:\s]*(\d{4}[-]\d{2,4})`,
		`(?i)A\.?Y\.?[:\s]*(\d{4}[-]\d{2,4})`,
		`\b(\d{4}[-]\d{2})\b`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// find number in rows like:
//
//	Total Income
//	160850
//
// or
//
//	Taxes Paid
//	7
//	9500
func extractNumberUnderLabel(lines []string, label string) float64 {
	for i, line := range lines {
		if cleanLabel(line) == label {
			for j := 1; j <= 4 && i+j < len(lines); j++ {
				cand := cleanLabel(lines[i+j])
				if len(cand) <= 1 {
					continue // skip row codes like "1", "7", "8"
				}
				cand = strings.ReplaceAll(cand, ",", "")
				if f, err := strconv.ParseFloat(cand, 64); err == nil {
					return f
				}
			}
		}
	}
	return 0
}

// === numeric extractors shared between ITR layouts ===

func extractAmount(text string, patterns []string) float64 {
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

func extractTotalIncome(text string) float64 {
	patterns := []string{
		`(?i)total\s*income[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)gross\s*total\s*income[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)income\s*under\s*all\s*heads[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
	}
	return extractAmount(text, patterns)
}

func extractTaxableIncome(text string) float64 {
	patterns := []string{
		`(?i)taxable\s*income[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)total\s*taxable\s*income[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)net\s*taxable\s*income[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
	}
	return extractAmount(text, patterns)
}

func extractTaxPaid(text string) float64 {
	patterns := []string{
		`(?i)tax\s*paid[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)total\s*tax\s*paid[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)taxes\s*paid[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)tax\s*liability[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
	}
	return extractAmount(text, patterns)
}

func extractRefundFromLines(lines []string, taxPaid float64) float64 {
	// ITR-V / Ack layout example:
	// "+)Tax Payable/-)Refundable6-7"
	// 8
	// -9500
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "refundable") || strings.Contains(lower, "refund") {
			for j := 1; j <= 3 && i+j < len(lines); j++ {
				cand := cleanLabel(lines[i+j])
				cand = strings.ReplaceAll(cand, ",", "")
				if f, err := strconv.ParseFloat(cand, 64); err == nil {
					if f < 0 {
						return -f
					}
					return f
				}
			}
		}
	}

	// Fallback: nothing explicit, just return 0 for safety
	_ = taxPaid
	return 0
}

func extractITRFilingDate(lines []string) string {
	// We accept dates like 21-08-2020, 21/08/2020
	dateRegex := regexp.MustCompile(`(\d{2})[-/](\d{2})[-/](\d{4})`)

	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), "electronically") ||
			strings.Contains(strings.ToLower(line), "submitted") ||
			strings.Contains(strings.ToLower(line), "on") ||
			strings.Contains(strings.ToLower(line), "acknowledgement") {

			if m := dateRegex.FindStringSubmatch(line); len(m) == 4 {
				raw := m[0]
				if t, err := time.Parse("02-01-2006", m[1]+"-"+m[2]+"-"+m[3]); err == nil {
					return t.Format("2006-01-02")
				}
				if t, err := time.Parse("02/01/2006", m[1]+"/"+m[2]+"/"+m[3]); err == nil {
					return t.Format("2006-01-02")
				}
				return raw
			}
		}
	}

	// Last fallback: any date anywhere
	for _, line := range lines {
		if m := dateRegex.FindStringSubmatch(line); len(m) == 4 {
			raw := m[0]
			if t, err := time.Parse("02-01-2006", m[1]+"-"+m[2]+"-"+m[3]); err == nil {
				return t.Format("2006-01-02")
			}
			if t, err := time.Parse("02/01/2006", m[1]+"/"+m[2]+"/"+m[3]); err == nil {
				return t.Format("2006-01-02")
			}
			return raw
		}
	}

	return ""
}
func extractNameSmart(lines []string) string {
	sectionWords := map[string]bool{
		"address": true, "status": true, "individual": true,
		"form number": true, "form": true, "itr": true,
	}

	for i, line := range lines {
		if strings.EqualFold(cleanLabel(line), "Name") {

			// check next 3 lines
			for j := 1; j <= 3 && i+j < len(lines); j++ {
				cand := cleanLabel(lines[i+j])
				l := strings.ToLower(cand)

				// ❌ Reject section headers
				if sectionWords[l] || len(cand) <= 2 {
					continue
				}

				// valid name begins with alphabet
				if regexp.MustCompile(`^[A-Za-z]`).MatchString(cand) {
					return cand
				}
			}

			// If everything looks like section headers → name not found
			return ""
		}
	}
	return ""
}

func extractRefundSmart(lines []string) float64 {
	for i, line := range lines {
		l := strings.ToLower(line)

		if strings.Contains(l, "refundable") || strings.Contains(l, "tax payable") {

			// scan next 4 lines
			for j := 1; j <= 4 && i+j < len(lines); j++ {
				cand := cleanLabel(lines[i+j])
				cand = strings.ReplaceAll(cand, ",", "")

				// Skip row index numbers like "1", "7", "8", "19"
				if len(cand) <= 2 {
					continue
				}

				// look for negative or large number
				if f, err := strconv.ParseFloat(cand, 64); err == nil {
					if f < 0 {
						return -f
					}
					if f > 1000 { // refund usually > 1000
						return f
					}
				}
			}
		}
	}
	return 0
}

// extractNumericValue extracts int/float even if stuck to stray characters.
// Returns -999999 if not a valid number.
func extractNumericValue(s string) float64 {
	// keep digits, minus, dot only
	re := regexp.MustCompile(`-?[0-9]+\.?[0-9]*`)
	match := re.FindString(s)
	if match == "" {
		return -999999
	}

	v, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return -999999
	}
	return v
}

// extractNumberUnderLabelSmart finds the numeric value under a label like "Total Income", "Taxes Paid", etc.
// It scans the next 3–5 lines and intelligently ignores row numbers like 1, 2, 7, 8.
func extractNumberUnderLabelSmart(lines []string, label string) float64 {
	clean := func(s string) string {
		s = strings.TrimSpace(strings.ReplaceAll(s, ":", ""))
		s = strings.ReplaceAll(s, "—", "-")
		s = strings.ReplaceAll(s, " ", "")
		return s
	}

	lowerLabel := strings.ToLower(label)

	for i, line := range lines {
		if strings.ToLower(strings.TrimSpace(line)) == lowerLabel {

			// Look ahead safely
			for j := 1; j <= 5 && i+j < len(lines); j++ {
				look := clean(lines[i+j])
				if look == "" {
					continue
				}

				// skip row indices like "1", "2", "8", "19"
				if regexp.MustCompile(`^[0-9]{1,2}$`).MatchString(look) {
					continue
				}

				// attempt numeric extraction
				v := extractNumericValue(look)
				if v != -999999 {
					return v
				}
			}
		}
	}

	return 0
}

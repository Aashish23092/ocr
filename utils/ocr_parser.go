package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Aashish23092/ocr-income-verification/dto"
)

// =============================
// SALARY SLIP PARSER
// =============================

func ParseSalarySlip(ocrText string) dto.SalarySlipData {
	return dto.SalarySlipData{
		PayMonth:      extractMonth(ocrText),
		NetSalary:     extractSalaryAmount(ocrText),
		AccountNumber: extractAccountNumber(ocrText),
		EmployeeName:  extractEmployeeName(ocrText),
		EmployerName:  extractEmployerName(ocrText),
	}
}

// extractEmployerName attempts to detect the company name from salary slips.
// Strategy:
// 1) First line(s) of salary slips almost always contain company name
// 2) Look for words like "Private Limited", "Pvt", "Ltd", "LLP", etc.
// 3) If detected, return that line as employer name
func extractEmployerName(text string) string {
	lines := strings.Split(text, "\n")

	// scan first 5 meaningful lines
	for i := 0; i < len(lines) && i < 6; i++ {
		l := strings.TrimSpace(lines[i])
		if l == "" {
			continue
		}

		upper := strings.ToUpper(l)

		// corporate suffix indicators
		if strings.Contains(upper, "PVT") ||
			strings.Contains(upper, "PRIVATE") ||
			strings.Contains(upper, "LTD") ||
			strings.Contains(upper, "LIMITED") ||
			strings.Contains(upper, "LLP") ||
			strings.Contains(upper, "TECHNOLOGY") ||
			strings.Contains(upper, "TECH") ||
			strings.Contains(upper, "SOLUTIONS") {

			// clean trailing punctuation
			l = strings.Trim(l, "-:•* ")
			return l
		}
	}

	return ""
}

func extractMonth(text string) string {
	months := []string{
		"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
		"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
	}

	textLower := strings.ToLower(text)
	for _, month := range months {
		if strings.Contains(textLower, strings.ToLower(month)) {
			yearRegex := regexp.MustCompile(`(?i)` + month + `[\s\-,]*(\d{4})`)
			if matches := yearRegex.FindStringSubmatch(text); len(matches) > 1 {
				return month + " " + matches[1]
			}
			return month
		}
	}

	dateRegex := regexp.MustCompile(`(\d{1,2})[/-](\d{4})`)
	if matches := dateRegex.FindStringSubmatch(text); len(matches) > 2 {
		return matches[1] + "/" + matches[2]
	}
	return "Unknown"
}

func extractSalaryAmount(text string) float64 {
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

// =============================
// ACCOUNT NUMBER & NAME HELPERS
// =============================

func extractAccountNumber(text string) string {
	cleaned := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(text, "—", "-"), ":", " "))

	explicit := []string{
		`account\s*no[\s\-]*([0-9]{9,18})`,
		`accountnumber[\s\-]*([0-9]{9,18})`,
		`a/c\s*no[\s\-]*([0-9]{9,18})`,
		`ac\s*no[\s\-]*([0-9]{9,18})`,
		`acc\s*no[\s\-]*([0-9]{9,18})`,
	}
	for _, p := range explicit {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(cleaned); len(m) > 1 {
			return m[1]
		}
	}

	masked := regexp.MustCompile(`x{4,}[0-9]{3,6}`)
	if m := masked.FindString(cleaned); m != "" {
		return regexp.MustCompile(`[0-9]+`).FindString(m)
	}

	fallback := regexp.MustCompile(`([0-9]{9,18})`)
	cands := fallback.FindAllString(cleaned, -1)
	for _, c := range cands {
		if len(c) >= 10 &&
			!strings.Contains(cleaned, "cust id "+c) &&
			!strings.Contains(cleaned, "customer id "+c) &&
			!strings.Contains(cleaned, "cif "+c) {
			return c
		}
	}
	return ""
}

// Employee name extraction (unchanged)

func extractEmployeeName(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "name") && strings.Contains(line, ":") {
			if i > 0 {
				candidate := cleanName(strings.TrimSpace(lines[i-1]))
				if isCleanName(candidate) {
					return candidate
				}
			}
			name := cleanName(extractNameAfterLabel(line))
			if isCleanName(name) {
				return name
			}
		}
	}
	return ""
}

func extractNameAfterLabel(line string) string {
	re := regexp.MustCompile(`(?i)name\s*:\s*([A-Za-z ]+)`)
	m := re.FindStringSubmatch(line)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func cleanName(s string) string {
	if s == "" {
		return s
	}

	stop := map[string]bool{
		"opening": true, "state": true, "branch": true, "bank": true,
		"acc": true, "account": true, "salary": true,
	}

	parts := strings.Fields(s)
	out := []string{}
	for _, p := range parts {
		if stop[strings.ToLower(p)] {
			break
		}
		out = append(out, p)
		if len(out) == 2 {
			break
		}
	}
	return strings.Join(out, " ")
}

func isCleanName(s string) bool {
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

// Account Holder Name (unchanged)

func extractAccountHolderName(text string) string {
	patterns := []string{
		`(?i)account\s*holder[\s:]*([A-Z][A-Za-z\s\.]+)`,
		`(?i)customer\s*name[\s:]*([A-Z][A-Za-z\s\.]+)`,
		`(?i)name[\s:]*([A-Z][A-Za-z\s\.]+)`,
	}

	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(text); len(m) > 1 {
			n := cleanName(m[1])
			if validName(n) {
				return n
			}
		}
	}

	// MR AASHISH RAWAT
	prefix := regexp.MustCompile(`(?m)(?i)\b(MR|MRS|MS|SHRI|SMT)\.?\s+[A-Z][A-Z\s]{2,50}`)
	if m := prefix.FindString(text); m != "" {
		parts := strings.Fields(m)
		if len(parts) >= 2 {
			n := cleanName(strings.Join(parts[1:], " "))
			if validName(n) {
				return n
			}
		}
	}

	return ""
}

func validName(n string) bool { return len(n) > 2 && len(n) < 50 }

// =============================================
// 🚀 NEW — FULL BANK STATEMENT PARSER
// =============================================

func ParseBankStatement(text string) dto.BankStatementData {
	clean := normalizeLines(text)

	return dto.BankStatementData{
		AccountNumber:     extractAccountNumber(text),
		AccountHolderName: extractAccountHolderName(text),
		Transactions:      parseBankTransactions(clean),
	}
}

// Main transaction dispatcher
func parseBankTransactions(lines []string) []dto.BankTransaction {
	tx := parseTabularTransactions(lines)
	if len(tx) > 0 {
		return tx
	}
	return parseLooseTransactions(lines)
}

// ----------------------
// 1. TABULAR FORMAT PARSER
// ----------------------
func parseTabularTransactions(lines []string) []dto.BankTransaction {
	dateRe := regexp.MustCompile(`^\s*(\d{1,2}[/-]\d{1,2}[/-]\d{2,4})`)
	var tx []dto.BankTransaction

	for _, line := range lines {
		if !dateRe.MatchString(line) {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		dateStr := parts[0]
		amountStr := parts[len(parts)-1]
		amount := mustParseAmount(amountStr)
		if amount == 0 {
			continue
		}

		desc := strings.Join(parts[1:len(parts)-1], " ")
		date, _ := parseDateSmart(dateStr)

		up := strings.ToUpper(desc + " " + amountStr)
		isCredit := strings.Contains(up, "CR") ||
			strings.Contains(up, "CREDIT") ||
			strings.Contains(up, "NEFT") ||
			strings.Contains(up, "UPI") ||
			strings.Contains(up, "SALARY")

		tx = append(tx, dto.BankTransaction{
			Date:        date,
			Amount:      amount,
			Description: desc,
			IsCredit:    isCredit,
		})
	}
	return tx
}

// ----------------------
// 2. LOOSE FORMAT PARSER
// ----------------------

func parseLooseTransactions(lines []string) []dto.BankTransaction {
	dateRe := regexp.MustCompile(`\d{1,2}[/-]\d{1,2}[/-]\d{2,4}`)
	amountRe := regexp.MustCompile(`[0-9,]+\.\d{2}`)

	var tx []dto.BankTransaction

	for _, line := range lines {
		d := dateRe.FindString(line)
		if d == "" {
			continue
		}
		amounts := amountRe.FindAllString(line, -1)
		if len(amounts) == 0 {
			continue
		}

		amount := mustParseAmount(amounts[len(amounts)-1])
		if amount == 0 {
			continue
		}

		desc := strings.TrimSpace(strings.Replace(line, amounts[len(amounts)-1], "", 1))
		date, _ := parseDateSmart(d)

		up := strings.ToUpper(desc)
		isCredit := strings.Contains(up, "CR") ||
			strings.Contains(up, "CREDIT") ||
			strings.Contains(up, "SAL") ||
			strings.Contains(up, "NEFT")

		tx = append(tx, dto.BankTransaction{
			Date:        date,
			Amount:      amount,
			Description: desc,
			IsCredit:    isCredit,
		})
	}
	return tx
}

// ----------------------
// DATE & AMOUNT HELPERS
// ----------------------

func parseDateSmart(s string) (time.Time, error) {
	formats := []string{
		"02/01/2006", "02/01/06",
		"02-01-2006", "02-01-06",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid date: %s", s)
}

func mustParseAmount(s string) float64 {
	s = strings.ToUpper(strings.ReplaceAll(s, ",", ""))
	s = strings.TrimSuffix(s, "CR")
	s = strings.TrimSuffix(s, "DR")
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

// =============================
// NAME COMPARISON HELPERS
// =============================

func NormalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, ".", "")
	return s
}

func CompareNames(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a2 := NormalizeString(a)
	b2 := NormalizeString(b)
	if a2 == b2 {
		return true
	}
	if strings.Contains(a2, b2) || strings.Contains(b2, a2) {
		return true
	}

	wa := strings.Fields(strings.ToLower(a))
	wb := strings.Fields(strings.ToLower(b))
	if len(wa) > len(wb) {
		wa, wb = wb, wa
	}

	match := 0
	for _, x := range wa {
		for _, y := range wb {
			if strings.Contains(y, x) || strings.Contains(x, y) {
				match++
				break
			}
		}
	}

	return float64(match)/float64(len(wa)) >= 0.5
}

func CalculateNameSimilarity(a, b string) float64 {
	a2 := NormalizeString(a)
	b2 := NormalizeString(b)

	if a2 == "" && b2 == "" {
		return 1.0
	}
	if a2 == "" || b2 == "" {
		return 0.0
	}

	dist := levenshteinDistance(a2, b2)
	maxLen := len(a2)
	if len(b2) > maxLen {
		maxLen = len(b2)
	}
	return 1 - float64(dist)/float64(maxLen)
}

func levenshteinDistance(a, b string) int {
	ra := []rune(a)
	rb := []rune(b)
	n, m := len(ra), len(rb)

	if n == 0 {
		return m
	}
	if m == 0 {
		return n
	}

	mat := make([][]int, n+1)
	for i := range mat {
		mat[i] = make([]int, m+1)
	}

	for i := 0; i <= n; i++ {
		mat[i][0] = i
	}
	for j := 0; j <= m; j++ {
		mat[0][j] = j
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			cost := 0
			if ra[i-1] != rb[j-1] {
				cost = 1
			}
			mat[i][j] = min(
				mat[i-1][j]+1,
				mat[i][j-1]+1,
				mat[i-1][j-1]+cost,
			)
		}
	}

	return mat[n][m]
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

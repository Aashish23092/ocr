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
		RawText:       ocrText,
	}
}

// ParseBankStatement extracts structured data from bank statement OCR text
func ParseBankStatement(ocrText string) dto.BankStatementData {
	return dto.BankStatementData{
		AccountNumber:     extractAccountNumber(ocrText),
		AccountHolderName: extractAccountHolderName(ocrText),
		Transactions:      extractSalaryCredits(ocrText),
		RawText:           ocrText,
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
		`(?i)net\s*(?:pay|salary|amount)[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)total\s*(?:pay|salary|amount)[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)(?:Rs\.?|INR|₹)\s*([0-9,]+\.?\d*)`,
		`(?i)salary[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
		`(?i)gross\s*(?:pay|salary)[:\s]*(?:Rs\.?|INR|₹)?\s*([0-9,]+\.?\d*)`,
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
	// Pattern for account numbers (typically 9-18 digits)
	patterns := []string{
		`(?i)a/?c(?:\s*no\.?|\s*number)[:\s]*([0-9]{9,18})`,
		`(?i)account\s*(?:no\.?|number)[:\s]*([0-9]{9,18})`,
		`(?i)acc(?:\s*no\.?|\s*number)[:\s]*([0-9]{9,18})`,
		`\b([0-9]{9,18})\b`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}

// extractEmployeeName extracts employee name from salary slip
func extractEmployeeName(text string) string {
	patterns := []string{
		`(?i)employee\s*name[:\s]*([A-Z][a-zA-Z\s\.]+)`,
		`(?i)name[:\s]*([A-Z][a-zA-Z\s\.]+)`,
		`(?i)emp\.\s*name[:\s]*([A-Z][a-zA-Z\s\.]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			name := strings.TrimSpace(matches[1])
			// Clean up the name (remove extra spaces, limit length)
			name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
			if len(name) > 2 && len(name) < 50 {
				return name
			}
		}
	}

	return ""
}

// extractAccountHolderName extracts account holder name from bank statement
func extractAccountHolderName(text string) string {
	patterns := []string{
		`(?i)account\s*holder[:\s]*([A-Z][a-zA-Z\s\.]+)`,
		`(?i)name[:\s]*([A-Z][a-zA-Z\s\.]+)`,
		`(?i)customer\s*name[:\s]*([A-Z][a-zA-Z\s\.]+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(text); len(matches) > 1 {
			name := strings.TrimSpace(matches[1])
			name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
			if len(name) > 2 && len(name) < 50 {
				return name
			}
		}
	}

	return ""
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
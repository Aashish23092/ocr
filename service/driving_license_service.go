package service

import (
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/Aashish23092/ocr-income-verification/client"
)

type DrivingLicenseService struct {
	paddle    *client.PaddleClient
	tesseract *client.TesseractClient
}

func NewDrivingLicenseService(paddle *client.PaddleClient, tesseract *client.TesseractClient) *DrivingLicenseService {
	return &DrivingLicenseService{
		paddle:    paddle,
		tesseract: tesseract,
	}
}

type DLResult struct {
	Name      string `json:"name"`
	DLNumber  string `json:"dl_number"`
	DOB       string `json:"dob"`
	IssueDate string `json:"issue_date"`
	ValidTill string `json:"valid_till"`
	Address   string `json:"address"`
	RawText   string `json:"raw_text"`
}

func (s *DrivingLicenseService) ExtractDLText(imageBytes []byte) (*DLResult, error) {
	var raw string
	var err error

	// -----------------------------
	// 1️⃣ Try PaddleOCR first
	// -----------------------------
	if s.paddle != nil {
		raw, err = s.paddle.ExtractText(imageBytes)
		if err == nil && len(raw) > 10 {
			log.Println("Driving License: PaddleOCR succeeded")
			return s.parseDL(raw), nil
		}
	}

	// -----------------------------
	// 2️⃣ Tesseract fallback
	// -----------------------------
	raw, err = s.tesseract.ExtractTextFromBytes(imageBytes)
	if err != nil {
		return nil, err
	}

	log.Println("Driving License: Fallback Tesseract used")
	return s.parseDL(raw), nil
}

// parseDate tries to parse dd/mm/yyyy into time.Time. Returns zero time on failure.
func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("02/01/2006", s)
	if err != nil {
		// sometimes OCR introduces '.' or '-' instead of '/'
		s2 := strings.ReplaceAll(s, "-", "/")
		s2 = strings.ReplaceAll(s2, ".", "/")
		t, err = time.Parse("02/01/2006", s2)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	}
	return t, true
}

func (s *DrivingLicenseService) parseDL(raw string) *DLResult {
	text := strings.ToUpper(raw)

	// general date regex (DD/MM/YYYY)
	reAnyDate := regexp.MustCompile(`\d{2}[/\-\.]\d{2}[/\-\.]\d{4}`)

	// 1) DL Number: common formats (two letters + two digits + rest)
	reDL := regexp.MustCompile(`\b[A-Z]{2}\s?\d{2}\s?\d{6,12}\b`)
	dlNumber := reDL.FindString(text)

	// 2) All dates in order of appearance
	allDates := reAnyDate.FindAllString(text, -1)

	// Helper to find first date after a marker
	findDateAfter := func(marker string) string {
		reMarker := regexp.MustCompile(marker)
		if idx := reMarker.FindStringIndex(text); idx != nil {
			after := text[idx[1]:]
			dates := reAnyDate.FindAllString(after, -1)
			if len(dates) > 0 {
				return dates[0]
			}
		}
		return ""
	}

	// 3) Issue date: try to find after "DATE OF ISSUE" or fallback to first date
	issueStr := findDateAfter(`DATE\s+OF\s+ISSUE|DATE\s+OF\s+ISSUED|DATE\s+ISSUE`)
	if issueStr == "" && len(allDates) > 0 {
		issueStr = allDates[0]
	}

	// 4) Valid Till: try to find after "VALID" marker or use the next date after issue occurrence
	validStr := findDateAfter(`VALID\s+TO|VALID\s+UPTO|VALID\s+TILL|VALID`)
	if validStr == "" {
		// locate index of issueStr in allDates and try to pick the next one
		if issueStr != "" && len(allDates) > 0 {
			// find position of issueStr in allDates
			pos := -1
			for i, d := range allDates {
				if d == issueStr {
					pos = i
					break
				}
			}
			// if found and next exists, use next; else if multiple dates and pos not found, use second date
			if pos >= 0 && pos+1 < len(allDates) {
				validStr = allDates[pos+1]
			} else if len(allDates) > 1 {
				// choose the second date as candidate
				if allDates[0] == issueStr {
					validStr = allDates[1]
				} else {
					// if issueStr isn't present in allDates (rare), pick second anyway
					validStr = allDates[1]
				}
			}
		} else if len(allDates) > 1 {
			// no identified issue but multiple dates present: heuristically pick second as valid
			validStr = allDates[1]
		}
	}

	// 5) DOB: find the first date AFTER "DATE OF BIRTH" marker (handles intervening tokens)
	dobStr := findDateAfter(`DATE\s+OF\s+BIRTH|DATE\s+BIRTH|DOB`)
	if dobStr == "" {
		// fallback: try to find a date near the token "BIRTH" by scanning lines
		lines := strings.Split(text, "\n")
		for i, ln := range lines {
			if strings.Contains(ln, "BIRTH") || strings.Contains(ln, "DOB") {
				// look next few lines for a date
				for j := i; j < i+4 && j < len(lines); j++ {
					if reAnyDate.MatchString(lines[j]) {
						dobStr = reAnyDate.FindString(lines[j])
						break
					}
				}
				if dobStr != "" {
					break
				}
			}
		}
	}
	// fallback further: if dob still empty, prefer last date if it's not issue/valid
	if dobStr == "" && len(allDates) > 0 {
		candidate := allDates[len(allDates)-1]
		if candidate != issueStr && candidate != validStr {
			dobStr = candidate
		}
	}

	// Parse dates and ensure ordering: issue <= valid
	issueTime, issueOK := parseDate(issueStr)
	validTime, validOK := parseDate(validStr)

	// If both parsed and valid is before issue, swap them
	if issueOK && validOK {
		if validTime.Before(issueTime) {
			// swap strings as well so returned strings match corrected semantics
			issueStr, validStr = validStr, issueStr
			issueTime, validTime = validTime, issueTime
		}
	} else if !issueOK && validOK {
		// If only valid parsed, and other dates exist, try to find the most-likely issue (earlier date)
		if len(allDates) > 0 {
			// find any date earlier than validTime
			for _, d := range allDates {
				if dt, ok := parseDate(d); ok {
					if dt.Before(validTime) {
						issueStr = d
						break
					}
				}
			}
		}
	} else if issueOK && !validOK {
		// If only issue parsed, try to choose a later date from allDates as valid
		if len(allDates) > 0 {
			for i := len(allDates) - 1; i >= 0; i-- {
				d := allDates[i]
				if dt, ok := parseDate(d); ok && dt.After(issueTime) {
					validStr = d
					break
				}
			}
		}
	}

	// 6) Name: try common markers "/NAME", "NAME", "DRIVER" contexts
	name := ""
	reName1 := regexp.MustCompile(`/?NAME[:\s]*([A-Z\s]{2,})`)
	if m := reName1.FindStringSubmatch(text); len(m) > 1 {
		name = strings.TrimSpace(m[1])
	} else {
		// fallback: try "NAME" marker lines
		lines := strings.Split(text, "\n")
		for i, ln := range lines {
			if strings.Contains(ln, "NAME") && i+1 < len(lines) {
				candidate := strings.TrimSpace(lines[i+1])
				// ignore short tokens like "A+" etc.
				if reAnyDate.MatchString(candidate) == false && len(candidate) > 1 && !strings.Contains(candidate, "BLOOD") {
					name = strings.TrimSpace(candidate)
					break
				}
			}
		}
	}

	// 7) Address: (left empty unless we detect 'ADDRESS' marker or long block after 'S/O' or 'SON' etc.)
	address := ""
	reAddr := regexp.MustCompile(`ADDRESS[:\s]+([A-Z0-9,\s\-\/]+)`)
	if m := reAddr.FindStringSubmatch(text); len(m) > 1 {
		address = strings.TrimSpace(m[1])
	} else {
		reSOW := regexp.MustCompile(`SON\/DAUGHTER\/WIFE\s+OF[\s:]*([A-Z0-9\s,.-\/]+)`)
		if m := reSOW.FindStringSubmatch(text); len(m) > 1 {
			address = strings.TrimSpace(m[1])
		}
	}

	return &DLResult{
		Name:      name,
		DLNumber:  dlNumber,
		DOB:       dobStr,
		IssueDate: issueStr,
		ValidTill: validStr,
		Address:   address,
		RawText:   raw,
	}
}

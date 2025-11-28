package dto

import (
	"encoding/xml"
	"fmt"
	"mime/multipart"
	"strings"
)

// AadhaarExtractRequest represents the request for Aadhaar extraction
type AadhaarExtractRequest struct {
	File     *multipart.FileHeader
	Password string
}

// Validate validates the Aadhaar extraction request
func (r *AadhaarExtractRequest) Validate() error {
	if r.File == nil {
		return fmt.Errorf("file is required")
	}

	// Validate file extension
	filename := strings.ToLower(r.File.Filename)
	validExtensions := []string{".pdf", ".png", ".jpg", ".jpeg"}
	valid := false
	for _, ext := range validExtensions {
		if strings.HasSuffix(filename, ext) {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid file type. Supported: PDF, PNG, JPG")
	}

	return nil
}

// AadhaarExtractResponse represents the response from Aadhaar extraction
type AadhaarExtractResponse struct {
	Name         string `json:"name"`
	DOB          string `json:"dob"`
	Gender       string `json:"gender"`
	Address      string `json:"address"`
	AadhaarLast4 string `json:"aadhaar_last4"`
	Source       string `json:"source"` // "qr" or "ocr"
}

// AadhaarQRData represents the XML structure in Aadhaar QR code
// Based on UIDAI's secure QR code format
type AadhaarQRData struct {
	XMLName     xml.Name `xml:"PrintLetterBarcodeData"`
	UID         string   `xml:"uid,attr"`
	Name        string   `xml:"name,attr"`
	Gender      string   `xml:"gender,attr"`
	YearOfBirth string   `xml:"yob,attr"`
	DateOfBirth string   `xml:"dob,attr"`
	CO          string   `xml:"co,attr"` // Care of
	House       string   `xml:"house,attr"`
	Street      string   `xml:"street,attr"`
	Landmark    string   `xml:"lm,attr"`
	Locality    string   `xml:"loc,attr"`
	VTC         string   `xml:"vtc,attr"` // Village/Town/City
	PO          string   `xml:"po,attr"`  // Post Office
	District    string   `xml:"dist,attr"`
	SubDistrict string   `xml:"subdist,attr"`
	State       string   `xml:"state,attr"`
	PC          string   `xml:"pc,attr"` // Pin Code
}

// GetFullAddress constructs the full address from QR data
func (q *AadhaarQRData) GetFullAddress() string {
	parts := []string{}

	if q.CO != "" {
		parts = append(parts, "C/O "+q.CO)
	}
	if q.House != "" {
		parts = append(parts, q.House)
	}
	if q.Street != "" {
		parts = append(parts, q.Street)
	}
	if q.Landmark != "" {
		parts = append(parts, q.Landmark)
	}
	if q.Locality != "" {
		parts = append(parts, q.Locality)
	}
	if q.VTC != "" {
		parts = append(parts, q.VTC)
	}
	if q.PO != "" {
		parts = append(parts, "PO "+q.PO)
	}
	if q.SubDistrict != "" {
		parts = append(parts, q.SubDistrict)
	}
	if q.District != "" {
		parts = append(parts, q.District)
	}
	if q.State != "" {
		parts = append(parts, q.State)
	}
	if q.PC != "" {
		parts = append(parts, q.PC)
	}

	return strings.Join(parts, ", ")
}

// GetLast4Digits returns the last 4 digits of Aadhaar number
func (q *AadhaarQRData) GetLast4Digits() string {
	if len(q.UID) >= 4 {
		return q.UID[len(q.UID)-4:]
	}
	return q.UID
}

// GetDOB returns the date of birth in a standardized format
func (q *AadhaarQRData) GetDOB() string {
	if q.DateOfBirth != "" {
		return q.DateOfBirth
	}
	if q.YearOfBirth != "" {
		return q.YearOfBirth
	}
	return ""
}

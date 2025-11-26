package client

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"

	"github.com/otiai10/gosseract/v2"
)

type TesseractClient struct {
	dataPath string
}

func NewTesseractClient(dataPath string) *TesseractClient {
	return &TesseractClient{
		dataPath: dataPath,
	}
}

// ExtractTextFromFile extracts text from an uploaded file using Tesseract OCR
func (tc *TesseractClient) ExtractTextFromFile(fileHeader *multipart.FileHeader) (string, error) {
	// Open uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Create temporary file
	tempFile, err := tc.CreateTempFile(file, fileHeader.Filename)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile)

	// Extract text using Tesseract
	text, err := tc.extractText(tempFile)
	if err != nil {
		return "", fmt.Errorf("OCR extraction failed: %w", err)
	}

	return text, nil
}

// CreateTempFile creates a temporary file from uploaded content
func (tc *TesseractClient) CreateTempFile(file multipart.File, filename string) (string, error) {
	ext := filepath.Ext(filename)
	tempFile, err := os.CreateTemp("", "ocr-*"+ext)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, file); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

func (tc *TesseractClient) extractText(filePath string) (string, error) {
	client := gosseract.NewClient()
	defer client.Close()

	// VERY IMPORTANT: Explicitly set correct tessdata path
	client.SetTessdataPrefix("/usr/share/tesseract-ocr/5/tessdata/")

	// Set language to English
	if err := client.SetLanguage("eng"); err != nil {
		return "", fmt.Errorf("failed to set language: %w", err)
	}

	// Set input image
	if err := client.SetImage(filePath); err != nil {
		return "", fmt.Errorf("failed to set image: %w", err)
	}

	// Extract text
	text, err := client.Text()
	if err != nil {
		return "", fmt.Errorf("failed to extract text: %w", err)
	}

	return text, nil
}

// ExtractTextAndQualityFromFile extracts text and quality scores from an uploaded file
func (tc *TesseractClient) ExtractTextAndQualityFromFile(fileHeader *multipart.FileHeader) (string, float64, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", 0, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	tempFile, err := tc.CreateTempFile(file, fileHeader.Filename)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile)

	return tc.ExtractTextAndQuality(tempFile)
}

func (tc *TesseractClient) ExtractTextAndQuality(filePath string) (string, float64, error) {
	client := gosseract.NewClient()
	defer client.Close()

	client.SetTessdataPrefix("/usr/share/tesseract-ocr/5/tessdata/")
	if err := client.SetLanguage("eng"); err != nil {
		return "", 0, fmt.Errorf("failed to set language: %w", err)
	}

	if err := client.SetImage(filePath); err != nil {
		return "", 0, fmt.Errorf("failed to set image: %w", err)
	}

	text, err := client.Text()
	if err != nil {
		return "", 0, fmt.Errorf("failed to extract text: %w", err)
	}

	// Get bounding boxes to calculate confidence
	boxes, err := client.GetBoundingBoxes(gosseract.RIL_WORD)
	if err != nil {
		// If bounding boxes fail, just return text and 0 confidence
		return text, 0, nil
	}

	var totalConf float64
	var count int
	for _, box := range boxes {
		totalConf += box.Confidence
		count++
	}

	avgConf := 0.0
	if count > 0 {
		avgConf = totalConf / float64(count)
	}

	return text, avgConf, nil
}

// Close performs cleanup
func (tc *TesseractClient) Close() {
	log.Println("Tesseract client closed")
}
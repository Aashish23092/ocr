package service

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"strings"

	"github.com/Aashish23092/ocr-income-verification/client"
	"github.com/Aashish23092/ocr-income-verification/dto"
	"github.com/Aashish23092/ocr-income-verification/utils"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

// AadhaarService handles Aadhaar card data extraction
type AadhaarService struct {
	tesseractClient *client.TesseractClient
	pdfProcessor    PDFProcessor
	paddleClient    *client.PaddleClient
}

// NewAadhaarService creates a new AadhaarService instance
func NewAadhaarService(tesseractClient *client.TesseractClient, pdfProcessor PDFProcessor) *AadhaarService {
	// Initialize PaddleOCR client (optional, falls back to Tesseract if unavailable)
	paddle, err := client.NewPaddleClient()
	if err != nil {
		log.Printf("Warning: PaddleOCR client initialization failed: %v. Will use Tesseract only.", err)
		paddle = nil
	}

	return &AadhaarService{
		tesseractClient: tesseractClient,
		pdfProcessor:    pdfProcessor,
		paddleClient:    paddle,
	}
}

// ExtractFromFile extracts Aadhaar data from a file (PDF or image)
func (s *AadhaarService) ExtractFromFile(ctx context.Context, fileData []byte, mimeType, password string) (*dto.AadhaarExtractResponse, error) {
	var img image.Image
	var err error

	// Handle PDF files
	if strings.Contains(mimeType, "pdf") {
		log.Println("Processing PDF file for Aadhaar extraction")

		// Extract images from PDF (we'll use the first page)
		images, err := s.pdfProcessor.ExtractImages(fileData, password)
		if err != nil {
			return nil, fmt.Errorf("failed to extract images from PDF: %w", err)
		}

		if len(images) == 0 {
			return nil, fmt.Errorf("no images found in PDF")
		}

		// Use the first page
		img = images[0]
		log.Println("Extracted first page from PDF as image")
	} else {
		// Handle image files (PNG, JPEG)
		log.Println("Processing image file for Aadhaar extraction")
		img, err = decodeImage(fileData, mimeType)
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %w", err)
		}
	}

	// Try QR code extraction first
	log.Println("Attempting QR code extraction...")
	qrResult, err := s.extractFromQR(img)
	if err == nil && qrResult != nil {
		log.Println("Successfully extracted data from QR code")
		return qrResult, nil
	}
	log.Printf("QR extraction failed or no QR found: %v. Falling back to OCR...", err)

	// Fallback to OCR
	log.Println("Attempting OCR extraction...")
	ocrResult, err := s.extractFromOCR(img)
	if err != nil {
		return nil, fmt.Errorf("both QR and OCR extraction failed: %w", err)
	}

	log.Println("Successfully extracted data using OCR")
	return ocrResult, nil
}

// extractFromQR attempts to extract Aadhaar data from QR code
func (s *AadhaarService) extractFromQR(img image.Image) (*dto.AadhaarExtractResponse, error) {
	// Convert image to BinaryBitmap for QR decoding
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return nil, fmt.Errorf("failed to create binary bitmap: %w", err)
	}

	// Create QR code reader
	qrReader := qrcode.NewQRCodeReader()

	// Decode QR code
	result, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode QR code: %w", err)
	}

	// Parse XML from QR code
	qrText := result.GetText()
	log.Printf("QR code decoded, length: %d bytes", len(qrText))

	var qrData dto.AadhaarQRData
	if err := xml.Unmarshal([]byte(qrText), &qrData); err != nil {
		return nil, fmt.Errorf("failed to parse QR XML data: %w", err)
	}

	// Build response from QR data
	response := &dto.AadhaarExtractResponse{
		Name:         qrData.Name,
		DOB:          qrData.GetDOB(),
		Gender:       qrData.Gender,
		Address:      qrData.GetFullAddress(),
		AadhaarLast4: qrData.GetLast4Digits(),
		Source:       "qr",
	}

	return response, nil
}

// extractFromOCR attempts to extract Aadhaar data using PaddleOCR (primary) or Tesseract (fallback)
func (s *AadhaarService) extractFromOCR(img image.Image) (*dto.AadhaarExtractResponse, error) {
	var text string
	var err error

	// Try PaddleOCR first if available
	if s.paddleClient != nil {
		log.Println("Attempting PaddleOCR extraction...")
		// Convert image.Image → PNG bytes before sending to PaddleOCR
		buf := new(bytes.Buffer)
		if err := png.Encode(buf, img); err != nil {
			log.Printf("failed to encode image for PaddleOCR: %v", err)
		} else {
			text, err = s.paddleClient.ExtractText(buf.Bytes())
		}

		if err != nil || len(strings.TrimSpace(text)) < 50 {
			log.Printf("PaddleOCR failed or extracted insufficient text (len=%d): %v. Falling back to Tesseract...", len(text), err)
			// Fall through to Tesseract
		} else {
			log.Printf("PaddleOCR succeeded, extracted %d characters", len(text))
			// PaddleOCR succeeded, skip Tesseract
			goto ParseText
		}
	} else {
		log.Println("PaddleOCR client not available, using Tesseract directly")
	}

	// Fallback to Tesseract
	{
		log.Println("Using Tesseract OCR...")
		// Save image to temporary file for Tesseract
		tempFile, err := saveAadhaarImageToTempFile(img)
		if err != nil {
			return nil, fmt.Errorf("failed to save temp image: %w", err)
		}
		defer os.Remove(tempFile)

		// Extract text using Tesseract
		text, _, err = s.tesseractClient.ExtractTextAndQuality(tempFile)
		if err != nil {
			return nil, fmt.Errorf("OCR extraction failed: %w", err)
		}
		log.Printf("Tesseract extracted %d characters", len(text))
	}

ParseText:
	// 🔥🔥 OCR DEBUG DUMP 🔥🔥
	log.Println("=========== OCR RAW OUTPUT BEGIN ===========")
	log.Println(text)
	log.Println("============ OCR RAW OUTPUT END ============")

	// FORCE PRINTF (Docker always shows this)
	fmt.Printf("\n\n----- OCR RAW TEXT (FORCE DUMP) -----\n%s\n----- END OCR RAW TEXT -----\n\n", text)

	// SAVE TO FILE (failsafe)
	os.WriteFile("/tmp/ocr_dump.txt", []byte(text), 0644)
	log.Println("OCR dump saved to /tmp/ocr_dump.txt")

	log.Printf("OCR extracted %d characters of text", len(text))

	// Parse Aadhaar data from OCR text
	result := utils.ParseAadhaarFromText(text)

	// Validate that we got at least some data
	if result.Name == "" && result.AadhaarLast4 == "" {
		return nil, fmt.Errorf("could not extract meaningful Aadhaar data from OCR text")
	}

	return &result, nil
}

// decodeImage decodes an image from bytes based on MIME type
func decodeImage(data []byte, mimeType string) (image.Image, error) {
	reader := bytes.NewReader(data)

	if strings.Contains(mimeType, "png") {
		return png.Decode(reader)
	} else if strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg") {
		return jpeg.Decode(reader)
	}

	// Try to decode anyway
	img, _, err := image.Decode(reader)
	return img, err
}

// saveAadhaarImageToTempFile saves an image to a temporary PNG file for OCR processing
func saveAadhaarImageToTempFile(img image.Image) (string, error) {
	tempFile, err := os.CreateTemp("", "aadhaar-ocr-*.png")
	if err != nil {
		return "", fmt.Errorf("failed to create temp image file: %w", err)
	}
	defer tempFile.Close()

	if err := png.Encode(tempFile, img); err != nil {
		return "", fmt.Errorf("failed to encode image to PNG: %w", err)
	}

	return tempFile.Name(), nil
}

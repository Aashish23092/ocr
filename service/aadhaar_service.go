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
	var images []image.Image
	var img image.Image
	var err error

	// ---------------------------------------------
	// 1️⃣ PDF → extract ALL pages as images
	// ---------------------------------------------
	if strings.Contains(mimeType, "pdf") {
		log.Println("Processing PDF file for Aadhaar extraction")

		images, err = s.pdfProcessor.ExtractImages(fileData, password)
		if err != nil {
			return nil, fmt.Errorf("failed to extract images from PDF: %w", err)
		}

		if len(images) == 0 {
			return nil, fmt.Errorf("no images found in PDF")
		}

		// Aadhaar identity info is almost always on page 2
		if len(images) > 1 {
			log.Println("Using page 2 (Aadhaar front side with Name/DOB/Gender)")
			img = images[1]
		} else {
			log.Println("PDF has only one page; using page 1")
			img = images[0]
		}
	} else {
		// ---------------------------------------------
		// 2️⃣ PNG/JPEG case
		// ---------------------------------------------
		log.Println("Processing image file for Aadhaar extraction")
		img, err = decodeImage(fileData, mimeType)
		if err != nil {
			return nil, fmt.Errorf("failed to decode image: %w", err)
		}
	}

	// ---------------------------------------------
	// 3️⃣ QR extraction (first attempt)
	// ---------------------------------------------
	log.Println("Attempting QR code extraction...")
	qrResult, err := s.extractFromQR(img)
	if err == nil && qrResult != nil {
		log.Println("Successfully extracted data from QR code")
		return qrResult, nil
	}
	log.Printf("QR extraction failed or no QR found: %v. Falling back to OCR...", err)

	// ---------------------------------------------
	// 4️⃣ OCR on ALL PAGES (Name/DOB/Gender often exist on page 2)
	// ---------------------------------------------
	var fullText strings.Builder

	if len(images) > 0 {
		log.Printf("Running OCR on %d pages...", len(images))
		for idx, page := range images {
			log.Printf("OCR on page %d...", idx+1)

			buf := new(bytes.Buffer)
			if err := png.Encode(buf, page); err != nil {
				log.Printf("Failed to encode page %d: %v", idx+1, err)
				continue
			}

			pageText, err := s.paddleClient.ExtractText(buf.Bytes())
			if err != nil {
				log.Printf("Page %d OCR failed: %v", idx+1, err)
				continue
			}

			fullText.WriteString("\n")
			fullText.WriteString(pageText)
		}
	} else {
		// Single image case
		pageText, err := s.paddleClient.ExtractText(fileData)
		if err != nil {
			return nil, fmt.Errorf("OCR extraction failed: %w", err)
		}
		fullText.WriteString(pageText)
	}

	ocrText := fullText.String()

	// Debug dump
	log.Println("=========== OCR RAW OUTPUT BEGIN ===========")
	log.Println(ocrText)
	log.Println("=========== OCR RAW OUTPUT END =============")

	// ---------------------------------------------
	// 5️⃣ Parse Aadhaar info from combined OCR text
	// ---------------------------------------------
	result := utils.ParseAadhaarFromText(ocrText)

	// If even combined OCR yields nothing meaningful → error
	if result.Name == "" && result.AadhaarLast4 == "" {
		return nil, fmt.Errorf("could not extract meaningful Aadhaar data from OCR text")
	}

	return &result, nil
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

// ExtractFromImages processes 2 or more Aadhaar images (front + back)
func (s *AadhaarService) ExtractFromImages(
	ctx context.Context,
	imagesData [][]byte,
	mimeTypes []string,
	password string,
) (*dto.AadhaarExtractResponse, error) {

	if len(imagesData) == 0 {
		return nil, fmt.Errorf("no images provided")
	}

	// Decode all images
	var images []image.Image
	for i := range imagesData {
		img, err := decodeImage(imagesData[i], mimeTypes[i])
		if err != nil {
			log.Printf("Failed to decode image %d: %v", i+1, err)
			continue
		}
		images = append(images, img)
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("failed to decode any Aadhaar image")
	}

	// -------------------------------------------------------------
	// 1️⃣ Try QR extraction from ALL pages (QR often on back side)
	// -------------------------------------------------------------
	for i, img := range images {
		log.Printf("Trying QR extraction on image %d...", i+1)
		qr, err := s.extractFromQR(img)
		if err == nil && qr != nil {
			log.Println("QR extraction succeeded")
			return qr, nil
		}
	}

	// -------------------------------------------------------------
	// 2️⃣ OCR on ALL images → Combine text intelligently
	// -------------------------------------------------------------
	var combined strings.Builder

	for i, img := range images {
		log.Printf("Running OCR on image %d...", i+1)

		buf := new(bytes.Buffer)
		if err := png.Encode(buf, img); err != nil {
			log.Printf("PNG encode failed for image %d: %v", i+1, err)
			continue
		}

		pageText, err := s.paddleClient.ExtractText(buf.Bytes())
		if err != nil {
			log.Printf("OCR failed for image %d: %v", i+1, err)
			continue
		}

		combined.WriteString("\n")
		combined.WriteString(pageText)
	}

	fullText := combined.String()

	log.Println("=========== OCR RAW OUTPUT BEGIN ===========")
	log.Println(fullText)
	log.Println("=========== OCR RAW OUTPUT END =============")

	// -------------------------------------------------------------
	// 3️⃣ Parse combined OCR text for Aadhaar data
	// -------------------------------------------------------------
	result := utils.ParseAadhaarFromText(fullText)

	if result.Name == "" && result.AadhaarLast4 == "" {
		return nil, fmt.Errorf("could not extract valid Aadhaar details from OCR text")
	}

	return &result, nil
}

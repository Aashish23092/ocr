package service

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"mime/multipart"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Aashish23092/ocr-income-verification/client"
	"github.com/Aashish23092/ocr-income-verification/dto"
	"github.com/Aashish23092/ocr-income-verification/utils"
)

type IncomeService struct {
	tesseractClient *client.TesseractClient
	pdfProcessor    PDFProcessor
	paddleClient    *client.PaddleClient
}

func NewIncomeService(
	tesseractClient *client.TesseractClient,
	pdfProcessor PDFProcessor,
	paddleClient *client.PaddleClient,
) *IncomeService {
	return &IncomeService{
		tesseractClient: tesseractClient,
		pdfProcessor:    pdfProcessor,
		paddleClient:    paddleClient,
	}
}

// VerifyIncome processes salary slips and bank statement, performs OCR and cross-verification
func (s *IncomeService) VerifyIncome(req *dto.IncomeVerificationRequest) (*dto.IncomeVerificationResponse, error) {
	// Parse metadata
	var metadata dto.UploadMetadata
	if err := json.Unmarshal([]byte(req.Metadata), &metadata); err != nil {
		return nil, fmt.Errorf("invalid metadata JSON: %w", err)
	}

	// Map filenames to files for easy access
	fileMap := make(map[string]*multipart.FileHeader)
	for _, file := range req.Files {
		fileMap[file.Filename] = file
	}

	var salarySlips []dto.SalarySlipData
	var bankStatements []dto.BankStatementData
	var mu sync.Mutex
	var wg sync.WaitGroup
	errors := make([]error, 0)

	// Process each document defined in metadata
	for _, docMeta := range metadata.Documents {
		fileHeader, ok := fileMap[docMeta.Filename]
		if !ok {
			log.Printf("Warning: File %s mentioned in metadata not found in upload", docMeta.Filename)
			continue
		}

		wg.Add(1)
		go func(meta dto.DocumentMeta, file *multipart.FileHeader) {
			defer wg.Done()

			// Open file to read bytes (needed for PDF processing)
			f, err := file.Open()
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to open file %s: %w", meta.Filename, err))
				mu.Unlock()
				return
			}
			defer f.Close()

			fileBytes, err := io.ReadAll(f)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to read file %s: %w", meta.Filename, err))
				mu.Unlock()
				return
			}

			// Process document
			result, err := s.ProcessDocument(context.Background(), file, fileBytes, meta)
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Errorf("failed to process file %s: %w", meta.Filename, err))
				mu.Unlock()
				return
			}

			mu.Lock()
			switch v := result.(type) {
			case dto.SalarySlipData:
				salarySlips = append(salarySlips, v)
			case dto.BankStatementData:
				bankStatements = append(bankStatements, v)
			}
			mu.Unlock()
		}(docMeta, fileHeader)
	}

	wg.Wait()

	if len(errors) > 0 {
		return nil, errors[0]
	}

	// Perform cross-verification
	crossCheckResult := s.CrossCheck(salarySlips, bankStatements)

	// Build response
	response := &dto.IncomeVerificationResponse{
		SalarySlips:     salarySlips,
		BankStatements:  bankStatements,
		CrossCheck:      crossCheckResult,
		MinQualityScore: 60.0, // Default threshold
		ProcessedAt:     time.Now().Format(time.RFC3339),
	}

	return response, nil
}

func (s *IncomeService) ProcessDocument(ctx context.Context, fileHeader *multipart.FileHeader, data []byte, meta dto.DocumentMeta) (interface{}, error) {
	var text string
	var err error
	var quality dto.DocumentQuality

	// Detect type based on extension
	isPDF := strings.HasSuffix(strings.ToLower(meta.Filename), ".pdf")

	if isPDF {
		// Try text extraction first
		text, err = s.pdfProcessor.ExtractText(data, meta.Password)
		if err != nil {
			log.Printf("PDF text extraction failed for %s: %v", meta.Filename, err)
			quality.Issues = append(quality.Issues, "pdf_text_extraction_failed")
		}

		// If text is empty or too short, try image extraction (scanned PDF)
		if len(strings.TrimSpace(text)) < 20 {
			log.Printf("PDF %s seems to be scanned or has minimal text, attempting image-based OCR", meta.Filename)

			images, imgErr := s.pdfProcessor.ExtractImages(data, meta.Password)
			if imgErr != nil || len(images) == 0 {
				log.Printf("Failed to extract images from PDF %s: %v", meta.Filename, imgErr)
				quality.Issues = append(quality.Issues, "pdf_image_extraction_failed")
			} else {
				// OCR each image and aggregate results
				var combinedText strings.Builder
				var totalConfidence float64
				var imageCount int

				for _, img := range images {
					tempImgFile, err := saveImageToTempFile(img)
					if err != nil {
						log.Printf("Failed to save temporary image for OCR: %v", err)
						continue
					}

					// Paddle first
					pageText, ocrErr := s.paddleClient.ExtractTextFromFile(tempImgFile)
					var pageConf float64 = 75.0

					// If Paddle fails, fallback to Tesseract
					if ocrErr != nil || len(strings.TrimSpace(pageText)) < 10 {
						pageText, pageConf, ocrErr = s.tesseractClient.ExtractTextAndQuality(tempImgFile)
					}
					if ocrErr != nil {
						log.Printf("OCR failed for a page in %s: %v", meta.Filename, ocrErr)
						os.Remove(tempImgFile) // Clean up on error
						continue
					}

					combinedText.WriteString(pageText)
					combinedText.WriteString("\n") // Page break
					totalConfidence += pageConf
					imageCount++

					os.Remove(tempImgFile) // Clean up immediately
				}

				if imageCount > 0 {
					text = combinedText.String()
					quality.OcrConfidence = totalConfidence / float64(imageCount)
					quality.ResolutionScore = 80.0 // Placeholder
					quality.FinalScore = (quality.OcrConfidence + quality.ResolutionScore) / 2
					if quality.FinalScore < 60 {
						quality.Issues = append(quality.Issues, "low_quality_document")
					}
				} else {
					quality.Issues = append(quality.Issues, "scanned_pdf_ocr_failed")
				}
			}
		} else {
			// Text-based PDF
			quality.OcrConfidence = 100.0
			quality.ResolutionScore = 100.0 // Vector PDF
			quality.FinalScore = 100.0
		}
	} else {
		// ---------------------------
		// 1. Try PaddleOCR first
		// ---------------------------
		paddleText, err := s.paddleClient.ExtractText(data)
		if err == nil && len(strings.TrimSpace(paddleText)) > 5 {
			text = paddleText

			quality.OcrConfidence = 75.0 // Default for PaddleOCR
			quality.ResolutionScore = 80.0
			quality.FinalScore = (quality.OcrConfidence + quality.ResolutionScore) / 2

			// Parse based on doc type
			if meta.DocType == dto.DocTypeSalarySlip {
				parsed := utils.ParseSalarySlip(text)
				parsed.Quality = quality
				return parsed, nil
			} else if meta.DocType == dto.DocTypeBankStatement {
				parsed := utils.ParseBankStatement(text)
				parsed.Quality = quality
				return parsed, nil
			}
		}

		// Image file
		var conf float64
		text, conf, err = s.tesseractClient.ExtractTextAndQualityFromFile(fileHeader)
		if err != nil {
			return nil, fmt.Errorf("image OCR failed: %w", err)
		}

		quality.OcrConfidence = conf
		quality.ResolutionScore = 80.0 // Placeholder, need image dimensions
		quality.FinalScore = (quality.OcrConfidence + quality.ResolutionScore) / 2

		if quality.FinalScore < 60 {
			quality.Issues = append(quality.Issues, "low_quality_document")
		}
	}

	// Parse based on doc type
	if meta.DocType == dto.DocTypeSalarySlip {
		data := utils.ParseSalarySlip(text)
		data.Quality = quality
		return data, nil
	} else if meta.DocType == dto.DocTypeBankStatement {
		data := utils.ParseBankStatement(text)
		data.Quality = quality
		return data, nil
	}

	return nil, fmt.Errorf("unknown document type: %s", meta.DocType)
}

func (s *IncomeService) CrossCheck(slips []dto.SalarySlipData, stmts []dto.BankStatementData) dto.CrossCheckResult {
	result := dto.CrossCheckResult{
		Notes: []string{},
	}

	if len(stmts) == 0 {
		result.Notes = append(result.Notes, "No bank statements provided for cross-check")
		return result
	}

	stmt := stmts[0] // Primary statement

	// Name Match
	for _, slip := range slips {
		if utils.CompareNames(slip.EmployeeName, stmt.AccountHolderName) {
			result.NameMatch = true
			result.NameSimilarity = 1.0 // Simplified
			break
		}
	}

	// Account Match
	for _, slip := range slips {
		if slip.AccountNumber != "" && stmt.AccountNumber != "" {
			if strings.ReplaceAll(slip.AccountNumber, " ", "") == strings.ReplaceAll(stmt.AccountNumber, " ", "") {
				result.AccountMatch = true
				break
			}
		}
	}

	// Salary Credit Match (Simplified)
	// Check if any credit matches net salary within a margin
	for _, slip := range slips {
		if slip.NetSalary > 0 {
			found := false
			for _, tx := range stmt.Transactions {
				if tx.IsCredit && tx.Amount == slip.NetSalary {
					found = true
					break
				}
			}
			if !found {
				result.MissingSalaryCredits = append(result.MissingSalaryCredits, fmt.Sprintf("Missing credit for %s: %.2f", slip.PayMonth, slip.NetSalary))
			}
		}
	}

	return result
}

// saveImageToTempFile saves an image.Image to a temporary PNG file.
func saveImageToTempFile(img image.Image) (string, error) {
	tempFile, err := os.CreateTemp("", "ocr-img-*.png")
	if err != nil {
		return "", fmt.Errorf("failed to create temp image file: %w", err)
	}
	defer tempFile.Close()

	if err := png.Encode(tempFile, img); err != nil {
		return "", fmt.Errorf("failed to encode image to PNG: %w", err)
	}

	return tempFile.Name(), nil
}

// AnalyzeITR processes an ITR document and extracts structured data
func (s *IncomeService) AnalyzeITR(fileHeader *multipart.FileHeader) (*dto.ITRResult, error) {
	log.Printf("Starting ITR analysis for file: %s", fileHeader.Filename)

	file, err := fileHeader.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var extractedText string
	isPDF := strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".pdf")

	// ---------------------------------------------------
	// CASE 1 — PDF (ITR files are ALWAYS PDF)
	// ---------------------------------------------------
	if isPDF {

		// 1) Try embedded PDF text
		text, err := s.pdfProcessor.ExtractText(fileBytes, "")
		if err == nil {
			extractedText = text
		}

		// 2) If extracted text is weak → use Paddle on PDF images
		if evaluateTextQuality(extractedText) < 50 {
			log.Println("PDF text is weak → using PaddleOCR on extracted images")

			images, err := s.pdfProcessor.ExtractImages(fileBytes, "")
			if err != nil || len(images) == 0 {
				log.Printf("Failed to extract images from PDF: %v", err)
			} else {
				var combined strings.Builder
				for _, img := range images {

					tmp, err := saveImageToTempFile(img)
					if err != nil {
						continue
					}

					paddleText, err := s.paddleClient.ExtractTextFromFile(tmp)
					os.Remove(tmp)

					if err == nil && len(strings.TrimSpace(paddleText)) > 10 {
						combined.WriteString(paddleText)
						combined.WriteString("\n")
					}
				}

				// Use PaddleOCR result if it's meaningful
				if len(strings.TrimSpace(combined.String())) > 20 {
					extractedText = combined.String()
				}
			}
		}

		// 3) If still empty → final fallback: Tesseract
		if len(strings.TrimSpace(extractedText)) == 0 {
			text, _, err := s.tesseractClient.ExtractTextAndQualityFromFile(fileHeader)
			if err == nil {
				extractedText = text
			}
		}

	} else {

		// ---------------------------------------------------
		// CASE 2 — Non-PDF → PNG/JPG → Paddle first
		// ---------------------------------------------------
		paddleText, err := s.paddleClient.ExtractText(fileBytes)
		if err == nil && len(strings.TrimSpace(paddleText)) > 5 {
			extractedText = paddleText
		} else {
			// fallback to Tesseract
			text, _, err := s.tesseractClient.ExtractTextAndQualityFromFile(fileHeader)
			if err != nil {
				return nil, fmt.Errorf("OCR failed: %w", err)
			}
			extractedText = text
		}
	}

	if len(strings.TrimSpace(extractedText)) == 0 {
		return nil, fmt.Errorf("no text could be extracted from the document")
	}

	result := utils.ParseITR(extractedText)

	log.Printf("ITR analysis done → PAN=%s Name=%s AY=%s", result.PAN, result.Name, result.AssessmentYear)

	return &result, nil
}

// evaluateTextQuality evaluates the quality of extracted text
// Returns a score from 0-100 based on text length and keyword presence
func evaluateTextQuality(text string) float64 {
	if text == "" {
		return 0.0
	}

	score := 0.0

	// Length score (max 40 points)
	textLen := len(strings.TrimSpace(text))
	if textLen > 500 {
		score += 40.0
	} else if textLen > 100 {
		score += 20.0
	} else if textLen > 20 {
		score += 10.0
	}

	// Keyword presence score (max 60 points)
	keywords := []string{
		"income", "tax", "pan", "assessment", "return",
		"total", "taxable", "refund", "filing",
	}

	textLower := strings.ToLower(text)
	keywordCount := 0
	for _, keyword := range keywords {
		if strings.Contains(textLower, keyword) {
			keywordCount++
		}
	}

	// Each keyword adds points (up to 60)
	score += float64(keywordCount) * 6.67

	if score > 100.0 {
		score = 100.0
	}

	return score
}

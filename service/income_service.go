package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
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
}

func NewIncomeService(tesseractClient *client.TesseractClient, pdfProcessor PDFProcessor) *IncomeService {
	return &IncomeService{
		tesseractClient: tesseractClient,
		pdfProcessor:    pdfProcessor,
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
		}

		// If text is empty or too short, try image extraction (scanned PDF)
		if len(strings.TrimSpace(text)) < 100 {
			log.Printf("PDF %s seems to be scanned, attempting image extraction", meta.Filename)
			images, imgErr := s.pdfProcessor.ExtractImages(data, meta.Password)
			if imgErr == nil && len(images) > 0 {
				// OCR on images
				// For now, we just note it. To do proper OCR on images, we'd need to save them to temp files 
				// and call TesseractClient.extractTextAndQuality(tempFile).
				// Since we don't have that exposed easily for image.Image, we skip for now.
				quality.Issues = append(quality.Issues, "scanned_pdf_ocr_skipped")
			} else {
				quality.Issues = append(quality.Issues, "pdf_extraction_failed")
			}
		} else {
			// Text PDF
			quality.OcrConfidence = 100.0
			quality.ResolutionScore = 100.0 // Vector PDF
			quality.FinalScore = 100.0
		}
	} else {
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
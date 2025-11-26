package service

import (
	"bytes"
	"fmt"
	"image"
	
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

// PDFProcessor defines the interface for processing PDF files.
type PDFProcessor interface {
	ExtractText(pdfData []byte, password string) (string, error)
	ExtractImages(pdfData []byte, password string) ([]image.Image, error)
}

type pdfProcessor struct{}

// NewPDFProcessor creates a new PDFProcessor instance.
func NewPDFProcessor() PDFProcessor {
	return &pdfProcessor{}
}

// decryptPDFBytes attempts to decrypt a PDF using the provided password.
// It returns the decrypted PDF data. If no password is provided or the PDF is not encrypted,
// it returns the original data.
func (p *pdfProcessor) decryptPDFBytes(pdfData []byte, password string) ([]byte, error) {
	if password == "" {
		return pdfData, nil // No password, nothing to do
	}

	// Use pdfcpu to decrypt the PDF data.
	rs := bytes.NewReader(pdfData)
	conf := model.NewDefaultConfiguration()
	conf.UserPW = password
	conf.OwnerPW = password

	// Create a writer to hold the decrypted PDF.
	var out bytes.Buffer
	w := &out

	err := api.Decrypt(rs, w, conf)
	if err != nil {
		// api.Decrypt returns an error if the password is wrong or the file is not encrypted.
		// We can check for "not encrypted" error and ignore it.
		if strings.Contains(err.Error(), "not encrypted") {
			return pdfData, nil
		}
		return nil, fmt.Errorf("failed to decrypt PDF: %w", err)
	}

	return out.Bytes(), nil
}

// ExtractText extracts text from a PDF. It handles encrypted PDFs if a password is provided.
func (p *pdfProcessor) ExtractText(pdfData []byte, password string) (string, error) {
	decryptedData, err := p.decryptPDFBytes(pdfData, password)
	if err != nil {
		return "", fmt.Errorf("could not decrypt PDF for text extraction: %w", err)
	}

	r, err := pdf.NewReader(bytes.NewReader(decryptedData), int64(len(decryptedData)))
	if err != nil {
		return "", err
	}

	var textBuilder strings.Builder
	totalPage := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		page := r.Page(pageIndex)
		if page.V.IsNull() {
			continue
		}

		rows, err := page.GetTextByRow()
		if err != nil {
			// Log the error but continue processing other pages.
			fmt.Printf("Error getting text from page %d: %v\n", pageIndex, err)
			continue
		}
		
		for _, row := range rows {
			for _, word := range row.Content {
				textBuilder.WriteString(word.S)
			}
			textBuilder.WriteString("\n")
		}
	}
	return textBuilder.String(), nil
}

// ExtractImages converts PDF pages to images. It's used for scanned PDFs.
// It uses Poppler's pdftoppm tool.
func (p *pdfProcessor) ExtractImages(pdfData []byte, password string) ([]image.Image, error) {
	decryptedData, err := p.decryptPDFBytes(pdfData, password)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt PDF for image extraction: %w", err)
	}

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "pdf_images_")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup

	// Create a temporary file for the PDF
	tempPDFPath := filepath.Join(tempDir, "doc.pdf")
	if err := os.WriteFile(tempPDFPath, decryptedData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp PDF: %w", err)
	}

	// Use pdftoppm to convert PDF to images
	// pdftoppm -png input.pdf output_prefix
	cmd := exec.Command("pdftoppm", "-png", tempPDFPath, filepath.Join(tempDir, "page"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("pdftoppm failed: %v\nOutput: %s", err, string(output))
	}

	// Read extracted images
	var images []image.Image
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp dir: %w", err)
	}

	for _, file := range files {
		// We only care about the generated PNG files
		if !strings.HasSuffix(file.Name(), ".png") {
			continue
		}
		
		imgPath := filepath.Join(tempDir, file.Name())
		imgFile, err := os.Open(imgPath)
		if err != nil {
			continue // Or log the error
		}

		img, _, err := image.Decode(imgFile)
		imgFile.Close() // Ensure the file is closed
		if err != nil {
			continue // Or log the error
		}
		images = append(images, img)
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images could be extracted from the PDF")
	}

	return images, nil
}

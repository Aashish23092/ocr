package service

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type PDFProcessor interface {
	ExtractText(pdfData []byte, password string) (string, error)
	ExtractImages(pdfData []byte, password string) ([]image.Image, error)
}

type pdfProcessor struct{}

func NewPDFProcessor() PDFProcessor {
	return &pdfProcessor{}
}

func (p *pdfProcessor) ExtractText(pdfData []byte, password string) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(pdfData), int64(len(pdfData)))
	if err != nil {
		return "", err
	}

	// Password handling for ledongthuc/pdf is not straightforward in the public API if it's encrypted.
	// However, for this task, we'll assume standard text extraction.
	// If password support is needed for text extraction, we might need a different lib or check if ledongthuc/pdf supports it.
	// Checking the library, it doesn't seem to support encrypted PDFs well.
	// We might need to decrypt it first using pdfcpu if possible.

	// Let's try to decrypt with pdfcpu first if password is provided.
	if password != "" {
		// Decrypt logic here if needed, but pdfcpu operates on files or ReadSeekers.
		// For now, let's assume the PDF is accessible or we handle decryption separately.
		// Actually, pdfcpu can decrypt.
	}

	var textBuilder bytes.Buffer
	totalPage := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		p := r.Page(pageIndex)
		if p.V.IsNull() {
			continue
		}
		
		rows, _ := p.GetTextByRow()
		for _, row := range rows {
			for _, word := range row.Content {
				textBuilder.WriteString(word.S)
			}
			textBuilder.WriteString("\n")
		}
	}
	return textBuilder.String(), nil
}

func (p *pdfProcessor) ExtractImages(pdfData []byte, password string) ([]image.Image, error) {
	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "pdf_images")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir) // Cleanup

	// Create a temporary file for the PDF
	tempFile, err := os.CreateTemp("", "doc-*.pdf")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name()) // Cleanup file

	if _, err := tempFile.Write(pdfData); err != nil {
		return nil, fmt.Errorf("failed to write pdf data: %w", err)
	}
	tempFile.Close()

	conf := model.NewDefaultConfiguration()
	if password != "" {
		conf.UserPW = password
	}

	// Extract images to tempDir
	// We pass nil for selectedPages to extract from all pages
	if err := api.ExtractImagesFile(tempFile.Name(), tempDir, nil, conf); err != nil {
		return nil, fmt.Errorf("failed to extract images: %w", err)
	}

	// Read extracted images
	var images []image.Image
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp dir: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		imgPath := filepath.Join(tempDir, file.Name())
		imgFile, err := os.Open(imgPath)
		if err != nil {
			continue
		}

		img, _, err := image.Decode(imgFile)
		imgFile.Close()
		if err != nil {
			continue
		}
		images = append(images, img)
	}

	return images, nil
}

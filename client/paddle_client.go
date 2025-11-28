package client

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// PaddleClient wraps PaddleOCR for text extraction from images
// It supports both English and Hindi models for Aadhaar cards
type PaddleClient struct {
	enModelDir string
	hiModelDir string
}

// NewPaddleClient creates a new PaddleOCR client
// It loads model paths from environment variables
func NewPaddleClient() (*PaddleClient, error) {
	enModelDir := os.Getenv("PADDLE_OCR_EN_MODEL_DIR")
	hiModelDir := os.Getenv("PADDLE_OCR_HI_MODEL_DIR")

	if enModelDir == "" {
		enModelDir = "/opt/paddleocr/models/en"
	}
	if hiModelDir == "" {
		hiModelDir = "/opt/paddleocr/models/hi"
	}

	log.Printf("PaddleOCR initialized with EN model: %s, HI model: %s", enModelDir, hiModelDir)

	return &PaddleClient{
		enModelDir: enModelDir,
		hiModelDir: hiModelDir,
	}, nil
}

// ExtractText extracts text from an image using PaddleOCR
// It runs both English and Hindi models and merges the results
func (p *PaddleClient) ExtractText(img image.Image) (string, error) {
	// Save image to temporary file
	tempFile, err := saveTempImage(img)
	if err != nil {
		return "", fmt.Errorf("failed to save temp image: %w", err)
	}
	defer os.Remove(tempFile)

	// Extract text using English model
	enText, err := p.runPaddleOCR(tempFile, "en", p.enModelDir)
	if err != nil {
		log.Printf("English PaddleOCR failed: %v", err)
	}

	// Extract text using Hindi model
	hiText, err := p.runPaddleOCR(tempFile, "hi", p.hiModelDir)
	if err != nil {
		log.Printf("Hindi PaddleOCR failed: %v", err)
	}

	// Merge and deduplicate results
	mergedText := mergeAndDeduplicate(enText, hiText)

	if mergedText == "" {
		return "", fmt.Errorf("PaddleOCR extracted no text from image")
	}

	log.Printf("PaddleOCR extracted %d characters (EN: %d, HI: %d)",
		len(mergedText), len(enText), len(hiText))

	return mergedText, nil
}

// ExtractTextFromPDFBytes extracts text from PDF bytes using PaddleOCR HTTP API
// This is used for ITR analysis when PDF text extraction quality is low
func (p *PaddleClient) ExtractTextFromPDFBytes(pdfBytes []byte) (string, error) {
	// PaddleOCR REST API endpoint
	apiURL := os.Getenv("PADDLEOCR_API_URL")
	if apiURL == "" {
		apiURL = "http://paddleocr:8866/predict/ocr_system"
	}

	// Encode PDF bytes to base64
	encodedPDF := base64.StdEncoding.EncodeToString(pdfBytes)

	// Prepare request payload
	payload := map[string]interface{}{
		"images": []string{encodedPDF},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request payload: %w", err)
	}

	// Make HTTP POST request
	resp, err := http.Post(apiURL, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed to call PaddleOCR API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("PaddleOCR API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result struct {
		Results [][]struct {
			Text       string  `json:"text"`
			Confidence float64 `json:"confidence"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode PaddleOCR response: %w", err)
	}

	// Extract text from results
	var textBuilder strings.Builder
	if len(result.Results) > 0 {
		for _, line := range result.Results[0] {
			textBuilder.WriteString(line.Text)
			textBuilder.WriteString("\n")
		}
	}

	extractedText := textBuilder.String()
	if extractedText == "" {
		return "", fmt.Errorf("PaddleOCR extracted no text from PDF")
	}

	log.Printf("PaddleOCR HTTP API extracted %d characters", len(extractedText))
	return extractedText, nil
}

// runPaddleOCR executes PaddleOCR Python CLI for a specific language
func (p *PaddleClient) runPaddleOCR(imagePath, lang, modelDir string) (string, error) {
	// Build PaddleOCR command
	// Using Python's paddleocr CLI
	cmd := exec.Command("python3", "-c", fmt.Sprintf(`
import sys
from paddleocr import PaddleOCR
import warnings
warnings.filterwarnings('ignore')

ocr = PaddleOCR(
    use_angle_cls=True,
    lang='%s',
    det_model_dir='%s/det',
    rec_model_dir='%s/rec',
    cls_model_dir='%s/cls',
    use_gpu=False,
    show_log=False
)

result = ocr.ocr('%s', cls=True)
if result and result[0]:
    for line in result[0]:
        if line and len(line) > 1:
            print(line[1][0])
`, lang, modelDir, modelDir, modelDir, imagePath))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("PaddleOCR command failed: %v, stderr: %s", err, stderr.String())
	}

	return stdout.String(), nil
}

// mergeAndDeduplicate combines English and Hindi OCR results
// and removes duplicate lines
func mergeAndDeduplicate(enText, hiText string) string {
	seen := make(map[string]bool)
	var result []string

	// Process English text first
	for _, line := range strings.Split(enText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized := strings.ToLower(line)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, line)
		}
	}

	// Add Hindi text (non-duplicates)
	for _, line := range strings.Split(hiText, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized := strings.ToLower(line)
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

// saveTempImage saves an image.Image to a temporary PNG file
func saveTempImage(img image.Image) (string, error) {
	tempFile, err := os.CreateTemp("", "paddle-ocr-*.png")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	if err := png.Encode(tempFile, img); err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to encode image: %w", err)
	}

	return tempFile.Name(), nil
}

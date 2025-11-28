package handler

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/Aashish23092/ocr-income-verification/dto"
	"github.com/Aashish23092/ocr-income-verification/service"
	"github.com/gin-gonic/gin"
)

// AadhaarHandler handles Aadhaar extraction requests
type AadhaarHandler struct {
	aadhaarService *service.AadhaarService
}

// NewAadhaarHandler creates a new AadhaarHandler instance
func NewAadhaarHandler(aadhaarService *service.AadhaarService) *AadhaarHandler {
	return &AadhaarHandler{
		aadhaarService: aadhaarService,
	}
}

// ExtractAadhaar handles the POST /aadhaar/extract endpoint
func (h *AadhaarHandler) ExtractAadhaar(c *gin.Context) {
	log.Println("Received Aadhaar extraction request")

	// Parse multipart form
	file, err := c.FormFile("file")
	if err != nil {
		h.sendError(c, http.StatusBadRequest, "File is required", err)
		return
	}

	// Get optional password
	password := c.PostForm("password")

	// Build request DTO
	request := &dto.AadhaarExtractRequest{
		File:     file,
		Password: password,
	}

	// Validate request
	if err := request.Validate(); err != nil {
		h.sendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}

	// Validate MIME type
	mimeType := file.Header.Get("Content-Type")
	if mimeType == "" {
		// Infer from extension
		mimeType = inferMimeType(file.Filename)
	}

	if !isValidMimeType(mimeType) {
		h.sendError(c, http.StatusBadRequest, "Invalid file type. Supported: PDF, PNG, JPEG", nil)
		return
	}

	log.Printf("Processing file: %s (type: %s, size: %d bytes)", file.Filename, mimeType, file.Size)

	// Read file data
	fileReader, err := file.Open()
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "Failed to open uploaded file", err)
		return
	}
	defer fileReader.Close()

	fileData, err := io.ReadAll(fileReader)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "Failed to read file data", err)
		return
	}

	// Call service layer
	ctx := context.Background()
	response, err := h.aadhaarService.ExtractFromFile(ctx, fileData, mimeType, password)
	if err != nil {
		// Check if it's a password error
		if strings.Contains(err.Error(), "decrypt") || strings.Contains(err.Error(), "password") {
			h.sendError(c, http.StatusBadRequest, "Failed to decrypt PDF. Check password.", err)
			return
		}
		h.sendError(c, http.StatusInternalServerError, "Failed to extract Aadhaar data", err)
		return
	}

	// Send success response
	log.Println("Aadhaar extraction completed successfully")
	c.JSON(http.StatusOK, response)
}

// sendError sends a structured error response
func (h *AadhaarHandler) sendError(c *gin.Context, statusCode int, message string, err error) {
	errorMsg := message
	if err != nil {
		errorMsg = err.Error()
		log.Printf("Error: %s - %v", message, err)
	}

	c.JSON(statusCode, dto.ErrorResponse{
		Error:   "AADHAAR_EXTRACTION_FAILED",
		Message: errorMsg,
		Code:    statusCode,
	})
}

// isValidMimeType checks if the MIME type is supported
func isValidMimeType(mimeType string) bool {
	validTypes := []string{
		"application/pdf",
		"image/png",
		"image/jpeg",
		"image/jpg",
	}

	mimeType = strings.ToLower(mimeType)
	for _, valid := range validTypes {
		if strings.Contains(mimeType, valid) {
			return true
		}
	}
	return false
}

// inferMimeType infers MIME type from file extension
func inferMimeType(filename string) string {
	lower := strings.ToLower(filename)
	if strings.HasSuffix(lower, ".pdf") {
		return "application/pdf"
	} else if strings.HasSuffix(lower, ".png") {
		return "image/png"
	} else if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
		return "image/jpeg"
	}
	return ""
}

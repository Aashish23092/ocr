package handler

import (
	"context"
	"io"
	"log"
	"mime/multipart"
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

	// Parse multipart form (must read both return values)
	form, err := c.MultipartForm()
	var files []*multipart.FileHeader

	// CASE A → multiple files uploaded under "file"
	if err == nil && form != nil && len(form.File["file"]) > 1 {
		files = form.File["file"]

	} else {
		// CASE B → single file
		f, err := c.FormFile("file")
		if err != nil {
			h.sendError(c, http.StatusBadRequest, "At least one file is required", err)
			return
		}
		files = []*multipart.FileHeader{f}
	}

	password := c.PostForm("password")

	// ----------------------------------------------------
	// CASE 1 → MULTIPLE IMAGE INPUTS
	// ----------------------------------------------------
	if len(files) > 1 {
		log.Printf("Received %d Aadhaar images → Multi-page Aadhaar processing", len(files))

		var imagesData [][]byte
		var mimeTypes []string

		for _, file := range files {
			reader, err := file.Open()
			if err != nil {
				h.sendError(c, http.StatusInternalServerError, "Failed to open one of the uploaded files", err)
				return
			}
			data, err := io.ReadAll(reader)
			reader.Close()

			if err != nil {
				h.sendError(c, http.StatusInternalServerError, "Failed to read uploaded image", err)
				return
			}

			mimeType := file.Header.Get("Content-Type")
			if mimeType == "" {
				mimeType = inferMimeType(file.Filename)
			}

			if !isValidMimeType(mimeType) {
				h.sendError(c, http.StatusBadRequest, "Invalid file type. Supported: PDF, PNG, JPEG", nil)
				return
			}

			imagesData = append(imagesData, data)
			mimeTypes = append(mimeTypes, mimeType)
		}

		// MULTI-PAGE Aadhaar extraction
		ctx := context.Background()
		result, err := h.aadhaarService.ExtractFromImages(ctx, imagesData, mimeTypes, password)
		if err != nil {
			h.sendError(c, http.StatusInternalServerError, "Failed to extract Aadhaar from multiple images", err)
			return
		}

		log.Println("Aadhaar extraction completed successfully (multi-image)")
		c.JSON(http.StatusOK, result)
		return
	}

	// ----------------------------------------------------
	// CASE 2 → SINGLE FILE INPUT
	// ----------------------------------------------------
	file := files[0]
	log.Printf("Processing single Aadhaar file: %s", file.Filename)

	mimeType := file.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = inferMimeType(file.Filename)
	}

	if !isValidMimeType(mimeType) {
		h.sendError(c, http.StatusBadRequest, "Invalid file type. Supported: PDF, PNG, JPEG", nil)
		return
	}

	reader, err := file.Open()
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "Failed to open uploaded file", err)
		return
	}
	defer reader.Close()

	fileData, err := io.ReadAll(reader)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "Failed to read file data", err)
		return
	}

	ctx := context.Background()
	result, err := h.aadhaarService.ExtractFromFile(ctx, fileData, mimeType, password)
	if err != nil {
		if strings.Contains(err.Error(), "decrypt") {
			h.sendError(c, http.StatusBadRequest, "Failed to decrypt PDF. Check password.", err)
			return
		}
		h.sendError(c, http.StatusInternalServerError, "Failed to extract Aadhaar", err)
		return
	}

	log.Println("Aadhaar extraction completed successfully")
	c.JSON(http.StatusOK, result)
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

package handler

import (
	"log"
	"net/http"
	"github.com/Aashish23092/ocr-income-verification/dto"
	"github.com/Aashish23092/ocr-income-verification/service"

	"github.com/gin-gonic/gin"
)

type IncomeHandler struct {
	incomeService *service.IncomeService
}

func NewIncomeHandler(incomeService *service.IncomeService) *IncomeHandler {
	return &IncomeHandler{
		incomeService: incomeService,
	}
}

// VerifyIncome handles the POST /income/verify endpoint
func (h *IncomeHandler) VerifyIncome(c *gin.Context) {
	log.Println("Received income verification request")

	// Parse multipart form
	form, err := c.MultipartForm()
	if err != nil {
		h.sendError(c, http.StatusBadRequest, "Failed to parse multipart form", err)
		return
	}

	// Extract files
	files := form.File["files[]"]
	if len(files) == 0 {
		h.sendError(c, http.StatusBadRequest, "No files provided", nil)
		return
	}

	// Extract metadata
	metadata := c.PostForm("metadata")
	if metadata == "" {
		h.sendError(c, http.StatusBadRequest, "Metadata is required", nil)
		return
	}

	// Build request DTO
	request := &dto.IncomeVerificationRequest{
		Files:    files,
		Metadata: metadata,
	}

	// Validate request
	if err := request.Validate(); err != nil {
		h.sendError(c, http.StatusBadRequest, err.Error(), err)
		return
	}

	log.Printf("Processing %d files", len(files))

	// Call service layer
	response, err := h.incomeService.VerifyIncome(request)
	if err != nil {
		h.sendError(c, http.StatusInternalServerError, "Failed to verify income", err)
		return
	}

	// Send success response
	log.Println("Income verification completed successfully")
	c.JSON(http.StatusOK, response)
}

// sendError sends a structured error response
func (h *IncomeHandler) sendError(c *gin.Context, statusCode int, message string, err error) {
	errorMsg := message
	if err != nil {
		errorMsg = err.Error()
		log.Printf("Error: %s - %v", message, err)
	}

	c.JSON(statusCode, dto.ErrorResponse{
		Error:   "VERIFICATION_FAILED",
		Message: errorMsg,
		Code:    statusCode,
	})
}
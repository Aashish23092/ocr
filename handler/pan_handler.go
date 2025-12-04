package handler

import (
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Aashish23092/ocr-income-verification/service"
	"github.com/gin-gonic/gin"
)

type PANHandler struct {
	PANService *service.PANService
}

func NewPANHandler(panService *service.PANService) *PANHandler {
	return &PANHandler{
		PANService: panService,
	}
}

func (h *PANHandler) ExtractPAN(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file missing"})
		return
	}
	defer file.Close()

	tempDir := "./uploads"
	os.MkdirAll(tempDir, 0755)

	filePath := filepath.Join(tempDir, header.Filename)
	out, _ := os.Create(filePath)
	defer out.Close()

	_, _ = io.Copy(out, file)

	result, err := h.PANService.ExtractPANData(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

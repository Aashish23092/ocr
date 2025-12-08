package handler

import (
	"io"
	"net/http"

	"github.com/Aashish23092/ocr-income-verification/service"
	"github.com/gin-gonic/gin"
)

type DrivingLicenseHandler struct {
	service *service.DrivingLicenseService
}

func NewDrivingLicenseHandler(s *service.DrivingLicenseService) *DrivingLicenseHandler {
	return &DrivingLicenseHandler{service: s}
}

func (h *DrivingLicenseHandler) ExtractDL(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file missing"})
		return
	}
	defer file.Close()

	bytes, _ := io.ReadAll(file)

	result, err := h.service.ExtractDLText(bytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to extract DL"})
		return
	}

	c.JSON(http.StatusOK, result)
}

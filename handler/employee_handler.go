package handler

import (
	"io"
	"net/http"

	"github.com/Aashish23092/ocr-income-verification/service"
	"github.com/gin-gonic/gin"
)

type EmployeeHandler struct {
	svc *service.EmployeeService
}

func NewEmployeeHandler(svc *service.EmployeeService) *EmployeeHandler {
	return &EmployeeHandler{svc: svc}
}

func (h *EmployeeHandler) VerifyEmployee(c *gin.Context) {

	empFile, _, err := c.Request.FormFile("employee_id_card")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "employee_id_card missing"})
		return
	}
	empBytes, _ := io.ReadAll(empFile)

	appFile, _, err := c.Request.FormFile("appointment_letter")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "appointment_letter missing"})
		return
	}
	appBytes, _ := io.ReadAll(appFile)

	resp, err := h.svc.ProcessEmployeeDocs(empBytes, appBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

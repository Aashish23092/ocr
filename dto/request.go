package dto

import (
	"mime/multipart"
	"errors"
)

// IncomeVerificationRequest represents the incoming request
type IncomeVerificationRequest struct {
	Files    []*multipart.FileHeader `form:"files[]" binding:"required"`
	Metadata string                  `form:"metadata" binding:"required"`
}

// Validate performs basic validation on the request
func (r *IncomeVerificationRequest) Validate() error {
	if len(r.Files) == 0 {
		return ErrInsufficientSalarySlips // Reuse error or create new one
	}
	if r.Metadata == "" {
		return errors.New("metadata is required")
	}
	return nil
}
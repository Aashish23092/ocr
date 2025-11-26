package dto

import "errors"

// Custom errors
var (
	ErrInsufficientSalarySlips = errors.New("minimum 6 salary slips required")
)

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// IncomeVerificationResponse is the final response structure
type IncomeVerificationResponse struct {
	SalarySlips     []SalarySlipData  `json:"salary_slips"`
	BankStatements  []BankStatementData `json:"bank_statements"`
	CrossCheck      CrossCheckResult   `json:"cross_check"`
	MinQualityScore float64            `json:"min_quality_score"`
	ProcessedAt     string             `json:"processed_at"`
}
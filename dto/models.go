package dto

import "time"

type DocumentType string

const (
	DocTypeSalarySlip    DocumentType = "salary_slip"
	DocTypeBankStatement DocumentType = "bank_statement"
)

type DocumentMeta struct {
	Filename string       `json:"filename"`
	DocType  DocumentType `json:"doc_type"`
	Password string       `json:"password,omitempty"`
}

type UploadMetadata struct {
	Documents []DocumentMeta `json:"documents"`
}

type DocumentQuality struct {
	ResolutionScore float64  `json:"resolution_score"`
	OcrConfidence   float64  `json:"ocr_confidence"`
	ContrastScore   float64  `json:"contrast_score"`
	FinalScore      float64  `json:"final_score"`
	Issues          []string `json:"issues"`
}

type SalarySlipData struct {
	EmployeeName  string          `json:"employee_name"`
	EmployerName  string          `json:"employer_name"`
	Designation   string          `json:"designation,omitempty"`
	Department    string          `json:"department,omitempty"`
	JoiningDate   *time.Time      `json:"joining_date,omitempty"`
	PayMonth      string          `json:"pay_month"` // "YYYY-MM"
	NetSalary     float64         `json:"net_salary"`
	AccountNumber string          `json:"account_number,omitempty"`
	IFSC          string          `json:"ifsc,omitempty"`
	Quality       DocumentQuality `json:"quality"`
}

type BankTransaction struct {
	Date        time.Time `json:"date"`
	Description string    `json:"description"`
	Amount      float64   `json:"amount"`
	IsCredit    bool      `json:"is_credit"`
	Balance     float64   `json:"balance,omitempty"`
	RawLine     string    `json:"raw_line,omitempty"`
}

type BankStatementData struct {
	AccountHolderName string            `json:"account_holder_name"`
	AccountNumber     string            `json:"account_number"`
	BankName          string            `json:"bank_name,omitempty"`
	IFSC              string            `json:"ifsc,omitempty"`
	PeriodFrom        *time.Time        `json:"period_from,omitempty"`
	PeriodTo          *time.Time        `json:"period_to,omitempty"`
	Transactions      []BankTransaction `json:"transactions"`
	Quality           DocumentQuality   `json:"quality"`
}

type CrossCheckResult struct {
	NameMatch            bool     `json:"name_match"`
	NameSimilarity       float64  `json:"name_similarity"`
	AccountMatch         bool     `json:"account_match"`
	MissingSalaryCredits []string `json:"missing_salary_credits"`
	Notes                []string `json:"notes"`
}

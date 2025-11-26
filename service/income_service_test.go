package service

import (
	"testing"

	"github.com/Aashish23092/ocr-income-verification/dto"
	"github.com/stretchr/testify/assert"
)

func TestCrossCheck(t *testing.T) {
	service := &IncomeService{}

	slips := []dto.SalarySlipData{
		{
			EmployeeName:  "John Doe",
			AccountNumber: "1234567890",
			NetSalary:     50000.00,
			PayMonth:      "October 2025",
		},
	}

	stmts := []dto.BankStatementData{
		{
			AccountHolderName: "John Doe",
			AccountNumber:     "1234567890",
			Transactions: []dto.BankTransaction{
				{
					IsCredit:    true,
					Amount:      50000.00,
					Description: "SALARY CREDIT",
				},
			},
		},
	}

	result := service.CrossCheck(slips, stmts)

	assert.True(t, result.NameMatch)
	assert.True(t, result.AccountMatch)
	assert.Empty(t, result.MissingSalaryCredits)
}

func TestCrossCheckMismatch(t *testing.T) {
	service := &IncomeService{}

	slips := []dto.SalarySlipData{
		{
			EmployeeName:  "John Doe",
			AccountNumber: "1234567890",
			NetSalary:     50000.00,
			PayMonth:      "October 2025",
		},
	}

	stmts := []dto.BankStatementData{
		{
			AccountHolderName: "Jane Doe",
			AccountNumber:     "0987654321",
			Transactions: []dto.BankTransaction{
				{
					IsCredit:    true,
					Amount:      40000.00,
					Description: "SALARY CREDIT",
				},
			},
		},
	}

	result := service.CrossCheck(slips, stmts)

	assert.False(t, result.NameMatch)
	assert.False(t, result.AccountMatch)
	assert.NotEmpty(t, result.MissingSalaryCredits)
}

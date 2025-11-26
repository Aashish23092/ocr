package utils

import (
	"testing"

	"github.com/Aashish23092/ocr-income-verification/dto"
	"github.com/stretchr/testify/assert"
)

func TestParseSalarySlip(t *testing.T) {
	text := `
		ABC Corp Ltd.
		Employee Name: John Doe
		Pay Slip for October 2025
		Account No: 1234567890
		Net Salary: Rs. 50,000.00
	`

	data := ParseSalarySlip(text)

	assert.Equal(t, "John Doe", data.EmployeeName)
	assert.Equal(t, "October 2025", data.PayMonth)
	assert.Equal(t, "1234567890", data.AccountNumber)
	assert.Equal(t, 50000.00, data.NetSalary)
}

func TestParseBankStatement(t *testing.T) {
	text := `
		HDFC Bank
		Account Holder: John Doe
		Account Number: 1234567890
		Date        Description             Amount
		15/10/2025  SALARY CREDIT           50,000.00
		20/10/2025  UPI PAYMENT             -500.00
	`

	data := ParseBankStatement(text)

	assert.Equal(t, "John Doe", data.AccountHolderName)
	assert.Equal(t, "1234567890", data.AccountNumber)
	assert.Equal(t, 1, len(data.Transactions))
	assert.Equal(t, 50000.00, data.Transactions[0].Amount)
	assert.Equal(t, "SALARY CREDIT", data.Transactions[0].Description)
}

func TestCompareNames(t *testing.T) {
	assert.True(t, CompareNames("John Doe", "John Doe"))
	assert.True(t, CompareNames("John Doe", "MR JOHN DOE"))
	assert.True(t, CompareNames("John Doe", "Doe John"))
	assert.False(t, CompareNames("John Doe", "Jane Doe"))
}

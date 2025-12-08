package service

import (
	"errors"
	"log"
	"strings"

	"github.com/Aashish23092/ocr-income-verification/dto"

	appointmentletter "github.com/Aashish23092/ocr-income-verification/utils/appointmentletter"
	employeeid "github.com/Aashish23092/ocr-income-verification/utils/employeeid"
)

type PaddleOCR interface {
	ExtractText([]byte) (string, error)
}

type EmployeeService struct {
	ocr PaddleOCR
}

func NewEmployeeService(ocr PaddleOCR) *EmployeeService {
	return &EmployeeService{ocr: ocr}
}

func (s *EmployeeService) ProcessEmployeeDocs(empCard, appLetter []byte) (*dto.EmployeeVerifyResponse, error) {

	// ------------------------
	// OCR Employee ID Card
	// ------------------------
	empText, err := s.ocr.ExtractText(empCard)
	if err != nil {
		return nil, errors.New("failed to OCR employee ID card")
	}
	log.Println("---- EMPLOYEE ID OCR TEXT ----")
	log.Println(empText)

	// ------------------------
	// OCR Appointment Letter
	// ------------------------
	appText, err := s.ocr.ExtractText(appLetter)
	if err != nil {
		return nil, errors.New("failed to OCR appointment letter")
	}

	// 🔥 ADD THIS
	log.Println("---- APPOINTMENT LETTER OCR TEXT ----")
	log.Println(appText)

	// ------------------------
	// Parse Employee ID Card
	// ------------------------
	empData := dto.EmployeeIDInfo{
		Name:        employeeid.ParseNameID(empText),
		EmployeeID:  employeeid.ParseEmployeeID(empText),
		Company:     employeeid.ParseCompanyID(empText),
		Designation: employeeid.ParseDesignationID(empText),
	}

	// ------------------------
	// Parse Appointment Letter
	// ------------------------
	appData := dto.AppointmentLetterInfo{
		Name:        appointmentletter.ParseNameLetter(appText),
		Company:     appointmentletter.ParseCompanyLetter(appText),
		Designation: appointmentletter.ParseDesignationLetter(appText),
		JoiningDate: appointmentletter.ParseJoiningDate(appText),
		Location:    appointmentletter.ParseLocationLetter(appText),
	}

	// ------------------------
	// Validation
	// ------------------------
	validation := dto.ValidationResult{
		NameMatch:    strings.EqualFold(empData.Name, appData.Name),
		CompanyMatch: strings.EqualFold(empData.Company, appData.Company),
	}

	// ------------------------
	// Final Response
	// ------------------------
	resp := dto.EmployeeVerifyResponse{
		EmployeeIDData:        empData,
		AppointmentLetterData: appData,
		Validation:            validation,
	}

	return &resp, nil
}

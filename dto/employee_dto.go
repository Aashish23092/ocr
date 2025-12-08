package dto

type EmployeeIDInfo struct {
	Name        string `json:"name"`
	EmployeeID  string `json:"employee_id"`
	Company     string `json:"company_name"`
	Designation string `json:"designation"`
}

type AppointmentLetterInfo struct {
	Name        string `json:"name"`
	Company     string `json:"company_name"`
	Designation string `json:"designation"`
	JoiningDate string `json:"joining_date"`
	Location    string `json:"location"`
}

type EmployeeVerifyResponse struct {
	EmployeeIDData        EmployeeIDInfo        `json:"employee_id_data"`
	AppointmentLetterData AppointmentLetterInfo `json:"appointment_letter_data"`
	Validation            ValidationResult      `json:"validation"`
}

type ValidationResult struct {
	NameMatch    bool `json:"name_match"`
	CompanyMatch bool `json:"company_match"`
}

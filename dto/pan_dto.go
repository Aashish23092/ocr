package dto

type PANResponse struct {
	PAN        string `json:"pan"`
	Name       string `json:"name"`
	FatherName string `json:"father_name"`
	DOB        string `json:"dob"`
	RawText    string `json:"raw_text"`
}

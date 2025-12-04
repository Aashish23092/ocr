package service

import (
	"github.com/Aashish23092/ocr-income-verification/client"
	"github.com/Aashish23092/ocr-income-verification/dto"
	"github.com/Aashish23092/ocr-income-verification/utils"
)

type PANService struct {
	Paddle *client.PaddleClient
}

func NewPANService(paddle *client.PaddleClient) *PANService {
	return &PANService{
		Paddle: paddle,
	}
}

func (s *PANService) ExtractPANData(imagePath string) (*dto.PANResponse, error) {

	// -----------------------------
	// USE YOUR ACTUAL PADDLE CLIENT
	// -----------------------------
	rawText, err := s.Paddle.ExtractTextFromFile(imagePath)
	if err != nil {
		return nil, err
	}

	parsed := utils.ParsePANText(rawText)

	return &dto.PANResponse{
		PAN:        parsed.PAN,
		Name:       parsed.Name,
		FatherName: parsed.FatherName,
		DOB:        parsed.DOB,
		RawText:    parsed.RawText,
	}, nil
}

package client

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
)

type PaddleClient struct {
	URL string
}

func NewPaddleClient() (*PaddleClient, error) {
	url := os.Getenv("PADDLE_OCR_URL")
	if url == "" {
		url = "http://paddle:8866/ocr"
	}
	return &PaddleClient{URL: url}, nil
}

func (p *PaddleClient) ExtractText(imageBytes []byte) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", "upload.jpg")
	if err != nil {
		return "", err
	}
	part.Write(imageBytes)
	writer.Close()

	req, err := http.NewRequest("POST", p.URL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var out struct {
		Text string `json:"text"`
	}
	json.NewDecoder(resp.Body).Decode(&out)

	return out.Text, nil
}

func (p *PaddleClient) ExtractTextFromFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return p.ExtractText(b)
}

func (p *PaddleClient) ExtractTextFromImageBytes(img []byte) (string, error) {
	return p.ExtractText(img)
}

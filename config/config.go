package config

import "os"

type Config struct {
	ServerPort        string
	TesseractDataPath string
	MaxFileSize       int64
}

func LoadConfig() *Config {
	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8080"
	}

	tesseractDataPath := os.Getenv("TESSDATA_PREFIX")
	if tesseractDataPath == "" {
		tesseractDataPath = "/usr/share/tesseract-ocr/4.00/tessdata"
	}

	return &Config{
		ServerPort:        serverPort,
		TesseractDataPath: tesseractDataPath,
		MaxFileSize:       10 * 1024 * 1024, // 10 MB
	}
}
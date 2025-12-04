package main

import (
	"log"
	"os"

	"github.com/Aashish23092/ocr-income-verification/client"
	"github.com/Aashish23092/ocr-income-verification/config"
	"github.com/Aashish23092/ocr-income-verification/handler"
	"github.com/Aashish23092/ocr-income-verification/service"

	"github.com/gin-gonic/gin"
)

func main() {

	// Tesseract configuration
	os.Setenv("TESSDATA_PREFIX", "/usr/share/tesseract-ocr/5/tessdata/")
	log.Println("TESSDATA_PREFIX set to:", os.Getenv("TESSDATA_PREFIX"))

	// Load application config
	cfg := config.LoadConfig()

	// Initialize Tesseract client
	tesseractClient := client.NewTesseractClient(cfg.TesseractDataPath)
	defer tesseractClient.Close()

	// Initialize PDF processor
	pdfProcessor := service.NewPDFProcessor()

	// ------------------------------------------
	// ⭐ Initialize PaddleOCR Client
	// ------------------------------------------
	paddleClient, err := client.NewPaddleClient()
	if err != nil {
		log.Printf("WARNING: PaddleOCR client could not initialize: %v", err)
		paddleClient = nil
	} else {
		log.Println("PaddleOCR client initialized successfully")
	}

	// ------------------------------------------
	// Income Service
	// ------------------------------------------
	incomeService := service.NewIncomeService(
		tesseractClient,
		pdfProcessor,
		paddleClient,
	)
	incomeHandler := handler.NewIncomeHandler(incomeService)

	// ------------------------------------------
	// Aadhaar Service
	// ------------------------------------------
	aadhaarService := service.NewAadhaarService(tesseractClient, pdfProcessor)
	aadhaarHandler := handler.NewAadhaarHandler(aadhaarService)

	// ------------------------------------------
	// PAN OCR Service + Handler
	// ------------------------------------------
	panService := service.NewPANService(paddleClient)
	panHandler := handler.NewPANHandler(panService)

	// ------------------------------------------
	// Gin Router
	// ------------------------------------------
	router := gin.Default()
	router.MaxMultipartMemory = 32 << 20

	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "OCR Income Verification",
		})
	})

	api := router.Group("/api/v1")
	{
		// Income
		income := api.Group("/income")
		{
			income.POST("/verify", incomeHandler.VerifyIncome)
		}

		// ITR
		itr := api.Group("/itr")
		{
			itr.POST("/analyze", incomeHandler.AnalyzeITR)
		}

		// Aadhaar
		aadhaar := api.Group("/aadhaar")
		{
			aadhaar.POST("/extract", aadhaarHandler.ExtractAadhaar)
		}

		//  PAN OCR API
		pan := api.Group("/pan")
		{
			pan.POST("/ocr", panHandler.ExtractPAN)
		}
	}

	log.Printf("Starting OCR Income Verification Service on port %s", cfg.ServerPort)
	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

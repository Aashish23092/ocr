package main

import (
	"log"
	"os" // <-- IMPORTANT
	"github.com/Aashish23092/ocr-income-verification/client"
	"github.com/Aashish23092/ocr-income-verification/config"
	"github.com/Aashish23092/ocr-income-verification/handler"
	"github.com/Aashish23092/ocr-income-verification/service"

	"github.com/gin-gonic/gin"
)

func main() {
	// VERY IMPORTANT: Set correct tessdata prefix for Tesseract v5
	os.Setenv("TESSDATA_PREFIX", "/usr/share/tesseract-ocr/5/tessdata/")
	log.Println("TESSDATA_PREFIX set to:", os.Getenv("TESSDATA_PREFIX"))

	// Initialize configuration
	cfg := config.LoadConfig()

	// Initialize Tesseract client
	tesseractClient := client.NewTesseractClient(cfg.TesseractDataPath)
	defer tesseractClient.Close()

	// Initialize PDF processor
	pdfProcessor := service.NewPDFProcessor()

	// Initialize service layer
	incomeService := service.NewIncomeService(tesseractClient, pdfProcessor)

	// Initialize handler layer
	incomeHandler := handler.NewIncomeHandler(incomeService)

	// Setup Gin router
	router := gin.Default()

	// Configure max multipart memory (32 MB)
	router.MaxMultipartMemory = 32 << 20

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "healthy",
			"service": "OCR Income Verification",
		})
	})

	// API routes
	api := router.Group("/api/v1")
	{
		income := api.Group("/income")
		{
			income.POST("/verify", incomeHandler.VerifyIncome)
		}
	}

	// Start server
	log.Printf("Starting OCR Income Verification Service on port %s", cfg.ServerPort)
	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

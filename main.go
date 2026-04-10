package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "media-encoder-service/docs"
	"media-encoder-service/handlers"
	"media-encoder-service/middleware"
)

// @title           Media Encoder Service API
// @version         1.0
// @description     This is an internal evidence processing microservice for Foxy Exam.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.email  support@foxyexam.com

// @license.name  Unlicense
// @license.url   http://unlicense.org

// @host      localhost:8097
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func main() {
	// Load .env (optional — Docker/env vars override)
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8097"
	}

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("MEDIA_ENCODER_SERVICE_API_KEY")
	}
	if apiKey == "" {
		log.Fatal("❌ API_KEY or MEDIA_ENCODER_SERVICE_API_KEY must be set")
	}

	// ─── Gin setup ─────────────────────────────────────────────
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// ─── Public routes ─────────────────────────────────────────
	r.GET("/health", handlers.Health)

	// ─── Protected routes ──────────────────────────────────────
	api := r.Group("/api", middleware.BearerAuth(apiKey))
	{
		// Raw FFmpeg execution
		api.POST("/ffmpeg/run", handlers.FfmpegRun)

		// Probe file duration (ms)
		api.POST("/ffmpeg/probe", handlers.FfmpegProbe)

		// High-level: stitch multiple chunks → single MP4
		api.POST("/stitch", handlers.Stitch)

		// High-level: extract JPEG snapshot at offset
		api.POST("/snapshot", handlers.Snapshot)

		// High-level: extract MP4 clip with watermark
		api.POST("/clip", handlers.Clip)

		// Encoder detection (GPU/CPU)
		api.GET("/encoder", handlers.DetectEncoder)
	}

	// ─── Swagger docs ──────────────────────────────────────────
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	log.Printf("🚀 Media Encoder Service listening on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

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

func main() {
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

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/health", handlers.Health)

	api := r.Group("/api", middleware.BearerAuth(apiKey))
	{
		api.POST("/ffmpeg/run", handlers.FfmpegRun)

		api.POST("/ffmpeg/probe", handlers.FfmpegProbe)

		api.POST("/stitch", handlers.Stitch)

		api.POST("/snapshot", handlers.Snapshot)

		api.POST("/clip", handlers.Clip)

		api.GET("/encoder", handlers.DetectEncoder)
	}

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	log.Printf("🚀 Media Encoder Service listening on :%s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

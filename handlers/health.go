package handlers

import (
	"net/http"
	"os/exec"
	"runtime"

	"github.com/gin-gonic/gin"
)

// Health returns service status and system info.
func Health(c *gin.Context) {
	ffmpegPath, ffmpegErr := exec.LookPath("ffmpeg")
	ffprobeOk := false
	if _, err := exec.LookPath("ffprobe"); err == nil {
		ffprobeOk = true
	}

	status := "ok"
	if ffmpegErr != nil {
		status = "degraded — ffmpeg not found in PATH"
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  status,
		"service": "media-encoder-service",
		"version": "1.0.0",
		"runtime": gin.H{
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
			"cpus":       runtime.NumCPU(),
		},
		"ffmpeg": gin.H{
			"path":     ffmpegPath,
			"available": ffmpegErr == nil,
			"ffprobe":  ffprobeOk,
		},
	})
}

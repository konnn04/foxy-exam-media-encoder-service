package handlers

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"media-encoder-service/services"
)

// FfmpegRunRequest represents a raw FFmpeg execution request.
type FfmpegRunRequest struct {
	Args       []string `json:"args" binding:"required"`
	TimeoutSec int      `json:"timeout_sec"`
}

// FfmpegRun executes arbitrary FFmpeg commands.
// POST /api/ffmpeg/run
func FfmpegRun(c *gin.Context) {
	var req FfmpegRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result := services.RunFFmpeg(req.Args, req.TimeoutSec)
	c.JSON(http.StatusOK, result)
}

// FfmpegProbeRequest represents a probe request.
type FfmpegProbeRequest struct {
	FilePath   string `json:"file_path" binding:"required"`
	TimeoutSec int    `json:"timeout_sec"`
}

// FfmpegProbe returns the duration (ms) of a media file.
// POST /api/ffmpeg/probe
func FfmpegProbe(c *gin.Context) {
	var req FfmpegProbeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	durationMs, err := services.ProbeDurationMs(req.FilePath, req.TimeoutSec)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":     false,
			"duration_ms": 0,
			"error":       err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"duration_ms": durationMs,
	})
}

// DetectEncoder returns the encoder flags after auto-probe (or MEDIA_ENCODER_VIDEO override).
// GET /api/encoder
func DetectEncoder(c *gin.Context) {
	includeBitrate := c.DefaultQuery("bitrate", "true") == "true"
	flags := services.DetectBestEncoder(includeBitrate)
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("MEDIA_ENCODER_VIDEO")))
	if mode == "" {
		mode = "auto"
	}

	c.JSON(http.StatusOK, gin.H{
		"encoder_flags": flags,
		"video_mode":    mode,
		"uses_hardware": strings.Contains(flags, "nvenc") || strings.Contains(flags, "qsv"),
	})
}

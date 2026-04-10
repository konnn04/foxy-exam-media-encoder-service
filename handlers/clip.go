package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"media-encoder-service/services"
)

// ClipRequest represents an extract-clip request.
type ClipRequest struct {
	// Source media file (absolute path)
	SourceFile string `json:"source_file" binding:"required"`
	// Start offset in seconds
	StartSec float64 `json:"start_sec"`
	// Duration in seconds (default: 15)
	DurationSec int `json:"duration_sec"`
	// Watermark text overlay (optional)
	Watermark string `json:"watermark"`
	// Output MP4 file (absolute path)
	OutputFile string `json:"output_file" binding:"required"`
	// Include audio track (default: false)
	IncludeAudio bool `json:"include_audio"`
	// Timeout in seconds
	TimeoutSec int `json:"timeout_sec"`
}

// Clip extracts an MP4 clip with optional watermark text overlay.
// POST /api/clip
func Clip(c *gin.Context) {
	var req ClipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := os.Stat(req.SourceFile); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source file not found"})
		return
	}

	duration := 15
	if req.DurationSec > 0 {
		duration = req.DurationSec
	}
	timeout := 120
	if req.TimeoutSec > 0 {
		timeout = req.TimeoutSec
	}

	// Ensure output directory
	if err := os.MkdirAll(filepath.Dir(req.OutputFile), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create output dir"})
		return
	}

	encoderFlags := services.DetectBestEncoder(true)
	encoderArgs := strings.Fields(encoderFlags)

	// -ss after -i: accurate clip; +faststart: web-friendly MP4 (duration + quick start).
	args := []string{
		"-y",
		"-analyzeduration", "5M",
		"-probesize", "5M",
		"-i", req.SourceFile,
		"-ss", formatSeconds(req.StartSec),
		"-t", fmt.Sprintf("%d", duration),
	}

	// Build video filter with optional watermark
	if req.Watermark != "" {
		escaped := strings.ReplaceAll(req.Watermark, "\\", "\\\\")
		escaped = strings.ReplaceAll(escaped, ":", "\\:")
		escaped = strings.ReplaceAll(escaped, "'", "\\'")
		vf := fmt.Sprintf("drawtext=text='%s':fontsize=18:fontcolor=white:borderw=2:bordercolor=black:x=10:y=10", escaped)
		args = append(args, "-vf", vf)
	}

	args = append(args, encoderArgs...)

	if !req.IncludeAudio {
		args = append(args, "-an")
	} else {
		args = append(args, "-c:a", "aac")
	}

	args = append(args, "-movflags", "+faststart", req.OutputFile)

	result := services.RunFFmpeg(args, timeout)

	info, statErr := os.Stat(req.OutputFile)
	outputValid := statErr == nil && info.Size() > 0

	var durationMs int64
	if outputValid {
		durationMs, _ = services.ProbeDurationMs(req.OutputFile, 15)
	}
	if outputValid && durationMs < 800 {
		_ = os.Remove(req.OutputFile)
		outputValid = false
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     result.Success && outputValid,
		"output_file": req.OutputFile,
		"output_size": safeClipSize(req.OutputFile),
		"duration_ms": durationMs,
	})
}

func formatSeconds(sec float64) string {
	return fmt.Sprintf("%.3f", sec)
}

func safeClipSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

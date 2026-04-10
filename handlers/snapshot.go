package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"media-encoder-service/services"
)

// SnapshotRequest represents an extract-snapshot request.
type SnapshotRequest struct {
	// Source media file (absolute path)
	SourceFile string `json:"source_file" binding:"required"`
	// Offset in seconds from the start of the file
	OffsetSec float64 `json:"offset_sec"`
	// Output JPEG file (absolute path)
	OutputFile string `json:"output_file" binding:"required"`
	// Timeout in seconds
	TimeoutSec int `json:"timeout_sec"`
}

// Snapshot extracts a single JPEG frame from a video at the given offset.
// POST /api/snapshot
func Snapshot(c *gin.Context) {
	var req SnapshotRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := os.Stat(req.SourceFile); os.IsNotExist(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source file not found: " + req.SourceFile})
		return
	}

	timeout := 30
	if req.TimeoutSec > 0 {
		timeout = req.TimeoutSec
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(req.OutputFile), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create output dir"})
		return
	}

	args := []string{
		"-y",
		"-analyzeduration", "5M",
		"-probesize", "5M",
		"-ss", formatSeconds(req.OffsetSec),
		"-i", req.SourceFile,
		"-vframes", "1",
		"-q:v", "2",
		req.OutputFile,
	}

	result := services.RunFFmpeg(args, timeout)

	info, statErr := os.Stat(req.OutputFile)
	outputValid := statErr == nil && info.Size() > 0

	c.JSON(http.StatusOK, gin.H{
		"success":     result.Success && outputValid,
		"output_file": req.OutputFile,
		"output_size": safeSize(req.OutputFile),
	})
}

func safeSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

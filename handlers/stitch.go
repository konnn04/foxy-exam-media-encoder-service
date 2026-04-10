package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"media-encoder-service/services"
)

// StitchRequest represents a request to stitch video chunks into one MP4.
type StitchRequest struct {
	InputFiles []string `json:"input_files" binding:"required,min=1"`
	OutputFile string   `json:"output_file" binding:"required"`
	Scale      string   `json:"scale"`
	TimeoutSec int      `json:"timeout_sec"`
	// When true (default), insert black reconnect placeholders if wall-clock gap between chunks > 2s.
	InsertReconnectGaps *bool `json:"insert_reconnect_gaps"`
}

type encodedPiece struct {
	mp4Path   string
	inputPath string
}

// Stitch re-encodes and concatenates multiple video chunks into a single MP4.
// POST /api/stitch
//
// Flow: re-encode each chunk → optional gap MP4 between chunks (reconnect timeline) → concat → output
func Stitch(c *gin.Context) {
	var req StitchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Scale == "" {
		req.Scale = "1280:720"
	}
	chunkTimeout := 600
	if req.TimeoutSec > 0 {
		chunkTimeout = req.TimeoutSec
	}
	insertGaps := true
	if req.InsertReconnectGaps != nil {
		insertGaps = *req.InsertReconnectGaps
	}

	workDir, err := os.MkdirTemp("", "stitch-*")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create temp dir: " + err.Error()})
		return
	}
	defer os.RemoveAll(workDir)

	encoderFlags := services.DetectBestEncoder(false)
	encoderArgs := strings.Fields(encoderFlags)

	var encoded []encodedPiece
	for i, inputFile := range req.InputFiles {
		if _, err := os.Stat(inputFile); os.IsNotExist(err) {
			continue
		}

		mp4Path := filepath.Join(workDir, fmt.Sprintf("chunk_%d.mp4", i))

		args := []string{
			"-y",
			"-analyzeduration", "5M",
			"-probesize", "5M",
			"-i", inputFile,
			"-vf", fmt.Sprintf("scale=%s:force_original_aspect_ratio=decrease,pad=%s:(ow-iw)/2:(oh-ih)/2", req.Scale, req.Scale),
		}
		args = append(args, encoderArgs...)
		args = append(args, "-c:a", "aac",
			"-minrate", "500k", "-maxrate", "1500k", "-bufsize", "2000k",
			mp4Path,
		)

		result := services.RunFFmpeg(args, chunkTimeout)
		if result.Success {
			encoded = append(encoded, encodedPiece{mp4Path: mp4Path, inputPath: inputFile})
		} else {
			log.Printf("[stitch] chunk %d encode failed (%s): %s", i, filepath.Base(inputFile), truncateStr(result.Output, 400))
		}
	}

	if len(encoded) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   "no chunks were re-encoded successfully",
		})
		return
	}

	var concatPaths []string
	gapTimeout := chunkTimeout
	if gapTimeout > 120 {
		gapTimeout = 120
	}

	for i := 0; i < len(encoded); i++ {
		concatPaths = append(concatPaths, encoded[i].mp4Path)

		if !insertGaps || i >= len(encoded)-1 {
			continue
		}

		cur := encoded[i]
		next := encoded[i+1]
		tsC := services.FirstEmbeddedUnixTimestamp(cur.inputPath)
		tsN := services.FirstEmbeddedUnixTimestamp(next.inputPath)
		if tsC == 0 || tsN == 0 {
			continue
		}

		durMs, err := services.ProbeDurationMs(cur.mp4Path, 15)
		if err != nil || durMs <= 0 {
			continue
		}
		durSec := float64(durMs) / 1000.0
		gap := float64(tsN) - (float64(tsC) + durSec)
		if gap <= 2 {
			continue
		}

		gapPath := filepath.Join(workDir, fmt.Sprintf("gap_%d.mp4", i))
		if services.BuildReconnectGapMP4(gapPath, gap, req.Scale, encoderArgs, gapTimeout) {
			concatPaths = append(concatPaths, gapPath)
		}
	}

	concatFile := filepath.Join(workDir, "concat.txt")
	if err := services.WriteConcatFile(concatPaths, concatFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write concat file: " + err.Error()})
		return
	}

	outputDir := filepath.Dir(req.OutputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create output dir"})
		return
	}

	concatArgs := []string{
		"-y",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-c", "copy",
		req.OutputFile,
	}

	result := services.RunFFmpeg(concatArgs, chunkTimeout)

	info, statErr := os.Stat(req.OutputFile)
	outputValid := statErr == nil && info.Size() > 1000

	c.JSON(http.StatusOK, gin.H{
		"success":         result.Success && outputValid,
		"output_file":     req.OutputFile,
		"output_size":     safeFileSize(req.OutputFile),
		"chunks_total":    len(req.InputFiles),
		"chunks_ok":       len(encoded),
		"concat_segments": len(concatPaths),
		"ffmpeg_output":   result.Output,
	})
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

func safeFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

package services

import (
	"fmt"
	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var embeddedTimestampRe = regexp.MustCompile(`\d{10,}`)

// FirstEmbeddedUnixTimestamp returns the first 10+ digit run in a filename (egress file naming).
func FirstEmbeddedUnixTimestamp(filePath string) int64 {
	base := filepath.Base(filePath)
	m := embeddedTimestampRe.FindString(base)
	if m == "" {
		return 0
	}
	v, err := strconv.ParseInt(m, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// ParseScale splits "1280:720" into width and height.
func ParseScale(scale string) (w, h int) {
	parts := strings.Split(scale, ":")
	if len(parts) != 2 {
		return 1280, 720
	}
	w, _ = strconv.Atoi(strings.TrimSpace(parts[0]))
	h, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	if w <= 0 || h <= 0 {
		return 1280, 720
	}
	return w, h
}

// BuildReconnectGapMP4 creates a black placeholder MP4 of gapSec seconds (audio silence).
// Matches Laravel StitchContinuousVideoJob::buildGapSegment intent when reconnect > 2s.
func BuildReconnectGapMP4(outPath string, gapSec float64, scale string, encoderArgs []string, timeoutSec int) bool {
	if gapSec <= 2 {
		return false
	}
	if gapSec > 3600 {
		gapSec = 3600
	}
	w, h := ParseScale(scale)
	colorSrc := fmt.Sprintf("color=c=black:s=%dx%d:r=30", w, h)
	args := []string{
		"-y",
		"-f", "lavfi", "-i", colorSrc,
		"-f", "lavfi", "-i", "anullsrc=channel_layout=stereo:sample_rate=48000",
		"-t", fmt.Sprintf("%.3f", gapSec),
	}
	args = append(args, encoderArgs...)
	args = append(args, "-c:a", "aac", "-pix_fmt", "yuv420p", outPath)

	res := RunFFmpeg(args, timeoutSec)
	if !res.Success {
		log.Printf("[stitch] gap segment ffmpeg failed: %s", truncate(res.Output, 400))
	}
	return res.Success
}

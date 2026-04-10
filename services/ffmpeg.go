package services

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// RunResult holds the result of an FFmpeg/FFprobe execution.
type RunResult struct {
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	ExitCode int    `json:"exit_code"`
	Duration float64 `json:"duration_sec,omitempty"`
}

// RunFFmpeg executes ffmpeg with the given arguments and a timeout.
func RunFFmpeg(args []string, timeoutSec int) RunResult {
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	elapsed := time.Since(start).Seconds()

	combined := stdout.String() + stderr.String()

	if ctx.Err() == context.DeadlineExceeded {
		log.Printf("[FFmpeg] Process timed out after %ds", timeoutSec)
		return RunResult{
			Success:  false,
			Output:   fmt.Sprintf("Process timed out after %ds\n%s", timeoutSec, truncate(combined, 1000)),
			ExitCode: -1,
			Duration: elapsed,
		}
	}

	if err != nil {
		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		log.Printf("[FFmpeg] Failed (exit=%d): %s", exitCode, truncate(combined, 500))
		return RunResult{
			Success:  false,
			Output:   truncate(combined, 2000),
			ExitCode: exitCode,
			Duration: elapsed,
		}
	}

	return RunResult{
		Success:  true,
		Output:   truncate(combined, 2000),
		ExitCode: 0,
		Duration: elapsed,
	}
}

// ProbeDurationMs runs ffprobe and returns the duration of a file in milliseconds.
func ProbeDurationMs(filepath string, timeoutSec int) (int64, error) {
	if timeoutSec <= 0 {
		timeoutSec = 15
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		filepath,
	)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	raw := strings.TrimSpace(stdout.String())
	if raw == "" || raw == "N/A" {
		return 0, fmt.Errorf("ffprobe returned empty duration")
	}

	secs, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse duration '%s': %w", raw, err)
	}

	return int64(secs * 1000), nil
}

// env MEDIA_ENCODER_VIDEO: auto (default) | cpu | nvenc | qsv
// env MEDIA_ENCODER_CPU_PRESET: ultrafast | veryfast | fast | medium (default veryfast)

var (
	autoEncoderOnce    sync.Once
	autoEncoderNoBit   string
	autoEncoderWithBit string
)

// DetectBestEncoder picks an encoder that works on this host (probe), not merely listed in ffmpeg -encoders.
// Docker/Alpine often lists h264_qsv but has no /dev/dri — previous logic logged "QSV" yet every real encode failed instantly.
func DetectBestEncoder(includeBitrate bool) string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("MEDIA_ENCODER_VIDEO")))
	if mode == "" {
		mode = "auto"
	}

	switch mode {
	case "cpu", "libx264":
		flags := libx264Encoder(includeBitrate)
		log.Println("[Encoder] Forced CPU (MEDIA_ENCODER_VIDEO=cpu):", flags)
		return flags
	case "nvenc", "h264_nvenc":
		flags := nvencEncoder(includeBitrate)
		log.Println("[Encoder] Forced NVENC:", flags)
		return flags
	case "qsv", "h264_qsv":
		flags := qsvEncoder(includeBitrate)
		log.Println("[Encoder] Forced QSV:", flags)
		return flags
	default:
		autoEncoderOnce.Do(resolveAutoEncoders)
		if includeBitrate {
			return autoEncoderWithBit
		}
		return autoEncoderNoBit
	}
}

func resolveAutoEncoders() {
	cpuNo := libx264Encoder(false)
	cpuWith := libx264Encoder(true)

	candidates := []struct {
		name string
		no   string
		with string
	}{
		{"h264_nvenc", nvencEncoder(false), nvencEncoder(true)},
		{"h264_qsv", qsvEncoder(false), qsvEncoder(true)},
	}

	for _, c := range candidates {
		if probeHardwareEncode(c.no) {
			log.Printf("[Encoder] Auto: using %s (probe encode OK)", c.name)
			autoEncoderNoBit = c.no
			autoEncoderWithBit = c.with
			return
		}
		log.Printf("[Encoder] Auto: %s not usable here (no GPU device / driver in container?)", c.name)
	}

	log.Println("[Encoder] Auto: libx264 CPU — set MEDIA_ENCODER_VIDEO=nvenc on NVIDIA host or pass /dev/dri for QSV")
	autoEncoderNoBit = cpuNo
	autoEncoderWithBit = cpuWith
}

func probeHardwareEncode(encoderFlagLine string) bool {
	args := strings.Fields(encoderFlagLine)
	full := []string{
		"-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=duration=0.06:size=320x240:rate=30",
	}
	full = append(full, args...)
	full = append(full, "-frames:v", "2", "-f", "null", "-")
	return RunFFmpeg(full, 30).Success
}

func cpuPreset() string {
	p := strings.ToLower(strings.TrimSpace(os.Getenv("MEDIA_ENCODER_CPU_PRESET")))
	switch p {
	case "ultrafast", "veryfast", "fast", "medium":
		return p
	default:
		return "veryfast"
	}
}

func libx264Encoder(includeBitrate bool) string {
	preset := cpuPreset()
	if includeBitrate {
		return fmt.Sprintf("-c:v libx264 -preset %s -b:v 1500k -maxrate 2000k -bufsize 3000k", preset)
	}
	return fmt.Sprintf("-c:v libx264 -preset %s -crf 26", preset)
}

func nvencEncoder(includeBitrate bool) string {
	if includeBitrate {
		return "-c:v h264_nvenc -preset p4 -b:v 1500k"
	}
	return "-c:v h264_nvenc -preset p4"
}

func qsvEncoder(includeBitrate bool) string {
	if includeBitrate {
		return "-c:v h264_qsv -preset fast -b:v 1500k"
	}
	return "-c:v h264_qsv -preset fast"
}

// WriteConcatFile creates an ffmpeg concat demuxer file with absolute paths.
func WriteConcatFile(paths []string, outputFile string) error {
	var lines []string
	for _, p := range paths {
		escaped := strings.ReplaceAll(p, "'", "'\\''")
		lines = append(lines, fmt.Sprintf("file '%s'", escaped))
	}
	content := strings.Join(lines, "\n") + "\n"

	return writeFile(outputFile, []byte(content))
}

// ─── Helpers ───────────────────────────────────────────────────

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[len(s)-maxLen:]
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

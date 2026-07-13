package ffmpeg

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// vmafScoreRegex matches lines like "[VMAF] score: 95.34" or "VMAF: 95.34"
// ffmpeg's libvmaf filter prints the score to stderr at the end of processing.
var vmafScoreRegex = regexp.MustCompile(`(?:\[VMAF\]|VMAF)(?:\s+score)?[:\s]*(\d+\.?\d*)`)

// VMAFScore computes the VMAF score between two video files using ffmpeg's libvmaf filter.
// higher scores indicate better quality (100 = lossless). Returns the score and an error
// if the computation fails (e.g., libvmaf not available, input files unreadable).
//
// The distorted (transcoded) file is compared against the reference (original) file.
// Both inputs are normalized with setpts=PTS-STARTPTS to avoid timestamp issues.
func (t *Transcoder) VMAFScore(ctx context.Context, distortedPath, referencePath string) (float64, error) {
	// Build the filtergraph: normalize timestamps, then compare distorted vs reference
	filter := "[0:v]setpts=PTS-STARTPTS[dist];[1:v]setpts=PTS-STARTPTS[ref];[dist][ref]libvmaf"

	args := []string{
		"-hide_banner",
		"-i", distortedPath,
		"-i", referencePath,
		"-filter_complex", filter,
		"-f", "null",
		"-",
	}

	cmd := exec.CommandContext(ctx, t.ffmpegPath, args...)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 0, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	var lastScore float64
	var found bool

	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if match := vmafScoreRegex.FindStringSubmatch(line); match != nil {
			if score, err := strconv.ParseFloat(match[1], 64); err == nil {
				lastScore = score
				found = true
			}
		}
	}

	if err := cmd.Wait(); err != nil {
		if found {
			// ffmpeg may exit non-zero even when it produces a score (e.g., on some
			// pixel format mismatches that still complete the comparison)
			return lastScore, nil
		}
		return 0, fmt.Errorf("ffmpeg VMAF computation failed: %w", err)
	}

	if !found {
		return 0, fmt.Errorf("VMAF score not found in ffmpeg output - libvmaf may not be available")
	}

	return lastScore, nil
}

// IsVMAFAvailable checks whether the configured ffmpeg binary supports the libvmaf filter.
// Returns true if libvmaf is available, false otherwise.
func (t *Transcoder) IsVMAFAvailable() bool {
	cmd := exec.Command(t.ffmpegPath, "-hide_banner", "-filters")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "libvmaf")
}

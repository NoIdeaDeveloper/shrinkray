package ffmpeg

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestVMAFScoreRegex(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		expect    float64
		wantMatch bool
	}{
		// ffmpeg libvmaf output formats (varies by version)
		{"vmaf bracket", "[VMAF] score: 95.34", 95.34, true},
		{"vmaf colon", "VMAF: 88.50", 88.50, true},
		{"vmaf space", "[VMAF] 99.99", 99.99, true},
		{"vmaf integer", "VMAF: 100", 100, true},
		{"vmaf low", "[VMAF] score: 20.5", 20.5, true},
		// Should not match
		{"no vmaf", "No such file or directory", 0, false},
		{"vmaf in word", "vmafscore.txt", 0, false},
		{"empty", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := vmafScoreRegex.FindStringSubmatch(tt.line)
			if tt.wantMatch {
				if match == nil {
					t.Fatalf("expected regex to match line: %q", tt.line)
				}
				got, err := strconv.ParseFloat(match[1], 64)
				if err != nil {
					t.Fatalf("failed to parse float from %q: %v", match[1], err)
				}
				if got != tt.expect {
					t.Errorf("parsed score = %v, expected %v", got, tt.expect)
				}
			} else {
				if match != nil {
					t.Errorf("expected no match for line %q, but got: %v", tt.line, match)
				}
			}
		})
	}
}

func TestIsVMAFAvailable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	transcoder := NewTranscoder("ffmpeg")
	available := transcoder.IsVMAFAvailable()

	// We can't guarantee VMAF is available in all test environments,
	// so just verify the method runs without panicking and returns a bool.
	t.Logf("IsVMAFAvailable: %v", available)
}

func TestIsVMAFAvailableMissingBinary(t *testing.T) {
	// Point to a non-existent ffmpeg binary
	transcoder := NewTranscoder("/nonexistent/ffmpeg")
	available := transcoder.IsVMAFAvailable()

	if available {
		t.Error("expected IsVMAFAvailable to return false for missing binary")
	}
}

func TestVMAFScore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testFile := filepath.Join(getTestdataPath(), "test_x264.mkv")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", testFile)
	}

	transcoder := NewTranscoder("ffmpeg")

	if !transcoder.IsVMAFAvailable() {
		t.Skip("libvmaf not available in this ffmpeg build")
	}

	// Comparing a file against itself should yield a very high VMAF score (near 100)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	score, err := transcoder.VMAFScore(ctx, testFile, testFile)
	if err != nil {
		t.Fatalf("VMAFScore failed: %v", err)
	}

	t.Logf("VMAF score (self-compare): %.2f", score)

	// A file compared against itself should be near-perfect (>= 95)
	if score < 95.0 {
		t.Errorf("expected VMAF score >= 95.0 for self-comparison, got %.2f", score)
	}
}

func TestVMAFScoreMissingFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	transcoder := NewTranscoder("ffmpeg")

	if !transcoder.IsVMAFAvailable() {
		t.Skip("libvmaf not available in this ffmpeg build")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := transcoder.VMAFScore(ctx, "/nonexistent/distorted.mkv", "/nonexistent/reference.mkv")
	if err == nil {
		t.Error("expected error for missing input files, got nil")
	}

	if !strings.Contains(err.Error(), "failed") && !strings.Contains(err.Error(), "VMAF") {
		t.Errorf("expected error to mention failure, got: %v", err)
	}
}

func TestVMAFScoreContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testFile := filepath.Join(getTestdataPath(), "test_x264.mkv")
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skipf("test file not found: %s", testFile)
	}

	transcoder := NewTranscoder("ffmpeg")

	if !transcoder.IsVMAFAvailable() {
		t.Skip("libvmaf not available in this ffmpeg build")
	}

	// Cancel context immediately - should cause ffmpeg to be killed
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := transcoder.VMAFScore(ctx, testFile, testFile)
	// We expect either an error (context cancelled) or a score (if ffmpeg was fast enough)
	// The key is that it doesn't hang or panic
	if err != nil {
		t.Logf("Got expected error from cancelled context: %v", err)
	} else {
		t.Logf("ffmpeg completed before context cancellation took effect")
	}
}

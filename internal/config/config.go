package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// FeatureFlags controls experimental features for phased rollout
type FeatureFlags struct {
	// VirtualScroll enables virtual scrolling in the UI (render only visible jobs)
	// When OFF: all jobs are rendered to DOM (current behavior)
	// When ON: only visible jobs + overscan are rendered
	VirtualScroll bool `yaml:"virtual_scroll"`

	// DeferredProbing enables streaming job discovery without blocking on ffprobe
	// When OFF: all files are probed before jobs are added (current behavior)
	// When ON: jobs are added as "pending_probe" and probed by workers on demand
	DeferredProbing bool `yaml:"deferred_probing"`

	// PaginatedInit enables paginated SSE init and job API responses
	// When OFF: SSE init sends all jobs at once (current behavior)
	// When ON: SSE init sends first page, frontend lazy-loads more
	PaginatedInit bool `yaml:"paginated_init"`

	// BatchedSSE is already implemented - kept for consistency
	BatchedSSE bool `yaml:"batched_sse"`

	// DeltaProgress is already implemented - kept for consistency
	DeltaProgress bool `yaml:"delta_progress"`
}

// DefaultFeatureFlags returns feature flags with performance features enabled by default
func DefaultFeatureFlags() FeatureFlags {
	return FeatureFlags{
		VirtualScroll:   true,  // Render only visible items for large queues
		DeferredProbing: true,  // Add jobs instantly, probe when worker picks up
		PaginatedInit:   false, // Not implemented yet
		BatchedSSE:      true,  // Batch add events to reduce SSE flood
		DeltaProgress:   true,  // Small progress payloads
	}
}

type Config struct {
	// MediaPath is the root directory to browse for media files
	MediaPath string `yaml:"media_path"`

	// TempPath is where temp files are written during transcoding
	// If empty, temp files go in the same directory as the source
	TempPath string `yaml:"temp_path"`

	// OriginalHandling determines what happens to original files after transcoding
	// Options: "replace" (rename original to .old), "keep" (keep original, new file replaces)
	OriginalHandling string `yaml:"original_handling"`

	// SubtitleHandling controls how unsupported subtitle codecs are handled when outputting MKV.
	// Options: "convert" (convert mov_text to SRT) or "drop" (drop unsupported subtitles).
	SubtitleHandling string `yaml:"subtitle_handling"`

	// Workers is the number of concurrent transcode jobs (default 1)
	Workers int `yaml:"workers"`

	// FFmpegPath is the path to ffmpeg binary (default: "ffmpeg")
	FFmpegPath string `yaml:"ffmpeg_path"`

	// FFprobePath is the path to ffprobe binary (default: "ffprobe")
	FFprobePath string `yaml:"ffprobe_path"`

	// QueueFile is where the job queue is persisted (default: config dir + queue.json)
	QueueFile string `yaml:"queue_file"`

	// HideProcessingTmp controls hiding shrinkray.tmp files from the UI
	HideProcessingTmp bool `yaml:"hide_processing_tmp"`

	// AllowSoftwareFallback controls whether GPU encode failures trigger automatic CPU retry.
	// Default: false (GPU failures fail the job with a clear message).
	// When enabled, Shrinkray will retry failed GPU encodes using CPU, which is slower but may succeed.
	AllowSoftwareFallback bool `yaml:"allow_software_fallback"`

	// QualityHEVC is the CRF value for HEVC encoding (lower = higher quality)
	// 0 = use encoder-specific default
	QualityHEVC int `yaml:"quality_hevc"`

	// QualityAV1 is the CRF value for AV1 encoding (lower = higher quality)
	// 0 = use encoder-specific default
	QualityAV1 int `yaml:"quality_av1"`

	// ScheduleEnabled enables time-based scheduling for transcoding
	ScheduleEnabled bool `yaml:"schedule_enabled"`

	// ScheduleStartHour is when transcoding is allowed to start (0-23, default 22 = 10 PM)
	ScheduleStartHour int `yaml:"schedule_start_hour"`

	// ScheduleEndHour is when transcoding must stop (0-23, default 6 = 6 AM)
	ScheduleEndHour int `yaml:"schedule_end_hour"`

	// KeepLargerFiles keeps transcoded files even if they're larger than the original
	// Useful for users who want codec consistency across their library
	KeepLargerFiles bool `yaml:"keep_larger_files"`

	// LogLevel controls logging verbosity: debug, info, warn, error (default: info)
	LogLevel string `yaml:"log_level"`

	// LayoutDesign controls the UI layout design.
	// Options: "split" (default) or "tabs".
	LayoutDesign string `yaml:"layout_design"`

	// Features contains feature flags for phased rollout of new functionality
	Features FeatureFlags `yaml:"features"`
}

// DefaultConfig returns a config with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		MediaPath:         "/media",
		TempPath:          "",
		OriginalHandling:  "replace",
		SubtitleHandling:  "convert",
		Workers:           1,
		FFmpegPath:        "ffmpeg",
		FFprobePath:       "ffprobe",
		QueueFile:         "",
		QualityHEVC:       0,
		QualityAV1:        0,
		ScheduleEnabled:   false,
		ScheduleStartHour: 22,
		ScheduleEndHour:   6,
		KeepLargerFiles:   false,
		LogLevel:          "info",
		LayoutDesign:      "split",
		Features:          DefaultFeatureFlags(),
	}
}

// Load reads config from a YAML file, applying defaults for missing values
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
		// No config file - use defaults
		applyFeatureFlagEnvOverrides(cfg)
		return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Apply defaults for empty values
	if cfg.FFmpegPath == "" {
		cfg.FFmpegPath = "ffmpeg"
	}
	if cfg.FFprobePath == "" {
		cfg.FFprobePath = "ffprobe"
	}
	if cfg.Workers < 1 {
		cfg.Workers = 1
	}
	if cfg.SubtitleHandling == "" {
		cfg.SubtitleHandling = "convert"
	} else if cfg.SubtitleHandling != "convert" && cfg.SubtitleHandling != "drop" {
		cfg.SubtitleHandling = "convert"
	}
	if cfg.LayoutDesign != "split" && cfg.LayoutDesign != "tabs" {
		cfg.LayoutDesign = "split"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	// Apply environment variable overrides for feature flags
	// This allows toggling features without modifying config files
	applyFeatureFlagEnvOverrides(cfg)

	return cfg, nil
}

// applyFeatureFlagEnvOverrides checks environment variables for feature flag overrides
// Environment variables take precedence over YAML config
// Use: SHRINKRAY_FEATURE_VIRTUAL_SCROLL=1 to enable, =0 to disable
func applyFeatureFlagEnvOverrides(cfg *Config) {
	if v := os.Getenv("SHRINKRAY_FEATURE_VIRTUAL_SCROLL"); v != "" {
		cfg.Features.VirtualScroll = envBool(v)
	}
	if v := os.Getenv("SHRINKRAY_FEATURE_DEFERRED_PROBING"); v != "" {
		cfg.Features.DeferredProbing = envBool(v)
	}
	if v := os.Getenv("SHRINKRAY_FEATURE_PAGINATED_INIT"); v != "" {
		cfg.Features.PaginatedInit = envBool(v)
	}
	if v := os.Getenv("SHRINKRAY_FEATURE_BATCHED_SSE"); v != "" {
		cfg.Features.BatchedSSE = envBool(v)
	}
	if v := os.Getenv("SHRINKRAY_FEATURE_DELTA_PROGRESS"); v != "" {
		cfg.Features.DeltaProgress = envBool(v)
	}
}

// envBool parses a boolean from an environment variable value
// Accepts: "1", "true", "yes", "on" for true; anything else is false
func envBool(v string) bool {
	b, err := strconv.ParseBool(v)
	if err == nil {
		return b
	}
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "yes", "on":
		return true
	case "no", "off":
		return false
	}
	return v == "1"
}

// Save writes the config to a YAML file
func (c *Config) Save(path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetTempDir returns the directory for temp files
// If TempPath is set, returns that; otherwise returns the directory of the source file
func (c *Config) GetTempDir(sourcePath string) string {
	if c.TempPath != "" {
		return c.TempPath
	}
	return filepath.Dir(sourcePath)
}

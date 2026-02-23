package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Blinko struct {
		BaseURL  string `yaml:"base_url"`
		JWTToken string `yaml:"jwt_token"`
	} `yaml:"blinko"`
	Watch struct {
		InputDir  string        `yaml:"input_dir"`
		FailedDir string        `yaml:"failed_dir"`
		Recursive bool          `yaml:"recursive"`
		StableFor time.Duration `yaml:"stable_for"`
		ScanEvery time.Duration `yaml:"scan_every"`
	} `yaml:"watch"`
	Processing struct {
		Workers        int           `yaml:"workers"`
		MaxRetries     int           `yaml:"max_retries"`
		RetryBaseDelay time.Duration `yaml:"retry_base_delay"`
		DeleteOnOK     bool          `yaml:"delete_on_success"`
		ArchiveDir     string        `yaml:"archive_dir"`
		QueueSize      int           `yaml:"queue_size"`
	} `yaml:"processing"`
	HTTP struct {
		Timeout time.Duration `yaml:"timeout"`
	} `yaml:"http"`
	Logging struct {
		Level string `yaml:"level"`
	} `yaml:"logging"`
	Metrics struct {
		Enabled    bool   `yaml:"enabled"`
		ListenAddr string `yaml:"listen_addr"`
	} `yaml:"metrics"`
}

func Load(path string) (Config, error) {
	var cfg Config

	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return cfg, fmt.Errorf("parse yaml: %w", err)
	}

	applyDefaults(&cfg)
	applyEnvOverrides(&cfg)

	if err := normalizeAndValidate(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Watch.StableFor == 0 {
		cfg.Watch.StableFor = 3 * time.Second
	}
	if cfg.Watch.ScanEvery == 0 {
		cfg.Watch.ScanEvery = 30 * time.Second
	}
	if cfg.Processing.Workers <= 0 {
		cfg.Processing.Workers = 2
	}
	if cfg.Processing.MaxRetries <= 0 {
		cfg.Processing.MaxRetries = 5
	}
	if cfg.Processing.RetryBaseDelay <= 0 {
		cfg.Processing.RetryBaseDelay = 2 * time.Second
	}
	if cfg.Processing.QueueSize <= 0 {
		cfg.Processing.QueueSize = 512
	}
	if cfg.HTTP.Timeout == 0 {
		cfg.HTTP.Timeout = 120 * time.Second
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Metrics.ListenAddr == "" {
		cfg.Metrics.ListenAddr = "127.0.0.1:9095"
	}
	if !hasDeleteFlagSet(cfg) {
		cfg.Processing.DeleteOnOK = true
	}
}

func hasDeleteFlagSet(cfg *Config) bool {
	_, ok := os.LookupEnv("BFD_DELETE_ON_SUCCESS")
	if ok {
		return true
	}
	return cfg.Processing.DeleteOnOK || cfg.Processing.ArchiveDir != ""
}

func applyEnvOverrides(cfg *Config) {
	overrideString(&cfg.Blinko.BaseURL, "BFD_BASE_URL")
	overrideString(&cfg.Blinko.JWTToken, "BFD_JWT_TOKEN")
	overrideString(&cfg.Watch.InputDir, "BFD_INPUT_DIR")
	overrideString(&cfg.Watch.FailedDir, "BFD_FAILED_DIR")
	overrideString(&cfg.Processing.ArchiveDir, "BFD_ARCHIVE_DIR")
	overrideString(&cfg.Logging.Level, "BFD_LOG_LEVEL")
	overrideString(&cfg.Metrics.ListenAddr, "BFD_METRICS_LISTEN_ADDR")
	overrideDuration(&cfg.Watch.StableFor, "BFD_STABLE_FOR")
	overrideDuration(&cfg.Watch.ScanEvery, "BFD_SCAN_EVERY")
	overrideDuration(&cfg.Processing.RetryBaseDelay, "BFD_RETRY_BASE_DELAY")
	overrideDuration(&cfg.HTTP.Timeout, "BFD_HTTP_TIMEOUT")
	overrideInt(&cfg.Processing.Workers, "BFD_WORKERS")
	overrideInt(&cfg.Processing.MaxRetries, "BFD_MAX_RETRIES")
	overrideInt(&cfg.Processing.QueueSize, "BFD_QUEUE_SIZE")
	overrideBool(&cfg.Watch.Recursive, "BFD_RECURSIVE")
	overrideBool(&cfg.Processing.DeleteOnOK, "BFD_DELETE_ON_SUCCESS")
	overrideBool(&cfg.Metrics.Enabled, "BFD_METRICS_ENABLED")
}

func overrideString(dst *string, env string) {
	if v, ok := os.LookupEnv(env); ok {
		*dst = strings.TrimSpace(v)
	}
}

func overrideInt(dst *int, env string) {
	if v, ok := os.LookupEnv(env); ok {
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil {
			*dst = n
		}
	}
}

func overrideBool(dst *bool, env string) {
	if v, ok := os.LookupEnv(env); ok {
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err == nil {
			*dst = b
		}
	}
}

func overrideDuration(dst *time.Duration, env string) {
	if v, ok := os.LookupEnv(env); ok {
		d, err := time.ParseDuration(strings.TrimSpace(v))
		if err == nil {
			*dst = d
		}
	}
}

func normalizeAndValidate(cfg *Config) error {
	cfg.Blinko.BaseURL = strings.TrimRight(strings.TrimSpace(cfg.Blinko.BaseURL), "/")
	cfg.Blinko.JWTToken = strings.TrimSpace(cfg.Blinko.JWTToken)
	cfg.Watch.InputDir = filepath.Clean(strings.TrimSpace(cfg.Watch.InputDir))

	if cfg.Blinko.BaseURL == "" {
		return errors.New("blinko.base_url is required")
	}
	if cfg.Blinko.JWTToken == "" {
		return errors.New("blinko.jwt_token is required")
	}
	if cfg.Watch.InputDir == "." || cfg.Watch.InputDir == "" {
		return errors.New("watch.input_dir is required")
	}

	absInput, err := filepath.Abs(cfg.Watch.InputDir)
	if err != nil {
		return fmt.Errorf("resolve watch.input_dir: %w", err)
	}
	cfg.Watch.InputDir = absInput

	if cfg.Watch.FailedDir == "" {
		cfg.Watch.FailedDir = filepath.Join(cfg.Watch.InputDir, "failed")
	}
	if !cfg.Processing.DeleteOnOK && cfg.Processing.ArchiveDir == "" {
		return errors.New("processing.archive_dir is required when delete_on_success is false")
	}

	cfg.Watch.FailedDir, err = filepath.Abs(cfg.Watch.FailedDir)
	if err != nil {
		return fmt.Errorf("resolve watch.failed_dir: %w", err)
	}

	if cfg.Processing.ArchiveDir != "" {
		cfg.Processing.ArchiveDir, err = filepath.Abs(cfg.Processing.ArchiveDir)
		if err != nil {
			return fmt.Errorf("resolve processing.archive_dir: %w", err)
		}
	}

	if cfg.Processing.Workers <= 0 || cfg.Processing.MaxRetries <= 0 || cfg.Processing.QueueSize <= 0 {
		return errors.New("workers, max_retries, and queue_size must be > 0")
	}
	if cfg.Watch.StableFor <= 0 || cfg.Watch.ScanEvery <= 0 || cfg.Processing.RetryBaseDelay <= 0 || cfg.HTTP.Timeout <= 0 {
		return errors.New("durations must be > 0")
	}

	if err := os.MkdirAll(cfg.Watch.InputDir, 0o755); err != nil {
		return fmt.Errorf("ensure watch.input_dir: %w", err)
	}
	if err := os.MkdirAll(cfg.Watch.FailedDir, 0o755); err != nil {
		return fmt.Errorf("ensure watch.failed_dir: %w", err)
	}
	if !cfg.Processing.DeleteOnOK {
		if err := os.MkdirAll(cfg.Processing.ArchiveDir, 0o755); err != nil {
			return fmt.Errorf("ensure processing.archive_dir: %w", err)
		}
	}

	return nil
}

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	AppEnv           string
	HTTPPort         int
	LogLevel         string
	ShutdownTimeout  time.Duration
	DatabaseURL      string
	RedisURL         string
	SchedulerInterval time.Duration
	SchedulerBatchSize int
	WorkerCount       int
	WorkerPollInterval time.Duration
	WorkerBatchSize   int
	JobLeaseTimeout   time.Duration
	ReaperInterval    time.Duration
	MaxRetries        int
	RetryBaseDelay    time.Duration
	RetryMaxDelay     time.Duration
	PlatformFeeBPS   int
	WithholdingTaxBPS int
	APIKey           string
}

// Load reads configuration from the environment with sensible defaults.
func Load() (Config, error) {
	cfg := Config{
		AppEnv:            getEnv("APP_ENV", "development"),
		HTTPPort:          getEnvInt("HTTP_PORT", 8080),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		ShutdownTimeout:   getEnvDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		RedisURL:          os.Getenv("REDIS_URL"),
		SchedulerInterval:  getEnvDuration("SCHEDULER_INTERVAL", 30*time.Second),
		SchedulerBatchSize: getEnvInt("SCHEDULER_BATCH_SIZE", 50),
		WorkerCount:        getEnvInt("WORKER_COUNT", 4),
		WorkerPollInterval: getEnvDuration("WORKER_POLL_INTERVAL", time.Second),
		WorkerBatchSize:    getEnvInt("WORKER_BATCH_SIZE", 1),
		JobLeaseTimeout:    getEnvDuration("JOB_LEASE_TIMEOUT", 5*time.Minute),
		ReaperInterval:     getEnvDuration("REAPER_INTERVAL", 60*time.Second),
		MaxRetries:         getEnvInt("MAX_RETRIES", 5),
		RetryBaseDelay:     getEnvDuration("RETRY_BASE_DELAY", 5*time.Second),
		RetryMaxDelay:      getEnvDuration("RETRY_MAX_DELAY", 15*time.Minute),
		PlatformFeeBPS:    getEnvInt("PLATFORM_FEE_BPS", 100),
		WithholdingTaxBPS: getEnvInt("WITHHOLDING_TAX_BPS", 1500),
		APIKey:            os.Getenv("API_KEY"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.RedisURL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}

	return cfg, nil
}

func (c Config) IsDevelopment() bool {
	return c.AppEnv == "development"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

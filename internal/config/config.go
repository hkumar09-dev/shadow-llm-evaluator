// Package config loads environment-specific settings from env/*.env files.
//
// All env files live under the env/ directory:
//
//	env/.env.local
//	env/.env.dev
//	env/.env.qa
//	env/.env.prod
//	env/.env.example
//
// Usage:
//
//	APP_ENV=local go run .     # loads env/.env.local
//	APP_ENV=dev  go run .     # loads env/.env.dev
//	APP_ENV=qa   go run .     # loads env/.env.qa
//	APP_ENV=prod go run .     # loads env/.env.prod
//
// Overrides:
//
//	ENV_DIR=env              # directory that holds .env.* files (default: env)
//	ENV_FILE=/path/to/file   # load this exact file instead
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// DefaultEnvDir is where .env.local / .env.dev / .env.qa / .env.prod live.
const DefaultEnvDir = "env"

// Supported environment names.
const (
	EnvLocal = "local"
	EnvDev   = "dev"
	EnvQA    = "qa"
	EnvProd  = "prod"
)

// Config holds every runtime setting used by the service.
type Config struct {
	AppEnv string // local | dev | qa | prod

	Addr     string
	GinMode  string
	LogLevel LogLevel

	// DigitalOcean Inference (OpenAI-compatible).
	// Empty URL → in-process simulator for that role.
	PrimaryLLMURL   string
	CandidateLLMURL string
	PrimaryModel    string // e.g. router:shadow-mode-llm-evaluator
	CandidateModel  string
	ModelAccessKey  string // Authorization: Bearer <key>

	ShadowTimeout     time.Duration
	ShadowMaxInflight int

	HTTPClientTimeout time.Duration
}

// LogLevel is parsed from LOG_LEVEL.
type LogLevel string

const (
	LogDebug LogLevel = "debug"
	LogInfo  LogLevel = "info"
	LogWarn  LogLevel = "warn"
	LogError LogLevel = "error"
)

// Load reads APP_ENV (default: local), loads env/.env.<env>, then builds Config.
func Load(ctx context.Context) (*Config, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	appEnv := strings.ToLower(strings.TrimSpace(envOr("APP_ENV", EnvLocal)))
	if err := validateAppEnv(appEnv); err != nil {
		return nil, err
	}

	envFile, err := resolveEnvFile(appEnv)
	if err != nil {
		return nil, err
	}
	if err := godotenv.Load(envFile); err != nil {
		return nil, fmt.Errorf("load env file %q: %w (create it or set ENV_FILE)", envFile, err)
	}

	appEnv = strings.ToLower(strings.TrimSpace(envOr("APP_ENV", appEnv)))
	if err := validateAppEnv(appEnv); err != nil {
		return nil, err
	}

	shadowTimeoutSec, err := envInt("SHADOW_TIMEOUT_SECONDS", 30)
	if err != nil {
		return nil, err
	}
	shadowMaxInflight, err := envInt("SHADOW_MAX_INFLIGHT", 32)
	if err != nil {
		return nil, err
	}
	httpTimeoutSec, err := envInt("HTTP_CLIENT_TIMEOUT_SECONDS", 30)
	if err != nil {
		return nil, err
	}

	logLevel := LogLevel(strings.ToLower(envOr("LOG_LEVEL", string(LogInfo))))
	if err := validateLogLevel(logLevel); err != nil {
		return nil, err
	}

	cfg := &Config{
		AppEnv:            appEnv,
		Addr:              resolveAddr(),
		GinMode:           envOr("GIN_MODE", "debug"),
		LogLevel:          logLevel,
		PrimaryLLMURL:     strings.TrimSpace(os.Getenv("PRIMARY_LLM_URL")),
		CandidateLLMURL:   strings.TrimSpace(os.Getenv("CANDIDATE_LLM_URL")),
		PrimaryModel:      strings.TrimSpace(os.Getenv("PRIMARY_MODEL")),
		CandidateModel:    strings.TrimSpace(os.Getenv("CANDIDATE_MODEL")),
		ModelAccessKey:    strings.TrimSpace(os.Getenv("MODEL_ACCESS_KEY")),
		ShadowTimeout:     time.Duration(shadowTimeoutSec) * time.Second,
		ShadowMaxInflight: shadowMaxInflight,
		HTTPClientTimeout: time.Duration(httpTimeoutSec) * time.Second,
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func resolveEnvFile(appEnv string) (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("ENV_FILE")); explicit != "" {
		return explicit, nil
	}

	envDir := envOr("ENV_DIR", DefaultEnvDir)
	path := filepath.Join(envDir, fmt.Sprintf(".env.%s", appEnv))
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("env file not found at %q: %w", path, err)
	}
	return path, nil
}

// Validate checks environment-specific rules.
func (c *Config) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("ADDR is required")
	}
	if c.ShadowTimeout <= 0 {
		return fmt.Errorf("SHADOW_TIMEOUT_SECONDS must be > 0")
	}
	if c.ShadowMaxInflight <= 0 {
		return fmt.Errorf("SHADOW_MAX_INFLIGHT must be > 0")
	}
	if c.HTTPClientTimeout <= 0 {
		return fmt.Errorf("HTTP_CLIENT_TIMEOUT_SECONDS must be > 0")
	}

	if c.AppEnv == EnvQA || c.AppEnv == EnvProd {
		if c.PrimaryLLMURL == "" {
			return fmt.Errorf("PRIMARY_LLM_URL is required in %s", c.AppEnv)
		}
		if c.CandidateLLMURL == "" {
			return fmt.Errorf("CANDIDATE_LLM_URL is required in %s", c.AppEnv)
		}
		if c.ModelAccessKey == "" || c.ModelAccessKey == "YOUR_MODEL_ACCESS_KEY" {
			return fmt.Errorf("MODEL_ACCESS_KEY is required in %s (create one in DigitalOcean → Inference → Model Access Keys)", c.AppEnv)
		}
		if c.PrimaryModel == "" {
			return fmt.Errorf("PRIMARY_MODEL is required in %s", c.AppEnv)
		}
		if c.CandidateModel == "" {
			return fmt.Errorf("CANDIDATE_MODEL is required in %s", c.AppEnv)
		}
	}
	return nil
}

// UsePrimarySimulator is true when no primary HTTP URL is configured.
func (c *Config) UsePrimarySimulator() bool {
	return c.PrimaryLLMURL == ""
}

// UseCandidateSimulator is true when no candidate HTTP URL is configured.
func (c *Config) UseCandidateSimulator() bool {
	return c.CandidateLLMURL == ""
}

func validateAppEnv(env string) error {
	switch env {
	case EnvLocal, EnvDev, EnvQA, EnvProd:
		return nil
	default:
		return fmt.Errorf("APP_ENV must be one of: local, dev, qa, prod (got %q)", env)
	}
}

func validateLogLevel(level LogLevel) error {
	switch level {
	case LogDebug, LogInfo, LogWarn, LogError:
		return nil
	default:
		return fmt.Errorf("LOG_LEVEL must be one of: debug, info, warn, error (got %q)", level)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func resolveAddr() string {
	if addr := os.Getenv("ADDR"); addr != "" {
		return addr
	}
	if port := os.Getenv("PORT"); port != "" {
		if strings.HasPrefix(port, ":") {
			return port
		}
		return ":" + port
	}
	return ":8080"
}

func envInt(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s=%q: %w", key, raw, err)
	}
	return n, nil
}

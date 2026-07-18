// Package main is the entrypoint for the shadow-llm-evaluator HTTP server.
//
// High-level request flow:
//
//  1. Client POSTs to /v1/primary
//  2. Handler calls the PRIMARY LLM synchronously and returns that response immediately
//  3. In the background, a CANDIDATE LLM is called with the same prompt
//  4. Primary vs candidate outputs are compared; mismatches are logged as clean JSON
//
// Why "shadow"? The candidate path must never slow down or break the user-facing
// primary path. It runs asynchronously and uses a detached context so it keeps
// running even if the client closes the HTTP connection.
//
// Configuration is loaded from env files under env/:
//
//	APP_ENV=local go run .   # env/.env.local
//	APP_ENV=dev   go run .   # env/.env.dev
//	APP_ENV=qa    go run .   # env/.env.qa
//	APP_ENV=prod  go run .   # env/.env.prod
package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/compare"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/config"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/handler"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/llm"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/shadow"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/simulator"
)

func main() {
	// Load all settings from env/.env.<APP_ENV> (default: env/.env.local).
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	// Apply gin mode from env (debug locally, release in qa/prod).
	gin.SetMode(cfg.GinMode)

	primarySim := simulator.NewPrimary()
	candidateSim := simulator.NewCandidate()

	// Wire primary Completer from config (simulator vs HTTP URL).
	var primary llm.Completer = primarySim
	primaryDesc := "in-process primary simulator"
	if !cfg.UsePrimarySimulator() {
		primary = llm.NewPrimaryClientWithTimeout(cfg.PrimaryLLMURL, cfg.HTTPClientTimeout)
		primaryDesc = cfg.PrimaryLLMURL
	}

	// Wire candidate Completer from config (simulator vs HTTP URL).
	var candidate llm.Completer = candidateSim
	candidateDesc := "in-process candidate simulator"
	if !cfg.UseCandidateSimulator() {
		candidate = llm.NewCandidateClientWithTimeout(cfg.CandidateLLMURL, cfg.HTTPClientTimeout)
		candidateDesc = cfg.CandidateLLMURL
	}

	shadowRunner := shadow.NewRunner(
		candidate,
		compare.NewContentComparator(),
		logger,
		shadow.WithTimeout(cfg.ShadowTimeout),
		shadow.WithMaxInflight(cfg.ShadowMaxInflight),
	)

	primaryHandler := handler.NewPrimaryHandler(primary, shadowRunner, logger)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)

	r.POST("/simulate/primary", simulator.PrimaryHandler(primarySim))
	r.POST("/v1/primary", primaryHandler.Handle)

	// Liveness/readiness for DigitalOcean App Platform (and Docker healthchecks).
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	logger.Info("server starting",
		"app_env", cfg.AppEnv,
		"addr", cfg.Addr,
		"gin_mode", cfg.GinMode,
		"log_level", string(cfg.LogLevel),
		"primary_llm", primaryDesc,
		"candidate_llm", candidateDesc,
		"shadow_timeout", cfg.ShadowTimeout.String(),
		"shadow_max_inflight", cfg.ShadowMaxInflight,
	)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// newLogger builds a JSON slog logger at the configured level.
func newLogger(level config.LogLevel) *slog.Logger {
	var lvl slog.Level
	switch level {
	case config.LogDebug:
		lvl = slog.LevelDebug
	case config.LogWarn:
		lvl = slog.LevelWarn
	case config.LogError:
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

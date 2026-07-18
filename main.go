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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)
	gin.SetMode(cfg.GinMode)

	primarySim := simulator.NewPrimary()
	candidateSim := simulator.NewCandidate()

	var primary llm.Completer = primarySim
	primaryDesc := "in-process primary simulator"
	if !cfg.UsePrimarySimulator() {
		primary = llm.NewPrimaryClientWithTimeout(
			cfg.PrimaryLLMURL,
			cfg.HTTPClientTimeout,
			llm.WithAPIKey(cfg.ModelAccessKey),
			llm.WithDefaultModel(cfg.PrimaryModel),
		)
		primaryDesc = cfg.PrimaryLLMURL + " model=" + cfg.PrimaryModel
	}

	var candidate llm.Completer = candidateSim
	candidateDesc := "in-process candidate simulator"
	if !cfg.UseCandidateSimulator() {
		candidate = llm.NewCandidateClientWithTimeout(
			cfg.CandidateLLMURL,
			cfg.HTTPClientTimeout,
			llm.WithAPIKey(cfg.ModelAccessKey),
			llm.WithDefaultModel(cfg.CandidateModel),
		)
		candidateDesc = cfg.CandidateLLMURL + " model=" + cfg.CandidateModel
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
		"model_access_key_set", cfg.ModelAccessKey != "" && cfg.ModelAccessKey != "YOUR_MODEL_ACCESS_KEY",
	)
	if err := r.Run(cfg.Addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

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

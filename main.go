package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/compare"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/config"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/handler"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/llm"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/shadow"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/simulator"
)

func main() {
	// Root application context — canceled on SIGINT/SIGTERM and passed everywhere.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load(ctx)
	if err != nil {
		return err
	}

	logger := newLogger(ctx, cfg.LogLevel)
	slog.SetDefault(logger)
	gin.SetMode(cfg.GinMode)

	primarySim := simulator.NewPrimary(ctx)
	candidateSim := simulator.NewCandidate(ctx)

	var primary llm.Completer = primarySim
	primaryDesc := "in-process primary simulator"
	if !cfg.UsePrimarySimulator() {
		primary = llm.NewPrimaryClientWithTimeout(
			ctx,
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
			ctx,
			cfg.CandidateLLMURL,
			cfg.HTTPClientTimeout,
			llm.WithAPIKey(cfg.ModelAccessKey),
			llm.WithDefaultModel(cfg.CandidateModel),
		)
		candidateDesc = cfg.CandidateLLMURL + " model=" + cfg.CandidateModel
	}

	shadowRunner := shadow.NewRunner(
		ctx,
		candidate,
		compare.NewContentComparator(ctx),
		logger,
		shadow.WithTimeout(cfg.ShadowTimeout),
		shadow.WithMaxInflight(cfg.ShadowMaxInflight),
	)

	primaryHandler := handler.NewPrimaryHandler(ctx, primary, shadowRunner, logger)
	healthHandler := handler.NewHealthHandler(ctx, cfg.AppEnv)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)

	r.POST("/simulate/primary", simulator.PrimaryHandler(ctx, primarySim))
	r.POST("/v1/primary", primaryHandler.Handle)
	r.GET("/healthz", healthHandler.Live)
	r.GET("/health", healthHandler.Live)
	r.GET("/ready", healthHandler.Ready)

	logger.InfoContext(ctx, "server starting",
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

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	// Use a fresh timeout independent of the already-canceled signal context.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	shadowRunner.Wait()
	return nil
}

func newLogger(ctx context.Context, level config.LogLevel) *slog.Logger {
	_ = ctx // reserved for context-aware handlers / request IDs later
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

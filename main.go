package main

import (
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/compare"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/handler"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/llm"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/shadow"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/simulator"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	addr := envOr("ADDR", ":8080")

	primarySim := simulator.NewPrimary()
	candidateSim := simulator.NewCandidate()

	var primary llm.Completer = primarySim
	var candidate llm.Completer = candidateSim
	primaryDesc := "in-process primary simulator"
	candidateDesc := "in-process candidate simulator"

	if url := os.Getenv("PRIMARY_LLM_URL"); url != "" {
		primary = llm.NewPrimaryClient(url)
		primaryDesc = url
	}
	if url := os.Getenv("CANDIDATE_LLM_URL"); url != "" {
		candidate = llm.NewCandidateClient(url)
		candidateDesc = url
	}

	shadowRunner := shadow.NewRunner(
		candidate,
		compare.NewContentComparator(),
		logger,
		shadow.WithTimeout(30*time.Second),
		shadow.WithMaxInflight(32),
	)

	primaryHandler := handler.NewPrimaryHandler(primary, shadowRunner, logger)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)

	r.POST("/simulate/primary", simulator.PrimaryHandler(primarySim))
	r.POST("/v1/primary", primaryHandler.Handle)

	logger.Info("server starting",
		"addr", addr,
		"primary_llm", primaryDesc,
		"candidate_llm", candidateDesc,
	)
	if err := r.Run(addr); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

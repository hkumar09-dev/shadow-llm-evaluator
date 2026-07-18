package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/handler"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/llm"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/simulator"
)

func main() {
	addr := envOr("ADDR", ":8080")

	sim := simulator.NewPrimary()
	var primary llm.Completer = sim
	primaryDesc := "in-process simulator"

	// Optional: POST to an external/simulated HTTP primary LLM instead.
	if url := os.Getenv("PRIMARY_LLM_URL"); url != "" {
		primary = llm.NewPrimaryClient(url)
		primaryDesc = url
	}

	primaryHandler := handler.NewPrimaryHandler(primary)

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())
	_ = r.SetTrustedProxies(nil)

	// Optional HTTP surface for the simulator (for direct probing / HTTP client mode).
	r.POST("/simulate/primary", simulator.PrimaryHandler(sim))

	// Synchronous primary route: call primary LLM and return immediately.
	r.POST("/v1/primary", primaryHandler.Handle)

	log.Printf("listening on %s (primary llm -> %s)", addr, primaryDesc)
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

// Package handler contains HTTP adapters.
//
// SOLID note (Single Responsibility): this package only translates HTTP into
// domain calls. It does NOT implement LLM logic, comparison, or shadowing.
package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/llm"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/shadow"
)

// PrimaryHandler is the HTTP entrypoint for the synchronous primary route.
//
// Dependencies are interfaces on purpose (Dependency Inversion):
//   - llm.Completer     → call primary model
//   - shadow.Evaluator  → kick off background candidate evaluation
type PrimaryHandler struct {
	primary llm.Completer
	shadow  shadow.Evaluator
	logger  *slog.Logger
}

// NewPrimaryHandler wires the handler with its collaborators.
func NewPrimaryHandler(primary llm.Completer, shadowEval shadow.Evaluator, logger *slog.Logger) *PrimaryHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &PrimaryHandler{
		primary: primary,
		shadow:  shadowEval,
		logger:  logger,
	}
}

// Handle implements:
//
//	POST /v1/primary
//
// Steps:
//  1. Parse JSON body into ChatRequest
//  2. Call primary LLM synchronously (user waits for this)
//  3. Write primary response to the client immediately
//  4. Trigger async shadow evaluation (does not block the response)
//
// Important: after c.JSON(...), the HTTP response is already sent. The shadow
// call still receives the request context, but the shadow package detaches
// cancellation so a client disconnect will not abort the candidate call.
func (h *PrimaryHandler) Handle(c *gin.Context) {
	var req models.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Synchronous primary path — this is what the end user receives.
	resp, err := h.primary.Complete(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("primary llm call failed", "error", err)
		c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Return immediately — do not wait for the candidate/shadow path.
	c.JSON(http.StatusOK, resp)

	// Fire-and-forget: shadow runner compares primary vs candidate in background.
	if h.shadow != nil {
		h.shadow.EvaluateAsync(c.Request.Context(), req, resp)
	}
}

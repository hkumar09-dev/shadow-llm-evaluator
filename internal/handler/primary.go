// Package handler contains HTTP adapters.
//
// SOLID note (Single Responsibility): this package only translates HTTP into
// domain calls. It does NOT implement LLM logic, comparison, or shadowing.
package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/ctxutil"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/llm"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/shadow"
)

// PrimaryHandler is the HTTP entrypoint for the synchronous primary route.
type PrimaryHandler struct {
	appCtx  context.Context // process lifetime (from main)
	primary llm.Completer
	shadow  shadow.Evaluator
	logger  *slog.Logger
}

// NewPrimaryHandler wires the handler with its collaborators.
func NewPrimaryHandler(appCtx context.Context, primary llm.Completer, shadowEval shadow.Evaluator, logger *slog.Logger) *PrimaryHandler {
	if appCtx == nil {
		appCtx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &PrimaryHandler{
		appCtx:  appCtx,
		primary: primary,
		shadow:  shadowEval,
		logger:  logger,
	}
}

// Handle implements POST /v1/primary.
//
// Context rules:
//   - Primary call uses request ctx linked to appCtx → cancels on client disconnect OR shutdown
//   - Shadow gets the same request ctx; Runner detaches client cancel but still honors appCtx
func (h *PrimaryHandler) Handle(c *gin.Context) {
	if err := h.appCtx.Err(); err != nil {
		c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "service shutting down"})
		return
	}

	ctx, cancel := ctxutil.Link(c.Request.Context(), h.appCtx)
	defer cancel()

	var req models.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	resp, err := h.primary.Complete(ctx, req)
	if err != nil {
		h.logger.ErrorContext(ctx, "primary llm call failed", "error", err)
		c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)

	if h.shadow != nil {
		h.shadow.EvaluateAsync(ctx, req, resp)
	}
}

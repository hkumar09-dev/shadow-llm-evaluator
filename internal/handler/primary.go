package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/llm"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/shadow"
)

// PrimaryHandler serves the synchronous primary route and triggers an
// asynchronous shadow evaluation against the candidate LLM.
type PrimaryHandler struct {
	primary llm.Completer
	shadow  shadow.Evaluator
	logger  *slog.Logger
}

// NewPrimaryHandler constructs a handler (depends on Completer + Evaluator abstractions).
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

// Handle calls the primary LLM synchronously, returns immediately, then
// schedules a background candidate evaluation that survives client disconnect.
func (h *PrimaryHandler) Handle(c *gin.Context) {
	var req models.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	resp, err := h.primary.Complete(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("primary llm call failed", "error", err)
		c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: err.Error()})
		return
	}

	// Return primary response to the user immediately.
	c.JSON(http.StatusOK, resp)

	// Fire-and-forget shadow evaluation (detached context inside the runner).
	if h.shadow != nil {
		h.shadow.EvaluateAsync(c.Request.Context(), req, resp)
	}
}

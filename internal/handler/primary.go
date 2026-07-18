package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/llm"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// PrimaryHandler serves the synchronous primary route: it calls the primary
// LLM and returns that response to the caller immediately.
type PrimaryHandler struct {
	client llm.Completer
}

// NewPrimaryHandler constructs a handler backed by the given completer.
func NewPrimaryHandler(client llm.Completer) *PrimaryHandler {
	return &PrimaryHandler{client: client}
}

// Handle receives a chat request, calls the primary LLM synchronously,
// and writes the LLM response back right away.
func (h *PrimaryHandler) Handle(c *gin.Context) {
	var req models.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
		return
	}

	resp, err := h.client.Complete(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusBadGateway, models.ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

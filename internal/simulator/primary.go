package simulator

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// Primary is an in-process simulated primary LLM (no HTTP loopback required).
type Primary struct{}

// NewPrimary returns a simulated primary LLM completer.
func NewPrimary() *Primary {
	return &Primary{}
}

// Complete returns a simulated chat completion immediately.
func (p *Primary) Complete(_ context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
	content := "Hello from simulated primary LLM."
	if n := len(req.Messages); n > 0 {
		if last := req.Messages[n-1].Content; last != "" {
			content = fmt.Sprintf("primary echo: %s", last)
		}
	}

	model := req.Model
	if model == "" {
		model = "primary-sim-v1"
	}

	return &models.ChatResponse{
		ID:     uuid.NewString(),
		Object: "chat.completion",
		Model:  model,
		Choices: []models.Choice{
			{
				Index: 0,
				Message: models.Message{
					Role:    "assistant",
					Content: content,
				},
			},
		},
	}, nil
}

// PrimaryHandler exposes the simulator as an HTTP POST endpoint.
func PrimaryHandler(primary *Primary) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
			return
		}

		resp, err := primary.Complete(c.Request.Context(), req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, resp)
	}
}

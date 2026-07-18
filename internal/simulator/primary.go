package simulator

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/ctxutil"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// Primary is an in-process simulated primary LLM.
type Primary struct {
	appCtx context.Context
}

// NewPrimary returns a simulated primary LLM completer.
func NewPrimary(appCtx context.Context) *Primary {
	if appCtx == nil {
		appCtx = context.Background()
	}
	return &Primary{appCtx: appCtx}
}

// Complete returns a simulated chat completion immediately.
func (p *Primary) Complete(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := p.appCtx.Err(); err != nil {
		return nil, err
	}

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

// PrimaryHandler exposes the simulator as POST /simulate/primary.
func PrimaryHandler(appCtx context.Context, primary *Primary) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := appCtx.Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, models.ErrorResponse{Error: "service shutting down"})
			return
		}

		ctx, cancel := ctxutil.Link(c.Request.Context(), appCtx)
		defer cancel()

		var req models.ChatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, models.ErrorResponse{Error: err.Error()})
			return
		}

		resp, err := primary.Complete(ctx, req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, models.ErrorResponse{Error: err.Error()})
			return
		}

		c.JSON(http.StatusOK, resp)
	}
}

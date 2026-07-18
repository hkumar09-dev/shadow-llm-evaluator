package simulator

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// Candidate is an in-process simulated candidate LLM.
type Candidate struct {
	appCtx context.Context
}

// NewCandidate returns a simulated candidate LLM completer.
func NewCandidate(appCtx context.Context) *Candidate {
	if appCtx == nil {
		appCtx = context.Background()
	}
	return &Candidate{appCtx: appCtx}
}

// Complete returns a simulated candidate chat completion.
func (c *Candidate) Complete(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.appCtx.Err(); err != nil {
		return nil, err
	}

	content := "Hello from simulated candidate LLM."
	if n := len(req.Messages); n > 0 {
		if last := req.Messages[n-1].Content; last != "" {
			content = fmt.Sprintf("candidate echo: %s", last)
		}
	}

	model := req.Model
	if model == "" {
		model = "candidate-sim-v1"
	} else {
		model = model + "-candidate"
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

package simulator

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// Candidate is an in-process simulated candidate LLM. Its responses differ
// from Primary so shadow comparison can detect mismatches in local demos.
type Candidate struct{}

// NewCandidate returns a simulated candidate LLM completer.
func NewCandidate() *Candidate {
	return &Candidate{}
}

// Complete returns a simulated candidate chat completion.
func (c *Candidate) Complete(_ context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
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

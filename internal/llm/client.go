package llm

import (
	"context"

	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// Completer synchronously produces a chat completion.
type Completer interface {
	Complete(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error)
}

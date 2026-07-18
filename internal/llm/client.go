// Package llm defines how this service talks to language models.
//
// Completer is the shared interface for both primary and candidate models.
// That lets the rest of the app swap simulators ↔ HTTP clients without
// changing handlers or shadow logic (Dependency Inversion + Liskov).
package llm

import (
	"context"

	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// Completer synchronously produces a chat completion.
//
// Implementations:
//   - simulator.Primary / simulator.Candidate  (local fakes)
//   - llm.HTTPClient                           (real remote endpoints)
type Completer interface {
	Complete(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error)
}

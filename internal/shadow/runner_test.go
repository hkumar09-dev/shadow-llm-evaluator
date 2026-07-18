package shadow_test

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/compare"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/shadow"
)

// stubCompleter is a fake llm.Completer used only in tests.
// It waits for `delay`, then either returns resp or reports that ctx was canceled.
type stubCompleter struct {
	sawCanceled atomic.Bool
	delay       time.Duration
	resp        *models.ChatResponse
}

func (s *stubCompleter) Complete(ctx context.Context, _ models.ChatRequest) (*models.ChatResponse, error) {
	select {
	case <-time.After(s.delay):
		if ctx.Err() != nil {
			s.sawCanceled.Store(true)
			return nil, ctx.Err()
		}
		return s.resp, nil
	case <-ctx.Done():
		// Context canceled/timed out before delay finished.
		s.sawCanceled.Store(true)
		return nil, ctx.Err()
	}
}

// TestEvaluateAsync_survivesRequestCancel proves the core shadow guarantee:
// canceling the HTTP request context must NOT cancel the candidate call.
func TestEvaluateAsync_survivesRequestCancel(t *testing.T) {
	candidate := &stubCompleter{
		delay: 150 * time.Millisecond,
		resp: &models.ChatResponse{
			Model: "candidate",
			Choices: []models.Choice{{
				Message: models.Message{Content: "candidate echo: hi"},
			}},
		},
	}

	runner := shadow.NewRunner(
		candidate,
		compare.NewContentComparator(),
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		shadow.WithTimeout(2*time.Second),
	)

	reqCtx, cancel := context.WithCancel(context.Background())
	primary := &models.ChatResponse{
		Model: "primary",
		Choices: []models.Choice{{
			Message: models.Message{Content: "primary echo: hi"},
		}},
	}

	runner.EvaluateAsync(reqCtx, models.ChatRequest{
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	}, primary)

	// Simulate client disconnect right after the primary response was sent.
	cancel()
	runner.Wait()

	if candidate.sawCanceled.Load() {
		t.Fatal("candidate call was canceled by request context; detached context should survive")
	}
}

// TestEvaluateAsync_respectsDetachedTimeout proves shadow work is still bounded:
// WithoutCancel removes client cancel, but WithTimeout still stops hung candidates.
func TestEvaluateAsync_respectsDetachedTimeout(t *testing.T) {
	candidate := &stubCompleter{
		delay: 500 * time.Millisecond, // longer than shadow timeout below
		resp:  &models.ChatResponse{Model: "candidate"},
	}

	runner := shadow.NewRunner(
		candidate,
		compare.NewContentComparator(),
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
		shadow.WithTimeout(50*time.Millisecond),
	)

	runner.EvaluateAsync(context.Background(), models.ChatRequest{
		Messages: []models.Message{{Role: "user", Content: "hi"}},
	}, &models.ChatResponse{Model: "primary"})
	runner.Wait()

	if !candidate.sawCanceled.Load() {
		t.Fatal("expected candidate to observe detached timeout cancellation")
	}
}

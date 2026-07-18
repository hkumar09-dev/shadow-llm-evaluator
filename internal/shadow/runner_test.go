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
		s.sawCanceled.Store(true)
		return nil, ctx.Err()
	}
}

func TestEvaluateAsync_survivesRequestCancel(t *testing.T) {
	appCtx := context.Background()
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
		appCtx,
		candidate,
		compare.NewContentComparator(appCtx),
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

	cancel()
	runner.Wait()

	if candidate.sawCanceled.Load() {
		t.Fatal("candidate call was canceled by request context; detached context should survive")
	}
}

func TestEvaluateAsync_respectsDetachedTimeout(t *testing.T) {
	appCtx := context.Background()
	candidate := &stubCompleter{
		delay: 500 * time.Millisecond,
		resp:  &models.ChatResponse{Model: "candidate"},
	}

	runner := shadow.NewRunner(
		appCtx,
		candidate,
		compare.NewContentComparator(appCtx),
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

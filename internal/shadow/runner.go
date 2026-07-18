package shadow

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/compare"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/llm"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// Evaluator runs asynchronous shadow evaluations against a candidate LLM.
type Evaluator interface {
	EvaluateAsync(reqCtx context.Context, req models.ChatRequest, primary *models.ChatResponse)
}

// Runner fires-and-forgets candidate completions, then compares them to the
// primary response. The request context is detached so client disconnects
// do not cancel the background work.
type Runner struct {
	candidate  llm.Completer
	comparator compare.Comparator
	logger     *slog.Logger
	timeout    time.Duration
	maxInflight int64

	inflight atomic.Int64
	wg       sync.WaitGroup
}

// Option configures a Runner.
type Option func(*Runner)

// WithTimeout sets the detached context timeout for candidate calls.
func WithTimeout(d time.Duration) Option {
	return func(r *Runner) {
		if d > 0 {
			r.timeout = d
		}
	}
}

// WithMaxInflight caps concurrent background shadow evaluations.
func WithMaxInflight(n int) Option {
	return func(r *Runner) {
		if n > 0 {
			r.maxInflight = int64(n)
		}
	}
}

// NewRunner constructs a shadow evaluator (Dependency Inversion on Completer + Comparator).
func NewRunner(candidate llm.Completer, comparator compare.Comparator, logger *slog.Logger, opts ...Option) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	if comparator == nil {
		comparator = compare.NewContentComparator()
	}

	r := &Runner{
		candidate:   candidate,
		comparator:  comparator,
		logger:      logger,
		timeout:     30 * time.Second,
		maxInflight: 32,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// EvaluateAsync schedules a background candidate call that outlives the
// primary HTTP request. Safe to call after the response has been written.
func (r *Runner) EvaluateAsync(reqCtx context.Context, req models.ChatRequest, primary *models.ChatResponse) {
	if r == nil || r.candidate == nil || primary == nil {
		return
	}

	if r.inflight.Add(1) > r.maxInflight {
		r.inflight.Add(-1)
		r.logger.Warn("shadow evaluation skipped: max inflight reached",
			"max_inflight", r.maxInflight,
		)
		return
	}

	// Detach from the HTTP request so client close / cancel does not abort shadow work.
	base := context.WithoutCancel(reqCtx)
	ctx, cancel := context.WithTimeout(base, r.timeout)

	// Copy request so the handler can mutate/reuse its stack value safely.
	reqCopy := cloneRequest(req)
	primaryCopy := cloneResponse(primary)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.inflight.Add(-1)
		defer cancel()
		defer func() {
			if rec := recover(); rec != nil {
				r.logger.Error("shadow evaluation panicked", "recover", rec)
			}
		}()

		r.evaluate(ctx, reqCopy, primaryCopy)
	}()
}

// Wait blocks until in-flight shadow evaluations finish (for graceful shutdown/tests).
func (r *Runner) Wait() {
	if r == nil {
		return
	}
	r.wg.Wait()
}

func (r *Runner) evaluate(ctx context.Context, req models.ChatRequest, primary *models.ChatResponse) {
	candidate, err := r.candidate.Complete(ctx, req)
	if err != nil {
		r.logger.Error("candidate llm call failed",
			"error", err,
			"primary_model", primary.Model,
		)
		return
	}

	result := r.comparator.Compare(primary, candidate)
	if result.Equal {
		r.logger.Info("shadow evaluation matched",
			"primary_model", primary.Model,
			"candidate_model", candidate.Model,
		)
		return
	}

	r.logger.Warn("shadow evaluation mismatched",
		"primary_model", primary.Model,
		"candidate_model", candidate.Model,
		"primary_payload", jsonString(result.PrimaryPayload),
		"candidate_payload", jsonString(result.CandidatePayload),
		"primary_content", result.PrimaryContent,
		"candidate_content", result.CandidateContent,
	)
}

func jsonString(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	return string(raw)
}

func cloneRequest(req models.ChatRequest) models.ChatRequest {
	out := req
	if req.Messages != nil {
		out.Messages = append([]models.Message(nil), req.Messages...)
	}
	return out
}

func cloneResponse(resp *models.ChatResponse) *models.ChatResponse {
	if resp == nil {
		return nil
	}
	out := *resp
	if resp.Choices != nil {
		out.Choices = append([]models.Choice(nil), resp.Choices...)
	}
	return &out
}

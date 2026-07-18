// Package shadow orchestrates asynchronous "shadow" evaluations.
//
// A shadow evaluation means: after (or alongside) serving the primary model
// response, we also send the same prompt to a candidate model in the background
// and compare the two outputs. The user never waits on the candidate.
//
// SOLID:
//   - Single Responsibility: only scheduling + comparing + logging mismatches
//   - Dependency Inversion: depends on llm.Completer and compare.Comparator
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

// Evaluator is the abstraction the HTTP handler depends on.
// Keeping this small (ISP) lets tests stub shadowing easily.
type Evaluator interface {
	// EvaluateAsync must not block the caller for the duration of the candidate call.
	EvaluateAsync(reqCtx context.Context, req models.ChatRequest, primary *models.ChatResponse)
}

// Runner is the default Evaluator implementation.
//
// Key design choice — detached context:
// Gin cancels c.Request.Context() when the client disconnects. If we passed that
// context into the candidate call, shadow work would stop early. Instead we use
// context.WithoutCancel + an explicit timeout so shadow work outlives the HTTP
// connection but still cannot run forever.
type Runner struct {
	candidate   llm.Completer      // candidate / challenger model
	comparator  compare.Comparator // decides equal vs mismatch
	logger      *slog.Logger
	timeout     time.Duration // max time for one shadow evaluation
	maxInflight int64         // concurrency cap for background jobs

	inflight atomic.Int64 // how many shadow goroutines are currently running
	wg       sync.WaitGroup
}

// Option is a functional option for configuring Runner (Open/Closed friendly).
type Option func(*Runner)

// WithTimeout sets how long a detached shadow call may run.
func WithTimeout(d time.Duration) Option {
	return func(r *Runner) {
		if d > 0 {
			r.timeout = d
		}
	}
}

// WithMaxInflight limits concurrent shadow goroutines.
// When the limit is hit, new shadow evaluations are skipped (never block primary).
func WithMaxInflight(n int) Option {
	return func(r *Runner) {
		if n > 0 {
			r.maxInflight = int64(n)
		}
	}
}

// NewRunner constructs a shadow evaluator.
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
// primary HTTP request. It is safe to call after the response has been written.
func (r *Runner) EvaluateAsync(reqCtx context.Context, req models.ChatRequest, primary *models.ChatResponse) {
	if r == nil || r.candidate == nil || primary == nil {
		return
	}

	// Cheap backpressure: never queue unbounded shadow work.
	if r.inflight.Add(1) > r.maxInflight {
		r.inflight.Add(-1)
		r.logger.Warn("shadow evaluation skipped: max inflight reached",
			"max_inflight", r.maxInflight,
		)
		return
	}

	// --- Context survival (the important bit) ---
	// WithoutCancel keeps values from reqCtx but drops cancellation from the client.
	// WithTimeout still bounds the work so a stuck candidate cannot leak forever.
	base := context.WithoutCancel(reqCtx)
	ctx, cancel := context.WithTimeout(base, r.timeout)

	// Copy inputs before spawning the goroutine (avoid data races / reuse bugs).
	reqCopy := cloneRequest(req)
	primaryCopy := cloneResponse(primary)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.inflight.Add(-1)
		defer cancel()
		// Shadow must never crash the whole process.
		defer func() {
			if rec := recover(); rec != nil {
				r.logger.Error("shadow evaluation panicked", "recover", rec)
			}
		}()

		r.evaluate(ctx, reqCopy, primaryCopy)
	}()
}

// Wait blocks until in-flight shadow evaluations finish.
// Useful for tests and graceful shutdown.
func (r *Runner) Wait() {
	if r == nil {
		return
	}
	r.wg.Wait()
}

// evaluate performs the candidate call and comparison on a background goroutine.
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

	// Mismatch: log clean JSON payloads for both sides so diffs are easy to inspect.
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

// cloneRequest shallow-copies the request and deep-copies the messages slice.
func cloneRequest(req models.ChatRequest) models.ChatRequest {
	out := req
	if req.Messages != nil {
		out.Messages = append([]models.Message(nil), req.Messages...)
	}
	return out
}

// cloneResponse copies the response so the background goroutine owns its data.
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

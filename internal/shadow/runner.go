// Package shadow orchestrates asynchronous "shadow" evaluations.
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
type Evaluator interface {
	EvaluateAsync(reqCtx context.Context, req models.ChatRequest, primary *models.ChatResponse)
}

// Runner is the default Evaluator implementation.
type Runner struct {
	appCtx      context.Context // process root from main — cancels shadow on shutdown
	candidate   llm.Completer
	comparator  compare.Comparator
	logger      *slog.Logger
	timeout     time.Duration
	maxInflight int64

	inflight atomic.Int64
	wg       sync.WaitGroup
}

// Option configures a Runner.
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
func WithMaxInflight(n int) Option {
	return func(r *Runner) {
		if n > 0 {
			r.maxInflight = int64(n)
		}
	}
}

// NewRunner constructs a shadow evaluator with the root app context from main.
func NewRunner(appCtx context.Context, candidate llm.Completer, comparator compare.Comparator, logger *slog.Logger, opts ...Option) *Runner {
	if appCtx == nil {
		appCtx = context.Background()
	}
	if logger == nil {
		logger = slog.Default()
	}
	if comparator == nil {
		comparator = compare.NewContentComparator(appCtx)
	}

	r := &Runner{
		appCtx:      appCtx,
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
// client HTTP request, but still stops when the app context is canceled.
func (r *Runner) EvaluateAsync(reqCtx context.Context, req models.ChatRequest, primary *models.ChatResponse) {
	if r == nil || r.candidate == nil || primary == nil {
		return
	}
	if err := r.appCtx.Err(); err != nil {
		r.logger.WarnContext(reqCtx, "shadow evaluation skipped: app shutting down")
		return
	}

	if r.inflight.Add(1) > r.maxInflight {
		r.inflight.Add(-1)
		r.logger.WarnContext(reqCtx, "shadow evaluation skipped: max inflight reached",
			"max_inflight", r.maxInflight,
		)
		return
	}

	// Drop client cancel, keep values; bound with timeout; also stop on app shutdown.
	base := context.WithoutCancel(reqCtx)
	ctx, cancel := context.WithTimeout(base, r.timeout)
	stopLink := context.AfterFunc(r.appCtx, cancel)

	reqCopy := cloneRequest(req)
	primaryCopy := cloneResponse(primary)

	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer r.inflight.Add(-1)
		defer cancel()
		defer stopLink()
		defer func() {
			if rec := recover(); rec != nil {
				r.logger.ErrorContext(ctx, "shadow evaluation panicked", "recover", rec)
			}
		}()

		r.evaluate(ctx, reqCopy, primaryCopy)
	}()
}

// Wait blocks until in-flight shadow evaluations finish.
func (r *Runner) Wait() {
	if r == nil {
		return
	}
	r.wg.Wait()
}

func (r *Runner) evaluate(ctx context.Context, req models.ChatRequest, primary *models.ChatResponse) {
	if err := ctx.Err(); err != nil {
		r.logger.WarnContext(ctx, "shadow evaluation aborted before candidate call", "error", err)
		return
	}

	candidate, err := r.candidate.Complete(ctx, req)
	if err != nil {
		r.logger.ErrorContext(ctx, "candidate llm call failed",
			"error", err,
			"primary_model", primary.Model,
		)
		return
	}

	result := r.comparator.Compare(ctx, primary, candidate)
	if result.Equal {
		r.logger.InfoContext(ctx, "shadow evaluation matched",
			"primary_model", primary.Model,
			"candidate_model", candidate.Model,
		)
		return
	}

	r.logger.WarnContext(ctx, "shadow evaluation mismatched",
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

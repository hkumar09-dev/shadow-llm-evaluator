// Package compare decides whether primary and candidate LLM outputs match.
package compare

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"

	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// Result is the outcome of comparing primary and candidate completions.
type Result struct {
	Equal            bool
	PrimaryContent   string
	CandidateContent string
	PrimaryPayload   json.RawMessage
	CandidatePayload json.RawMessage
}

type mismatchPayload struct {
	Model     string          `json:"model"`
	Content   string          `json:"content"`
	Extracted json.RawMessage `json:"extracted_json,omitempty"`
}

// Comparator compares two LLM responses.
type Comparator interface {
	Compare(ctx context.Context, primary, candidate *models.ChatResponse) Result
}

// ContentComparator compares the first choice's assistant message content.
type ContentComparator struct {
	appCtx context.Context
}

// NewContentComparator returns the default content-based comparator.
func NewContentComparator(appCtx context.Context) *ContentComparator {
	if appCtx == nil {
		appCtx = context.Background()
	}
	return &ContentComparator{appCtx: appCtx}
}

// Compare returns Equal=true when assistant contents match.
func (c *ContentComparator) Compare(ctx context.Context, primary, candidate *models.ChatResponse) Result {
	if err := ctx.Err(); err != nil {
		return Result{}
	}
	if c != nil && c.appCtx != nil {
		if err := c.appCtx.Err(); err != nil {
			return Result{}
		}
	}

	pContent := assistantContent(primary)
	cContent := assistantContent(candidate)

	res := Result{
		Equal:            pContent == cContent,
		PrimaryContent:   pContent,
		CandidateContent: cContent,
	}
	if res.Equal {
		return res
	}

	res.PrimaryPayload = buildMismatchPayload(ctx, modelName(primary), pContent)
	res.CandidatePayload = buildMismatchPayload(ctx, modelName(candidate), cContent)
	return res
}

func assistantContent(resp *models.ChatResponse) string {
	if resp == nil || len(resp.Choices) == 0 {
		return ""
	}
	return resp.Choices[0].Message.Content
}

func modelName(resp *models.ChatResponse) string {
	if resp == nil {
		return ""
	}
	return resp.Model
}

func buildMismatchPayload(ctx context.Context, model, content string) json.RawMessage {
	if err := ctx.Err(); err != nil {
		return json.RawMessage(`{}`)
	}
	payload := mismatchPayload{
		Model:     model,
		Content:   content,
		Extracted: extractJSON(ctx, content),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func extractJSON(ctx context.Context, content string) json.RawMessage {
	if err := ctx.Err(); err != nil {
		return nil
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	if json.Valid([]byte(trimmed)) {
		return compactJSON(trimmed)
	}
	return extractEmbeddedJSON(ctx, trimmed)
}

func compactJSON(raw string) json.RawMessage {
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(raw)); err != nil {
		return json.RawMessage(raw)
	}
	return compact.Bytes()
}

func extractEmbeddedJSON(ctx context.Context, s string) json.RawMessage {
	if err := ctx.Err(); err != nil {
		return nil
	}
	startObj := strings.Index(s, "{")
	startArr := strings.Index(s, "[")

	var start int
	switch {
	case startObj >= 0 && startArr >= 0:
		if startObj < startArr {
			start = startObj
		} else {
			start = startArr
		}
	case startObj >= 0:
		start = startObj
	case startArr >= 0:
		start = startArr
	default:
		return nil
	}

	candidate := s[start:]
	for end := len(candidate); end > 0; end-- {
		if err := ctx.Err(); err != nil {
			return nil
		}
		chunk := strings.TrimSpace(candidate[:end])
		if json.Valid([]byte(chunk)) {
			return compactJSON(chunk)
		}
	}
	return nil
}

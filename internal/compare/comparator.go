// Package compare decides whether primary and candidate LLM outputs match.
//
// On mismatch it builds clean JSON payloads that include:
//   - model name
//   - full assistant content
//   - any JSON extracted from that content (if present)
//
// Those payloads are what the shadow runner logs for debugging.
package compare

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// Result is the outcome of comparing primary and candidate completions.
type Result struct {
	Equal            bool            // true when assistant contents are identical
	PrimaryContent   string          // raw assistant text from primary
	CandidateContent string          // raw assistant text from candidate
	PrimaryPayload   json.RawMessage // clean JSON blob for logging (mismatch only)
	CandidatePayload json.RawMessage // clean JSON blob for logging (mismatch only)
}

// mismatchPayload is the clean JSON shape logged when models disagree.
// Example:
//
//	{"model":"primary-sim-v1","content":"primary echo: hi","extracted_json":{"task":"demo"}}
type mismatchPayload struct {
	Model     string          `json:"model"`
	Content   string          `json:"content"`
	Extracted json.RawMessage `json:"extracted_json,omitempty"`
}

// Comparator is the abstraction used by the shadow runner (ISP + DIP).
type Comparator interface {
	Compare(primary, candidate *models.ChatResponse) Result
}

// ContentComparator compares the first choice's assistant message content.
type ContentComparator struct{}

// NewContentComparator returns the default content-based comparator.
func NewContentComparator() *ContentComparator {
	return &ContentComparator{}
}

// Compare returns Equal=true when assistant contents match.
// On mismatch it builds structured JSON payloads for each side.
func (c *ContentComparator) Compare(primary, candidate *models.ChatResponse) Result {
	pContent := assistantContent(primary)
	cContent := assistantContent(candidate)

	res := Result{
		Equal:            pContent == cContent,
		PrimaryContent:   pContent,
		CandidateContent: cContent,
	}
	if res.Equal {
		// No need to build payloads when everything matches.
		return res
	}

	// Build log-friendly JSON for each side (content + optional extracted JSON).
	res.PrimaryPayload = buildMismatchPayload(modelName(primary), pContent)
	res.CandidatePayload = buildMismatchPayload(modelName(candidate), cContent)
	return res
}

// assistantContent returns the first choice's message text (empty if missing).
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

// buildMismatchPayload marshals a clean, consistent JSON object for logging.
func buildMismatchPayload(model, content string) json.RawMessage {
	payload := mismatchPayload{
		Model:     model,
		Content:   content,
		Extracted: extractJSON(content), // nil/omitted when content has no JSON
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

// extractJSON returns compact JSON if content is itself JSON, or embeds JSON
// inside surrounding text (common with LLM answers like: `Sure! {"a":1}`).
func extractJSON(content string) json.RawMessage {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}

	// Whole content is valid JSON.
	if json.Valid([]byte(trimmed)) {
		return compactJSON([]byte(trimmed))
	}

	// Try to pull out an embedded object/array.
	return extractEmbeddedJSON(trimmed)
}

func compactJSON(raw []byte) json.RawMessage {
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return json.RawMessage(raw)
	}
	return compact.Bytes()
}

// extractEmbeddedJSON finds the first '{' or '[' and walks backward from the
// end until the substring is valid JSON. This is a simple heuristic — good
// enough for logging mismatches, not a full JSON repair parser.
func extractEmbeddedJSON(s string) json.RawMessage {
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
		return nil // no JSON-looking content
	}

	candidate := s[start:]
	// Shrink from the end until we find a valid JSON document.
	for end := len(candidate); end > 0; end-- {
		chunk := strings.TrimSpace(candidate[:end])
		if json.Valid([]byte(chunk)) {
			return compactJSON([]byte(chunk))
		}
	}
	return nil
}

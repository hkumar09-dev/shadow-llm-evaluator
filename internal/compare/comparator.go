package compare

import (
	"bytes"
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

// mismatchPayload is the clean JSON shape logged when models disagree.
type mismatchPayload struct {
	Model     string          `json:"model"`
	Content   string          `json:"content"`
	Extracted json.RawMessage `json:"extracted_json,omitempty"`
}

// Comparator compares two LLM responses (Interface Segregation).
type Comparator interface {
	Compare(primary, candidate *models.ChatResponse) Result
}

// ContentComparator compares assistant message content and builds clean
// JSON mismatch payloads when the models disagree.
type ContentComparator struct{}

// NewContentComparator returns the default content-based comparator.
func NewContentComparator() *ContentComparator {
	return &ContentComparator{}
}

// Compare returns Equal=true when assistant contents match. On mismatch it
// builds structured JSON payloads for each side (content + any extracted JSON).
func (c *ContentComparator) Compare(primary, candidate *models.ChatResponse) Result {
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

	res.PrimaryPayload = buildMismatchPayload(modelName(primary), pContent)
	res.CandidatePayload = buildMismatchPayload(modelName(candidate), cContent)
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

func buildMismatchPayload(model, content string) json.RawMessage {
	payload := mismatchPayload{
		Model:     model,
		Content:   content,
		Extracted: extractJSON(content),
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

// extractJSON returns compact JSON if content is (or embeds) a JSON value.
func extractJSON(content string) json.RawMessage {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}

	if json.Valid([]byte(trimmed)) {
		return compactJSON([]byte(trimmed))
	}
	return extractEmbeddedJSON(trimmed)
}

func compactJSON(raw []byte) json.RawMessage {
	var compact bytes.Buffer
	if err := json.Compact(&compact, raw); err != nil {
		return json.RawMessage(raw)
	}
	return compact.Bytes()
}

func extractEmbeddedJSON(s string) json.RawMessage {
	startObj := strings.Index(s, "{")
	startArr := strings.Index(s, "[")
	start := -1
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
		chunk := strings.TrimSpace(candidate[:end])
		if json.Valid([]byte(chunk)) {
			return compactJSON([]byte(chunk))
		}
	}
	return nil
}

// Package models holds shared request/response DTOs (data transfer objects).
// Keeping these in one place avoids circular imports between handler/llm/shadow.
package models

// Message is a single chat turn (role + content), similar to OpenAI chat format.
type Message struct {
	Role    string `json:"role"`    // e.g. "user", "assistant", "system"
	Content string `json:"content"` // the text for this turn
}

// ChatRequest is the JSON body accepted by POST /v1/primary
// and forwarded to both primary and candidate LLMs.
type ChatRequest struct {
	Model    string    `json:"model"`                               // optional model name hint
	Messages []Message `json:"messages" binding:"required,min=1"` // at least one message required
}

// Choice is one completion option from the LLM (we usually use Choices[0]).
type Choice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

// ChatResponse is returned to the caller immediately after the primary LLM responds.
// The same shape is expected from candidate models for easy comparison.
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"` // e.g. "chat.completion"
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// ErrorResponse is a simple JSON error body for HTTP 4xx/5xx replies.
type ErrorResponse struct {
	Error string `json:"error"`
}

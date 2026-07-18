package models

// Message is a single chat turn.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the payload accepted by the primary route and forwarded
// to the primary LLM endpoint.
type ChatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages" binding:"required,min=1"`
}

// Choice is one completion option from the LLM.
type Choice struct {
	Index   int     `json:"index"`
	Message Message `json:"message"`
}

// ChatResponse is returned to the caller immediately after the primary
// LLM responds.
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

// ErrorResponse is a JSON error body.
type ErrorResponse struct {
	Error string `json:"error"`
}

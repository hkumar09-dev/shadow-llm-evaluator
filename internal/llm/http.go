package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

// HTTPClient POSTs OpenAI-compatible chat completions to a remote LLM endpoint
// (e.g. DigitalOcean Inference: https://inference.do-ai.run/v1/chat/completions).
type HTTPClient struct {
	name         string // "primary" or "candidate"
	baseURL      string
	apiKey       string // MODEL_ACCESS_KEY — sent as Authorization: Bearer ...
	defaultModel string // used when the inbound request has no model set
	httpClient   *http.Client
}

// ClientOption configures an HTTPClient.
type ClientOption func(*HTTPClient)

// WithAPIKey sets the Bearer token (DigitalOcean Model Access Key).
func WithAPIKey(key string) ClientOption {
	return func(c *HTTPClient) {
		c.apiKey = key
	}
}

// WithDefaultModel sets the model id when the request omits one
// (e.g. "router:shadow-mode-llm-evaluator").
func WithDefaultModel(model string) ClientOption {
	return func(c *HTTPClient) {
		c.defaultModel = model
	}
}

// WithTimeout overrides the HTTP client timeout.
func WithTimeout(d time.Duration) ClientOption {
	return func(c *HTTPClient) {
		if d > 0 {
			c.httpClient.Timeout = d
		}
	}
}

// NewHTTPClient creates a named HTTP completer targeting baseURL.
func NewHTTPClient(name, baseURL string, opts ...ClientOption) *HTTPClient {
	c := &HTTPClient{
		name:    name,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewPrimaryClientWithTimeout creates a primary HTTP client (compat helper).
func NewPrimaryClientWithTimeout(baseURL string, timeout time.Duration, opts ...ClientOption) *HTTPClient {
	opts = append([]ClientOption{WithTimeout(timeout)}, opts...)
	return NewHTTPClient("primary", baseURL, opts...)
}

// NewCandidateClientWithTimeout creates a candidate HTTP client (compat helper).
func NewCandidateClientWithTimeout(baseURL string, timeout time.Duration, opts ...ClientOption) *HTTPClient {
	opts = append([]ClientOption{WithTimeout(timeout)}, opts...)
	return NewHTTPClient("candidate", baseURL, opts...)
}

// outboundRequest is the JSON body DigitalOcean / OpenAI expect.
// stream is always false — we need a full JSON response to compare models.
type outboundRequest struct {
	Model    string           `json:"model"`
	Messages []models.Message `json:"messages"`
	Stream   bool             `json:"stream"`
}

// Complete posts the request to the configured LLM and waits for the response.
func (c *HTTPClient) Complete(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}
	if model == "" {
		return nil, fmt.Errorf("%s llm: model is required (set request.model or PRIMARY_MODEL/CANDIDATE_MODEL)", c.name)
	}

	payload := outboundRequest{
		Model:    model,
		Messages: req.Messages,
		Stream:   false, // never stream — shadow compare needs complete JSON
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", c.name, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", c.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call %s llm: %w", c.name, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s response: %w", c.name, err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s llm status %d: %s", c.name, resp.StatusCode, string(respBody))
	}

	var chatResp models.ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", c.name, err)
	}

	return &chatResp, nil
}

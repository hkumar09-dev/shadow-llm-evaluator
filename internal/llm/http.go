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

// HTTPClient POSTs OpenAI-compatible chat completions to a remote LLM endpoint.
type HTTPClient struct {
	appCtx       context.Context
	name         string
	baseURL      string
	apiKey       string
	defaultModel string
	httpClient   *http.Client
}

// ClientOption configures an HTTPClient.
type ClientOption func(*HTTPClient)

// WithAPIKey sets the Bearer token.
func WithAPIKey(key string) ClientOption {
	return func(c *HTTPClient) {
		c.apiKey = key
	}
}

// WithDefaultModel sets the model id when the request omits one.
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
func NewHTTPClient(appCtx context.Context, name, baseURL string, opts ...ClientOption) *HTTPClient {
	if appCtx == nil {
		appCtx = context.Background()
	}
	c := &HTTPClient{
		appCtx:  appCtx,
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

// NewPrimaryClientWithTimeout creates a primary HTTP client.
func NewPrimaryClientWithTimeout(appCtx context.Context, baseURL string, timeout time.Duration, opts ...ClientOption) *HTTPClient {
	opts = append([]ClientOption{WithTimeout(timeout)}, opts...)
	return NewHTTPClient(appCtx, "primary", baseURL, opts...)
}

// NewCandidateClientWithTimeout creates a candidate HTTP client.
func NewCandidateClientWithTimeout(appCtx context.Context, baseURL string, timeout time.Duration, opts ...ClientOption) *HTTPClient {
	opts = append([]ClientOption{WithTimeout(timeout)}, opts...)
	return NewHTTPClient(appCtx, "candidate", baseURL, opts...)
}

type outboundRequest struct {
	Model    string           `json:"model"`
	Messages []models.Message `json:"messages"`
	Stream   bool             `json:"stream"`
}

// Complete posts the request to the configured LLM and waits for the response.
func (c *HTTPClient) Complete(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.appCtx.Err(); err != nil {
		return nil, fmt.Errorf("%s llm: app shutting down: %w", c.name, err)
	}

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
		Stream:   false,
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
	defer func() { _ = resp.Body.Close() }()

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

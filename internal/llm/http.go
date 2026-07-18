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

// HTTPClient POSTs chat requests to a remote LLM endpoint.
type HTTPClient struct {
	name       string
	baseURL    string
	httpClient *http.Client
}

// NewHTTPClient creates a named HTTP completer targeting baseURL.
func NewHTTPClient(name, baseURL string) *HTTPClient {
	return &HTTPClient{
		name:    name,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewPrimaryClient is a convenience constructor for the primary HTTP LLM.
func NewPrimaryClient(baseURL string) *HTTPClient {
	return NewHTTPClient("primary", baseURL)
}

// NewCandidateClient is a convenience constructor for the candidate HTTP LLM.
func NewCandidateClient(baseURL string) *HTTPClient {
	return NewHTTPClient("candidate", baseURL)
}

// Complete posts the request to the configured LLM and waits for the response.
func (c *HTTPClient) Complete(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal %s request: %w", c.name, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", c.name, err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

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

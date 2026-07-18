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

// PrimaryClient synchronously POSTs chat requests to a primary LLM endpoint.
type PrimaryClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPrimaryClient creates a client that calls the given primary LLM URL.
func NewPrimaryClient(baseURL string) *PrimaryClient {
	return &PrimaryClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Complete posts the request to the primary LLM and waits for the response.
func (c *PrimaryClient) Complete(ctx context.Context, req models.ChatRequest) (*models.ChatResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal primary request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create primary request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call primary llm: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read primary response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("primary llm status %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp models.ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("decode primary response: %w", err)
	}

	return &chatResp, nil
}

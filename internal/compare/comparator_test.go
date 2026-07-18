package compare_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/compare"
	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/models"
)

func TestContentComparator_equal(t *testing.T) {
	ctx := context.Background()
	c := compare.NewContentComparator(ctx)
	a := response("primary-sim", `{"answer":1}`)
	b := response("candidate-sim", `{"answer":1}`)

	res := c.Compare(ctx, a, b)
	if !res.Equal {
		t.Fatalf("expected equal contents")
	}
}

func TestContentComparator_mismatchBuildsCleanJSONPayloads(t *testing.T) {
	ctx := context.Background()
	c := compare.NewContentComparator(ctx)
	a := response("primary-sim", `prefix {"answer":1,"ok":true}`)
	b := response("candidate-sim", `{"answer":2,"ok":false}`)

	res := c.Compare(ctx, a, b)
	if res.Equal {
		t.Fatalf("expected mismatch")
	}

	var primary, candidate map[string]any
	if err := json.Unmarshal(res.PrimaryPayload, &primary); err != nil {
		t.Fatalf("primary payload not JSON: %v (%s)", err, res.PrimaryPayload)
	}
	if err := json.Unmarshal(res.CandidatePayload, &candidate); err != nil {
		t.Fatalf("candidate payload not JSON: %v (%s)", err, res.CandidatePayload)
	}

	if primary["model"] != "primary-sim" || candidate["model"] != "candidate-sim" {
		t.Fatalf("models not preserved in payloads: %v %v", primary, candidate)
	}
	if primary["content"] == candidate["content"] {
		t.Fatalf("expected different content fields in mismatch payloads")
	}

	primaryExtracted, _ := primary["extracted_json"].(map[string]any)
	candidateExtracted, _ := candidate["extracted_json"].(map[string]any)
	if primaryExtracted["answer"].(float64) != 1 {
		t.Fatalf("unexpected primary extracted_json: %v", primaryExtracted)
	}
	if candidateExtracted["answer"].(float64) != 2 {
		t.Fatalf("unexpected candidate extracted_json: %v", candidateExtracted)
	}
}

func response(model, content string) *models.ChatResponse {
	return &models.ChatResponse{
		Model: model,
		Choices: []models.Choice{{
			Message: models.Message{Role: "assistant", Content: content},
		}},
	}
}

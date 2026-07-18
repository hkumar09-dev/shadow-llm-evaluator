package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hkumar09-dev/shadow-llm-evaluator/internal/config"
)

func TestLoad_localEnvFile(t *testing.T) {
	// Isolate from the developer's real environment variables.
	clearConfigEnv(t)

	dir := t.TempDir()
	content := "" +
		"APP_ENV=local\n" +
		"ADDR=:9090\n" +
		"GIN_MODE=debug\n" +
		"LOG_LEVEL=debug\n" +
		"PRIMARY_LLM_URL=\n" +
		"CANDIDATE_LLM_URL=\n" +
		"SHADOW_TIMEOUT_SECONDS=12\n" +
		"SHADOW_MAX_INFLIGHT=7\n" +
		"HTTP_CLIENT_TIMEOUT_SECONDS=9\n"
	path := filepath.Join(dir, ".env.local")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("APP_ENV", "local")
	t.Setenv("ENV_FILE", path)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9090" {
		t.Fatalf("Addr=%q", cfg.Addr)
	}
	if cfg.ShadowMaxInflight != 7 {
		t.Fatalf("ShadowMaxInflight=%d", cfg.ShadowMaxInflight)
	}
	if !cfg.UsePrimarySimulator() || !cfg.UseCandidateSimulator() {
		t.Fatal("expected simulators when URLs are empty")
	}
}

func TestLoad_prodRequiresLLMURLs(t *testing.T) {
	clearConfigEnv(t)

	dir := t.TempDir()
	content := "" +
		"APP_ENV=prod\n" +
		"ADDR=:8080\n" +
		"GIN_MODE=release\n" +
		"LOG_LEVEL=info\n" +
		"PRIMARY_LLM_URL=\n" +
		"CANDIDATE_LLM_URL=\n" +
		"PRIMARY_MODEL=\n" +
		"CANDIDATE_MODEL=\n" +
		"MODEL_ACCESS_KEY=\n" +
		"SHADOW_TIMEOUT_SECONDS=30\n" +
		"SHADOW_MAX_INFLIGHT=32\n" +
		"HTTP_CLIENT_TIMEOUT_SECONDS=30\n"
	path := filepath.Join(dir, ".env.prod")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("APP_ENV", "prod")
	t.Setenv("ENV_FILE", path)

	if _, err := config.Load(); err == nil {
		t.Fatal("expected error when prod URLs/key/models are empty")
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"APP_ENV", "ENV_FILE", "ENV_DIR", "ADDR", "GIN_MODE", "LOG_LEVEL",
		"PRIMARY_LLM_URL", "CANDIDATE_LLM_URL", "PRIMARY_MODEL", "CANDIDATE_MODEL",
		"MODEL_ACCESS_KEY",
		"SHADOW_TIMEOUT_SECONDS", "SHADOW_MAX_INFLIGHT", "HTTP_CLIENT_TIMEOUT_SECONDS",
	}
	for _, k := range keys {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}
}

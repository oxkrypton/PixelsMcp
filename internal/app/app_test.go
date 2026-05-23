package app

import (
	"reflect"
	"testing"
)

func TestConfigFromEnvDefaultsToStdio(t *testing.T) {
	cfg, err := configFromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatalf("configFromEnv returned error: %v", err)
	}

	if cfg.transport != transportStdio {
		t.Fatalf("transport = %q, want %q", cfg.transport, transportStdio)
	}
	if cfg.addr != defaultHTTPAddr {
		t.Fatalf("addr = %q, want %q", cfg.addr, defaultHTTPAddr)
	}
	if cfg.mcpEndpoint != defaultMCPEndpoint {
		t.Fatalf("mcpEndpoint = %q, want %q", cfg.mcpEndpoint, defaultMCPEndpoint)
	}
	if cfg.healthEndpoint != defaultHealthEndpoint {
		t.Fatalf("healthEndpoint = %q, want %q", cfg.healthEndpoint, defaultHealthEndpoint)
	}
	if cfg.baseURL != defaultBaseURL {
		t.Fatalf("baseURL = %q, want %q", cfg.baseURL, defaultBaseURL)
	}
	if cfg.imageModel != defaultImageModel {
		t.Fatalf("imageModel = %q, want %q", cfg.imageModel, defaultImageModel)
	}
	if cfg.imageSaveDir != defaultImageSaveDir {
		t.Fatalf("imageSaveDir = %q, want %q", cfg.imageSaveDir, defaultImageSaveDir)
	}
}

func TestConfigFromEnvHTTP(t *testing.T) {
	env := map[string]string{
		"PIXELSMCP_TRANSPORT":       "HTTP",
		"PIXELSMCP_ADDR":            "127.0.0.1:9000",
		"PIXELSMCP_ENDPOINT":        "api/mcp/",
		"PIXELSMCP_HEALTH_ENDPOINT": "ready",
		"PIXELSMCP_CORS_ORIGINS":    "https://example.com, https://app.example.com ",
		"PIXELSMCP_API_KEY":         "test-key",
		"PIXELSMCP_BASE_URL":        "https://example.invalid",
		"PIXELSMCP_IMAGE_MODEL":     "Custom/Model",
		"PIXELSMCP_IMAGE_SAVE_DIR":  "/tmp/pixelsmcp",
	}

	cfg, err := configFromEnv(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("configFromEnv returned error: %v", err)
	}

	if cfg.transport != transportHTTP {
		t.Fatalf("transport = %q, want %q", cfg.transport, transportHTTP)
	}
	if cfg.addr != "127.0.0.1:9000" {
		t.Fatalf("addr = %q, want 127.0.0.1:9000", cfg.addr)
	}
	if cfg.mcpEndpoint != "/api/mcp" {
		t.Fatalf("mcpEndpoint = %q, want /api/mcp", cfg.mcpEndpoint)
	}
	if cfg.healthEndpoint != "/ready" {
		t.Fatalf("healthEndpoint = %q, want /ready", cfg.healthEndpoint)
	}
	if cfg.apiKey != "test-key" {
		t.Fatalf("apiKey = %q, want test-key", cfg.apiKey)
	}
	if cfg.baseURL != "https://example.invalid" {
		t.Fatalf("baseURL = %q, want https://example.invalid", cfg.baseURL)
	}
	if cfg.imageModel != "Custom/Model" {
		t.Fatalf("imageModel = %q, want Custom/Model", cfg.imageModel)
	}
	if cfg.imageSaveDir != "/tmp/pixelsmcp" {
		t.Fatalf("imageSaveDir = %q, want /tmp/pixelsmcp", cfg.imageSaveDir)
	}
	wantOrigins := []string{"https://example.com", "https://app.example.com"}
	if !reflect.DeepEqual(cfg.corsOrigins, wantOrigins) {
		t.Fatalf("corsOrigins = %#v, want %#v", cfg.corsOrigins, wantOrigins)
	}
}

func TestConfigFromEnvRejectsInvalidTransport(t *testing.T) {
	_, err := configFromEnv(func(key string) string {
		if key == "PIXELSMCP_TRANSPORT" {
			return "tcp"
		}
		return ""
	})
	if err == nil {
		t.Fatal("configFromEnv returned nil error")
	}
}

func TestConfigFromEnvRejectsSharedEndpoints(t *testing.T) {
	env := map[string]string{
		"PIXELSMCP_ENDPOINT":        "/mcp",
		"PIXELSMCP_HEALTH_ENDPOINT": "/mcp/",
	}

	_, err := configFromEnv(func(key string) string { return env[key] })
	if err == nil {
		t.Fatal("configFromEnv returned nil error")
	}
}


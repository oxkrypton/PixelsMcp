package app

import (
	"reflect"
	"strings"
	"testing"
	"time"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

func TestConfigFromEnvDefaults(t *testing.T) {
	cfg, err := configFromEnv(func(string) string { return "" })
	if err != nil {
		t.Fatalf("configFromEnv returned error: %v", err)
	}

	if cfg.transport != transportHTTP {
		t.Fatalf("transport = %q, want %q", cfg.transport, transportHTTP)
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
	if cfg.provider != imagegen.DefaultProvider {
		t.Fatalf("provider = %q, want %q", cfg.provider, imagegen.DefaultProvider)
	}
	if cfg.baseURL != "" {
		t.Fatalf("baseURL = %q, want empty", cfg.baseURL)
	}
	if cfg.model != imagegen.DefaultModel {
		t.Fatalf("model = %q, want %q", cfg.model, imagegen.DefaultModel)
	}
	if cfg.timeout != imagegen.DefaultRequestTimeout {
		t.Fatalf("timeout = %v, want %v", cfg.timeout, imagegen.DefaultRequestTimeout)
	}
	if cfg.imageSaveDir != imagegen.DefaultSaveDir {
		t.Fatalf("imageSaveDir = %q, want %q", cfg.imageSaveDir, imagegen.DefaultSaveDir)
	}
}

func TestConfigFromEnvAllowsDeveloperStdioTransport(t *testing.T) {
	cfg, err := configFromEnv(func(key string) string {
		if key == "PIXELSMCP_TRANSPORT" {
			return "stdio"
		}
		return ""
	})
	if err != nil {
		t.Fatalf("configFromEnv returned error: %v", err)
	}

	if cfg.transport != transportStdio {
		t.Fatalf("transport = %q, want %q", cfg.transport, transportStdio)
	}
}

func TestConfigFromEnvHTTP(t *testing.T) {
	env := map[string]string{
		"PIXELSMCP_TRANSPORT":       "HTTP",
		"PIXELSMCP_ADDR":            "127.0.0.1:9000",
		"PIXELSMCP_ENDPOINT":        "api/mcp/",
		"PIXELSMCP_HEALTH_ENDPOINT": "ready",
		"PIXELSMCP_CORS_ORIGINS":    "https://example.com, https://app.example.com ",
		"PIXELSMCP_PROVIDER":        "openai-compatible",
		"PIXELSMCP_API_KEY":         "test-key",
		"PIXELSMCP_BASE_URL":        "https://example.invalid",
		"PIXELSMCP_MODEL":           "Custom/Model",
		"PIXELSMCP_EXTRA_HEADERS":   `{"X-Client":"PixelsMcp","X-Env":"test"}`,
		"PIXELSMCP_TIMEOUT":         "45s",
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
	if cfg.provider != "openai-compatible" {
		t.Fatalf("provider = %q, want openai-compatible", cfg.provider)
	}
	if cfg.apiKey != "test-key" {
		t.Fatalf("apiKey = %q, want test-key", cfg.apiKey)
	}
	if cfg.baseURL != "https://example.invalid" {
		t.Fatalf("baseURL = %q, want https://example.invalid", cfg.baseURL)
	}
	if cfg.model != "Custom/Model" {
		t.Fatalf("model = %q, want Custom/Model", cfg.model)
	}
	if cfg.timeout != 45*time.Second {
		t.Fatalf("timeout = %v, want 45s", cfg.timeout)
	}
	if cfg.imageSaveDir != "/tmp/pixelsmcp" {
		t.Fatalf("imageSaveDir = %q, want /tmp/pixelsmcp", cfg.imageSaveDir)
	}
	wantHeaders := map[string]string{"X-Client": "PixelsMcp", "X-Env": "test"}
	if !reflect.DeepEqual(cfg.extraHeaders, wantHeaders) {
		t.Fatalf("extraHeaders = %#v, want %#v", cfg.extraHeaders, wantHeaders)
	}
	wantOrigins := []string{"https://example.com", "https://app.example.com"}
	if !reflect.DeepEqual(cfg.corsOrigins, wantOrigins) {
		t.Fatalf("corsOrigins = %#v, want %#v", cfg.corsOrigins, wantOrigins)
	}
}

func TestWriteUsageDescribesHTTPDefaultAndDeveloperSetup(t *testing.T) {
	var out strings.Builder
	writeUsage(&out)

	content := out.String()
	for _, want := range []string{
		"pixelsmcp           Run the HTTP MCP service",
		"pixelsmcp init      Create or update developer .env.local",
		"Set PIXELSMCP_TRANSPORT=stdio only for local developer MCP debugging.",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("usage missing %q:\n%s", want, content)
		}
	}
}

func TestConfigFromEnvModelPrefersCurrentNameOverLegacyName(t *testing.T) {
	env := map[string]string{
		"PIXELSMCP_MODEL":       "Current/Model",
		"PIXELSMCP_IMAGE_MODEL": "Legacy/Model",
	}

	cfg, err := configFromEnv(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("configFromEnv returned error: %v", err)
	}

	if cfg.model != "Current/Model" {
		t.Fatalf("model = %q, want Current/Model", cfg.model)
	}
}

func TestConfigFromEnvModelFallbackToLegacyName(t *testing.T) {
	env := map[string]string{
		"PIXELSMCP_IMAGE_MODEL": "Legacy/Model",
	}

	cfg, err := configFromEnv(func(key string) string { return env[key] })
	if err != nil {
		t.Fatalf("configFromEnv returned error: %v", err)
	}

	if cfg.model != "Legacy/Model" {
		t.Fatalf("model = %q, want Legacy/Model", cfg.model)
	}
}

func TestConfigFromEnvRejectsInvalidProvider(t *testing.T) {
	_, err := configFromEnv(func(key string) string {
		if key == "PIXELSMCP_PROVIDER" {
			return "unsupported"
		}
		return ""
	})
	if err == nil {
		t.Fatal("configFromEnv returned nil error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %v, want provider value", err)
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

func TestConfigFromEnvRejectsInvalidTimeout(t *testing.T) {
	_, err := configFromEnv(func(key string) string {
		if key == "PIXELSMCP_TIMEOUT" {
			return "not-a-duration"
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

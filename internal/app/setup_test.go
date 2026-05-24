package app

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInitInteractiveWritesAndVerifiesConfig(t *testing.T) {
	var modelListCalls int

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization header = %q, want Bearer test-key", got)
			}
			if got := r.Header.Get("X-Client"); got != "PixelsMcp" {
				t.Fatalf("extra header = %q, want PixelsMcp", got)
			}
			modelListCalls++
			_, _ = w.Write([]byte(`{"data":[{"id":"alpha"},{"id":"Custom/Model"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, ".env.local")

	stdin := strings.NewReader("\n" +
		apiSrv.URL + "\n" +
		"test-key\n" +
		"{\"X-Client\":\"PixelsMcp\"}\n" +
		"15s\n" +
		"2\n")
	var stdout bytes.Buffer

	err := runInit(configPath, stdin, &stdout, io.Discard, func(string) string { return "" }, true)
	if err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	if modelListCalls != 2 {
		t.Fatalf("model list request count = %d, want 2", modelListCalls)
	}
	if strings.Contains(stdout.String(), "Model list unavailable") {
		t.Fatalf("unexpected fallback prompt output:\n%s", stdout.String())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		`PIXELSMCP_PROVIDER="openai-compatible"`,
		`PIXELSMCP_TRANSPORT="http"`,
		`PIXELSMCP_BASE_URL="` + apiSrv.URL + `"`,
		`PIXELSMCP_API_KEY="test-key"`,
		`PIXELSMCP_MODEL="Custom/Model"`,
		`PIXELSMCP_EXTRA_HEADERS="{\"X-Client\":\"PixelsMcp\"}"`,
		`PIXELSMCP_TIMEOUT="15s"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("config file missing %q:\n%s", want, content)
		}
	}
}

func TestRunInitNonInteractiveWritesConfig(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[{"id":"Custom/Model"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	env := map[string]string{
		"PIXELSMCP_PROVIDER":       "openai-compatible",
		"PIXELSMCP_API_KEY":        "test-key",
		"PIXELSMCP_BASE_URL":       apiSrv.URL,
		"PIXELSMCP_MODEL":          "Custom/Model",
		"PIXELSMCP_EXTRA_HEADERS":  `{"X-Client":"PixelsMcp"}`,
		"PIXELSMCP_TIMEOUT":        "20s",
		"PIXELSMCP_IMAGE_SAVE_DIR": "/tmp/pixelsmcp",
	}

	dir := t.TempDir()
	configPath := filepath.Join(dir, ".env.local")

	err := runInit(configPath, strings.NewReader(""), io.Discard, io.Discard, func(key string) string { return env[key] }, false)
	if err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		`PIXELSMCP_PROVIDER="openai-compatible"`,
		`PIXELSMCP_TRANSPORT="http"`,
		`PIXELSMCP_BASE_URL="` + apiSrv.URL + `"`,
		`PIXELSMCP_API_KEY="test-key"`,
		`PIXELSMCP_MODEL="Custom/Model"`,
		`PIXELSMCP_EXTRA_HEADERS="{\"X-Client\":\"PixelsMcp\"}"`,
		`PIXELSMCP_TIMEOUT="20s"`,
		`PIXELSMCP_IMAGE_SAVE_DIR="/tmp/pixelsmcp"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("config file missing %q:\n%s", want, content)
		}
	}
}

func TestNonInteractiveInitConfigRejectsUnsupportedProviderWithValue(t *testing.T) {
	_, err := nonInteractiveInitConfig(config{
		provider: "unsupported",
		apiKey:   "test-key",
		baseURL:  "https://example.invalid",
	})
	if err == nil {
		t.Fatal("nonInteractiveInitConfig returned nil error")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("error = %v, want provider value", err)
	}
}

func TestRunInitRollsBackOnValidationFailure(t *testing.T) {
	var modelListCalls int
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			modelListCalls++
			if modelListCalls == 1 {
				_, _ = w.Write([]byte(`{"data":[{"id":"alpha"},{"id":"Custom/Model"}]}`))
				return
			}
			http.Error(w, "validation failed", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, ".env.local")
	oldContent := []byte("OLD=1\n")
	if err := os.WriteFile(configPath, oldContent, 0o600); err != nil {
		t.Fatalf("seed config file: %v", err)
	}

	stdin := strings.NewReader("\n" +
		apiSrv.URL + "\n" +
		"test-key\n" +
		"{\"X-Client\":\"PixelsMcp\"}\n" +
		"15s\n" +
		"2\n")

	err := runInit(configPath, stdin, io.Discard, io.Discard, func(string) string { return "" }, true)
	if err == nil {
		t.Fatal("runInit returned nil error")
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read rolled back config file: %v", err)
	}
	if !bytes.Equal(data, oldContent) {
		t.Fatalf("config file = %q, want rollback to %q", string(data), string(oldContent))
	}
}

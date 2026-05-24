package imagegen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatibleProviderGenerate(t *testing.T) {
	var captured openAICompatibleGenerationRequest

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case openAIGenerationPath:
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization header = %q, want Bearer test-key", got)
			}
			if got := r.Header.Get("X-Client"); got != "PixelsMcp" {
				t.Fatalf("extra header = %q, want PixelsMcp", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("X-Trace-Id", "trace-123")
			_, _ = w.Write([]byte(`{"images":[{"url":"https://example.invalid/image.png"}],"timings":{"inference":9.5},"seed":321,"model":"Returned/Model"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	provider, err := NewProvider(ProviderConfig{
		Provider:     ProviderOpenAICompatible,
		APIKey:       "test-key",
		BaseURL:      apiSrv.URL,
		Model:        "Requested/Model",
		ExtraHeaders: map[string]string{"X-Client": "PixelsMcp"},
		Client:       apiSrv.Client(),
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	result, err := provider.Generate(context.Background(), "a blue robot")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if captured.Model != "Requested/Model" {
		t.Fatalf("model = %q, want Requested/Model", captured.Model)
	}
	if captured.Prompt != "a blue robot" {
		t.Fatalf("prompt = %q, want a blue robot", captured.Prompt)
	}
	if result.Model != "Returned/Model" {
		t.Fatalf("result model = %q, want Returned/Model", result.Model)
	}
	if result.ImageURL != "https://example.invalid/image.png" {
		t.Fatalf("imageURL = %q, want image url", result.ImageURL)
	}
	if result.TraceID != "trace-123" {
		t.Fatalf("traceID = %q, want trace-123", result.TraceID)
	}
	if result.Seed != 321 {
		t.Fatalf("seed = %d, want 321", result.Seed)
	}
	if result.InferenceMS != 9.5 {
		t.Fatalf("inferenceMS = %v, want 9.5", result.InferenceMS)
	}
}

func TestOpenAICompatibleProviderListModelsAndValidate(t *testing.T) {
	var seenPaths []string

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPaths = append(seenPaths, r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization header = %q, want Bearer test-key", got)
		}
		if got := r.Header.Get("X-Client"); got != "PixelsMcp" {
			t.Fatalf("extra header = %q, want PixelsMcp", got)
		}

		switch r.URL.Path {
		case openAIModelsPath:
			_, _ = w.Write([]byte(`{"data":[{"id":"alpha"},{"id":"Custom/Model"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	provider, err := NewProvider(ProviderConfig{
		APIKey:       "test-key",
		BaseURL:      apiSrv.URL,
		Model:        "Custom/Model",
		ExtraHeaders: map[string]string{"X-Client": "PixelsMcp"},
		Client:       apiSrv.Client(),
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	models, err := provider.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels returned error: %v", err)
	}
	if got, want := strings.Join(models, ","), "alpha,Custom/Model"; got != want {
		t.Fatalf("models = %q, want %q", got, want)
	}

	if err := provider.Validate(context.Background()); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	if got, want := strings.Join(seenPaths, ","), "/v1/models,/v1/models"; got != want {
		t.Fatalf("paths = %q, want %q", got, want)
	}
}

func TestOpenAICompatibleProviderReturnsStatusError(t *testing.T) {
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "provider boom", http.StatusServiceUnavailable)
	}))
	defer apiSrv.Close()

	provider, err := NewProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: apiSrv.URL,
		Model:   "Custom/Model",
		Client:  apiSrv.Client(),
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	if _, err := provider.Generate(context.Background(), "a blue robot"); err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Fatalf("Generate error = %v, want status 503", err)
	}
}

package imagegen

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newTestHTTPClient(handler func(*http.Request) (*http.Response, error)) *http.Client {
	return &http.Client{Transport: roundTripperFunc(handler)}
}

func newTestResponse(status int, body string, headers map[string]string) *http.Response {
	h := make(http.Header)
	for key, value := range headers {
		h.Set(key, value)
	}

	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Header:     h,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestOpenAICompatibleProviderGenerate(t *testing.T) {
	var captured map[string]any

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
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
			return newTestResponse(http.StatusOK, `{"images":[{"url":"https://example.invalid/image.png"}],"timings":{"inference":9.5},"seed":321,"model":"Returned/Model"}`, map[string]string{
				"X-Trace-Id": "trace-123",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	provider, err := NewProvider(ProviderConfig{
		Provider:     ProviderOpenAICompatible,
		APIKey:       "test-key",
		BaseURL:      "http://example.invalid",
		Model:        "Requested/Model",
		ExtraHeaders: map[string]string{"X-Client": "PixelsMcp"},
		Client:       client,
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	result, err := provider.Generate(context.Background(), "a blue robot", GenerationOptions{
		ImageSize:         "1024x1024",
		GuidanceScale:     7.25,
		NumInferenceSteps: 32,
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if got, ok := captured["model"].(string); !ok || got != "Requested/Model" {
		t.Fatalf("model = %#v, want Requested/Model", captured["model"])
	}
	if got, ok := captured["prompt"].(string); !ok || got != "a blue robot" {
		t.Fatalf("prompt = %#v, want a blue robot", captured["prompt"])
	}
	if got, ok := captured["image_size"].(string); !ok || got != "1024x1024" {
		t.Fatalf("image_size = %#v, want 1024x1024", captured["image_size"])
	}
	if got, ok := captured["guidance_scale"].(float64); !ok || got != 7.25 {
		t.Fatalf("guidance_scale = %#v, want 7.25", captured["guidance_scale"])
	}
	if got, ok := captured["num_inference_steps"].(float64); !ok || got != 32 {
		t.Fatalf("num_inference_steps = %#v, want 32", captured["num_inference_steps"])
	}
	if _, ok := captured["batch_size"]; ok {
		t.Fatalf("batch_size = %#v, want omitted", captured["batch_size"])
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

func TestOpenAICompatibleProviderGenerateOmitsUnsetOptions(t *testing.T) {
	var captured map[string]any

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case openAIGenerationPath:
			if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"https://example.invalid/image.png"}],"seed":321}`, nil), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	provider, err := NewProvider(ProviderConfig{
		Provider: ProviderOpenAICompatible,
		APIKey:   "test-key",
		BaseURL:  "http://example.invalid",
		Model:    "Requested/Model",
		Client:   client,
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	if _, err := provider.Generate(context.Background(), "a blue robot", GenerationOptions{}); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	for _, key := range []string{"image_size", "guidance_scale", "num_inference_steps", "batch_size"} {
		if _, ok := captured[key]; ok {
			t.Fatalf("%s = %#v, want omitted", key, captured[key])
		}
	}
}

func TestOpenAICompatibleProviderListModelsAndValidate(t *testing.T) {
	var seenPaths []string

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		seenPaths = append(seenPaths, r.URL.Path)
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("authorization header = %q, want Bearer test-key", got)
		}
		if got := r.Header.Get("X-Client"); got != "PixelsMcp" {
			t.Fatalf("extra header = %q, want PixelsMcp", got)
		}

		switch r.URL.Path {
		case openAIModelsPath:
			return newTestResponse(http.StatusOK, `{"data":[{"id":"alpha"},{"id":"Custom/Model"}]}`, nil), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	provider, err := NewProvider(ProviderConfig{
		APIKey:       "test-key",
		BaseURL:      "http://example.invalid",
		Model:        "Custom/Model",
		ExtraHeaders: map[string]string{"X-Client": "PixelsMcp"},
		Client:       client,
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

func TestOpenAICompatibleProviderValidateRejectsMissingModel(t *testing.T) {
	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case openAIModelsPath:
			return newTestResponse(http.StatusOK, `{"data":[{"id":"alpha"},{"id":"Other/Model"}]}`, nil), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	provider, err := NewProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		Model:   "Custom/Model",
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	if err := provider.Validate(context.Background()); err == nil || !strings.Contains(err.Error(), "Custom/Model") {
		t.Fatalf("Validate error = %v, want missing model error", err)
	}
}

func TestOpenAICompatibleProviderReturnsStatusError(t *testing.T) {
	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		return newTestResponse(http.StatusServiceUnavailable, "provider boom", nil), nil
	})

	provider, err := NewProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		Model:   "Custom/Model",
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	if _, err := provider.Generate(context.Background(), "a blue robot", GenerationOptions{}); err == nil || !strings.Contains(err.Error(), "status 503") {
		t.Fatalf("Generate error = %v, want status 503", err)
	}
}

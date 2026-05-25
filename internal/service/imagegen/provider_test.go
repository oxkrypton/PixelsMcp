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
		Seed:              int64Ptr(42),
		NegativePrompt:    " blurry, low quality ",
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
	if got, ok := captured["negative_prompt"].(string); !ok || got != "blurry, low quality" {
		t.Fatalf("negative_prompt = %#v, want trimmed negative prompt", captured["negative_prompt"])
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
	if got, ok := captured["seed"].(float64); !ok || got != 42 {
		t.Fatalf("seed = %#v, want 42", captured["seed"])
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

	for _, key := range []string{"image", "image_size", "guidance_scale", "num_inference_steps", "seed", "negative_prompt", "batch_size"} {
		if _, ok := captured[key]; ok {
			t.Fatalf("%s = %#v, want omitted", key, captured[key])
		}
	}
}

func TestOpenAICompatibleProviderGenerateWithReferenceImage(t *testing.T) {
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
		Provider:       ProviderOpenAICompatible,
		APIKey:         "test-key",
		BaseURL:        "http://example.invalid",
		Model:          "Text/Model",
		ReferenceModel: "Reference/Model",
		Client:         client,
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	result, err := provider.Generate(context.Background(), "make it cinematic", GenerationOptions{
		ImageSize:      "1024x1024",
		GuidanceScale:  7.5,
		ReferenceImage: "https://example.invalid/reference.png",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if got, ok := captured["model"].(string); !ok || got != "Reference/Model" {
		t.Fatalf("model = %#v, want Reference/Model", captured["model"])
	}
	if got, ok := captured["image"].(string); !ok || got != "https://example.invalid/reference.png" {
		t.Fatalf("image = %#v, want reference image", captured["image"])
	}
	if _, ok := captured["image_size"]; ok {
		t.Fatalf("image_size = %#v, want omitted for reference image model", captured["image_size"])
	}
	if _, ok := captured["guidance_scale"]; ok {
		t.Fatalf("guidance_scale = %#v, want omitted for reference image model", captured["guidance_scale"])
	}
	if result.Model != "Reference/Model" {
		t.Fatalf("result model = %q, want Reference/Model", result.Model)
	}
	if !result.UsedReferenceImage {
		t.Fatal("UsedReferenceImage = false, want true")
	}
}

func TestOpenAICompatibleProviderRequiresReferenceModelForReferenceImage(t *testing.T) {
	provider, err := NewProvider(ProviderConfig{
		Provider: ProviderOpenAICompatible,
		APIKey:   "test-key",
		BaseURL:  "http://example.invalid",
		Model:    "Text/Model",
		Client: newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
			t.Fatal("unexpected provider request")
			return nil, nil
		}),
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	_, err = provider.Generate(context.Background(), "make it cinematic", GenerationOptions{
		ReferenceImage: "https://example.invalid/reference.png",
	})
	if err == nil || !strings.Contains(err.Error(), "reference image model") {
		t.Fatalf("Generate error = %v, want missing reference model error", err)
	}
}

func TestOpenAICompatibleProviderRejectsUnsupportedReferenceImage(t *testing.T) {
	provider, err := NewProvider(ProviderConfig{
		Provider:       ProviderOpenAICompatible,
		APIKey:         "test-key",
		BaseURL:        "http://example.invalid",
		Model:          "Text/Model",
		ReferenceModel: "Reference/Model",
		Client: newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
			t.Fatal("unexpected provider request")
			return nil, nil
		}),
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	for _, referenceImage := range []string{
		"/tmp/reference.png",
		"iVBORw0KGgo=",
		"data:text/plain;base64,SGVsbG8=",
	} {
		_, err = provider.Generate(context.Background(), "make it cinematic", GenerationOptions{
			ReferenceImage: referenceImage,
		})
		if err == nil || !strings.Contains(err.Error(), "http(s) URL or a provider-prepared data:image URL") {
			t.Fatalf("Generate(%q) error = %v, want unsupported reference image error", referenceImage, err)
		}
	}
}

func TestOpenAICompatibleProviderSupportsPreparedDataURLReferenceImage(t *testing.T) {
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
		Provider:       ProviderOpenAICompatible,
		APIKey:         "test-key",
		BaseURL:        "http://example.invalid",
		Model:          "Text/Model",
		ReferenceModel: "Reference/Model",
		Client:         client,
	})
	if err != nil {
		t.Fatalf("NewProvider returned error: %v", err)
	}

	result, err := provider.Generate(context.Background(), "make it cinematic", GenerationOptions{
		ReferenceImage: "data:image/png;base64,iVBORw0KGgo=",
	})
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if got, ok := captured["model"].(string); !ok || got != "Reference/Model" {
		t.Fatalf("model = %#v, want Reference/Model", captured["model"])
	}
	if got, ok := captured["image"].(string); !ok || got != "data:image/png;base64,iVBORw0KGgo=" {
		t.Fatalf("image = %#v, want prepared data url", captured["image"])
	}
	if !result.UsedReferenceImage {
		t.Fatal("UsedReferenceImage = false, want true")
	}
}

func int64Ptr(value int64) *int64 {
	return &value
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

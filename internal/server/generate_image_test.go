package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
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

func fileURI(p string) string {
	return "file://" + filepath.ToSlash(p)
}

type testRootsRequester struct {
	result *mcp.ListRootsResult
	err    error
}

func (r testRootsRequester) RequestRoots(context.Context, mcp.ListRootsRequest) (*mcp.ListRootsResult, error) {
	return r.result, r.err
}

func TestGenerateImageToolReturnsStructuredResult(t *testing.T) {
	var capturedBody map[string]any

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization header = %q, want Bearer test-key", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				t.Fatalf("decode generation request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/images/1.png"}],"timings":{"inference":42.25},"seed":99}`, map[string]string{
				"X-Trace-Id": "trace-123",
			}), nil
		case "/images/1.png":
			return newTestResponse(http.StatusOK, "fake-png", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	saveDir := t.TempDir()
	service, err := imagegen.NewService(imagegen.Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		Model:   "Custom/Model",
		SaveDir: saveDir,
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	handler := &generateImageToolHandler{service: service}
	wrapped := mcp.NewTypedToolHandler(handler.handle)

	result, err := wrapped(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "generate_image",
			Arguments: map[string]any{
				"prompt":              "a white cat sitting on a window",
				"background_color":    "#ff00ff",
				"image_size":          "1024x1024",
				"guidance_scale":      6.75,
				"num_inference_steps": 24,
				"seed":                1234,
				"negative_prompt":     "blurry, gradient background",
			},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handler returned error result: %+v", result)
	}
	if got := len(result.Content); got != 1 {
		t.Fatalf("content length = %d, want 1", got)
	}

	parsed, ok := result.StructuredContent.(*imagegen.Result)
	if !ok {
		t.Fatalf("structured content type = %T, want *imagegen.Result", result.StructuredContent)
	}
	if parsed.ImageURL != "http://example.invalid/images/1.png" {
		t.Fatalf("imageURL = %q, want generated url", parsed.ImageURL)
	}
	if parsed.TraceID != "trace-123" {
		t.Fatalf("traceID = %q, want trace-123", parsed.TraceID)
	}
	if parsed.Seed != 99 {
		t.Fatalf("seed = %d, want 99", parsed.Seed)
	}
	if parsed.InferenceMS != 42.25 {
		t.Fatalf("inferenceMS = %v, want 42.25", parsed.InferenceMS)
	}
	prompt, ok := capturedBody["prompt"].(string)
	if !ok {
		t.Fatalf("generation prompt = %#v, want string", capturedBody["prompt"])
	}
	if parsed.Prompt != prompt {
		t.Fatalf("prompt = %q, want %q", parsed.Prompt, prompt)
	}
	for _, want := range []string{
		"a white cat sitting on a window",
		"Use a SOLID #FF00FF background (#FF00FF) with absolutely no gradients, no transparency.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("generation prompt missing %q:\n%s", want, prompt)
		}
	}
	if parsed.LocalPath == "" {
		t.Fatal("localPath is empty")
	}
	if _, err := os.Stat(parsed.LocalPath); err != nil {
		t.Fatalf("saved image not found: %v", err)
	}
	if got, want := string(mustReadFile(t, parsed.LocalPath)), "fake-png"; got != want {
		t.Fatalf("saved image content = %q, want %q", got, want)
	}
	if capturedBody["model"] != "Custom/Model" {
		t.Fatalf("generation model = %#v, want Custom/Model", capturedBody["model"])
	}
	if got, ok := capturedBody["image_size"].(string); !ok || got != "1024x1024" {
		t.Fatalf("image_size = %#v, want 1024x1024", capturedBody["image_size"])
	}
	if got, ok := capturedBody["guidance_scale"].(float64); !ok || got != 6.75 {
		t.Fatalf("guidance_scale = %#v, want 6.75", capturedBody["guidance_scale"])
	}
	if got, ok := capturedBody["num_inference_steps"].(float64); !ok || got != 24 {
		t.Fatalf("num_inference_steps = %#v, want 24", capturedBody["num_inference_steps"])
	}
	if got, ok := capturedBody["seed"].(float64); !ok || got != 1234 {
		t.Fatalf("seed = %#v, want 1234", capturedBody["seed"])
	}
	if got, ok := capturedBody["negative_prompt"].(string); !ok || got != "blurry, gradient background" {
		t.Fatalf("negative_prompt = %#v, want negative prompt", capturedBody["negative_prompt"])
	}
	if filepath.Dir(parsed.LocalPath) != saveDir {
		t.Fatalf("local path = %q, want file under %q", parsed.LocalPath, saveDir)
	}
}

func TestGenerateImageToolSupportsReferenceImage(t *testing.T) {
	var capturedBody map[string]any

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				t.Fatalf("decode generation request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/images/1.png"}],"seed":99}`, nil), nil
		case "/images/1.png":
			return newTestResponse(http.StatusOK, "fake-png", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	service, err := imagegen.NewService(imagegen.Config{
		APIKey:         "test-key",
		BaseURL:        "http://example.invalid",
		Model:          "Text/Model",
		ReferenceModel: "Reference/Model",
		SaveDir:        t.TempDir(),
		Client:         client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	handler := &generateImageToolHandler{service: service}
	wrapped := mcp.NewTypedToolHandler(handler.handle)

	result, err := wrapped(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "generate_image",
			Arguments: map[string]any{
				"prompt":          "turn the reference into a game item",
				"image_size":      "1024x1024",
				"reference_image": "https://example.invalid/reference.png",
			},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handler returned error result: %+v", result)
	}

	parsed, ok := result.StructuredContent.(*imagegen.Result)
	if !ok {
		t.Fatalf("structured content type = %T, want *imagegen.Result", result.StructuredContent)
	}
	if capturedBody["model"] != "Reference/Model" {
		t.Fatalf("generation model = %#v, want Reference/Model", capturedBody["model"])
	}
	if capturedBody["image"] != "https://example.invalid/reference.png" {
		t.Fatalf("reference image = %#v, want reference url", capturedBody["image"])
	}
	if _, ok := capturedBody["image_size"]; ok {
		t.Fatalf("image_size = %#v, want omitted", capturedBody["image_size"])
	}
	if !parsed.UsedReferenceImage {
		t.Fatal("usedReferenceImage = false, want true")
	}
}

func TestGenerateImageToolUsesClientRootsSaveDir(t *testing.T) {
	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/images/1.png"}],"seed":99}`, nil), nil
		case "/images/1.png":
			return newTestResponse(http.StatusOK, "fake-png", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	rootDir := t.TempDir()
	service, err := imagegen.NewService(imagegen.Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		Model:   "Custom/Model",
		SaveDir: t.TempDir(),
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	handler := &generateImageToolHandler{
		service: service,
		roots: testRootsRequester{
			result: &mcp.ListRootsResult{
				Roots: []mcp.Root{
					{URI: fileURI(rootDir)},
				},
			},
		},
	}
	wrapped := mcp.NewTypedToolHandler(handler.handle)

	result, err := wrapped(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "generate_image",
			Arguments: map[string]any{
				"prompt": "a white cat sitting on a window",
			},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handler returned error result: %+v", result)
	}

	parsed, ok := result.StructuredContent.(*imagegen.Result)
	if !ok {
		t.Fatalf("structured content type = %T, want *imagegen.Result", result.StructuredContent)
	}

	wantDir := filepath.Join(rootDir, "generated-images")
	if filepath.Dir(parsed.LocalPath) != wantDir {
		t.Fatalf("local path = %q, want file under %q", parsed.LocalPath, wantDir)
	}
	if _, err := os.Stat(parsed.LocalPath); err != nil {
		t.Fatalf("saved image not found: %v", err)
	}
}

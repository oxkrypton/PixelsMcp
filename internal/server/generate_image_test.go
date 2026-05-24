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
				"prompt":           "a white cat sitting on a window",
				"background_color": "#ff00ff",
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
	if filepath.Dir(parsed.LocalPath) != saveDir {
		t.Fatalf("local path = %q, want file under %q", parsed.LocalPath, saveDir)
	}
}

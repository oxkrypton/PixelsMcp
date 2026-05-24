package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

func TestGenerateImageToolReturnsStructuredResult(t *testing.T) {
	var capturedBody map[string]any

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization header = %q, want Bearer test-key", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				t.Fatalf("decode generation request: %v", err)
			}
			w.Header().Set("X-Trace-Id", "trace-123")
			_, _ = w.Write([]byte(`{"images":[{"url":"http://` + r.Host + `/images/1.png"}],"timings":{"inference":42.25},"seed":99}`))
		case "/images/1.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("fake-png"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	saveDir := t.TempDir()
	service := imagegen.NewService(imagegen.Config{
		APIKey:  "test-key",
		BaseURL: apiSrv.URL,
		Model:   "Custom/Model",
		SaveDir: saveDir,
		Client:  apiSrv.Client(),
	})

	handler := &generateImageToolHandler{service: service}
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
	if got := len(result.Content); got != 1 {
		t.Fatalf("content length = %d, want 1", got)
	}

	parsed, ok := result.StructuredContent.(*imagegen.Result)
	if !ok {
		t.Fatalf("structured content type = %T, want *imagegen.Result", result.StructuredContent)
	}
	if parsed.Prompt != "a white cat sitting on a window" {
		t.Fatalf("prompt = %q, want prompt", parsed.Prompt)
	}
	if parsed.ImageURL != apiSrv.URL+"/images/1.png" {
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
	if parsed.LocalPath == "" {
		t.Fatal("localPath is empty")
	}
	if _, err := os.Stat(parsed.LocalPath); err != nil {
		t.Fatalf("saved image not found: %v", err)
	}
	if got, want := string(mustReadFile(t, parsed.LocalPath)), "fake-png"; got != want {
		t.Fatalf("saved image content = %q, want %q", got, want)
	}
	if capturedBody["prompt"] != "a white cat sitting on a window" {
		t.Fatalf("generation prompt = %#v, want prompt", capturedBody["prompt"])
	}
	if capturedBody["model"] != "Custom/Model" {
		t.Fatalf("generation model = %#v, want Custom/Model", capturedBody["model"])
	}
	if filepath.Dir(parsed.LocalPath) != saveDir {
		t.Fatalf("local path = %q, want file under %q", parsed.LocalPath, saveDir)
	}
}

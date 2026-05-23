package imagegen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGenerateDownloadsAndSavesImage(t *testing.T) {
	var gotRequest generationRequest

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization header = %q, want Bearer test-key", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("X-Trace-Id", "trace-abc")
			_, _ = w.Write([]byte(`{"images":[{"url":"http://` + r.Host + `/image.png"}],"timings":{"inference":17.5},"seed":123}`))
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("PNGDATA"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	saveDir := t.TempDir()
	svc := NewService(Config{
		APIKey:  "test-key",
		BaseURL: apiSrv.URL,
		Model:   "Custom/Model",
		SaveDir: saveDir,
		Client:  apiSrv.Client(),
	})

	result, err := svc.Generate(context.Background(), "a robot in a garden")
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if gotRequest.Prompt != "a robot in a garden" {
		t.Fatalf("prompt = %q, want prompt", gotRequest.Prompt)
	}
	if gotRequest.Model != "Custom/Model" {
		t.Fatalf("model = %q, want Custom/Model", gotRequest.Model)
	}
	if result.TraceID != "trace-abc" {
		t.Fatalf("traceID = %q, want trace-abc", result.TraceID)
	}
	if result.Seed != 123 {
		t.Fatalf("seed = %d, want 123", result.Seed)
	}
	if result.InferenceMS != 17.5 {
		t.Fatalf("inferenceMS = %v, want 17.5", result.InferenceMS)
	}
	if result.LocalPath == "" {
		t.Fatal("local path is empty")
	}

	data, err := os.ReadFile(result.LocalPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(data) != "PNGDATA" {
		t.Fatalf("saved file content = %q, want PNGDATA", string(data))
	}
}

func TestGenerateRejectsEmptyPrompt(t *testing.T) {
	svc := NewService(Config{APIKey: "test-key"})

	if _, err := svc.Generate(context.Background(), " "); err == nil {
		t.Fatal("Generate returned nil error for empty prompt")
	}
}

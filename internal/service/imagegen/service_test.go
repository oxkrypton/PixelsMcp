package imagegen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGenerateDownloadsAndSavesImage(t *testing.T) {
	var gotRequest openAICompatibleGenerationRequest

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
	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: apiSrv.URL,
		Model:   "Custom/Model",
		SaveDir: saveDir,
		Client:  apiSrv.Client(),
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

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
	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	if _, err := svc.Generate(context.Background(), " "); err == nil {
		t.Fatal("Generate returned nil error for empty prompt")
	}
}

func TestGenerateSpriteSheetBuildsPromptAndSavesImage(t *testing.T) {
	var gotRequest openAICompatibleGenerationRequest

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			w.Header().Set("X-Trace-Id", "trace-sprite")
			_, _ = w.Write([]byte(`{"images":[{"url":"http://` + r.Host + `/sprite.png"}],"timings":{"inference":31.25},"seed":456}`))
		case "/sprite.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("SPRITEDATA"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	saveDir := t.TempDir()
	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: apiSrv.URL,
		Model:   "Custom/Model",
		SaveDir: saveDir,
		Client:  apiSrv.Client(),
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	result, err := svc.GenerateSpriteSheet(context.Background(), SpriteSheetOptions{
		Prompt:     "pixel knight with a blue cape",
		Action:     "dash strike",
		FrameCount: 9,
		Layout:     "3x3",
		Generation: GenerationOptions{
			ImageSize:         "1024x1024",
			GuidanceScale:     7.5,
			NumInferenceSteps: 28,
		},
	})
	if err != nil {
		t.Fatalf("GenerateSpriteSheet returned error: %v", err)
	}

	for _, want := range []string{
		"16-bit pixel art style",
		"pixel knight with a blue cape",
		"Action: dash strike.",
		"Frame count: 9.",
		"Layout: 3x3.",
		"3 by 3 grid",
		"Each frame is exactly 64x64 pixels.",
		"nearest-neighbor filtering",
		"limited color palette",
		"Leave 2px spacing between frames.",
		"solid light-gray background",
	} {
		if !strings.Contains(gotRequest.Prompt, want) {
			t.Fatalf("sprite prompt missing %q:\n%s", want, gotRequest.Prompt)
		}
	}
	if gotRequest.ImageSize != "1024x1024" {
		t.Fatalf("imageSize = %q, want 1024x1024", gotRequest.ImageSize)
	}
	if gotRequest.GuidanceScale != 7.5 {
		t.Fatalf("guidanceScale = %v, want 7.5", gotRequest.GuidanceScale)
	}
	if gotRequest.NumInferenceSteps != 28 {
		t.Fatalf("numInferenceSteps = %d, want 28", gotRequest.NumInferenceSteps)
	}
	if result.SourcePrompt != "pixel knight with a blue cape" {
		t.Fatalf("sourcePrompt = %q, want original prompt", result.SourcePrompt)
	}
	if result.Action != "dash strike" {
		t.Fatalf("action = %q, want dash strike", result.Action)
	}
	if result.FrameCount != 9 {
		t.Fatalf("frameCount = %d, want 9", result.FrameCount)
	}
	if result.Layout != "3x3" {
		t.Fatalf("layout = %q, want 3x3", result.Layout)
	}
	if result.Prompt != gotRequest.Prompt {
		t.Fatalf("result prompt = %q, want generated prompt", result.Prompt)
	}
	if !strings.Contains(result.LocalPath, "sprite-sheet-") {
		t.Fatalf("local path = %q, want sprite-sheet filename", result.LocalPath)
	}

	data, err := os.ReadFile(result.LocalPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(data) != "SPRITEDATA" {
		t.Fatalf("saved file content = %q, want SPRITEDATA", string(data))
	}
}

func TestGenerateSpriteSheetDefaultsToHorizontalLayout(t *testing.T) {
	var gotRequest openAICompatibleGenerationRequest

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			_, _ = w.Write([]byte(`{"images":[{"url":"http://` + r.Host + `/sprite.png"}],"seed":789}`))
		case "/sprite.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("SPRITEDATA"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: apiSrv.URL,
		SaveDir: t.TempDir(),
		Client:  apiSrv.Client(),
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	result, err := svc.GenerateSpriteSheet(context.Background(), SpriteSheetOptions{
		Prompt:     "robot mascot",
		Action:     "idle",
		FrameCount: 4,
	})
	if err != nil {
		t.Fatalf("GenerateSpriteSheet returned error: %v", err)
	}

	if result.Layout != "horizontal" {
		t.Fatalf("layout = %q, want horizontal", result.Layout)
	}
	if !strings.Contains(gotRequest.Prompt, "one horizontal row") {
		t.Fatalf("sprite prompt missing horizontal instruction:\n%s", gotRequest.Prompt)
	}
}

func TestGenerateSpriteSheetRejectsMissingRequiredFields(t *testing.T) {
	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	tests := []struct {
		name string
		opts SpriteSheetOptions
	}{
		{
			name: "empty prompt",
			opts: SpriteSheetOptions{Prompt: " ", Action: "idle", FrameCount: 4},
		},
		{
			name: "empty action",
			opts: SpriteSheetOptions{Prompt: "robot mascot", Action: " ", FrameCount: 4},
		},
		{
			name: "non-positive frame count",
			opts: SpriteSheetOptions{Prompt: "robot mascot", Action: "idle", FrameCount: 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.GenerateSpriteSheet(context.Background(), tt.opts); err == nil {
				t.Fatal("GenerateSpriteSheet returned nil error")
			}
		})
	}
}

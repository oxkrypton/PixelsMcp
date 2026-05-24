package imagegen

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestGenerateDownloadsAndSavesImage(t *testing.T) {
	var gotRequest openAICompatibleGenerationRequest

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
				t.Fatalf("authorization header = %q, want Bearer test-key", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/image.png"}],"timings":{"inference":17.5},"seed":123}`, map[string]string{
				"X-Trace-Id": "trace-abc",
			}), nil
		case "/image.png":
			return newTestResponse(http.StatusOK, "PNGDATA", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	saveDir := t.TempDir()
	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		Model:   "Custom/Model",
		SaveDir: saveDir,
		Client:  client,
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

func TestGenerateWithOptionsBuildsBackgroundColorPrompt(t *testing.T) {
	var gotRequest openAICompatibleGenerationRequest

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/image.png"}],"seed":222}`, nil), nil
		case "/image.png":
			return newTestResponse(http.StatusOK, "PNGDATA", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		Model:   "Custom/Model",
		SaveDir: t.TempDir(),
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	result, err := svc.GenerateWithOptions(context.Background(), "a robot in a garden", GenerationOptions{
		BackgroundColor: "  #ff00ff  ",
		Seed:            int64Ptr(777),
		NegativePrompt:  " blurry, shadows ",
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions returned error: %v", err)
	}

	for _, want := range []string{
		"a robot in a garden",
		"Use a SOLID #FF00FF background (#FF00FF) with absolutely no gradients, no transparency.",
	} {
		if !strings.Contains(gotRequest.Prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, gotRequest.Prompt)
		}
	}
	if result.Prompt != gotRequest.Prompt {
		t.Fatalf("result prompt = %q, want generated prompt", result.Prompt)
	}
	if gotRequest.Seed == nil || *gotRequest.Seed != 777 {
		t.Fatalf("seed = %#v, want 777", gotRequest.Seed)
	}
	if gotRequest.NegativePrompt != "blurry, shadows" {
		t.Fatalf("negativePrompt = %q, want trimmed negative prompt", gotRequest.NegativePrompt)
	}
}

func TestGenerateWithOptionsRejectsInvalidBackgroundColor(t *testing.T) {
	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	tests := []struct {
		name string
		opts GenerationOptions
	}{
		{
			name: "missing hash",
			opts: GenerationOptions{BackgroundColor: "00ff00"},
		},
		{
			name: "short hex",
			opts: GenerationOptions{BackgroundColor: "#123"},
		},
		{
			name: "invalid hex digit",
			opts: GenerationOptions{BackgroundColor: "#12GG34"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.GenerateWithOptions(context.Background(), "a robot in a garden", tt.opts); err == nil {
				t.Fatal("GenerateWithOptions returned nil error")
			}
		})
	}
}

func TestGenerateSpriteSheetBuildsPromptAndSavesImage(t *testing.T) {
	var gotRequest openAICompatibleGenerationRequest

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/sprite.png"}],"timings":{"inference":31.25},"seed":456}`, map[string]string{
				"X-Trace-Id": "trace-sprite",
			}), nil
		case "/sprite.png":
			return newTestResponse(http.StatusOK, "SPRITEDATA", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	saveDir := t.TempDir()
	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		Model:   "Custom/Model",
		SaveDir: saveDir,
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	result, err := svc.GenerateSpriteSheet(context.Background(), SpriteSheetOptions{
		Prompt:      "pixel knight with a blue cape",
		Action:      "dash strike",
		FrameCount:  9,
		Layout:      "3x3",
		FrameWidth:  64,
		FrameHeight: 64,
		Spacing:     2,
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
		"Canvas geometry: total image is exactly 196x196 pixels.",
		"Frame geometry: 9 cells, each cell exactly 64x64 pixels, with exactly 2px spacing between cells and no outer padding.",
		"hard square pixels",
		"nearest-neighbor filtering",
		"limited color palette",
		"Place exactly one character in each cell.",
		"Use a solid light-gray background (#D9D9D9) in every empty pixel, with no texture, no gradients, and no transparency.",
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

func TestGenerateSpriteSheetWithReferenceImageUsesReferenceModel(t *testing.T) {
	var gotRequest openAICompatibleGenerationRequest

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/sprite.png"}],"seed":456}`, nil), nil
		case "/sprite.png":
			return newTestResponse(http.StatusOK, "SPRITEDATA", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	svc, err := NewService(Config{
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

	result, err := svc.GenerateSpriteSheet(context.Background(), SpriteSheetOptions{
		Prompt:     "robot mascot",
		Action:     "idle",
		FrameCount: 4,
		Generation: GenerationOptions{
			ImageSize:      "1024x1024",
			ReferenceImage: "data:image/png;base64,AAA",
		},
	})
	if err != nil {
		t.Fatalf("GenerateSpriteSheet returned error: %v", err)
	}

	if gotRequest.Model != "Reference/Model" {
		t.Fatalf("model = %q, want Reference/Model", gotRequest.Model)
	}
	if gotRequest.Image != "data:image/png;base64,AAA" {
		t.Fatalf("image = %q, want data URL", gotRequest.Image)
	}
	if gotRequest.ImageSize != "" {
		t.Fatalf("imageSize = %q, want omitted", gotRequest.ImageSize)
	}
	if !result.UsedReferenceImage {
		t.Fatal("UsedReferenceImage = false, want true")
	}
}

func TestGenerateSpriteSheetBuildsBackgroundColorPrompt(t *testing.T) {
	var gotRequest openAICompatibleGenerationRequest

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/sprite.png"}],"seed":654}`, nil), nil
		case "/sprite.png":
			return newTestResponse(http.StatusOK, "SPRITEDATA", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		Model:   "Custom/Model",
		SaveDir: t.TempDir(),
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	result, err := svc.GenerateSpriteSheet(context.Background(), SpriteSheetOptions{
		Prompt:      "pixel knight with a blue cape",
		Action:      "dash strike",
		FrameCount:  9,
		Layout:      "3x3",
		FrameWidth:  48,
		FrameHeight: 48,
		Spacing:     4,
		Generation: GenerationOptions{
			BackgroundColor: " #00ff00 ",
		},
	})
	if err != nil {
		t.Fatalf("GenerateSpriteSheet returned error: %v", err)
	}

	for _, want := range []string{
		"Use a SOLID #00FF00 background (#00FF00) with absolutely no gradients, no transparency.",
		"pixel knight with a blue cape",
		"Action: dash strike.",
		"Frame count: 9.",
		"Layout: 3x3.",
		"Canvas geometry: total image is exactly 152x152 pixels.",
		"Frame geometry: 9 cells, each cell exactly 48x48 pixels, with exactly 4px spacing between cells and no outer padding.",
	} {
		if !strings.Contains(gotRequest.Prompt, want) {
			t.Fatalf("sprite prompt missing %q:\n%s", want, gotRequest.Prompt)
		}
	}
	if strings.Contains(gotRequest.Prompt, "light-gray background") {
		t.Fatalf("sprite prompt still contains default background:\n%s", gotRequest.Prompt)
	}
	if result.Prompt != gotRequest.Prompt {
		t.Fatalf("result prompt = %q, want generated prompt", result.Prompt)
	}
}

func TestGenerateSpriteSheetDefaultsToHorizontalLayout(t *testing.T) {
	var gotRequest openAICompatibleGenerationRequest

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/sprite.png"}],"seed":789}`, nil), nil
		case "/sprite.png":
			return newTestResponse(http.StatusOK, "SPRITEDATA", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		SaveDir: t.TempDir(),
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	result, err := svc.GenerateSpriteSheet(context.Background(), SpriteSheetOptions{
		Prompt:      "robot mascot",
		Action:      "idle",
		FrameCount:  4,
		FrameWidth:  64,
		FrameHeight: 64,
		Spacing:     2,
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
	for _, want := range []string{
		"Canvas geometry: total image is exactly 262x64 pixels.",
		"Frame geometry: 4 cells, each cell exactly 64x64 pixels, with exactly 2px spacing between cells and no outer padding.",
	} {
		if !strings.Contains(gotRequest.Prompt, want) {
			t.Fatalf("sprite prompt missing %q:\n%s", want, gotRequest.Prompt)
		}
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

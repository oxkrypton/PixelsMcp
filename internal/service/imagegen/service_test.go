package imagegen

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testReferencePNGData() []byte {
	return []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
}

func writeReferencePNG(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "reference.png")
	if err := os.WriteFile(path, testReferencePNGData(), 0o644); err != nil {
		t.Fatalf("write reference image: %v", err)
	}
	return path
}

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
	if result.SavedPath == "" {
		t.Fatal("saved path is empty")
	}
	if result.LocalPath != result.SavedPath {
		t.Fatalf("localPath = %q, want savedPath %q", result.LocalPath, result.SavedPath)
	}
	if !filepath.IsAbs(result.SavedPath) {
		t.Fatalf("savedPath = %q, want absolute path", result.SavedPath)
	}

	data, err := os.ReadFile(result.LocalPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if string(data) != "PNGDATA" {
		t.Fatalf("saved file content = %q, want PNGDATA", string(data))
	}
}

func TestGenerateWithOptionsSavesToAbsoluteOutputDirectory(t *testing.T) {
	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/image.png"}],"seed":321}`, nil), nil
		case "/image.png":
			return newTestResponse(http.StatusOK, "PNGDATA", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	saveDir := t.TempDir()
	outputDir := t.TempDir()
	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		SaveDir: saveDir,
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	result, err := svc.GenerateWithOptions(context.Background(), "a robot in a garden", GenerationOptions{
		OutputPath: outputDir,
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions returned error: %v", err)
	}

	if filepath.Dir(result.SavedPath) != outputDir {
		t.Fatalf("savedPath = %q, want file under %q", result.SavedPath, outputDir)
	}
	if result.LocalPath != result.SavedPath {
		t.Fatalf("localPath = %q, want savedPath %q", result.LocalPath, result.SavedPath)
	}
	data, err := os.ReadFile(result.SavedPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if got := string(data); got != "PNGDATA" {
		t.Fatalf("saved file content = %q, want PNGDATA", got)
	}
	if entries, err := os.ReadDir(saveDir); err != nil {
		t.Fatalf("read saveDir: %v", err)
	} else if len(entries) != 0 {
		t.Fatalf("saveDir entries = %d, want 0", len(entries))
	}
}

func TestGenerateWithOptionsSavesToAbsoluteOutputFileAndAddsExtension(t *testing.T) {
	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/image"}],"seed":654}`, nil), nil
		case "/image":
			return newTestResponse(http.StatusOK, "JPEGDATA", map[string]string{
				"Content-Type": "image/jpeg",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	outputPath := filepath.Join(t.TempDir(), "custom-image")
	svc, err := NewService(Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
		SaveDir: t.TempDir(),
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	result, err := svc.GenerateWithOptions(context.Background(), "a robot in a garden", GenerationOptions{
		OutputPath: outputPath,
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions returned error: %v", err)
	}

	wantPath := outputPath + ".jpg"
	if result.SavedPath != wantPath {
		t.Fatalf("savedPath = %q, want %q", result.SavedPath, wantPath)
	}
	data, err := os.ReadFile(result.SavedPath)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if got := string(data); got != "JPEGDATA" {
		t.Fatalf("saved file content = %q, want JPEGDATA", got)
	}
}

func TestGenerateWithOptionsRejectsRelativeOutputPath(t *testing.T) {
	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/image.png"}],"seed":123}`, nil), nil
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
		SaveDir: t.TempDir(),
		Client:  client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	_, err = svc.GenerateWithOptions(context.Background(), "a robot in a garden", GenerationOptions{
		OutputPath: "generated-images/slime.png",
	})
	if err == nil {
		t.Fatal("GenerateWithOptions returned nil error")
	}
	if !strings.Contains(err.Error(), "output_path must be an absolute path") {
		t.Fatalf("error = %q, want output_path error", err.Error())
	}
}

func TestGenerateWithOptionsSupportsReferencePath(t *testing.T) {
	var gotRequest openAICompatibleGenerationRequest

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&gotRequest); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/image.png"}],"seed":123}`, nil), nil
		case "/image.png":
			return newTestResponse(http.StatusOK, "PNGDATA", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	referencePath := writeReferencePNG(t)
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

	result, err := svc.GenerateWithOptions(context.Background(), "a robot in a garden", GenerationOptions{
		ImageSize:      "1024x1024",
		ReferencePath:  referencePath,
		GuidanceScale:  7.5,
		NegativePrompt: "blurry",
	})
	if err != nil {
		t.Fatalf("GenerateWithOptions returned error: %v", err)
	}

	if gotRequest.Model != "Reference/Model" {
		t.Fatalf("model = %q, want Reference/Model", gotRequest.Model)
	}
	if gotRequest.Image != "data:image/png;base64,iVBORw0KGgo=" {
		t.Fatalf("image = %q, want reference data URL", gotRequest.Image)
	}
	if gotRequest.ImageSize != "" {
		t.Fatalf("imageSize = %q, want omitted", gotRequest.ImageSize)
	}
	if gotRequest.GuidanceScale != 0 {
		t.Fatalf("guidanceScale = %v, want omitted", gotRequest.GuidanceScale)
	}
	if !result.UsedReferenceImage {
		t.Fatal("UsedReferenceImage = false, want true")
	}
}

func TestGenerateWithOptionsRejectsInvalidReferenceInputs(t *testing.T) {
	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		t.Fatal("unexpected provider request")
		return nil, nil
	})

	svc, err := NewService(Config{
		APIKey:         "test-key",
		BaseURL:        "http://example.invalid",
		ReferenceModel: "Reference/Model",
		SaveDir:        t.TempDir(),
		Client:         client,
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	validReferencePath := writeReferencePNG(t)
	unsupportedPath := filepath.Join(t.TempDir(), "reference.txt")
	if err := os.WriteFile(unsupportedPath, []byte("not an image"), 0o644); err != nil {
		t.Fatalf("write unsupported reference file: %v", err)
	}
	oversizedPath := filepath.Join(t.TempDir(), "reference.png")
	oversizedData := append(testReferencePNGData(), make([]byte, int(maxReferenceImageBytes))...)
	if err := os.WriteFile(oversizedPath, oversizedData, 0o644); err != nil {
		t.Fatalf("write oversized reference file: %v", err)
	}

	tests := []struct {
		name string
		opts GenerationOptions
		want string
	}{
		{
			name: "both reference inputs",
			opts: GenerationOptions{
				ReferenceImage: "https://example.invalid/reference.png",
				ReferencePath:  validReferencePath,
			},
			want: "mutually exclusive",
		},
		{
			name: "data url reference image",
			opts: GenerationOptions{ReferenceImage: "data:image/png;base64,AAA"},
			want: "reference_image must be an http(s) URL",
		},
		{
			name: "raw base64 reference image",
			opts: GenerationOptions{ReferenceImage: "iVBORw0KGgo="},
			want: "reference_image must be an http(s) URL",
		},
		{
			name: "relative reference path",
			opts: GenerationOptions{ReferencePath: "reference.png"},
			want: "reference_path must be an absolute path",
		},
		{
			name: "missing reference path",
			opts: GenerationOptions{ReferencePath: filepath.Join(t.TempDir(), "missing.png")},
			want: "check reference_path",
		},
		{
			name: "directory reference path",
			opts: GenerationOptions{ReferencePath: t.TempDir()},
			want: "regular image file",
		},
		{
			name: "unsupported reference file",
			opts: GenerationOptions{ReferencePath: unsupportedPath},
			want: "PNG, JPEG, GIF, WebP, BMP, or TIFF",
		},
		{
			name: "oversized reference file",
			opts: GenerationOptions{ReferencePath: oversizedPath},
			want: "10 MB",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.GenerateWithOptions(context.Background(), "a robot in a garden", tt.opts)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("GenerateWithOptions error = %v, want it to contain %q", err, tt.want)
			}
		})
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

func TestGenerateSpriteSheetWithReferencePathUsesReferenceModel(t *testing.T) {
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

	referencePath := writeReferencePNG(t)
	result, err := svc.GenerateSpriteSheet(context.Background(), SpriteSheetOptions{
		Prompt:     "robot mascot",
		Action:     "idle",
		FrameCount: 4,
		Generation: GenerationOptions{
			ImageSize:     "1024x1024",
			ReferencePath: referencePath,
		},
	})
	if err != nil {
		t.Fatalf("GenerateSpriteSheet returned error: %v", err)
	}

	if gotRequest.Model != "Reference/Model" {
		t.Fatalf("model = %q, want Reference/Model", gotRequest.Model)
	}
	if gotRequest.Image != "data:image/png;base64,iVBORw0KGgo=" {
		t.Fatalf("image = %q, want reference data URL", gotRequest.Image)
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

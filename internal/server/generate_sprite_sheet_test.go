package server

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

func TestGenerateSpriteSheetToolReturnsStructuredResult(t *testing.T) {
	var capturedBody map[string]any

	client := newTestHTTPClient(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				t.Fatalf("decode generation request: %v", err)
			}
			return newTestResponse(http.StatusOK, `{"images":[{"url":"http://example.invalid/images/sprite.png"}],"timings":{"inference":27.5},"seed":88}`, nil), nil
		case "/images/sprite.png":
			return newTestResponse(http.StatusOK, "fake-sprite", map[string]string{
				"Content-Type": "image/png",
			}), nil
		default:
			return newTestResponse(http.StatusNotFound, "not found", nil), nil
		}
	})

	saveDir := t.TempDir()
	outputPath := filepath.Join(t.TempDir(), "sprite-output.png")
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

	handler := &generateSpriteSheetToolHandler{service: service}
	wrapped := mcp.NewTypedToolHandler(handler.handle)

	result, err := wrapped(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "generate_sprite_sheet",
			Arguments: map[string]any{
				"prompt":              "a wizard with a red robe",
				"action":              "spell cast",
				"frame_count":         6,
				"layout":              "vertical",
				"frame_width":         32,
				"frame_height":        48,
				"spacing":             4,
				"background_color":    "#00ff00",
				"image_size":          "1024x1024",
				"guidance_scale":      6.5,
				"num_inference_steps": 30,
				"seed":                9876,
				"negative_prompt":     "extra limbs, realistic 3D",
				"output_path":         outputPath,
			},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handler returned error result: %+v", result)
	}

	parsed, ok := result.StructuredContent.(*imagegen.SpriteSheetResult)
	if !ok {
		t.Fatalf("structured content type = %T, want *imagegen.SpriteSheetResult", result.StructuredContent)
	}
	if parsed.SourcePrompt != "a wizard with a red robe" {
		t.Fatalf("sourcePrompt = %q, want prompt", parsed.SourcePrompt)
	}
	if parsed.Action != "spell cast" {
		t.Fatalf("action = %q, want spell cast", parsed.Action)
	}
	if parsed.FrameCount != 6 {
		t.Fatalf("frameCount = %d, want 6", parsed.FrameCount)
	}
	if parsed.Layout != "vertical" {
		t.Fatalf("layout = %q, want vertical", parsed.Layout)
	}
	if parsed.LocalPath == "" {
		t.Fatal("localPath is empty")
	}
	if parsed.SavedPath != outputPath {
		t.Fatalf("savedPath = %q, want %q", parsed.SavedPath, outputPath)
	}
	if parsed.LocalPath != parsed.SavedPath {
		t.Fatalf("localPath = %q, want savedPath %q", parsed.LocalPath, parsed.SavedPath)
	}
	if filepath.Dir(parsed.LocalPath) == saveDir {
		t.Fatalf("local path = %q, did not use output_path", parsed.LocalPath)
	}
	if got, want := string(mustReadFile(t, parsed.LocalPath)), "fake-sprite"; got != want {
		t.Fatalf("saved image content = %q, want %q", got, want)
	}

	prompt, ok := capturedBody["prompt"].(string)
	if !ok {
		t.Fatalf("generation prompt = %#v, want string", capturedBody["prompt"])
	}
	for _, want := range []string{"Action: spell cast.", "Frame count: 6.", "Layout: vertical.", "vertical column"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("generation prompt missing %q:\n%s", want, prompt)
		}
	}
	for _, want := range []string{
		"Canvas geometry: total image is exactly 32x308 pixels.",
		"Frame geometry: 6 cells, each cell exactly 32x48 pixels, with exactly 4px spacing between cells and no outer padding.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("generation prompt missing %q:\n%s", want, prompt)
		}
	}
	if !strings.Contains(prompt, "Use a SOLID #00FF00 background (#00FF00) with absolutely no gradients, no transparency.") {
		t.Fatalf("generation prompt missing solid background instruction:\n%s", prompt)
	}
	if strings.Contains(prompt, "light-gray background") {
		t.Fatalf("generation prompt still contains default background:\n%s", prompt)
	}
	if got, ok := capturedBody["image_size"].(string); !ok || got != "1024x1024" {
		t.Fatalf("image_size = %#v, want 1024x1024", capturedBody["image_size"])
	}
	if got, ok := capturedBody["guidance_scale"].(float64); !ok || got != 6.5 {
		t.Fatalf("guidance_scale = %#v, want 6.5", capturedBody["guidance_scale"])
	}
	if got, ok := capturedBody["num_inference_steps"].(float64); !ok || got != 30 {
		t.Fatalf("num_inference_steps = %#v, want 30", capturedBody["num_inference_steps"])
	}
	if got, ok := capturedBody["seed"].(float64); !ok || got != 9876 {
		t.Fatalf("seed = %#v, want 9876", capturedBody["seed"])
	}
	if got, ok := capturedBody["negative_prompt"].(string); !ok || got != "extra limbs, realistic 3D" {
		t.Fatalf("negative_prompt = %#v, want negative prompt", capturedBody["negative_prompt"])
	}
}

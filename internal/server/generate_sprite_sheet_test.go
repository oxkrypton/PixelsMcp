package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

func TestGenerateSpriteSheetToolReturnsStructuredResult(t *testing.T) {
	var capturedBody map[string]any

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images/generations":
			if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
				t.Fatalf("decode generation request: %v", err)
			}
			_, _ = w.Write([]byte(`{"images":[{"url":"http://` + r.Host + `/images/sprite.png"}],"timings":{"inference":27.5},"seed":88}`))
		case "/images/sprite.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("fake-sprite"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiSrv.Close()

	saveDir := t.TempDir()
	service, err := imagegen.NewService(imagegen.Config{
		APIKey:  "test-key",
		BaseURL: apiSrv.URL,
		Model:   "Custom/Model",
		SaveDir: saveDir,
		Client:  apiSrv.Client(),
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
				"prompt":      "a wizard with a red robe",
				"action":      "spell cast",
				"frame_count": 6,
				"layout":      "vertical",
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
	if filepath.Dir(parsed.LocalPath) != saveDir {
		t.Fatalf("local path = %q, want file under %q", parsed.LocalPath, saveDir)
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
}

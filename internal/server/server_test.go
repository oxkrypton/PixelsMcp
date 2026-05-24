package server

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

func TestMCPServerRegistersImageAndSpriteSheetTools(t *testing.T) {
	service, err := imagegen.NewService(imagegen.Config{
		APIKey:  "test-key",
		BaseURL: "http://example.invalid",
	})
	if err != nil {
		t.Fatalf("NewService returned error: %v", err)
	}

	srv := New("PixelsMcp", "0.1.0", service)

	tools := srv.ListTools()
	if got := len(tools); got != 2 {
		t.Fatalf("tool count = %d, want 2", got)
	}
	if _, ok := tools["generate_image"]; !ok {
		t.Fatalf("generate_image tool not registered: %#v", tools)
	}
	if _, ok := tools["generate_sprite_sheet"]; !ok {
		t.Fatalf("generate_sprite_sheet tool not registered: %#v", tools)
	}
}

func TestToolSchemasExposeCommonGenerationFields(t *testing.T) {
	imageSchema := rawToolSchema(t, newGenerateImageTool())
	assertSchemaProperties(t, imageSchema, []string{
		"prompt",
		"background_color",
		"image_size",
		"guidance_scale",
		"num_inference_steps",
		"seed",
		"negative_prompt",
	})

	spriteSchema := rawToolSchema(t, newGenerateSpriteSheetTool())
	assertSchemaProperties(t, spriteSchema, []string{
		"prompt",
		"action",
		"frame_count",
		"layout",
		"background_color",
		"image_size",
		"guidance_scale",
		"num_inference_steps",
		"seed",
		"negative_prompt",
	})
}

func rawToolSchema(t *testing.T, tool mcp.Tool) map[string]any {
	t.Helper()

	var schema map[string]any
	if err := json.Unmarshal(tool.RawInputSchema, &schema); err != nil {
		t.Fatalf("unmarshal raw input schema: %v", err)
	}
	return schema
}

func assertSchemaProperties(t *testing.T, schema map[string]any, names []string) {
	t.Helper()

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %#v, want object", schema["properties"])
	}
	for _, name := range names {
		if _, ok := properties[name]; !ok {
			t.Fatalf("schema missing property %q: %#v", name, properties)
		}
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return data
}

package server

import (
	"os"
	"testing"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

func TestMCPServerRegistersImageAndSpriteSheetTools(t *testing.T) {
	srv := New("PixelsMcp", "0.1.0", imagegen.NewService(imagegen.Config{
		APIKey: "test-key",
	}))

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

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	return data
}

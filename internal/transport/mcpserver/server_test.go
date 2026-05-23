package mcpserver

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/oxkrypton/PixelsMcp/internal/service/echo"
)

func TestEchoToolReturnsSameMessage(t *testing.T) {
	handler := &echoToolHandler{service: echo.NewService()}
	wrapped := mcp.NewTypedToolHandler(handler.handle)

	result, err := wrapped(context.Background(), mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "echo",
			Arguments: map[string]any{
				"message": "hello",
			},
		},
	})
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if result.IsError {
		t.Fatalf("handler returned error result: %+v", result)
	}

	if got := mcp.GetTextFromContent(result.Content[0]); got != "hello" {
		t.Fatalf("echo tool returned %q, want %q", got, "hello")
	}
}

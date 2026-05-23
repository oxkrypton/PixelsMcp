package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

type GenerateImageArgs struct {
	Prompt string `json:"prompt" jsonschema:"Text prompt used to generate the image"`
}

func New(name, version string, imageService *imagegen.Service) *mcpgo.MCPServer {
	srv := mcpgo.NewMCPServer(name, version, mcpgo.WithToolCapabilities(false))

	handler := &generateImageToolHandler{
		service: imageService,
	}

	srv.AddTool(newGenerateImageTool(), mcp.NewTypedToolHandler(handler.handle))

	return srv
}

type generateImageToolHandler struct {
	service *imagegen.Service
}

func (h *generateImageToolHandler) handle(ctx context.Context, _ mcp.CallToolRequest, args GenerateImageArgs) (*mcp.CallToolResult, error) {
	result, err := h.service.Generate(ctx, args.Prompt)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("image generation failed: %v", err)), nil
	}

	return mcp.NewToolResultStructuredOnly(result), nil
}

func newGenerateImageTool() mcp.Tool {
	return mcp.NewTool(
		"generate_image",
		mcp.WithDescription("Generate an image from a prompt, save it locally, and return the file information"),
		mcp.WithInputSchema[GenerateImageArgs](),
	)
}

package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

type GenerateImageArgs struct {
	Prompt string `json:"prompt" jsonschema:"Text prompt used to generate the image"`
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

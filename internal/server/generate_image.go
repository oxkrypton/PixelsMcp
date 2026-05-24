package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

type GenerateImageArgs struct {
	Prompt          string `json:"prompt" jsonschema:"Text prompt used to generate the image"`
	BackgroundColor string `json:"background_color,omitempty" jsonschema:"Optional solid background color in #RRGGBB format, such as #00FF00 or #FF00FF"`
}

type generateImageToolHandler struct {
	service *imagegen.Service
}

func (h *generateImageToolHandler) handle(ctx context.Context, _ mcp.CallToolRequest, args GenerateImageArgs) (*mcp.CallToolResult, error) {
	result, err := h.service.GenerateWithOptions(ctx, args.Prompt, imagegen.GenerationOptions{
		BackgroundColor: args.BackgroundColor,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("image generation failed: %v", err)), nil
	}

	return mcp.NewToolResultStructuredOnly(result), nil
}

func newGenerateImageTool() mcp.Tool {
	return mcp.NewTool(
		"generate_image",
		mcp.WithDescription("Generate an image from a prompt and optional solid background color, save it locally, and return the file information"),
		mcp.WithInputSchema[GenerateImageArgs](),
	)
}

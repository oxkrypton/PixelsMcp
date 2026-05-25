package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

type generateImageToolHandler struct {
	service *imagegen.Service
}

func (h *generateImageToolHandler) handle(ctx context.Context, _ mcp.CallToolRequest, args GenerateImageArgs) (*mcp.CallToolResult, error) {
	result, err := h.service.GenerateWithOptions(ctx, args.Prompt, args.GenerationArgs.generationOptions())
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("image generation failed: %v", err)), nil
	}

	return mcp.NewToolResultStructuredOnly(result), nil
}

func newGenerateImageTool() mcp.Tool {
	return mcp.NewTool(
		"generate_image",
		mcp.WithDescription("Generate an image from a prompt, optional reference image, optional solid background color, optional tuning args, seed, negative prompt, and output_path. Reference images may be http(s) URLs via reference_image or absolute local file paths via reference_path. Save the result on the server. To save into a caller workspace, pass an absolute output_path. The response saved_path is the actual absolute file path written by the server."),
		mcp.WithInputSchema[GenerateImageArgs](),
	)
}

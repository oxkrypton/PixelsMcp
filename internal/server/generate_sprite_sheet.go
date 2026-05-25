package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

type generateSpriteSheetToolHandler struct {
	service *imagegen.Service
}

func (h *generateSpriteSheetToolHandler) handle(ctx context.Context, _ mcp.CallToolRequest, args GenerateSpriteSheetArgs) (*mcp.CallToolResult, error) {
	result, err := h.service.GenerateSpriteSheet(ctx, imagegen.SpriteSheetOptions{
		Prompt:      args.Prompt,
		Action:      args.Action,
		FrameCount:  args.FrameCount,
		Layout:      args.Layout,
		FrameWidth:  args.FrameWidth,
		FrameHeight: args.FrameHeight,
		Spacing:     args.Spacing,
		Generation:  args.GenerationArgs.generationOptions(),
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("sprite sheet generation failed: %v", err)), nil
	}

	return mcp.NewToolResultStructuredOnly(result), nil
}

func newGenerateSpriteSheetTool() mcp.Tool {
	return mcp.NewTool(
		"generate_sprite_sheet",
		mcp.WithDescription("Generate a sprite sheet image from a prompt, action, frame count, layout, optional reference image, optional frame geometry, optional solid background color, optional tuning args, seed, negative prompt, and output_path. Reference images may be http(s) URLs, data:image base64 URLs, or raw base64 image data. Save the result on the server. To save into a caller workspace, pass an absolute output_path. The response saved_path is the actual absolute file path written by the server; image_data_base64 is also returned as a fallback."),
		mcp.WithInputSchema[GenerateSpriteSheetArgs](),
	)
}

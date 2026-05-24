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
		mcp.WithDescription("Generate a sprite sheet image from a prompt, action, frame count, layout, optional reference image, optional frame geometry, optional solid background color, optional tuning args, seed, and negative prompt, save it locally, and return the file information"),
		mcp.WithInputSchema[GenerateSpriteSheetArgs](),
	)
}

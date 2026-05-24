package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

type GenerateSpriteSheetArgs struct {
	Prompt            string  `json:"prompt" jsonschema:"Character or subject description used to generate the sprite sheet"`
	Action            string  `json:"action" jsonschema:"Animation action or motion to generate, such as idle, walk, attack, jump, or cast"`
	FrameCount        int     `json:"frame_count" jsonschema:"Number of animation frames to request in the sprite sheet"`
	Layout            string  `json:"layout,omitempty" jsonschema:"Sprite sheet layout, such as horizontal, vertical, or 3x3"`
	FrameWidth        int     `json:"frame_width,omitempty" jsonschema:"Optional frame width in pixels, default 64"`
	FrameHeight       int     `json:"frame_height,omitempty" jsonschema:"Optional frame height in pixels, default 64"`
	Spacing           int     `json:"spacing,omitempty" jsonschema:"Optional spacing between frames in pixels, default 2"`
	BackgroundColor   string  `json:"background_color,omitempty" jsonschema:"Optional solid background color in #RRGGBB format, such as #00FF00 or #FF00FF"`
	ImageSize         string  `json:"image_size,omitempty" jsonschema:"Optional output image size, such as 1024x1024"`
	GuidanceScale     float64 `json:"guidance_scale,omitempty" jsonschema:"Optional prompt adherence strength"`
	NumInferenceSteps int     `json:"num_inference_steps,omitempty" jsonschema:"Optional number of inference steps to request"`
	Seed              *int64  `json:"seed,omitempty" jsonschema:"Optional generation seed for reproducible outputs"`
	NegativePrompt    string  `json:"negative_prompt,omitempty" jsonschema:"Optional text describing what to avoid in the generated image"`
}

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
		Generation: imagegen.GenerationOptions{
			BackgroundColor:   args.BackgroundColor,
			ImageSize:         args.ImageSize,
			GuidanceScale:     args.GuidanceScale,
			NumInferenceSteps: args.NumInferenceSteps,
			Seed:              args.Seed,
			NegativePrompt:    args.NegativePrompt,
		},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("sprite sheet generation failed: %v", err)), nil
	}

	return mcp.NewToolResultStructuredOnly(result), nil
}

func newGenerateSpriteSheetTool() mcp.Tool {
	return mcp.NewTool(
		"generate_sprite_sheet",
		mcp.WithDescription("Generate a sprite sheet image from a prompt, action, frame count, layout, optional frame geometry, optional solid background color, optional tuning args, seed, and negative prompt, save it locally, and return the file information"),
		mcp.WithInputSchema[GenerateSpriteSheetArgs](),
	)
}

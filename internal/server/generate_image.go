package server

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

type GenerateImageArgs struct {
	Prompt            string  `json:"prompt" jsonschema:"Text prompt used to generate the image"`
	BackgroundColor   string  `json:"background_color,omitempty" jsonschema:"Optional solid background color in #RRGGBB format, such as #00FF00 or #FF00FF"`
	ImageSize         string  `json:"image_size,omitempty" jsonschema:"Optional output image size, such as 1024x1024"`
	GuidanceScale     float64 `json:"guidance_scale,omitempty" jsonschema:"Optional prompt adherence strength"`
	NumInferenceSteps int     `json:"num_inference_steps,omitempty" jsonschema:"Optional number of inference steps to request"`
	Seed              *int64  `json:"seed,omitempty" jsonschema:"Optional generation seed for reproducible outputs"`
	NegativePrompt    string  `json:"negative_prompt,omitempty" jsonschema:"Optional text describing what to avoid in the generated image"`
	ReferenceImage    string  `json:"reference_image,omitempty" jsonschema:"Optional reference image as an http(s) URL or data:image base64 URL"`
}

type generateImageToolHandler struct {
	service *imagegen.Service
}

func (h *generateImageToolHandler) handle(ctx context.Context, _ mcp.CallToolRequest, args GenerateImageArgs) (*mcp.CallToolResult, error) {
	result, err := h.service.GenerateWithOptions(ctx, args.Prompt, imagegen.GenerationOptions{
		BackgroundColor:   args.BackgroundColor,
		ImageSize:         args.ImageSize,
		GuidanceScale:     args.GuidanceScale,
		NumInferenceSteps: args.NumInferenceSteps,
		Seed:              args.Seed,
		NegativePrompt:    args.NegativePrompt,
		ReferenceImage:    args.ReferenceImage,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("image generation failed: %v", err)), nil
	}

	return mcp.NewToolResultStructuredOnly(result), nil
}

func newGenerateImageTool() mcp.Tool {
	return mcp.NewTool(
		"generate_image",
		mcp.WithDescription("Generate an image from a prompt, optional reference image, optional solid background color, optional tuning args, seed, and negative prompt, save it locally, and return the file information"),
		mcp.WithInputSchema[GenerateImageArgs](),
	)
}

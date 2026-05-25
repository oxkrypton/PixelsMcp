package server

import imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"

type GenerationArgs struct {
	BackgroundColor   string  `json:"background_color,omitempty" jsonschema:"Optional solid background color in #RRGGBB format, such as #00FF00 or #FF00FF"`
	ImageSize         string  `json:"image_size,omitempty" jsonschema:"Optional output image size, such as 1024x1024"`
	GuidanceScale     float64 `json:"guidance_scale,omitempty" jsonschema:"Optional prompt adherence strength"`
	NumInferenceSteps int     `json:"num_inference_steps,omitempty" jsonschema:"Optional number of inference steps to request"`
	Seed              *int64  `json:"seed,omitempty" jsonschema:"Optional generation seed for reproducible outputs"`
	NegativePrompt    string  `json:"negative_prompt,omitempty" jsonschema:"Optional text describing what to avoid in the generated image"`
	ReferenceImage    string  `json:"reference_image,omitempty" jsonschema:"Optional reference image as an http(s) URL, a data:image base64 URL, or raw base64 image data without the data: prefix. If your client renders data:image strings as images, pass only the raw base64 payload; the server will restore the data URL before calling the image provider."`
}

func (a GenerationArgs) generationOptions() imagegen.GenerationOptions {
	return imagegen.GenerationOptions{
		BackgroundColor:   a.BackgroundColor,
		ImageSize:         a.ImageSize,
		GuidanceScale:     a.GuidanceScale,
		NumInferenceSteps: a.NumInferenceSteps,
		Seed:              a.Seed,
		NegativePrompt:    a.NegativePrompt,
		ReferenceImage:    a.ReferenceImage,
	}
}

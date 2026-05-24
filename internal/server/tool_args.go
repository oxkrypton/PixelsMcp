package server

type GenerateImageArgs struct {
	Prompt string `json:"prompt" jsonschema:"Text prompt used to generate the image"`
	GenerationArgs
}

type GenerateSpriteSheetArgs struct {
	Prompt      string `json:"prompt" jsonschema:"Character or subject description used to generate the sprite sheet"`
	Action      string `json:"action" jsonschema:"Animation action or motion to generate, such as idle, walk, attack, jump, or cast"`
	FrameCount  int    `json:"frame_count" jsonschema:"Number of animation frames to request in the sprite sheet"`
	Layout      string `json:"layout,omitempty" jsonschema:"Sprite sheet layout, such as horizontal, vertical, or 3x3"`
	FrameWidth  int    `json:"frame_width,omitempty" jsonschema:"Optional frame width in pixels, default 64"`
	FrameHeight int    `json:"frame_height,omitempty" jsonschema:"Optional frame height in pixels, default 64"`
	Spacing     int    `json:"spacing,omitempty" jsonschema:"Optional spacing between frames in pixels, default 2"`
	GenerationArgs
}

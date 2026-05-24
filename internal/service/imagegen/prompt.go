package imagegen

import (
	"errors"
	"fmt"
	"strings"
)

const (
	defaultSpriteSheetFrameWidth  = 64
	defaultSpriteSheetFrameHeight = 64
	defaultSpriteSheetSpacing     = 2
)

func buildSpriteSheetPrompt(sourcePrompt, action string, frameCount int, layout string, frameWidth, frameHeight, spacing int) string {
	layoutInstruction := spriteSheetLayoutInstruction(layout)
	geometryInstruction := spriteSheetGeometryInstruction(frameCount, layout, frameWidth, frameHeight, spacing)

	return fmt.Sprintf(`%s

Create a single sprite sheet in a 16-bit pixel art style for this animation.
Action: %s.
Frame count: %d.
Layout: %s.
%s
%s
Use pixel-perfect rendering with hard square pixels, crisp edges, no anti-aliasing, no blur, nearest-neighbor filtering, and a limited color palette.
Keep the same character design, proportions, style, camera angle, lighting, and scale in every frame.
Show clear incremental motion across frames.
Place exactly one character in each cell.
Do not add text labels, frame numbers, UI elements, watermarks, decorative borders, or extra rows.`,
		sourcePrompt,
		action,
		frameCount,
		layout,
		layoutInstruction,
		geometryInstruction,
	)
}

func buildPromptWithBackground(prompt, backgroundColor, defaultInstruction string) (string, error) {
	instruction, err := buildBackgroundInstruction(backgroundColor)
	if err != nil {
		return "", err
	}
	if instruction == "" {
		instruction = strings.TrimSpace(defaultInstruction)
	}
	if instruction == "" {
		return prompt, nil
	}

	return prompt + "\n\n" + instruction, nil
}

func buildBackgroundInstruction(backgroundColor string) (string, error) {
	color, err := normalizeBackgroundColor(backgroundColor)
	if err != nil {
		return "", err
	}
	if color == "" {
		return "", nil
	}

	return fmt.Sprintf("Use a SOLID %s background (%s) with absolutely no gradients, no transparency.", color, color), nil
}

func normalizeBackgroundColor(value string) (string, error) {
	color := strings.ToUpper(strings.TrimSpace(value))
	if color == "" {
		return "", nil
	}
	if len(color) != 7 || color[0] != '#' {
		return "", errors.New("background_color must be in #RRGGBB format")
	}
	for _, r := range color[1:] {
		if !isHexDigit(r) {
			return "", errors.New("background_color must be in #RRGGBB format")
		}
	}
	return color, nil
}

func isHexDigit(r rune) bool {
	switch {
	case r >= '0' && r <= '9':
		return true
	case r >= 'a' && r <= 'f':
		return true
	case r >= 'A' && r <= 'F':
		return true
	default:
		return false
	}
}

func spriteSheetLayoutInstruction(layout string) string {
	switch strings.ToLower(strings.TrimSpace(layout)) {
	case "horizontal", "row", "strip":
		return "Arrange the frames in one horizontal row, ordered left to right."
	case "vertical", "column":
		return "Arrange the frames in one vertical column, ordered top to bottom."
	case "3x3", "3 by 3", "3-by-3", "3*3":
		return "Arrange the frames in a 3 by 3 grid, ordered left to right and top to bottom."
	default:
		return "Arrange the frames using the requested layout exactly as written."
	}
}

func spriteSheetGeometryInstruction(frameCount int, layout string, frameWidth, frameHeight, spacing int) string {
	frameWidth = positiveOrDefault(frameWidth, defaultSpriteSheetFrameWidth)
	frameHeight = positiveOrDefault(frameHeight, defaultSpriteSheetFrameHeight)
	spacing = positiveOrDefault(spacing, defaultSpriteSheetSpacing)

	frameInstruction := fmt.Sprintf(
		"Frame geometry: %d cells, each cell exactly %dx%d pixels, with exactly %dpx spacing between cells and no outer padding.",
		frameCount,
		frameWidth,
		frameHeight,
		spacing,
	)

	if width, height, ok := spriteSheetCanvasSize(frameCount, layout, frameWidth, frameHeight, spacing); ok {
		return fmt.Sprintf(
			"Canvas geometry: total image is exactly %dx%d pixels.\n%s",
			width,
			height,
			frameInstruction,
		)
	}

	return frameInstruction
}

func spriteSheetCanvasSize(frameCount int, layout string, frameWidth, frameHeight, spacing int) (int, int, bool) {
	if frameCount <= 0 {
		return 0, 0, false
	}

	switch strings.ToLower(strings.TrimSpace(layout)) {
	case "horizontal", "row", "strip":
		return frameCount*frameWidth + max(frameCount-1, 0)*spacing, frameHeight, true
	case "vertical", "column":
		return frameWidth, frameCount*frameHeight + max(frameCount-1, 0)*spacing, true
	case "3x3", "3 by 3", "3-by-3", "3*3":
		return 3*frameWidth + 2*spacing, 3*frameHeight + 2*spacing, true
	default:
		return 0, 0, false
	}
}

func positiveOrDefault(value, defaultValue int) int {
	if value > 0 {
		return value
	}
	return defaultValue
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

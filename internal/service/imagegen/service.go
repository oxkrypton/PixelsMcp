package imagegen

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	Provider       string
	APIKey         string
	BaseURL        string
	Model          string
	ReferenceModel string
	ExtraHeaders   map[string]string
	Timeout        time.Duration
	SaveDir        string
	Client         *http.Client
}

type Service struct {
	provider Provider
	saveDir  string
	client   *http.Client
}

const (
	defaultSpriteSheetFrameWidth  = 64
	defaultSpriteSheetFrameHeight = 64
	defaultSpriteSheetSpacing     = 2
)

type Result struct {
	Prompt             string    `json:"prompt"`
	Model              string    `json:"model"`
	ImageURL           string    `json:"image_url"`
	LocalPath          string    `json:"local_path"`
	Seed               int64     `json:"seed,omitempty"`
	InferenceMS        float64   `json:"inference_ms,omitempty"`
	TraceID            string    `json:"trace_id,omitempty"`
	UsedReferenceImage bool      `json:"used_reference_image,omitempty"`
	ContentType        string    `json:"content_type,omitempty"`
	Bytes              int64     `json:"bytes,omitempty"`
	GeneratedAt        time.Time `json:"generated_at"`
	DownloadedAt       time.Time `json:"downloaded_at"`
}

type SpriteSheetOptions struct {
	Prompt      string
	Action      string
	FrameCount  int
	Layout      string
	FrameWidth  int
	FrameHeight int
	Spacing     int
	Generation  GenerationOptions
}

type SpriteSheetResult struct {
	Result
	SourcePrompt string `json:"source_prompt"`
	Action       string `json:"action"`
	FrameCount   int    `json:"frame_count"`
	Layout       string `json:"layout"`
}

func NewService(cfg Config) (*Service, error) {
	provider, err := NewProvider(ProviderConfig{
		Provider:       cfg.Provider,
		APIKey:         cfg.APIKey,
		BaseURL:        cfg.BaseURL,
		Model:          cfg.Model,
		ReferenceModel: cfg.ReferenceModel,
		ExtraHeaders:   cfg.ExtraHeaders,
		Timeout:        cfg.Timeout,
		Client:         cfg.Client,
	})
	if err != nil {
		return nil, err
	}

	saveDir := strings.TrimSpace(cfg.SaveDir)
	if saveDir == "" {
		saveDir = DefaultSaveDir
	}

	return &Service{
		provider: provider,
		saveDir:  saveDir,
		client:   newHTTPClient(cfg.Client, cfg.Timeout),
	}, nil
}

func (s *Service) Generate(ctx context.Context, prompt string) (*Result, error) {
	return s.GenerateWithOptions(ctx, prompt, GenerationOptions{})
}

func (s *Service) GenerateWithOptions(ctx context.Context, prompt string, generation GenerationOptions) (*Result, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}

	prompt, err := buildPromptWithBackground(prompt, generation.BackgroundColor, "")
	if err != nil {
		return nil, err
	}

	return s.generate(ctx, prompt, "", generation)
}

func (s *Service) GenerateSpriteSheet(ctx context.Context, opts SpriteSheetOptions) (*SpriteSheetResult, error) {
	sourcePrompt := strings.TrimSpace(opts.Prompt)
	if sourcePrompt == "" {
		return nil, errors.New("prompt is required")
	}

	action := strings.TrimSpace(opts.Action)
	if action == "" {
		return nil, errors.New("action is required")
	}

	if opts.FrameCount <= 0 {
		return nil, errors.New("frame_count must be greater than 0")
	}

	layout := strings.TrimSpace(opts.Layout)
	if layout == "" {
		layout = "horizontal"
	}

	frameWidth := opts.FrameWidth
	if frameWidth <= 0 {
		frameWidth = defaultSpriteSheetFrameWidth
	}
	frameHeight := opts.FrameHeight
	if frameHeight <= 0 {
		frameHeight = defaultSpriteSheetFrameHeight
	}
	spacing := opts.Spacing
	if spacing <= 0 {
		spacing = defaultSpriteSheetSpacing
	}

	generationPrompt := buildSpriteSheetPrompt(sourcePrompt, action, opts.FrameCount, layout, frameWidth, frameHeight, spacing)
	generationPrompt, err := buildPromptWithBackground(generationPrompt, opts.Generation.BackgroundColor, "Use a solid light-gray background (#D9D9D9) in every empty pixel, with no texture, no gradients, and no transparency.")
	if err != nil {
		return nil, err
	}

	result, err := s.generate(ctx, generationPrompt, "sprite-sheet", opts.Generation)
	if err != nil {
		return nil, err
	}

	return &SpriteSheetResult{
		Result:       *result,
		SourcePrompt: sourcePrompt,
		Action:       action,
		FrameCount:   opts.FrameCount,
		Layout:       layout,
	}, nil
}

func (s *Service) generate(ctx context.Context, prompt string, fileNamePrefix string, generation GenerationOptions) (*Result, error) {
	generated, err := s.provider.Generate(ctx, prompt, generation)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(generated.ImageURL) == "" {
		return nil, errors.New("image generation response did not include an image url")
	}

	model := strings.TrimSpace(generated.Model)
	if model == "" {
		model = DefaultModel
	}

	result := &Result{
		Prompt:             prompt,
		Model:              model,
		ImageURL:           strings.TrimSpace(generated.ImageURL),
		Seed:               generated.Seed,
		InferenceMS:        generated.InferenceMS,
		TraceID:            strings.TrimSpace(generated.TraceID),
		UsedReferenceImage: generated.UsedReferenceImage,
		GeneratedAt:        time.Now().UTC(),
	}

	localPath, contentType, bytesWritten, err := s.downloadImage(ctx, result.ImageURL, result, fileNamePrefix)
	if err != nil {
		return nil, err
	}
	result.LocalPath = localPath
	result.ContentType = contentType
	result.Bytes = bytesWritten
	result.DownloadedAt = time.Now().UTC()

	return result, nil
}

func (s *Service) downloadImage(ctx context.Context, imageURL string, result *Result, fileNamePrefix string) (string, string, int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", "", 0, fmt.Errorf("create image download request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", 0, fmt.Errorf("download generated image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(body) > 0 {
			return "", "", 0, fmt.Errorf("download generated image failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return "", "", 0, fmt.Errorf("download generated image failed: status %d", resp.StatusCode)
	}

	if err := os.MkdirAll(s.saveDir, 0o755); err != nil {
		return "", "", 0, fmt.Errorf("create image output directory: %w", err)
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	ext := extensionForContentType(contentType)
	if ext == "" {
		ext = extensionFromURL(imageURL)
	}
	if ext == "" {
		ext = ".png"
	}

	fileName := buildFileName(fileNamePrefix, result.Model, result.Seed, ext)
	filePath := filepath.Join(s.saveDir, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		return "", "", 0, fmt.Errorf("create image file: %w", err)
	}
	defer file.Close()

	written, err := io.Copy(file, resp.Body)
	if err != nil {
		return "", "", 0, fmt.Errorf("save generated image: %w", err)
	}

	return filePath, contentType, written, nil
}

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

func extensionForContentType(contentType string) string {
	switch {
	case strings.Contains(contentType, "image/png"):
		return ".png"
	case strings.Contains(contentType, "image/jpeg"):
		return ".jpg"
	case strings.Contains(contentType, "image/jpg"):
		return ".jpg"
	case strings.Contains(contentType, "image/webp"):
		return ".webp"
	case strings.Contains(contentType, "image/gif"):
		return ".gif"
	default:
		return ""
	}
}

func extensionFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return filepath.Ext(parsed.Path)
}

func buildFileName(prefix, model string, seed int64, ext string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		" ", "-",
		":", "-",
		"?", "-",
		"&", "-",
		"=", "-",
		",", "-",
	)
	safeModel := replacer.Replace(model)
	if safeModel == "" {
		safeModel = "image"
	}

	prefix = replacer.Replace(strings.TrimSpace(prefix))
	if prefix != "" {
		prefix += "-"
	}

	return fmt.Sprintf("%s%s-%d-%d%s", prefix, safeModel, time.Now().UTC().UnixNano(), seed, ext)
}

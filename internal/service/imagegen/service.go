package imagegen

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
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

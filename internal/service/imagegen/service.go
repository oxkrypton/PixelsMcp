package imagegen

import (
	"bytes"
	"context"
	"encoding/json"
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

const (
	defaultModel   = "Kwai-Kolors/Kolors"
	defaultSaveDir = "generated-images"
	generationPath = "/v1/images/generations"
)

type Config struct {
	APIKey  string
	BaseURL string
	Model   string
	SaveDir string
	Client  *http.Client
}

type Service struct {
	apiKey  string
	baseURL string
	model   string
	saveDir string
	client  *http.Client
}

type Result struct {
	Prompt       string    `json:"prompt"`
	Model        string    `json:"model"`
	ImageURL     string    `json:"image_url"`
	LocalPath    string    `json:"local_path"`
	Seed         int64     `json:"seed,omitempty"`
	InferenceMS  float64   `json:"inference_ms,omitempty"`
	TraceID      string    `json:"trace_id,omitempty"`
	ContentType  string    `json:"content_type,omitempty"`
	Bytes        int64     `json:"bytes,omitempty"`
	GeneratedAt  time.Time `json:"generated_at"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

type generationRequest struct {
	Model             string  `json:"model"`
	Prompt            string  `json:"prompt"`
	ImageSize         string  `json:"image_size,omitempty"`
	BatchSize         int     `json:"batch_size,omitempty"`
	NumInferenceSteps int     `json:"num_inference_steps,omitempty"`
	GuidanceScale     float64 `json:"guidance_scale,omitempty"`
}

type generationResponse struct {
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
	Timings struct {
		Inference float64 `json:"inference"`
	} `json:"timings"`
	Seed int64 `json:"seed"`
}

func NewService(cfg Config) *Service {
	client := cfg.Client
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Minute}
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}

	saveDir := strings.TrimSpace(cfg.SaveDir)
	if saveDir == "" {
		saveDir = defaultSaveDir
	}

	return &Service{
		apiKey:  strings.TrimSpace(cfg.APIKey),
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		saveDir: saveDir,
		client:  client,
	}
}

func (s *Service) Generate(ctx context.Context, prompt string) (*Result, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}
	if s.apiKey == "" {
		return nil, errors.New("image generation api key is required")
	}
	if s.baseURL == "" {
		return nil, errors.New("image generation base url is required")
	}

	reqBody := generationRequest{
		Model:  s.model,
		Prompt: prompt,
	}

	rawReq, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal generation request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+generationPath, bytes.NewReader(rawReq))
	if err != nil {
		return nil, fmt.Errorf("create generation request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call image generation api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		if len(body) > 0 {
			return nil, fmt.Errorf("image generation request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
		return nil, fmt.Errorf("image generation request failed: status %d", resp.StatusCode)
	}

	var genResp generationResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return nil, fmt.Errorf("decode image generation response: %w", err)
	}
	if len(genResp.Images) == 0 || strings.TrimSpace(genResp.Images[0].URL) == "" {
		return nil, errors.New("image generation response did not include an image url")
	}

	imageURL := strings.TrimSpace(genResp.Images[0].URL)
	result := &Result{
		Prompt:      prompt,
		Model:       s.model,
		ImageURL:    imageURL,
		Seed:        genResp.Seed,
		InferenceMS: genResp.Timings.Inference,
		TraceID:     firstHeaderValue(resp.Header, "X-Trace-Id", "X-Request-Id", "X-Provider-Trace-Id"),
		GeneratedAt: time.Now().UTC(),
	}

	localPath, contentType, bytesWritten, err := s.downloadImage(ctx, imageURL, result)
	if err != nil {
		return nil, err
	}
	result.LocalPath = localPath
	result.ContentType = contentType
	result.Bytes = bytesWritten
	result.DownloadedAt = time.Now().UTC()

	return result, nil
}

func (s *Service) downloadImage(ctx context.Context, imageURL string, result *Result) (string, string, int64, error) {
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

	fileName := buildFileName(result.Model, result.Seed, ext)
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

func buildFileName(model string, seed int64, ext string) string {
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

	return fmt.Sprintf("%s-%d-%d%s", safeModel, time.Now().UTC().UnixNano(), seed, ext)
}

func firstHeaderValue(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

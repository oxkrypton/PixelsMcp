package imagegen

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

const (
	openAIGenerationPath = "/v1/images/generations"
	openAIModelsPath     = "/v1/models"
)

type openAICompatibleProvider struct {
	apiKey         string
	baseURL        string
	model          string
	referenceModel string
	extraHeaders   map[string]string
	client         *http.Client
}

type openAICompatibleGenerationRequest struct {
	Model             string  `json:"model"`
	Prompt            string  `json:"prompt"`
	NegativePrompt    string  `json:"negative_prompt,omitempty"`
	ImageSize         string  `json:"image_size,omitempty"`
	Image             string  `json:"image,omitempty"`
	BatchSize         int     `json:"batch_size,omitempty"`
	NumInferenceSteps int     `json:"num_inference_steps,omitempty"`
	GuidanceScale     float64 `json:"guidance_scale,omitempty"`
	Seed              *int64  `json:"seed,omitempty"`
}

type openAICompatibleGenerationResponse struct {
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
	Timings struct {
		Inference float64 `json:"inference"`
	} `json:"timings"`
	Seed  int64  `json:"seed"`
	Model string `json:"model"`
}

type openAICompatibleModelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func newOpenAICompatibleProvider(cfg ProviderConfig) (*openAICompatibleProvider, error) {
	apiKey := strings.TrimSpace(cfg.APIKey)
	if apiKey == "" {
		return nil, errors.New("image generation api key is required")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("image generation base url is required")
	}

	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = DefaultModel
	}

	return &openAICompatibleProvider{
		apiKey:         apiKey,
		baseURL:        baseURL,
		model:          model,
		referenceModel: strings.TrimSpace(cfg.ReferenceModel),
		extraHeaders:   cloneStringMap(cfg.ExtraHeaders),
		client:         newHTTPClient(cfg.Client, cfg.Timeout),
	}, nil
}

func (p *openAICompatibleProvider) Name() string {
	return ProviderOpenAICompatible
}

func (p *openAICompatibleProvider) Generate(ctx context.Context, prompt string, opts GenerationOptions) (*GenerationResult, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}

	model := p.model
	referenceImage := strings.TrimSpace(opts.ReferenceImage)
	usedReferenceImage := referenceImage != ""
	if usedReferenceImage {
		if strings.TrimSpace(p.referenceModel) == "" {
			return nil, errors.New("reference image model is required when reference_image is provided")
		}
		normalizedReferenceImage, err := normalizeReferenceImage(referenceImage)
		if err != nil {
			return nil, err
		}
		referenceImage = normalizedReferenceImage
		model = strings.TrimSpace(p.referenceModel)
	}

	reqBody := openAICompatibleGenerationRequest{
		Model:             model,
		Prompt:            prompt,
		NegativePrompt:    strings.TrimSpace(opts.NegativePrompt),
		ImageSize:         strings.TrimSpace(opts.ImageSize),
		NumInferenceSteps: opts.NumInferenceSteps,
		GuidanceScale:     opts.GuidanceScale,
		Seed:              opts.Seed,
	}
	if usedReferenceImage {
		reqBody.Image = referenceImage
		reqBody.ImageSize = ""
		reqBody.GuidanceScale = 0
	}

	rawReq, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal generation request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+openAIGenerationPath, bytes.NewReader(rawReq))
	if err != nil {
		return nil, fmt.Errorf("create generation request: %w", err)
	}
	p.applyHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call image generation api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError("image generation request failed", resp)
	}

	var genResp openAICompatibleGenerationResponse
	if err := json.NewDecoder(resp.Body).Decode(&genResp); err != nil {
		return nil, fmt.Errorf("decode image generation response: %w", err)
	}
	if len(genResp.Images) == 0 || strings.TrimSpace(genResp.Images[0].URL) == "" {
		return nil, errors.New("image generation response did not include an image url")
	}

	imageURL := strings.TrimSpace(genResp.Images[0].URL)
	resultModel := strings.TrimSpace(genResp.Model)
	if resultModel == "" {
		resultModel = model
	}

	seed := genResp.Seed
	if seed == 0 && opts.Seed != nil {
		seed = *opts.Seed
	}

	return &GenerationResult{
		Model:              resultModel,
		ImageURL:           imageURL,
		Seed:               seed,
		InferenceMS:        genResp.Timings.Inference,
		TraceID:            firstHeaderValue(resp.Header, "X-Siliconcloud-Trace-Id", "X-Trace-Id", "X-Request-Id", "X-Provider-Trace-Id"),
		UsedReferenceImage: usedReferenceImage,
	}, nil
}

func (p *openAICompatibleProvider) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+openAIModelsPath, nil)
	if err != nil {
		return nil, fmt.Errorf("create model list request: %w", err)
	}
	p.applyHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call model list api: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, responseError("model list request failed", resp)
	}

	var modelResp openAICompatibleModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelResp); err != nil {
		return nil, fmt.Errorf("decode model list response: %w", err)
	}

	models := make([]string, 0, len(modelResp.Data))
	for _, item := range modelResp.Data {
		if id := strings.TrimSpace(item.ID); id != "" {
			models = append(models, id)
		}
	}

	return models, nil
}

func (p *openAICompatibleProvider) Validate(ctx context.Context) error {
	model := strings.TrimSpace(p.model)
	if model == "" {
		return errors.New("image generation model is required")
	}

	models, err := p.ListModels(ctx)
	if err != nil {
		return fmt.Errorf("validate image generation provider: %w", err)
	}
	if len(models) == 0 {
		return errors.New("provider validation failed: model list is empty")
	}
	if !containsModel(models, model) {
		return fmt.Errorf("provider validation failed: model %q was not returned by provider", model)
	}

	return nil
}

func containsModel(models []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, model := range models {
		if strings.TrimSpace(model) == target {
			return true
		}
	}
	return false
}

func isSupportedReferenceImage(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "http://") ||
		strings.HasPrefix(value, "https://") ||
		strings.HasPrefix(value, "data:image/")
}

func normalizeReferenceImage(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}

	if isSupportedReferenceImage(value) {
		return value, nil
	}

	compact := strings.Join(strings.Fields(value), "")
	if compact == "" {
		return "", errors.New("reference_image must be an http(s) URL, a data:image base64 URL, or raw base64 image data")
	}

	decoded, err := decodeBase64ImagePayload(compact)
	if err != nil {
		return "", errors.New("reference_image must be an http(s) URL, a data:image base64 URL, or raw base64 image data")
	}

	mimeType := inferImageMimeType(decoded)
	if mimeType == "" {
		return "", errors.New("raw reference_image base64 must decode to a PNG, JPEG, GIF, WebP, BMP, or TIFF image")
	}

	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(decoded), nil
}

func decodeBase64ImagePayload(value string) ([]byte, error) {
	decoders := []func(string) ([]byte, error){
		base64.StdEncoding.DecodeString,
		base64.RawStdEncoding.DecodeString,
		base64.URLEncoding.DecodeString,
		base64.RawURLEncoding.DecodeString,
	}
	for _, decode := range decoders {
		if decoded, err := decode(value); err == nil {
			return decoded, nil
		}
	}
	return nil, errors.New("invalid base64 payload")
}

func inferImageMimeType(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return "image/gif"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp"
	case len(data) >= 2 && bytes.Equal(data[:2], []byte("BM")):
		return "image/bmp"
	case len(data) >= 4 && (bytes.Equal(data[:4], []byte{0x49, 0x49, 0x2A, 0x00}) || bytes.Equal(data[:4], []byte{0x4D, 0x4D, 0x00, 0x2A})):
		return "image/tiff"
	default:
		return ""
	}
}

func (p *openAICompatibleProvider) applyHeaders(req *http.Request) {
	for key, value := range p.extraHeaders {
		if strings.TrimSpace(key) == "" || strings.TrimSpace(value) == "" {
			continue
		}
		req.Header.Set(http.CanonicalHeaderKey(key), value)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
}

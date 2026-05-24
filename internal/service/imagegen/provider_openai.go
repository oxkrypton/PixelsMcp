package imagegen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const (
	openAIGenerationPath = "/v1/images/generations"
	openAIModelsPath     = "/v1/models"
)

type openAICompatibleProvider struct {
	apiKey       string
	baseURL      string
	model        string
	extraHeaders map[string]string
	client       *http.Client
}

type openAICompatibleGenerationRequest struct {
	Model             string  `json:"model"`
	Prompt            string  `json:"prompt"`
	ImageSize         string  `json:"image_size,omitempty"`
	BatchSize         int     `json:"batch_size,omitempty"`
	NumInferenceSteps int     `json:"num_inference_steps,omitempty"`
	GuidanceScale     float64 `json:"guidance_scale,omitempty"`
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

type generationRequest = openAICompatibleGenerationRequest
type generationResponse = openAICompatibleGenerationResponse

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
		apiKey:       apiKey,
		baseURL:      baseURL,
		model:        model,
		extraHeaders: cloneStringMap(cfg.ExtraHeaders),
		client:       newHTTPClient(cfg.Client, cfg.Timeout),
	}, nil
}

func (p *openAICompatibleProvider) Name() string {
	return ProviderOpenAICompatible
}

func (p *openAICompatibleProvider) Generate(ctx context.Context, prompt string) (*GenerationResult, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return nil, errors.New("prompt is required")
	}

	reqBody := openAICompatibleGenerationRequest{
		Model:  p.model,
		Prompt: prompt,
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
	model := strings.TrimSpace(genResp.Model)
	if model == "" {
		model = p.model
	}

	return &GenerationResult{
		Model:       model,
		ImageURL:    imageURL,
		Seed:        genResp.Seed,
		InferenceMS: genResp.Timings.Inference,
		TraceID:     firstHeaderValue(resp.Header, "X-Trace-Id", "X-Request-Id", "X-Provider-Trace-Id"),
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+openAIModelsPath, nil)
	if err != nil {
		return fmt.Errorf("create provider validation request: %w", err)
	}
	p.applyHeaders(req)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("validate image generation provider: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return responseError("provider validation request failed", resp)
	}

	var modelResp openAICompatibleModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelResp); err != nil {
		return fmt.Errorf("decode provider validation response: %w", err)
	}
	_ = modelResp
	return nil
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

func responseError(prefix string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if len(body) > 0 {
		return fmt.Errorf("%s: status %d: %s", prefix, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return fmt.Errorf("%s: status %d", prefix, resp.StatusCode)
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	clone := make(map[string]string, len(src))
	for key, value := range src {
		clone[key] = value
	}
	return clone
}

func firstHeaderValue(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

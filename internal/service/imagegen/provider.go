package imagegen

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const (
	ProviderOpenAICompatible = "openai-compatible"
	DefaultProvider          = ProviderOpenAICompatible
	DefaultModel             = "Kwai-Kolors/Kolors"
	DefaultReferenceModel    = "Qwen/Qwen-Image-Edit-2509"
	DefaultSaveDir           = "./generated-images"
	DefaultRequestTimeout    = 2 * time.Minute
)

type ProviderConfig struct {
	Provider       string
	APIKey         string
	BaseURL        string
	Model          string
	ReferenceModel string
	ExtraHeaders   map[string]string
	Timeout        time.Duration
	Client         *http.Client
}

type GenerationOptions struct {
	ImageSize         string
	GuidanceScale     float64
	NumInferenceSteps int
	Seed              *int64
	NegativePrompt    string
	BackgroundColor   string
	ReferenceImage    string
	ReferencePath     string
	OutputPath        string
}

type Provider interface {
	Name() string
	Generate(ctx context.Context, prompt string, opts GenerationOptions) (*GenerationResult, error)
	ListModels(ctx context.Context) ([]string, error)
	Validate(ctx context.Context) error
}

type GenerationResult struct {
	Model              string
	ImageURL           string
	Seed               int64
	InferenceMS        float64
	TraceID            string
	UsedReferenceImage bool
}

func NewProvider(cfg ProviderConfig) (Provider, error) {
	switch normalizeProviderName(cfg.Provider) {
	case "", ProviderOpenAICompatible, "openai":
		return newOpenAICompatibleProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported image provider %q", cfg.Provider)
	}
}

func normalizeProviderName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func newHTTPClient(base *http.Client, timeout time.Duration) *http.Client {
	var client *http.Client
	if base != nil {
		clone := *base
		client = &clone
	} else {
		client = &http.Client{}
	}

	if timeout > 0 {
		client.Timeout = timeout
	} else if client.Timeout <= 0 {
		client.Timeout = DefaultRequestTimeout
	}

	return client
}

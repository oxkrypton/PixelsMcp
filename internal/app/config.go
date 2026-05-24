package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

type transport string

const (
	transportStdio transport = "stdio"
	transportHTTP  transport = "http"

	defaultHTTPAddr       = ":8080"
	defaultMCPEndpoint    = "/mcp"
	defaultHealthEndpoint = "/healthz"
)

type config struct {
	transport      transport
	addr           string
	mcpEndpoint    string
	healthEndpoint string
	corsOrigins    []string
	provider       string
	apiKey         string
	baseURL        string
	model          string
	extraHeaders   map[string]string
	timeout        time.Duration
	imageSaveDir   string
}

func configFromEnv(getenv func(string) string) (config, error) {
	cfg := config{
		transport:      transportHTTP,
		addr:           defaultHTTPAddr,
		mcpEndpoint:    defaultMCPEndpoint,
		healthEndpoint: defaultHealthEndpoint,
		provider:       imagegen.DefaultProvider,
		model:          imagegen.DefaultModel,
		timeout:        imagegen.DefaultRequestTimeout,
		imageSaveDir:   imagegen.DefaultSaveDir,
	}

	if value := strings.TrimSpace(getenv("PIXELSMCP_TRANSPORT")); value != "" {
		cfg.transport = transport(strings.ToLower(value))
	}
	if value := strings.TrimSpace(getenv("PIXELSMCP_ADDR")); value != "" {
		cfg.addr = value
	}
	if value := strings.TrimSpace(getenv("PIXELSMCP_ENDPOINT")); value != "" {
		cfg.mcpEndpoint = normalizePath(value)
	}
	if value := strings.TrimSpace(getenv("PIXELSMCP_HEALTH_ENDPOINT")); value != "" {
		cfg.healthEndpoint = normalizePath(value)
	}
	if value := strings.TrimSpace(getenv("PIXELSMCP_CORS_ORIGINS")); value != "" {
		for _, origin := range strings.Split(value, ",") {
			if origin = strings.TrimSpace(origin); origin != "" {
				cfg.corsOrigins = append(cfg.corsOrigins, origin)
			}
		}
	}
	if value := strings.TrimSpace(getenv("PIXELSMCP_PROVIDER")); value != "" {
		provider := normalizeSupportedProvider(value)
		if provider == "" {
			return config{}, fmt.Errorf("PIXELSMCP_PROVIDER must be one of %q or %q, got %q", imagegen.ProviderOpenAICompatible, "openai", value)
		}
		cfg.provider = provider
	}
	cfg.apiKey = strings.TrimSpace(getenv("PIXELSMCP_API_KEY"))
	if value := strings.TrimSpace(getenv("PIXELSMCP_BASE_URL")); value != "" {
		cfg.baseURL = value
	}
	if value := strings.TrimSpace(getenv("PIXELSMCP_MODEL")); value != "" {
		cfg.model = value
	} else if value := strings.TrimSpace(getenv("PIXELSMCP_IMAGE_MODEL")); value != "" {
		cfg.model = value
	}
	if value := strings.TrimSpace(getenv("PIXELSMCP_EXTRA_HEADERS")); value != "" {
		headers, err := parseExtraHeaders(value)
		if err != nil {
			return config{}, fmt.Errorf("PIXELSMCP_EXTRA_HEADERS: %w", err)
		}
		cfg.extraHeaders = headers
	}
	if value := strings.TrimSpace(getenv("PIXELSMCP_TIMEOUT")); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return config{}, fmt.Errorf("PIXELSMCP_TIMEOUT: %w", err)
		}
		cfg.timeout = timeout
	}
	if value := strings.TrimSpace(getenv("PIXELSMCP_IMAGE_SAVE_DIR")); value != "" {
		cfg.imageSaveDir = value
	}

	switch cfg.transport {
	case transportStdio, transportHTTP:
	default:
		return config{}, fmt.Errorf("PIXELSMCP_TRANSPORT must be one of stdio or http, got %q", cfg.transport)
	}
	if cfg.mcpEndpoint == cfg.healthEndpoint {
		return config{}, fmt.Errorf("PIXELSMCP_ENDPOINT and PIXELSMCP_HEALTH_ENDPOINT must be different")
	}

	return cfg, nil
}

func normalizePath(value string) string {
	value = "/" + strings.Trim(value, "/")
	if value == "/" {
		return value
	}
	return strings.TrimRight(value, "/")
}

func parseExtraHeaders(value string) (map[string]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	if strings.HasPrefix(value, "{") {
		var parsed map[string]string
		if err := json.Unmarshal([]byte(value), &parsed); err != nil {
			return nil, err
		}
		return parsed, nil
	}

	headers := make(map[string]string)
	for _, item := range splitHeaderItems(value) {
		key, val, ok := strings.Cut(item, "=")
		if !ok {
			key, val, ok = strings.Cut(item, ":")
		}
		if !ok {
			return nil, fmt.Errorf("invalid header %q", item)
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if key == "" {
			return nil, fmt.Errorf("invalid header %q", item)
		}
		headers[key] = val
	}

	return headers, nil
}

func splitHeaderItems(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
}

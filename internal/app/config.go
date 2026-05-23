package app

import (
	"fmt"
	"strings"
)

type transport string

const (
	transportStdio transport = "stdio"
	transportHTTP  transport = "http"

	defaultHTTPAddr       = ":8080"
	defaultMCPEndpoint    = "/mcp"
	defaultHealthEndpoint = "/healthz"
	defaultImageSaveDir   = "./generated-images"
	defaultImageModel     = "Kwai-Kolors/Kolors"
	defaultBaseURL        = ""
)

type config struct {
	transport      transport
	addr           string
	mcpEndpoint    string
	healthEndpoint string
	corsOrigins    []string
	apiKey         string
	baseURL        string
	imageModel     string
	imageSaveDir   string
}

func configFromEnv(getenv func(string) string) (config, error) {
	cfg := config{
		transport:      transportStdio,
		addr:           defaultHTTPAddr,
		mcpEndpoint:    defaultMCPEndpoint,
		healthEndpoint: defaultHealthEndpoint,
		baseURL:        defaultBaseURL,
		imageModel:     defaultImageModel,
		imageSaveDir:   defaultImageSaveDir,
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
	cfg.apiKey = strings.TrimSpace(getenv("PIXELSMCP_API_KEY"))
	if value := strings.TrimSpace(getenv("PIXELSMCP_BASE_URL")); value != "" {
		cfg.baseURL = value
	}
	if value := strings.TrimSpace(getenv("PIXELSMCP_IMAGE_MODEL")); value != "" {
		cfg.imageModel = value
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

package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/server"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
	"github.com/oxkrypton/PixelsMcp/internal/transport/mcpserver"
)

const (
	serverName    = "PixelsMcp"
	serverVersion = "0.1.0"
)

func Run() error {
	_ = godotenv.Load(".env.local", ".env")

	cfg, err := configFromEnv(os.Getenv)
	if err != nil {
		return err
	}
	if cfg.apiKey == "" {
		return errors.New("PIXELSMCP_API_KEY is required")
	}
	if cfg.baseURL == "" {
		return errors.New("PIXELSMCP_BASE_URL is required")
	}

	imageService := imagegen.NewService(imagegen.Config{
		APIKey:  cfg.apiKey,
		BaseURL: cfg.baseURL,
		Model:   cfg.imageModel,
		SaveDir: cfg.imageSaveDir,
	})
	mcpServer := mcpserver.New(serverName, serverVersion, imageService)

	switch cfg.transport {
	case transportStdio:
		return server.ServeStdio(mcpServer)
	case transportHTTP:
		return serveHTTP(mcpServer, cfg)
	default:
		return fmt.Errorf("unsupported transport %q", cfg.transport)
	}
}

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
	shutdownTimeout       = 10 * time.Second
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

func serveHTTP(mcpServer *server.MCPServer, cfg config) error {
	opts := []server.StreamableHTTPOption{
		server.WithHeartbeatInterval(30 * time.Second),
		server.WithSessionIdleTTL(24 * time.Hour),
	}
	if len(cfg.corsOrigins) > 0 {
		opts = append(opts, server.WithStreamableHTTPCORS(
			server.WithCORSAllowedOrigins(cfg.corsOrigins...),
			server.WithCORSAllowedMethods(http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodOptions),
			server.WithCORSAllowedHeaders("Content-Type", server.HeaderKeySessionID),
			server.WithCORSExposedHeaders(server.HeaderKeySessionID),
		))
	}

	streamableSrv := server.NewStreamableHTTPServer(mcpServer, opts...)
	mux := http.NewServeMux()
	mux.Handle(cfg.mcpEndpoint, streamableSrv)
	mux.HandleFunc(cfg.healthEndpoint, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	httpServer := &http.Server{
		Addr:              cfg.addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("PixelsMcp listening on %s%s", cfg.addr, cfg.mcpEndpoint)
		errCh <- httpServer.ListenAndServe()
	}()

	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stopCh)

	select {
	case sig := <-stopCh:
		log.Printf("received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		shutdownErr := streamableSrv.Shutdown(ctx)
		if err := httpServer.Shutdown(ctx); err != nil && shutdownErr == nil {
			shutdownErr = err
		}
		return shutdownErr
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

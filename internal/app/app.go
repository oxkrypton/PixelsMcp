package app

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"github.com/oxkrypton/PixelsMcp/internal/server"
	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
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
	mcpServer := server.New(serverName, serverVersion, imageService)

	switch cfg.transport {
	case transportStdio:
		return mcpgo.ServeStdio(mcpServer)
	case transportHTTP:
		return serveHTTP(mcpServer, cfg)
	default:
		return fmt.Errorf("unsupported transport %q", cfg.transport)
	}
}

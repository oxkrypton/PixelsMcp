package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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
	return Execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
}

func Execute(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	_ = godotenv.Load(".env.local", ".env")

	if len(args) > 0 {
		switch strings.ToLower(strings.TrimSpace(args[0])) {
		case "init", "setup":
			return runInit(".env.local", stdin, stdout, stderr, os.Getenv, isInteractiveTerminal(stdin))
		case "help", "-h", "--help":
			writeUsage(stdout)
			return nil
		default:
			return fmt.Errorf("unknown command %q", args[0])
		}
	}

	return runServer(os.Getenv)
}

func runServer(getenv func(string) string) error {
	cfg, err := configFromEnv(getenv)
	if err != nil {
		return err
	}
	if cfg.apiKey == "" {
		return errors.New("PIXELSMCP_API_KEY is required")
	}
	if cfg.baseURL == "" {
		return errors.New("PIXELSMCP_BASE_URL is required")
	}

	imageService, err := imagegen.NewService(imagegen.Config{
		Provider:     cfg.provider,
		APIKey:       cfg.apiKey,
		BaseURL:      cfg.baseURL,
		Model:        cfg.model,
		ExtraHeaders: cfg.extraHeaders,
		Timeout:      cfg.timeout,
		SaveDir:      cfg.imageSaveDir,
	})
	if err != nil {
		return err
	}

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

func writeUsage(out io.Writer) {
	_, _ = fmt.Fprintln(out, "PixelsMcp")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Usage:")
	_, _ = fmt.Fprintln(out, "  pixelsmcp           Run the HTTP MCP service")
	_, _ = fmt.Fprintln(out, "  pixelsmcp init      Create or update developer .env.local")
	_, _ = fmt.Fprintln(out, "  pixelsmcp setup     Alias for init")
	_, _ = fmt.Fprintln(out, "")
	_, _ = fmt.Fprintln(out, "Set PIXELSMCP_TRANSPORT=stdio only for local developer MCP debugging.")
}

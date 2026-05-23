package app

import (
	"github.com/mark3labs/mcp-go/server"

	"github.com/oxkrypton/PixelsMcp/internal/service/echo"
	"github.com/oxkrypton/PixelsMcp/internal/transport/mcpserver"
)

const (
	serverName    = "PixelsMcp"
	serverVersion = "0.1.0"
)

func Run() error {
	echoService := echo.NewService()
	mcpServer := mcpserver.New(serverName, serverVersion, echoService)

	return server.ServeStdio(mcpServer)
}

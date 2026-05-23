package mcpserver

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	echosvc "github.com/oxkrypton/PixelsMcp/internal/service/echo"
)

type EchoArgs struct {
	Message string `json:"message" jsonschema:"Message to echo back"`
}

func New(name, version string, echoService *echosvc.Service) *server.MCPServer {
	srv := server.NewMCPServer(name, version, server.WithToolCapabilities(false))

	handler := &echoToolHandler{
		service: echoService,
	}

	srv.AddTool(newEchoTool(), mcp.NewTypedToolHandler(handler.handle))

	return srv
}

type echoToolHandler struct {
	service *echosvc.Service
}

func (h *echoToolHandler) handle(ctx context.Context, _ mcp.CallToolRequest, args EchoArgs) (*mcp.CallToolResult, error) {
	echoed, err := h.service.Echo(ctx, args.Message)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("echo failed: %v", err)), nil
	}

	return mcp.NewToolResultText(echoed), nil
}

func newEchoTool() mcp.Tool {
	return mcp.NewTool(
		"echo",
		mcp.WithDescription("Return the provided message unchanged"),
		mcp.WithInputSchema[EchoArgs](),
	)
}

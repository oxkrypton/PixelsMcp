package server

import (
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

func New(name, version string, imageService *imagegen.Service) *mcpgo.MCPServer {
	srv := mcpgo.NewMCPServer(name, version, mcpgo.WithToolCapabilities(false), mcpgo.WithRoots())

	imageHandler := &generateImageToolHandler{
		service: imageService,
		roots:   srv,
	}
	spriteSheetHandler := &generateSpriteSheetToolHandler{
		service: imageService,
		roots:   srv,
	}

	srv.AddTool(newGenerateImageTool(), mcp.NewTypedToolHandler(imageHandler.handle))
	srv.AddTool(newGenerateSpriteSheetTool(), mcp.NewTypedToolHandler(spriteSheetHandler.handle))

	return srv
}

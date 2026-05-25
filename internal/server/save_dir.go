package server

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

type rootsRequester interface {
	RequestRoots(context.Context, mcp.ListRootsRequest) (*mcp.ListRootsResult, error)
}

func saveDirFromRoots(ctx context.Context, roots rootsRequester) string {
	if roots == nil {
		return ""
	}

	result, err := roots.RequestRoots(ctx, mcp.ListRootsRequest{})
	if err != nil || result == nil {
		return ""
	}

	for _, root := range result.Roots {
		if saveDir, ok := saveDirFromRootURI(root.URI); ok {
			return saveDir
		}
	}

	return ""
}

func saveDirFromRootURI(rootURI string) (string, bool) {
	rootURI = strings.TrimSpace(rootURI)
	if rootURI == "" {
		return "", false
	}

	parsed, err := url.Parse(rootURI)
	if err != nil || parsed.Scheme != "file" {
		return "", false
	}
	if parsed.Host != "" && parsed.Host != "localhost" {
		return "", false
	}

	path := parsed.Path
	if path == "" {
		return "", false
	}
	if unescaped, err := url.PathUnescape(path); err == nil {
		path = unescaped
	}
	if len(path) >= 3 && path[0] == '/' && path[2] == ':' {
		path = path[1:]
	}

	return filepath.Join(filepath.FromSlash(path), "generated-images"), true
}

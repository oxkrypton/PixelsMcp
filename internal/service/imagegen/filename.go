package imagegen

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

func extensionForContentType(contentType string) string {
	switch {
	case strings.Contains(contentType, "image/png"):
		return ".png"
	case strings.Contains(contentType, "image/jpeg"):
		return ".jpg"
	case strings.Contains(contentType, "image/jpg"):
		return ".jpg"
	case strings.Contains(contentType, "image/webp"):
		return ".webp"
	case strings.Contains(contentType, "image/gif"):
		return ".gif"
	default:
		return ""
	}
}

func extensionFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return filepath.Ext(parsed.Path)
}

func buildFileName(prefix, model string, seed int64, ext string) string {
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		" ", "-",
		":", "-",
		"?", "-",
		"&", "-",
		"=", "-",
		",", "-",
	)
	safeModel := replacer.Replace(model)
	if safeModel == "" {
		safeModel = "image"
	}

	prefix = replacer.Replace(strings.TrimSpace(prefix))
	if prefix != "" {
		prefix += "-"
	}

	return fmt.Sprintf("%s%s-%d-%d%s", prefix, safeModel, time.Now().UTC().UnixNano(), seed, ext)
}

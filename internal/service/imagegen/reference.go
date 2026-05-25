package imagegen

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const maxReferenceImageBytes int64 = 10 * 1024 * 1024

func prepareReferenceInput(generation GenerationOptions) (GenerationOptions, error) {
	referenceImage := strings.TrimSpace(generation.ReferenceImage)
	referencePath := strings.TrimSpace(generation.ReferencePath)

	if referenceImage != "" && referencePath != "" {
		return generation, errors.New("reference_image and reference_path are mutually exclusive")
	}

	if referenceImage != "" {
		if !isReferenceImageURL(referenceImage) {
			return generation, errors.New("reference_image must be an http(s) URL; use reference_path for local image files")
		}
		generation.ReferenceImage = referenceImage
		return generation, nil
	}

	if referencePath != "" {
		referenceDataURL, err := referenceDataURLFromPath(referencePath)
		if err != nil {
			return generation, err
		}
		generation.ReferenceImage = referenceDataURL
		generation.ReferencePath = ""
	}

	return generation, nil
}

func referenceDataURLFromPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("reference_path must be an absolute path")
	}

	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("check reference_path: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("reference_path must point to a regular image file")
	}
	if info.Size() > maxReferenceImageBytes {
		return "", fmt.Errorf("reference_path image exceeds the 10 MB limit")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read reference_path: %w", err)
	}
	if int64(len(data)) > maxReferenceImageBytes {
		return "", fmt.Errorf("reference_path image exceeds the 10 MB limit")
	}

	mimeType := inferImageMimeType(data)
	if mimeType == "" {
		return "", errors.New("reference_path must point to a PNG, JPEG, GIF, WebP, BMP, or TIFF image")
	}

	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data), nil
}

func normalizeReferenceImage(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if isProviderReferenceImage(value) {
		return value, nil
	}
	return "", errors.New("reference_image must be an http(s) URL or a provider-prepared data:image URL")
}

func isReferenceImageURL(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

func isProviderReferenceImage(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return isReferenceImageURL(value) || strings.HasPrefix(value, "data:image/")
}

func inferImageMimeType(data []byte) string {
	switch {
	case len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png"
	case len(data) >= 3 && bytes.Equal(data[:3], []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case len(data) >= 6 && (bytes.Equal(data[:6], []byte("GIF87a")) || bytes.Equal(data[:6], []byte("GIF89a"))):
		return "image/gif"
	case len(data) >= 12 && bytes.Equal(data[:4], []byte("RIFF")) && bytes.Equal(data[8:12], []byte("WEBP")):
		return "image/webp"
	case len(data) >= 2 && bytes.Equal(data[:2], []byte("BM")):
		return "image/bmp"
	case len(data) >= 4 && (bytes.Equal(data[:4], []byte{0x49, 0x49, 0x2A, 0x00}) || bytes.Equal(data[:4], []byte{0x4D, 0x4D, 0x00, 0x2A})):
		return "image/tiff"
	default:
		return ""
	}
}

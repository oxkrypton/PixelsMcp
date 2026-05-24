package app

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

const defaultInteractiveBaseURL = "https://api.openai.com"

func promptProvider(reader *bufio.Reader, stdout io.Writer, current string) (string, error) {
	defaultValue := normalizeSupportedProvider(current)
	if defaultValue == "" {
		defaultValue = imagegen.DefaultProvider
	}

	value, err := promptLine(reader, stdout, "Provider", defaultValue, true)
	if err != nil {
		return "", err
	}

	provider := normalizeSupportedProvider(value)
	if provider == "" {
		return "", fmt.Errorf("unsupported provider %q", value)
	}

	return provider, nil
}

func promptModelFromList(reader *bufio.Reader, stdout io.Writer, models []string, current string) (string, error) {
	defaultModel := strings.TrimSpace(current)
	if defaultModel == "" {
		defaultModel = models[0]
	}
	if idx := indexOf(models, defaultModel); idx >= 0 {
		defaultModel = models[idx]
	} else {
		defaultModel = models[0]
	}

	_, _ = fmt.Fprintln(stdout, "Available models:")
	for i, model := range models {
		_, _ = fmt.Fprintf(stdout, "  %d) %s\n", i+1, model)
	}

	value, err := promptLine(reader, stdout, "Model", defaultModel, true)
	if err != nil {
		return "", err
	}

	if idx, err := strconv.Atoi(value); err == nil {
		if idx >= 1 && idx <= len(models) {
			return models[idx-1], nil
		}
		return "", fmt.Errorf("model selection %d is out of range", idx)
	}

	if strings.TrimSpace(value) == "" {
		return defaultModel, nil
	}
	return strings.TrimSpace(value), nil
}

func promptExtraHeaders(reader *bufio.Reader, stdout io.Writer, current map[string]string) (map[string]string, error) {
	defaultValue := encodeStringMap(current)
	value, err := promptLine(reader, stdout, "Extra headers (JSON object)", defaultValue, false)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(value) == "" {
		return current, nil
	}
	return parseExtraHeaders(value)
}

func promptDuration(reader *bufio.Reader, stdout io.Writer, current time.Duration) (time.Duration, error) {
	defaultValue := current.String()
	value, err := promptLine(reader, stdout, "Timeout", defaultValue, true)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(value) == "" {
		return current, nil
	}
	return time.ParseDuration(strings.TrimSpace(value))
}

func promptRequiredLine(reader *bufio.Reader, stdout io.Writer, label, defaultValue string) (string, error) {
	return promptLine(reader, stdout, label, defaultValue, true)
}

func promptSecretLine(reader *bufio.Reader, stdout io.Writer, label, current string) (string, error) {
	display := ""
	if strings.TrimSpace(current) != "" {
		display = "set"
	}

	for {
		if display != "" {
			_, _ = fmt.Fprintf(stdout, "%s [%s]: ", label, display)
		} else {
			_, _ = fmt.Fprintf(stdout, "%s: ", label)
		}

		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}

		value := strings.TrimSpace(line)
		if value == "" {
			if strings.TrimSpace(current) != "" {
				return current, nil
			}
			return "", fmt.Errorf("%s is required", strings.ToLower(label))
		}
		return value, nil
	}
}

func promptLine(reader *bufio.Reader, stdout io.Writer, label, defaultValue string, required bool) (string, error) {
	for {
		if defaultValue != "" {
			_, _ = fmt.Fprintf(stdout, "%s [%s]: ", label, defaultValue)
		} else {
			_, _ = fmt.Fprintf(stdout, "%s: ", label)
		}

		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}

		value := strings.TrimSpace(line)
		if value == "" {
			if defaultValue != "" {
				return defaultValue, nil
			}
			if required {
				return "", fmt.Errorf("%s is required", strings.ToLower(label))
			}
			return "", nil
		}
		return value, nil
	}
}

func normalizeSupportedProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", imagegen.ProviderOpenAICompatible, "openai":
		return imagegen.ProviderOpenAICompatible
	default:
		return ""
	}
}

func isInteractiveTerminal(stdin io.Reader) bool {
	file, ok := stdin.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func indexOf(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}

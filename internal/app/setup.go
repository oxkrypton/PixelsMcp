package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

const defaultInteractiveBaseURL = "https://api.openai.com"

func runInit(configPath string, stdin io.Reader, stdout, stderr io.Writer, getenv func(string) string, interactive bool) error {
	cfg, err := configFromEnv(getenv)
	if err != nil {
		return err
	}

	if interactive {
		cfg, err = interactiveInitConfig(cfg, stdin, stdout)
		if err != nil {
			return err
		}
	} else {
		cfg, err = nonInteractiveInitConfig(cfg)
		if err != nil {
			return err
		}
	}

	provider, err := imagegen.NewProvider(imagegen.ProviderConfig{
		Provider:     cfg.provider,
		APIKey:       cfg.apiKey,
		BaseURL:      cfg.baseURL,
		Model:        cfg.model,
		ExtraHeaders: cfg.extraHeaders,
		Timeout:      cfg.timeout,
	})
	if err != nil {
		return err
	}

	backup, existed, err := readFileIfExists(configPath)
	if err != nil {
		return err
	}

	if err := writeEnvFile(configPath, serializeConfig(cfg)); err != nil {
		return err
	}

	verifyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := provider.Validate(verifyCtx); err != nil {
		if restoreErr := restoreFile(configPath, backup, existed); restoreErr != nil {
			return fmt.Errorf("%w (and rollback failed: %v)", err, restoreErr)
		}
		return err
	}

	_, _ = fmt.Fprintln(stdout, "Saved configuration to "+configPath)
	return nil
}

func interactiveInitConfig(base config, stdin io.Reader, stdout io.Writer) (config, error) {
	reader := bufio.NewReader(stdin)
	cfg := base

	provider, err := promptProvider(reader, stdout, cfg.provider)
	if err != nil {
		return config{}, err
	}
	cfg.provider = provider

	baseURLDefault := strings.TrimSpace(cfg.baseURL)
	if baseURLDefault == "" {
		baseURLDefault = defaultInteractiveBaseURL
	}
	cfg.baseURL, err = promptRequiredLine(reader, stdout, "Base URL", baseURLDefault)
	if err != nil {
		return config{}, err
	}

	cfg.apiKey, err = promptSecretLine(reader, stdout, "API key", cfg.apiKey)
	if err != nil {
		return config{}, err
	}

	extraHeaders, err := promptExtraHeaders(reader, stdout, cfg.extraHeaders)
	if err != nil {
		return config{}, err
	}
	cfg.extraHeaders = extraHeaders

	timeout, err := promptDuration(reader, stdout, cfg.timeout)
	if err != nil {
		return config{}, err
	}
	cfg.timeout = timeout

	probe, err := imagegen.NewProvider(imagegen.ProviderConfig{
		Provider:     cfg.provider,
		APIKey:       cfg.apiKey,
		BaseURL:      cfg.baseURL,
		Model:        cfg.model,
		ExtraHeaders: cfg.extraHeaders,
		Timeout:      cfg.timeout,
	})
	if err != nil {
		return config{}, err
	}

	models, err := probe.ListModels(context.Background())
	if err == nil && len(models) > 0 {
		cfg.model, err = promptModelFromList(reader, stdout, models, cfg.model)
		if err != nil {
			return config{}, err
		}
	} else {
		if err != nil {
			_, _ = fmt.Fprintln(stdout, "Model list unavailable; enter a model manually.")
		}
		cfg.model, err = promptRequiredLine(reader, stdout, "Model", cfg.model)
		if err != nil {
			return config{}, err
		}
	}

	return cfg, nil
}

func nonInteractiveInitConfig(cfg config) (config, error) {
	rawProvider := cfg.provider
	provider := normalizeSupportedProvider(rawProvider)
	if provider == "" {
		return config{}, fmt.Errorf("unsupported provider %q for non-interactive init", rawProvider)
	}
	cfg.provider = provider
	if strings.TrimSpace(cfg.baseURL) == "" {
		return config{}, errors.New("PIXELSMCP_BASE_URL is required for non-interactive init")
	}
	if strings.TrimSpace(cfg.apiKey) == "" {
		return config{}, errors.New("PIXELSMCP_API_KEY is required for non-interactive init")
	}
	if strings.TrimSpace(cfg.model) == "" {
		cfg.model = imagegen.DefaultModel
	}
	return cfg, nil
}

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

func serializeConfig(cfg config) map[string]string {
	values := map[string]string{
		"PIXELSMCP_TRANSPORT":       string(cfg.transport),
		"PIXELSMCP_ADDR":            cfg.addr,
		"PIXELSMCP_ENDPOINT":        cfg.mcpEndpoint,
		"PIXELSMCP_HEALTH_ENDPOINT": cfg.healthEndpoint,
		"PIXELSMCP_CORS_ORIGINS":    strings.Join(cfg.corsOrigins, ","),
		"PIXELSMCP_PROVIDER":        cfg.provider,
		"PIXELSMCP_BASE_URL":        cfg.baseURL,
		"PIXELSMCP_API_KEY":         cfg.apiKey,
		"PIXELSMCP_MODEL":           cfg.model,
		"PIXELSMCP_EXTRA_HEADERS":   encodeStringMap(cfg.extraHeaders),
		"PIXELSMCP_TIMEOUT":         cfg.timeout.String(),
		"PIXELSMCP_IMAGE_SAVE_DIR":  cfg.imageSaveDir,
	}

	return values
}

func encodeStringMap(values map[string]string) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		if strings.TrimSpace(key) != "" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteByte('{')
	for i, key := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Quote(key))
		b.WriteByte(':')
		b.WriteString(strconv.Quote(values[key]))
	}
	b.WriteByte('}')
	return b.String()
}

func indexOf(values []string, target string) int {
	for i, value := range values {
		if value == target {
			return i
		}
	}
	return -1
}

func readFileIfExists(path string) ([]byte, bool, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return data, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	return nil, false, err
}

func restoreFile(path string, data []byte, existed bool) error {
	if !existed {
		return os.Remove(path)
	}
	return os.WriteFile(path, data, 0o600)
}

func writeEnvFile(path string, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("# Generated by PixelsMcp init for developer debugging\n")
	for _, key := range keys {
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(strconv.Quote(values[key]))
		b.WriteByte('\n')
	}

	return os.WriteFile(path, []byte(b.String()), 0o600)
}

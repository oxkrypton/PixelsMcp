package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	imagegen "github.com/oxkrypton/PixelsMcp/internal/service/imagegen"
)

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
		Provider:       cfg.provider,
		APIKey:         cfg.apiKey,
		BaseURL:        cfg.baseURL,
		Model:          cfg.model,
		ReferenceModel: cfg.referenceModel,
		ExtraHeaders:   cfg.extraHeaders,
		Timeout:        cfg.timeout,
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
		Provider:       cfg.provider,
		APIKey:         cfg.apiKey,
		BaseURL:        cfg.baseURL,
		Model:          cfg.model,
		ReferenceModel: cfg.referenceModel,
		ExtraHeaders:   cfg.extraHeaders,
		Timeout:        cfg.timeout,
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

	cfg.referenceModel, err = promptRequiredLine(reader, stdout, "Reference model", cfg.referenceModel)
	if err != nil {
		return config{}, err
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
	if strings.TrimSpace(cfg.referenceModel) == "" {
		cfg.referenceModel = imagegen.DefaultReferenceModel
	}
	return cfg, nil
}

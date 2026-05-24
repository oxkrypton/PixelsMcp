package imagegen

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func responseError(prefix string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if len(body) > 0 {
		return fmt.Errorf("%s: status %d: %s", prefix, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return fmt.Errorf("%s: status %d", prefix, resp.StatusCode)
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}

	clone := make(map[string]string, len(src))
	for key, value := range src {
		clone[key] = value
	}
	return clone
}

func firstHeaderValue(header http.Header, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

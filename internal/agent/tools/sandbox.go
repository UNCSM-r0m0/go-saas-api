package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPSandboxClient executes code via HTTP to the sandbox-service.
type HTTPSandboxClient struct {
	baseURL string
	client  *http.Client
}

// NewHTTPSandboxClient creates a new HTTP sandbox client.
func NewHTTPSandboxClient(baseURL string) *HTTPSandboxClient {
	return &HTTPSandboxClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Execute sends code to the sandbox-service and returns the output.
func (c *HTTPSandboxClient) Execute(ctx context.Context, code, language string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"code":     code,
		"language": language,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/execute", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("sandbox returned %d", resp.StatusCode)
	}

	var result struct {
		Output string `json:"output"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("sandbox error: %s", result.Error)
	}
	return result.Output, nil
}

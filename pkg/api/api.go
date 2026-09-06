package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL string
	Secret  string
	client  *http.Client
}

func NewClient(baseURL, secret string) *Client {
	return &Client{
		BaseURL: baseURL,
		Secret:  secret,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *Client) do(method, path string, body []byte) ([]byte, int, error) {
	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, 0, err
	}

	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	return respBody, resp.StatusCode, err
}

func (c *Client) Reload(configPath string) error {
	payload, _ := json.Marshal(map[string]string{"path": configPath})
	data, code, err := c.do("PUT", "/configs?force=true", payload)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	if code != http.StatusOK && code != http.StatusNoContent {
		return fmt.Errorf("HTTP %d: %s", code, string(data))
	}
	return nil
}

func (c *Client) GetVersion() (string, error) {
	data, code, err := c.do("GET", "/version", nil)
	if err != nil {
		return "", err
	}
	if code != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", code, string(data))
	}
	var res struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return "", err
	}
	return res.Version, nil
}

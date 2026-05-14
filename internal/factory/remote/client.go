package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

type PackMetadata struct {
	ID                int    `json:"id"`
	PackName          string `json:"pack_name"`
	PackVersion       string `json:"pack_version"`
	SchemaVersion     string `json:"schema_version"`
	CardSchemaVersion string `json:"card_schema_version"`
	CompiledCaseCount int    `json:"compiled_case_count"`
	Checksum          string `json:"checksum"`
	Publisher         string `json:"publisher"`
	CreatedAt         string `json:"created_at"`
	DownloadURL       string `json:"download_url,omitempty"`
}

func (c *Client) PublishPack(ctx context.Context, packPath string) (*PackMetadata, error) {
	data, err := os.ReadFile(packPath)
	if err != nil {
		return nil, fmt.Errorf("read pack: %w", err)
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/factory/packs", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	var metadata PackMetadata
	if err := c.doJSON(req, http.StatusCreated, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (c *Client) GetLatestPack(ctx context.Context) (*PackMetadata, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/factory/packs/latest", nil)
	if err != nil {
		return nil, err
	}
	var metadata PackMetadata
	if err := c.doJSON(req, http.StatusOK, &metadata); err != nil {
		return nil, err
	}
	return &metadata, nil
}

func (c *Client) DownloadLatestPack(ctx context.Context, destPath string) (*PackMetadata, error) {
	metadata, err := c.GetLatestPack(ctx)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest(ctx, http.MethodGet, "/factory/packs/latest/download", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("download latest factory pack: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, responseError("download latest factory pack", resp)
	}
	out, err := os.Create(destPath)
	if err != nil {
		return nil, fmt.Errorf("create destination pack: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return nil, fmt.Errorf("write destination pack: %w", err)
	}
	if err := out.Close(); err != nil {
		return nil, fmt.Errorf("close destination pack: %w", err)
	}
	return metadata, nil
}

func (c *Client) newRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	base, err := normalizedBaseURL(c.BaseURL)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, body)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(c.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))
	}
	return req, nil
}

func (c *Client) doJSON(req *http.Request, wantStatus int, out any) error {
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("factory server request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		return responseError("factory server request", resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode factory server response: %w", err)
	}
	return nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func normalizedBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("factory server URL is required")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid factory server URL")
	}
	return strings.TrimRight(raw, "/"), nil
}

func responseError(action string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = resp.Status
	}
	return fmt.Errorf("%s: %s", action, message)
}

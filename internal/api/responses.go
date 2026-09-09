package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ProbeOptions struct {
	Model     string
	BaseURL   string
	APIKey    string
	Timeout   time.Duration
	UserAgent string
}

type ProbeResult struct {
	Endpoint       string
	HTTPStatus     int
	ResponseStatus string
	ResponseModel  string
	Duration       time.Duration
}

type Client struct {
	options    ProbeOptions
	httpClient *http.Client
}

func NewProbeClient(options ProbeOptions) (*Client, error) {
	if options.Model == "" {
		return nil, errors.New("model is required")
	}
	if options.BaseURL == "" {
		return nil, errors.New("base_url is required")
	}
	if options.Timeout <= 0 {
		options.Timeout = 60 * time.Second
	}
	if options.UserAgent == "" {
		options.UserAgent = "zola"
	}
	u, err := url.Parse(options.BaseURL)
	if err != nil {
		return nil, err
	}
	if u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("base_url must be an http(s) URL")
	}
	options.BaseURL = strings.TrimRight(options.BaseURL, "/")
	return &Client{
		options: options,
		httpClient: &http.Client{
			Timeout: options.Timeout,
		},
	}, nil
}

func (c *Client) Probe(ctx context.Context) (ProbeResult, error) {
	endpoint := c.options.BaseURL + "/responses"
	payload := map[string]any{
		"model":             c.options.Model,
		"input":             "Reply with the single word OK.",
		"stream":            false,
		"max_output_tokens": 8,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("encode probe request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ProbeResult{}, fmt.Errorf("build probe request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.options.UserAgent)
	if c.options.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.options.APIKey)
	}

	started := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(started)
	if err != nil {
		return ProbeResult{
			Endpoint: endpoint,
			Duration: duration,
		}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	result := ProbeResult{
		Endpoint:   endpoint,
		HTTPStatus: resp.StatusCode,
		Duration:   duration,
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return result, statusError(resp.StatusCode)
	}

	var response struct {
		ID     string `json:"id"`
		Model  string `json:"model"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return result, fmt.Errorf("parse Responses API response: %w", err)
	}
	result.ResponseStatus = response.Status
	result.ResponseModel = response.Model
	if result.ResponseStatus == "" {
		result.ResponseStatus = "completed"
	}
	return result, nil
}

func statusError(status int) error {
	switch {
	case status == http.StatusUnauthorized:
		return errors.New("authentication failed: the provider rejected the API key")
	case status == http.StatusForbidden:
		return errors.New("authentication failed: the API key is not allowed to use this endpoint")
	case status == http.StatusNotFound:
		return errors.New("endpoint or model not found; check base_url and model")
	case status == http.StatusTooManyRequests:
		return errors.New("rate limited: the provider returned HTTP 429")
	case status >= 500:
		return fmt.Errorf("provider server error: HTTP %d", status)
	default:
		return fmt.Errorf("provider rejected the probe: HTTP %d", status)
	}
}

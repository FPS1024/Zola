package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestProbeResponsesAPI(t *testing.T) {
	var received struct {
		Model     string `json:"model"`
		Input     string `json:"input"`
		Stream    bool   `json:"stream"`
		MaxTokens int    `json:"max_output_tokens"`
	}
	client, err := NewProbeClient(ProbeOptions{
		Model:     "deepseek-v4-flash",
		BaseURL:   "https://api.test.invalid",
		APIKey:    "sk-secret",
		Timeout:   time.Second,
		UserAgent: "zola-test",
	})
	if err != nil {
		t.Fatalf("NewProbeClient: %v", err)
	}
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/responses" {
			t.Errorf("path = %q, want /responses", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-secret" {
			t.Errorf("Authorization = %q, want Bearer sk-secret", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		return jsonResponse(r, `{"id":"resp_1","model":"deepseek-v4-flash","status":"completed"}`), nil
	})

	result, err := client.Probe(context.Background())
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if result.HTTPStatus != http.StatusOK {
		t.Fatalf("HTTPStatus = %d, want 200", result.HTTPStatus)
	}
	if result.ResponseStatus != "completed" {
		t.Fatalf("ResponseStatus = %q, want completed", result.ResponseStatus)
	}
	if result.ResponseModel != "deepseek-v4-flash" {
		t.Fatalf("ResponseModel = %q, want deepseek-v4-flash", result.ResponseModel)
	}
	if received.Model != "deepseek-v4-flash" || received.Stream || received.MaxTokens == 0 {
		t.Fatalf("received probe request = %+v", received)
	}
}

func TestProbeReportsAuthFailureWithoutSecret(t *testing.T) {
	client, err := NewProbeClient(ProbeOptions{
		Model:     "deepseek-v4-flash",
		BaseURL:   "https://api.test.invalid",
		APIKey:    "sk-secret",
		Timeout:   time.Second,
		UserAgent: "zola-test",
	})
	if err != nil {
		t.Fatalf("NewProbeClient: %v", err)
	}
	client.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Status:     "401 Unauthorized",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"Bearer sk-secret is invalid"}}`)),
			Request:    r,
		}, nil
	})
	_, err = client.Probe(context.Background())
	if err == nil {
		t.Fatal("Probe succeeded, want error")
	}
	if strings.Contains(err.Error(), "sk-secret") {
		t.Fatalf("error leaks API key: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(r *http.Request, body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(strings.NewReader(body)),
		Request: r,
	}
}

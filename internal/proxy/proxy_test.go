package proxy

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zola/internal/config"
)

func TestHealthDoesNotExposeKey(t *testing.T) {
	handler := NewHandler(HandlerOptions{
		Provider: config.Provider{
			ID: "deepseek", Name: "DeepSeek", Model: "deepseek-v4-flash",
			WireAPI: config.WireAPIResponses, BaseURL: "https://api.deepseek.com",
		},
		APIKey: "sk-secret",
	})
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "sk-secret") {
		t.Fatal("health response leaks API key")
	}
}

func TestForwardResponsesNonStreaming(t *testing.T) {
	var upstreamRequest *http.Request
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		upstreamRequest = r
		body := `{"id":"resp_1","model":"deepseek-v4-flash","status":"completed"}`
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})}
	handler := NewHandler(HandlerOptions{
		Provider: config.Provider{
			ID: "deepseek", Name: "DeepSeek", Model: "deepseek-v4-flash",
			WireAPI: config.WireAPIResponses, BaseURL: "https://api.deepseek.com",
		},
		APIKey:     "sk-secret",
		HTTPClient: client,
		Logger:     log.New(io.Discard, "", 0),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"deepseek-v4-flash","input":"hello","stream":false}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if upstreamRequest == nil {
		t.Fatal("upstream request was not made")
	}
	if upstreamRequest.URL.String() != "https://api.deepseek.com/responses" {
		t.Fatalf("upstream URL = %q", upstreamRequest.URL.String())
	}
	if got := upstreamRequest.Header.Get("Authorization"); got != "Bearer sk-secret" {
		t.Fatalf("Authorization = %q", got)
	}
	if !strings.Contains(rec.Body.String(), `"status":"completed"`) {
		t.Fatalf("response = %s", rec.Body.String())
	}
}

func TestForwardRewritesDisguisedModel(t *testing.T) {
	var upstreamModel string
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode upstream body: %v", err)
		}
		upstreamModel, _ = body["model"].(string)
		return jsonResponse(r, `{"id":"resp_1","model":"deepseek-v4-pro","status":"completed"}`), nil
	})}
	handler := NewHandler(HandlerOptions{
		Provider: config.Provider{
			ID: "deepseek", Name: "DeepSeek", Model: "deepseek-v4-pro",
			CodexModel: "gpt-5.4", WireAPI: config.WireAPIResponses,
			BaseURL: "https://api.deepseek.com",
		},
		APIKey:     "sk-secret",
		HTTPClient: client,
		Logger:     log.New(io.Discard, "", 0),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"gpt-5.4","input":"hello","stream":false}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if upstreamModel != "deepseek-v4-pro" {
		t.Fatalf("upstream model = %q, want deepseek-v4-pro", upstreamModel)
	}
}

func TestForwardSSEResponseFlushes(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header: http.Header{
				"Content-Type": []string{"text/event-stream"},
			},
			Body: io.NopCloser(strings.NewReader(
				"event: response.created\ndata: {\"sequence_number\":0}\n\n" +
					"event: response.completed\ndata: {\"sequence_number\":1}\n\n",
			)),
			Request: r,
		}, nil
	})}
	handler := NewHandler(HandlerOptions{
		Provider: config.Provider{
			ID: "deepseek", Name: "DeepSeek", Model: "deepseek-v4-flash",
			WireAPI: config.WireAPIResponses, BaseURL: "https://api.deepseek.com",
		},
		APIKey:     "sk-secret",
		HTTPClient: client,
		Logger:     log.New(io.Discard, "", 0),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"deepseek-v4-flash","input":"hello","stream":true}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if !strings.Contains(rec.Body.String(), "response.completed") {
		t.Fatalf("SSE body = %s", rec.Body.String())
	}
}

func TestLogDoesNotIncludeBodyOrKey(t *testing.T) {
	var logs bytes.Buffer
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Status:     "401 Unauthorized",
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"error":"bad key"}`)),
			Request:    r,
		}, nil
	})}
	handler := NewHandler(HandlerOptions{
		Provider: config.Provider{
			ID: "deepseek", Name: "DeepSeek", Model: "deepseek-v4-flash",
			WireAPI: config.WireAPIResponses, BaseURL: "https://api.deepseek.com",
		},
		APIKey:     "sk-secret",
		HTTPClient: client,
		Logger:     log.New(&logs, "", 0),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"deepseek-v4-flash","input":"top secret prompt","stream":false}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if logs.Len() == 0 {
		t.Fatal("expected access log")
	}
	if strings.Contains(logs.String(), "sk-secret") || strings.Contains(logs.String(), "top secret prompt") {
		t.Fatalf("log leaks sensitive data: %s", logs.String())
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

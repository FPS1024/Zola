package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"zola/internal/config"
)

const maxRequestBody = 64 << 20

type HandlerOptions struct {
	Provider   config.Provider
	APIKey     string
	HTTPClient *http.Client
	Logger     *log.Logger
}

type Handler struct {
	provider config.Provider
	apiKey   string
	client   *http.Client
	logger   *log.Logger
}

func NewHandler(options HandlerOptions) http.Handler {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{}
	}
	logger := options.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	return &Handler{
		provider: options.Provider,
		apiKey:   options.APIKey,
		client:   client,
		logger:   logger,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
	model, stream, handled := h.route(rec, r)
	if !handled {
		http.NotFound(rec, r)
	}
	h.logger.Printf("%s %s provider=%s model=%s stream=%t status=%d duration=%s",
		r.Method,
		r.URL.Path,
		h.provider.ID,
		model,
		stream,
		rec.status,
		time.Since(started).Round(time.Millisecond),
	)
}

func (h *Handler) route(w http.ResponseWriter, r *http.Request) (string, bool, bool) {
	switch {
	case r.URL.Path == "/health":
		h.health(w, r)
		return "", false, true
	case r.URL.Path == "/v1/responses" || r.URL.Path == "/responses":
		if r.Method != http.MethodPost {
			writeStatus(w, http.StatusMethodNotAllowed)
			return "", false, true
		}
		if h.provider.WireAPI == config.WireAPIChat {
			model, stream, _ := h.forwardResponsesToChat(w, r)
			return model, stream, true
		}
		model, stream, _ := h.forward(w, r, "responses")
		return model, stream, true
	case r.URL.Path == "/v1/chat/completions" || r.URL.Path == "/chat/completions":
		if r.Method != http.MethodPost {
			writeStatus(w, http.StatusMethodNotAllowed)
			return "", false, true
		}
		model, stream, _ := h.forward(w, r, "chat/completions")
		return model, stream, true
	case r.URL.Path == "/v1/models" || r.URL.Path == "/models":
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeStatus(w, http.StatusMethodNotAllowed)
			return "", false, true
		}
		_, _, _ = h.forward(w, r, "models")
		return "", false, true
	default:
		return "", false, false
	}
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"provider":       h.provider.ID,
		"wire_api":       h.provider.WireAPI,
		"codex_model":    h.provider.CodexModelName(),
		"upstream_model": h.provider.Model,
		"context_window": h.provider.ContextWindow,
	})
}

func (h *Handler) forward(w http.ResponseWriter, r *http.Request, upstreamPath string) (string, bool, error) {
	var body []byte
	var err error
	if r.Body != nil {
		body, err = io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
		if err != nil {
			writeStatus(w, http.StatusBadRequest)
			return "", false, fmt.Errorf("read request body: %w", err)
		}
	}

	var payload map[string]any
	if len(body) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if err := decoder.Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "request body must be valid JSON"})
			return "", false, fmt.Errorf("decode request body: %w", err)
		}
	}
	model, _ := payload["model"].(string)
	stream, _ := payload["stream"].(bool)
	body, err = rewriteRequestModel(body, payload, h.provider)
	if err != nil {
		writeStatus(w, http.StatusBadGateway)
		return model, stream, fmt.Errorf("rewrite request model: %w", err)
	}

	endpoint := strings.TrimRight(h.provider.BaseURL, "/") + "/" + upstreamPath
	req, err := http.NewRequestWithContext(r.Context(), r.Method, endpoint, bytes.NewReader(body))
	if err != nil {
		writeStatus(w, http.StatusBadGateway)
		return model, stream, fmt.Errorf("build upstream request: %w", err)
	}
	copyRequestHeaders(req.Header, r.Header)
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}
	if len(body) > 0 {
		req.ContentLength = int64(len(body))
	}

	resp, err := h.client.Do(req)
	if err != nil {
		writeStatus(w, http.StatusBadGateway)
		return model, stream, fmt.Errorf("upstream request failed: %w", err)
	}
	defer resp.Body.Close()

	copyResponseHeaders(w.Header(), resp.Header)
	rec, ok := w.(*responseRecorder)
	if ok {
		rec.WriteHeader(resp.StatusCode)
	} else {
		w.WriteHeader(resp.StatusCode)
	}
	if resp.Body != nil {
		if err := copyAndFlush(w, resp.Body); err != nil {
			return model, stream, fmt.Errorf("stream upstream response: %w", err)
		}
	}
	return model, stream, nil
}

func (h *Handler) forwardResponsesToChat(w http.ResponseWriter, r *http.Request) (string, bool, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBody))
	if err != nil {
		writeStatus(w, http.StatusBadRequest)
		return "", false, fmt.Errorf("read request body: %w", err)
	}
	var payload map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "request body must be valid JSON"})
		return "", false, fmt.Errorf("decode request body: %w", err)
	}
	model, _ := payload["model"].(string)
	stream, _ := payload["stream"].(bool)
	if h.provider.IsDisguised() {
		payload["model"] = h.provider.Model
	}
	chatBody, err := responsesToChatBody(payload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return model, stream, fmt.Errorf("convert Responses request to Chat Completions: %w", err)
	}

	endpoint := strings.TrimRight(h.provider.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(chatBody))
	if err != nil {
		writeStatus(w, http.StatusBadGateway)
		return model, stream, fmt.Errorf("build upstream chat request: %w", err)
	}
	copyRequestHeaders(req.Header, r.Header)
	if h.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	req.ContentLength = int64(len(chatBody))

	resp, err := h.client.Do(req)
	if err != nil {
		writeStatus(w, http.StatusBadGateway)
		return model, stream, fmt.Errorf("upstream chat request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		copyResponseHeaders(w.Header(), resp.Header)
		rec, ok := w.(*responseRecorder)
		if ok {
			rec.WriteHeader(resp.StatusCode)
		} else {
			w.WriteHeader(resp.StatusCode)
		}
		_ = copyAndFlush(w, resp.Body)
		return model, stream, nil
	}

	if stream && strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		copyResponseHeaders(w.Header(), resp.Header)
		w.Header().Set("Content-Type", "text/event-stream")
		rec, ok := w.(*responseRecorder)
		if ok {
			rec.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		flush := func() {}
		if flusher, ok := w.(interface{ Flush() }); ok {
			flush = flusher.Flush
		}
		if err := chatSSEToResponsesSSE(resp.Body, w, flush, map[string]any{"model": model}); err != nil {
			return model, stream, fmt.Errorf("convert Chat SSE to Responses SSE: %w", err)
		}
		return model, stream, nil
	}

	var chatPayload map[string]any
	chatDecoder := json.NewDecoder(io.LimitReader(resp.Body, maxRequestBody))
	chatDecoder.UseNumber()
	if err := chatDecoder.Decode(&chatPayload); err != nil {
		writeStatus(w, http.StatusBadGateway)
		return model, stream, fmt.Errorf("decode chat response: %w", err)
	}
	response := chatBodyToResponses(chatPayload)
	if h.provider.IsDisguised() {
		response["model"] = h.provider.CodexModelName()
	}
	writeJSON(w, http.StatusOK, response)
	return model, stream, nil
}

func rewriteRequestModel(body []byte, payload map[string]any, provider config.Provider) ([]byte, error) {
	if len(body) == 0 || !provider.IsDisguised() {
		return body, nil
	}
	payload["model"] = provider.Model
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func writeStatus(w http.ResponseWriter, status int) {
	w.WriteHeader(status)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

var hopHeaders = map[string]struct{}{
	"connection":          {},
	"proxy-connection":    {},
	"keep-alive":          {},
	"proxy-authenticate":  {},
	"proxy-authorization": {},
	"te":                  {},
	"trailer":             {},
	"transfer-encoding":   {},
	"upgrade":             {},
}

func copyRequestHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		lower := strings.ToLower(key)
		if _, skip := hopHeaders[lower]; skip {
			continue
		}
		if lower == "authorization" || lower == "host" || lower == "content-length" || lower == "accept-encoding" {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func copyResponseHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		lower := strings.ToLower(key)
		if _, skip := hopHeaders[lower]; skip {
			continue
		}
		if lower == "content-length" || lower == "content-encoding" {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.wrote {
		return
	}
	r.status = status
	r.wrote = true
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	return r.ResponseWriter.Write(p)
}

func (r *responseRecorder) Flush() {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func copyAndFlush(dst io.Writer, src io.Reader) error {
	buffer := make([]byte, 32*1024)
	flusher, canFlush := dst.(interface{ Flush() })
	for {
		n, readErr := src.Read(buffer)
		if n > 0 {
			if _, writeErr := dst.Write(buffer[:n]); writeErr != nil {
				return writeErr
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

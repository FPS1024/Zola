package proxy

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"zola/internal/config"
)

func TestResponsesRequestToChatRequest(t *testing.T) {
	var responses map[string]any
	if err := json.Unmarshal([]byte(`{
		"model":"deepseek-chat",
		"stream":false,
		"instructions":"Answer concisely.",
		"max_output_tokens":64,
		"input":[
			{"type":"message","role":"user","content":[{"type":"input_text","text":"What is the weather?"}]},
			{"type":"function_call","call_id":"call_1","name":"get_weather","arguments":"{\"city\":\"shanghai\"}"},
			{"type":"function_call_output","call_id":"call_1","output":"sunny"}
		],
		"tools":[{"type":"function","name":"get_weather","description":"weather","parameters":{"type":"object"}}],
		"tool_choice":"auto"
	}`), &responses); err != nil {
		t.Fatalf("unmarshal responses request: %v", err)
	}

	body, err := responsesToChatBody(responses)
	if err != nil {
		t.Fatalf("responsesToChatBody: %v", err)
	}
	var chat map[string]any
	if err := json.Unmarshal(body, &chat); err != nil {
		t.Fatalf("unmarshal chat body: %v", err)
	}
	if chat["input"] != nil || chat["instructions"] != nil {
		t.Fatalf("responses-only fields were forwarded: %v", chat)
	}
	if chat["max_tokens"] == nil {
		t.Fatalf("max_tokens missing: %v", chat)
	}
	messages, ok := chat["messages"].([]any)
	if !ok || len(messages) != 4 {
		t.Fatalf("messages = %#v, want system/user/assistant/tool", messages)
	}
	system := messages[0].(map[string]any)
	if system["role"] != "system" || system["content"] != "Answer concisely." {
		t.Fatalf("system message = %#v", system)
	}
	assistant := messages[2].(map[string]any)
	calls, ok := assistant["tool_calls"].([]any)
	if !ok || len(calls) != 1 {
		t.Fatalf("assistant tool_calls missing: %#v", assistant)
	}
	call := calls[0].(map[string]any)
	if call["id"] != "call_1" {
		t.Fatalf("tool call id = %#v", call["id"])
	}
	toolMessage := messages[3].(map[string]any)
	if toolMessage["role"] != "tool" || toolMessage["content"] != "sunny" {
		t.Fatalf("tool message = %#v", toolMessage)
	}
	chatTools, ok := chat["tools"].([]any)
	if !ok || len(chatTools) != 1 {
		t.Fatalf("chat tools missing: %#v", chat["tools"])
	}
	chatTool := chatTools[0].(map[string]any)
	if _, ok := chatTool["function"].(map[string]any); !ok {
		t.Fatalf("chat tool wrapper missing: %#v", chatTool)
	}
}

func TestChatResponseToResponsesBody(t *testing.T) {
	var chat map[string]any
	if err := json.Unmarshal([]byte(`{
		"id":"chatcmpl-1",
		"object":"chat.completion",
		"model":"deepseek-chat",
		"created":1700000000,
		"choices":[{
			"index":0,
			"finish_reason":"tool_calls",
			"message":{
				"role":"assistant",
				"content":"Let me check.",
				"tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"shanghai\"}"}}]
			}
		}],
		"usage":{"prompt_tokens":10,"completion_tokens":5,"total_tokens":15}
	}`), &chat); err != nil {
		t.Fatalf("unmarshal chat response: %v", err)
	}
	response := chatBodyToResponses(chat)
	if response["object"] != "response" || !strings.HasPrefix(response["id"].(string), "resp_") {
		t.Fatalf("response envelope = %#v", response)
	}
	output, ok := response["output"].([]any)
	if !ok || len(output) != 2 {
		t.Fatalf("output = %#v, want message + tool call", output)
	}
	message := output[0].(map[string]any)
	if message["type"] != "message" {
		t.Fatalf("first output = %#v", message)
	}
	call := output[1].(map[string]any)
	if call["type"] != "function_call" || call["name"] != "get_weather" {
		t.Fatalf("function call output = %#v", call)
	}
	usage := response["usage"].(map[string]any)
	if usage["input_tokens"] == nil || usage["output_tokens"] == nil {
		t.Fatalf("usage conversion failed: %#v", usage)
	}
}

func TestChatSSEToResponsesSSE(t *testing.T) {
	chat := "data: {\"id\":\"chatcmpl-1\",\"model\":\"deepseek-chat\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"Hello\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"get_weather\",\"arguments\":\"{\\\"city\\\":\\\"shanghai\\\"}\"}}]},\"finish_reason\":\"tool_calls\"}]}\n\n" +
		"data: [DONE]\n\n"
	var out strings.Builder
	if err := chatSSEToResponsesSSE(strings.NewReader(chat), &out, nil, map[string]any{"model": "deepseek-chat"}); err != nil {
		t.Fatalf("chatSSEToResponsesSSE: %v", err)
	}
	text := out.String()
	for _, expected := range []string{
		"event: response.created",
		"event: response.output_text.delta",
		`"delta":"Hello"`,
		`"delta":" world"`,
		"event: response.function_call_arguments.delta",
		`"name":"get_weather"`,
		"event: response.completed",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("SSE output missing %q:\n%s", expected, text)
		}
	}
}

func TestProxyConvertsResponsesToChatUpstream(t *testing.T) {
	var upstreamBody map[string]any
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("upstream path = %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &upstreamBody)
		return jsonResponse(r, `{"id":"chatcmpl-1","object":"chat.completion","model":"deepseek-chat","created":1700000000,"choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"Hello from chat"}}],"usage":{"prompt_tokens":4,"completion_tokens":3,"total_tokens":7}}`), nil
	})}
	handler := NewHandler(HandlerOptions{
		Provider: config.Provider{
			ID: "old-deepseek", Name: "Old DeepSeek", Model: "deepseek-chat",
			WireAPI: config.WireAPIChat, BaseURL: "https://api.deepseek.com",
		},
		APIKey:     "sk-secret",
		HTTPClient: client,
		Logger:     log.New(io.Discard, "", 0),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"deepseek-chat","input":"Hello","stream":false}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if upstreamBody == nil || upstreamBody["messages"] == nil {
		t.Fatalf("upstream request was not converted: %#v", upstreamBody)
	}
	if !strings.Contains(rec.Body.String(), "Hello from chat") {
		t.Fatalf("responses body = %s", rec.Body.String())
	}
}

func TestProxyConvertsResponsesSSEToChatSSE(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		chat := "data: {\"id\":\"chatcmpl-1\",\"model\":\"deepseek-chat\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"ok\"},\"finish_reason\":null}]}\n\n" +
			"data: [DONE]\n\n"
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(chat)),
			Request:    r,
		}, nil
	})}
	handler := NewHandler(HandlerOptions{
		Provider: config.Provider{
			ID: "old-deepseek", Name: "Old DeepSeek", Model: "deepseek-chat",
			WireAPI: config.WireAPIChat, BaseURL: "https://api.deepseek.com",
		},
		APIKey:     "sk-secret",
		HTTPClient: client,
		Logger:     log.New(io.Discard, "", 0),
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses",
		strings.NewReader(`{"model":"deepseek-chat","input":"Hello","stream":true}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "response.completed") {
		t.Fatalf("responses SSE = %s", rec.Body.String())
	}
}

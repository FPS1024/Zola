package proxy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// responsesToChatBody converts a Responses API request to a Chat Completions
// request. It intentionally keeps the unknown fields it can safely forward.
func responsesToChatBody(payload map[string]any) ([]byte, error) {
	messages, err := inputToChatMessages(payload["input"])
	if err != nil {
		return nil, err
	}
	if instructions, ok := payload["instructions"].(string); ok && instructions != "" {
		messages = append([]any{map[string]any{
			"role":    "system",
			"content": instructions,
		}}, messages...)
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("responses input did not contain any chat messages")
	}

	chat := map[string]any{}
	for key, value := range payload {
		switch key {
		case "input", "instructions":
			continue
		case "store", "truncation", "previous_response_id", "metadata", "text":
			continue
		case "include":
			if includesUsage(value) {
				chat["stream_options"] = map[string]any{"include_usage": true}
			}
			continue
		case "max_output_tokens":
			chat["max_tokens"] = value
		case "reasoning":
			// Old chat APIs have no shared reasoning schema. The model request
			// should still work without it; Codex can request effort separately.
			continue
		case "tools":
			chat["tools"] = convertTools(value)
		case "tool_choice":
			chat["tool_choice"] = convertToolChoice(value)
		case "parallel_tool_calls":
			chat["parallel_tool_calls"] = value
		default:
			chat[key] = value
		}
	}
	chat["messages"] = messages
	return json.Marshal(chat)
}

func includesUsage(raw any) bool {
	items, ok := raw.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if item == "usage" {
			return true
		}
	}
	return false
}

func inputToChatMessages(input any) ([]any, error) {
	messages := []any{}
	if text, ok := input.(string); ok {
		return append(messages, map[string]any{
			"role":    "user",
			"content": text,
		}), nil
	}

	items, ok := input.([]any)
	if !ok {
		return nil, fmt.Errorf("unsupported responses input type %T", input)
	}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		switch item["type"] {
		case "message":
			role, _ := item["role"].(string)
			if role == "" {
				role = "user"
			} else if role == "developer" {
				role = "system"
			}
			messages = append(messages, map[string]any{
				"role":    role,
				"content": chatContent(item["content"]),
			})
		case "function_call":
			messages = appendAssistantToolCall(messages, item)
		case "function_call_output":
			output := jsonStringValue(item["output"])
			messages = append(messages, map[string]any{
				"role":         "tool",
				"tool_call_id": item["call_id"],
				"content":      output,
			})
		case "reasoning":
			// Reasoning summaries cannot be represented in Chat Completions
			// input and do not affect tool calls.
			continue
		default:
			continue
		}
	}
	return messages, nil
}

func appendAssistantToolCall(messages []any, item map[string]any) []any {
	var assistant map[string]any
	if len(messages) > 0 {
		if candidate, ok := messages[len(messages)-1].(map[string]any); ok && candidate["role"] == "assistant" {
			assistant = candidate
		}
	}
	if assistant == nil {
		assistant = map[string]any{
			"role":    "assistant",
			"content": nil,
		}
		messages = append(messages, assistant)
	}
	toolCalls, _ := assistant["tool_calls"].([]any)
	toolCalls = append(toolCalls, map[string]any{
		"id":   item["call_id"],
		"type": "function",
		"function": map[string]any{
			"name":      item["name"],
			"arguments": item["arguments"],
		},
	})
	assistant["tool_calls"] = toolCalls
	return messages
}

func chatContent(content any) any {
	switch value := content.(type) {
	case nil:
		return nil
	case string:
		return value
	case []any:
		var parts []string
		for _, raw := range value {
			part, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			switch part["type"] {
			case "input_text", "output_text", "text":
				parts = append(parts, jsonStringValue(part["text"]))
			}
		}
		return strings.Join(parts, "\n")
	default:
		return jsonStringValue(content)
	}
}

func jsonStringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		b, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

func convertTools(raw any) any {
	tools, ok := raw.([]any)
	if !ok {
		return raw
	}
	converted := []any{}
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if !ok {
			continue
		}
		if tool["type"] != "function" {
			continue
		}
		function := map[string]any{}
		for _, key := range []string{"name", "description", "parameters"} {
			if value, exists := tool[key]; exists {
				function[key] = value
			}
		}
		converted = append(converted, map[string]any{
			"type":     "function",
			"function": function,
		})
	}
	return converted
}

func convertToolChoice(raw any) any {
	choice, ok := raw.(map[string]any)
	if !ok || choice["type"] != "function" {
		return raw
	}
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name": choice["name"],
		},
	}
}

func chatBodyToResponses(payload map[string]any) map[string]any {
	id := stringValue(payload["id"])
	if id == "" {
		id = fmt.Sprintf("chat-%d", time.Now().Unix())
	}
	model := stringValue(payload["model"])
	created := createdAt(payload["created"])
	responseID := "resp_" + strings.TrimPrefix(id, "resp_")
	output := []any{}
	status := "completed"

	if choices, ok := payload["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if message, ok := choice["message"].(map[string]any); ok {
				if content := stringValue(chatContent(message["content"])); content != "" {
					output = append(output, messageOutputItem(responseID, content))
				}
				if calls, ok := message["tool_calls"].([]any); ok {
					for _, raw := range calls {
						if call, ok := raw.(map[string]any); ok {
							output = append(output, toolCallOutputItem(call))
						}
					}
				}
			}
			switch choice["finish_reason"] {
			case "length":
				status = "incomplete"
			}
		}
	}

	response := map[string]any{
		"id":         responseID,
		"object":     "response",
		"created_at": created,
		"status":     status,
		"model":      model,
		"output":     output,
	}
	if status == "incomplete" {
		response["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	if usage := usageToResponses(payload["usage"]); usage != nil {
		response["usage"] = usage
	}
	return response
}

func messageOutputItem(responseID, content string) map[string]any {
	return map[string]any{
		"id":     "msg_" + strings.TrimPrefix(responseID, "resp_"),
		"type":   "message",
		"status": "completed",
		"role":   "assistant",
		"content": []any{
			map[string]any{
				"type":        "output_text",
				"text":        content,
				"annotations": []any{},
			},
		},
	}
}

func toolCallOutputItem(call map[string]any) map[string]any {
	function, _ := call["function"].(map[string]any)
	callID := stringValue(call["id"])
	if callID == "" {
		callID = stringValue(call["call_id"])
	}
	return map[string]any{
		"id":        callID,
		"type":      "function_call",
		"call_id":   callID,
		"name":      stringValue(function["name"]),
		"arguments": jsonStringValue(function["arguments"]),
		"status":    "completed",
	}
}

func usageToResponses(raw any) map[string]any {
	usage, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	converted := map[string]any{}
	for from, to := range map[string]string{
		"prompt_tokens":     "input_tokens",
		"completion_tokens": "output_tokens",
		"total_tokens":      "total_tokens",
	} {
		if value, exists := usage[from]; exists {
			converted[to] = value
		}
	}
	if len(converted) == 0 {
		return nil
	}
	return converted
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func createdAt(raw any) int64 {
	switch value := raw.(type) {
	case json.Number:
		created, _ := value.Int64()
		return created
	case float64:
		return int64(value)
	case int64:
		return value
	default:
		return time.Now().Unix()
	}
}

type sseEmitter struct {
	writer io.Writer
	flush  func()
	seq    int
}

func (e *sseEmitter) emit(event string, data map[string]any) error {
	data["type"] = event
	data["sequence_number"] = e.seq
	encoded, err := json.Marshal(data)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(e.writer, "event: %s\ndata: %s\n\n", event, encoded); err != nil {
		return err
	}
	e.seq++
	if e.flush != nil {
		e.flush()
	}
	return nil
}

func chatSSEToResponsesSSE(body io.Reader, writer io.Writer, flush func(), base map[string]any) error {
	emitter := &sseEmitter{writer: writer, flush: flush}
	responseID := "resp_zola_" + fmt.Sprint(time.Now().UnixNano())
	baseResponse := map[string]any{
		"id":         responseID,
		"object":     "response",
		"created_at": time.Now().Unix(),
		"status":     "in_progress",
		"model":      stringValue(base["model"]),
		"output":     []any{},
	}
	if err := emitter.emit("response.created", map[string]any{"response": baseResponse}); err != nil {
		return err
	}

	stream := &chatStreamState{
		emitter:    emitter,
		responseID: responseID,
		model:      stringValue(base["model"]),
	}
	readerBuf := bufio.NewReader(body)
	var dataLines []string
	for {
		line, err := readerBuf.ReadString('\n')
		trimmed := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(trimmed, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(trimmed, "data:")))
		} else if trimmed == "" && len(dataLines) > 0 {
			data := strings.Join(dataLines, "\n")
			dataLines = nil
			if data == "[DONE]" {
				return stream.finish()
			}
			if err := stream.handleChunk(data); err != nil {
				return err
			}
		}
		if err == io.EOF {
			if len(dataLines) > 0 {
				if err := stream.handleChunk(strings.Join(dataLines, "\n")); err != nil {
					return err
				}
			}
			return stream.finish()
		}
		if err != nil {
			return err
		}
	}
}

type chatStreamState struct {
	emitter            *sseEmitter
	responseID         string
	model              string
	messageID          string
	text               strings.Builder
	textOpen           bool
	messageOutputIndex int
	tools              map[int]*streamToolCall
	toolOrder          []int
	nextOutputIndex    int
	finishReason       string
	usage              map[string]any
}

type streamToolCall struct {
	id          string
	name        string
	args        strings.Builder
	outputIndex int
}

func (s *chatStreamState) handleChunk(data string) error {
	var chunk map[string]any
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil
	}
	if usage, ok := chunk["usage"].(map[string]any); ok {
		s.usage = usage
	}
	choices, ok := chunk["choices"].([]any)
	if !ok || len(choices) == 0 {
		return nil
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return nil
	}
	if reason, ok := choice["finish_reason"].(string); ok && reason != "" {
		s.finishReason = reason
	}
	delta, ok := choice["delta"].(map[string]any)
	if !ok {
		return nil
	}
	if content, ok := delta["content"].(string); ok && content != "" {
		if err := s.startText(); err != nil {
			return err
		}
		s.text.WriteString(content)
		if err := s.emitter.emit("response.output_text.delta", map[string]any{
			"output_index":  s.messageOutputIndex,
			"content_index": 0,
			"delta":         content,
		}); err != nil {
			return err
		}
	}
	if calls, ok := delta["tool_calls"].([]any); ok {
		for _, raw := range calls {
			call, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			index := intIndex(call["index"])
			tool := &streamToolCall{}
			if function, ok := call["function"].(map[string]any); ok {
				if name := stringValue(function["name"]); name != "" {
					tool.name = name
				}
			}
			tool.id = stringValue(call["id"])
			if tool.id == "" {
				tool.id = fmt.Sprintf("call_%d", index)
			}
			if err := s.ensureTool(index, tool); err != nil {
				return err
			}
			existing := s.tools[index]
			if function, ok := call["function"].(map[string]any); ok {
				if arguments := jsonStringValue(function["arguments"]); arguments != "" {
					existing.args.WriteString(arguments)
					if err := s.emitter.emit("response.function_call_arguments.delta", map[string]any{
						"output_index": existing.outputIndex,
						"item_id":      existing.id,
						"delta":        arguments,
					}); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (s *chatStreamState) startText() error {
	if s.textOpen {
		return nil
	}
	s.textOpen = true
	s.messageID = "msg_" + s.responseID
	s.messageOutputIndex = s.nextOutputIndex
	s.nextOutputIndex++
	if err := s.emitter.emit("response.output_item.added", map[string]any{
		"output_index": s.messageOutputIndex,
		"item": map[string]any{
			"id":      s.messageID,
			"type":    "message",
			"status":  "in_progress",
			"role":    "assistant",
			"content": []any{},
		},
	}); err != nil {
		return err
	}
	return s.emitter.emit("response.content_part.added", map[string]any{
		"output_index":  s.messageOutputIndex,
		"content_index": 0,
		"part": map[string]any{
			"type":        "output_text",
			"text":        "",
			"annotations": []any{},
		},
	})
}

func (s *chatStreamState) ensureTool(index int, tool *streamToolCall) error {
	if s.tools == nil {
		s.tools = map[int]*streamToolCall{}
	}
	if existing, ok := s.tools[index]; ok {
		if tool.name != "" && existing.name == "" {
			existing.name = tool.name
		}
		if existing.id == "" {
			existing.id = tool.id
		}
		return nil
	}
	tool.outputIndex = s.nextOutputIndex
	s.nextOutputIndex++
	s.tools[index] = tool
	s.toolOrder = append(s.toolOrder, index)
	return s.emitter.emit("response.output_item.added", map[string]any{
		"output_index": tool.outputIndex,
		"item": map[string]any{
			"id":        tool.id,
			"type":      "function_call",
			"call_id":   tool.id,
			"name":      tool.name,
			"arguments": tool.args.String(),
			"status":    "in_progress",
		},
	})
}

func (s *chatStreamState) finish() error {
	output := []any{}
	if s.textOpen {
		text := s.text.String()
		if err := s.emitter.emit("response.output_text.done", map[string]any{
			"output_index":  s.messageOutputIndex,
			"content_index": 0,
			"text":          text,
		}); err != nil {
			return err
		}
		if err := s.emitter.emit("response.content_part.done", map[string]any{
			"output_index":  s.messageOutputIndex,
			"content_index": 0,
			"part": map[string]any{
				"type":        "output_text",
				"text":        text,
				"annotations": []any{},
			},
		}); err != nil {
			return err
		}
		item := map[string]any{
			"id":      s.messageID,
			"type":    "message",
			"status":  "completed",
			"role":    "assistant",
			"content": []any{map[string]any{"type": "output_text", "text": text, "annotations": []any{}}},
		}
		output = append(output, item)
		if err := s.emitter.emit("response.output_item.done", map[string]any{
			"output_index": s.messageOutputIndex,
			"item":         item,
		}); err != nil {
			return err
		}
	}
	for _, toolIndex := range s.toolOrder {
		tool := s.tools[toolIndex]
		arguments := tool.args.String()
		item := map[string]any{
			"id":        tool.id,
			"type":      "function_call",
			"call_id":   tool.id,
			"name":      tool.name,
			"arguments": arguments,
			"status":    "completed",
		}
		if err := s.emitter.emit("response.function_call_arguments.done", map[string]any{
			"output_index": tool.outputIndex,
			"item_id":      tool.id,
			"arguments":    arguments,
		}); err != nil {
			return err
		}
		output = append(output, item)
		if err := s.emitter.emit("response.output_item.done", map[string]any{
			"output_index": tool.outputIndex,
			"item":         item,
		}); err != nil {
			return err
		}
	}
	status := "completed"
	if s.finishReason == "length" {
		status = "incomplete"
	}
	response := map[string]any{
		"id":         s.responseID,
		"object":     "response",
		"created_at": time.Now().Unix(),
		"status":     status,
		"model":      s.model,
		"output":     output,
	}
	if status == "incomplete" {
		response["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	}
	if usage := usageToResponses(s.usage); usage != nil {
		response["usage"] = usage
	}
	return s.emitter.emit("response.completed", map[string]any{"response": response})
}

func intIndex(value any) int {
	switch number := value.(type) {
	case json.Number:
		index, _ := number.Int64()
		return int(index)
	case float64:
		return int(number)
	case int64:
		return int(number)
	default:
		return 0
	}
}

package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/tools"
	toolshared "github.com/sipeed/picoclaw/pkg/tools/shared"
)

type mockTool struct {
	name   string
	desc   string
	params map[string]any
	result *toolshared.ToolResult
}

func (t *mockTool) Name() string               { return t.name }
func (t *mockTool) Description() string        { return t.desc }
func (t *mockTool) Parameters() map[string]any { return t.params }
func (t *mockTool) Execute(_ context.Context, _ map[string]any) *toolshared.ToolResult {
	return t.result
}

func TestNewServer(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.Register(&mockTool{
		name: "echo",
		desc: "echoes input back",
		params: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}},
		},
		result: &toolshared.ToolResult{ForLLM: "hello world"},
	})

	cfg := config.MCPExposeConfig{Enabled: true, Name: "test-server"}
	server := NewServer(cfg, registry)

	assert.NotNil(t, server)
	assert.NotNil(t, server.Handler())
}

func TestMCPServerInitialize(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.Register(&mockTool{
		name:   "greet",
		desc:   "greets the user",
		params: map[string]any{"type": "object", "properties": map[string]any{}},
		result: &toolshared.ToolResult{ForLLM: "hello!"},
	})

	cfg := config.MCPExposeConfig{Enabled: true, Name: "picoclaw-test"}
	server := NewServer(cfg, registry)

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	req, _ := http.NewRequest("POST", ts.URL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]any
	assert.NoError(t, json.Unmarshal(respBody, &result))

	assert.Equal(t, "2.0", result["jsonrpc"])
	assert.Equal(t, float64(1), result["id"])
	assert.NotNil(t, result["result"])

	res := result["result"].(map[string]any)
	serverInfo := res["serverInfo"].(map[string]any)
	assert.Equal(t, "picoclaw-test", serverInfo["name"])
}

func TestMCPServerToolCall(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.Register(&mockTool{
		name: "echo",
		desc: "echoes input back",
		params: map[string]any{
			"type":       "object",
			"properties": map[string]any{"text": map[string]any{"type": "string"}},
		},
		result: &toolshared.ToolResult{ForLLM: "echoed: hello"},
	})

	cfg := config.MCPExposeConfig{Enabled: true}
	server := NewServer(cfg, registry)

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	initBody := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	initReq, _ := http.NewRequest("POST", ts.URL, strings.NewReader(initBody))
	initReq.Header.Set("Content-Type", "application/json")
	initReq.Header.Set("Accept", "application/json, text/event-stream")
	initResp, err := http.DefaultClient.Do(initReq)
	assert.NoError(t, err)
	initResp.Body.Close()

	notifBody := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	notifReq, _ := http.NewRequest("POST", ts.URL, strings.NewReader(notifBody))
	notifReq.Header.Set("Content-Type", "application/json")
	notifReq.Header.Set("Accept", "application/json, text/event-stream")
	notifResp, err := http.DefaultClient.Do(notifReq)
	assert.NoError(t, err)
	notifResp.Body.Close()

	callBody := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hello"}}}`
	callReq, _ := http.NewRequest("POST", ts.URL, strings.NewReader(callBody))
	callReq.Header.Set("Content-Type", "application/json")
	callReq.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(callReq)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]any
	assert.NoError(t, json.Unmarshal(respBody, &result))

	assert.Equal(t, float64(2), result["id"])
	res := result["result"].(map[string]any)
	content := res["content"].([]any)
	assert.Len(t, content, 1)
	textContent := content[0].(map[string]any)
	assert.Equal(t, "text", textContent["type"])
	assert.Equal(t, "echoed: hello", textContent["text"])
}

func TestMCPServerAuth_Unauthorized(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.Register(&mockTool{
		name:   "test",
		desc:   "test tool",
		params: map[string]any{"type": "object", "properties": map[string]any{}},
		result: &toolshared.ToolResult{ForLLM: "ok"},
	})

	cfg := config.MCPExposeConfig{Enabled: true, AuthToken: "secret-token-123"}
	server := NewServer(cfg, registry)

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	// No auth header
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	resp, err := http.Post(ts.URL, "application/json", strings.NewReader(body))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()

	// Wrong token
	req, _ := http.NewRequest("POST", ts.URL, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	resp.Body.Close()
}

func TestMCPServerAuth_Authorized(t *testing.T) {
	registry := tools.NewToolRegistry()
	registry.Register(&mockTool{
		name:   "test",
		desc:   "test tool",
		params: map[string]any{"type": "object", "properties": map[string]any{}},
		result: &toolshared.ToolResult{ForLLM: "ok"},
	})

	cfg := config.MCPExposeConfig{Enabled: true, AuthToken: "secret-token-123"}
	server := NewServer(cfg, registry)

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`
	req, _ := http.NewRequest("POST", ts.URL, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer secret-token-123")
	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestConvertResult(t *testing.T) {
	tests := []struct {
		name    string
		input   *tools.ToolResult
		wantErr bool
	}{
		{
			name:    "nil result",
			input:   nil,
			wantErr: true,
		},
		{
			name:    "success result",
			input:   &tools.ToolResult{ForLLM: "file contents here"},
			wantErr: false,
		},
		{
			name:    "error result",
			input:   &tools.ToolResult{ForLLM: "permission denied", IsError: true},
			wantErr: true,
		},
		{
			name:    "empty result",
			input:   &tools.ToolResult{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertResult(tt.input)
			assert.Equal(t, tt.wantErr, result.IsError)
			assert.Len(t, result.Content, 1)
		})
	}
}

func TestConvertResult_Truncation(t *testing.T) {
	largeContent := strings.Repeat("x", maxResultBytes+100)
	result := convertResult(&tools.ToolResult{ForLLM: largeContent})

	text := result.Content[0].(*mcp.TextContent).Text
	assert.True(t, len(text) <= maxResultBytes+20)
	assert.True(t, strings.HasSuffix(text, "...(truncated)"))
}

func TestParseArguments(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want map[string]any
	}{
		{"nil", nil, map[string]any{}},
		{"empty", json.RawMessage(""), map[string]any{}},
		{"valid", json.RawMessage(`{"name":"test","count":42}`), map[string]any{"name": "test", "count": float64(42)}},
		{"invalid json", json.RawMessage(`{broken`), map[string]any{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseArguments(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}

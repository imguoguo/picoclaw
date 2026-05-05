package mcp

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/tools"
)

const maxResultBytes = 256 * 1024

// Server exposes PicoClaw's tools as an MCP server accessible via Streamable HTTP.
type Server struct {
	mcpServer *mcp.Server
	handler   http.Handler
}

// NewServer creates an MCP server that exposes the given tool registry.
func NewServer(cfg config.MCPExposeConfig, registry *tools.ToolRegistry) *Server {
	name := cfg.Name
	if name == "" {
		name = "picoclaw"
	}

	mcpServer := mcp.NewServer(
		&mcp.Implementation{Name: name, Version: "0.1.0"},
		&mcp.ServerOptions{},
	)

	registerTools(mcpServer, registry)

	var handler http.Handler = mcp.NewStreamableHTTPHandler(
		func(_ *http.Request) *mcp.Server { return mcpServer },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
	)

	if cfg.AuthToken != "" {
		handler = authMiddleware(cfg.AuthToken, handler)
	}

	return &Server{mcpServer: mcpServer, handler: handler}
}

// Handler returns the HTTP handler for the MCP server endpoint.
func (s *Server) Handler() http.Handler {
	return s.handler
}

// authMiddleware enforces Bearer token authentication.
func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		provided := strings.TrimPrefix(auth, "Bearer ")
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// registerTools registers all tools from the PicoClaw ToolRegistry into the MCP server.
func registerTools(mcpServer *mcp.Server, registry *tools.ToolRegistry) {
	allTools := registry.GetAll()
	for _, t := range allTools {
		addToolToServer(mcpServer, t, registry)
	}
	logger.InfoCF("mcp-server", "Registered tools for MCP server",
		map[string]any{"count": len(allTools)})
}

// addToolToServer adapts a PicoClaw Tool to the MCP server's tool format.
func addToolToServer(mcpServer *mcp.Server, t tools.Tool, registry *tools.ToolRegistry) {
	params := t.Parameters()
	if params == nil {
		params = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	if _, ok := params["type"]; !ok {
		params["type"] = "object"
	}

	mcpTool := &mcp.Tool{
		Name:        t.Name(),
		Description: t.Description(),
		InputSchema: params,
	}

	toolName := t.Name()
	handler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := parseArguments(req.Params.Arguments)
		result := registry.Execute(ctx, toolName, args)
		return convertResult(result), nil
	}

	mcpServer.AddTool(mcpTool, handler)
}

// parseArguments converts raw JSON arguments to map[string]any.
func parseArguments(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return map[string]any{}
	}
	return args
}

// convertResult converts a PicoClaw ToolResult to an MCP CallToolResult.
func convertResult(result *tools.ToolResult) *mcp.CallToolResult {
	if result == nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "tool returned no result"}},
			IsError: true,
		}
	}

	output := result.ContentForLLM()
	if output == "" {
		output = "(empty)"
	}

	if len(output) > maxResultBytes {
		output = output[:maxResultBytes] + "\n...(truncated)"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: output}},
		IsError: result.IsError,
	}
}

# MCP Server Mode

PicoClaw can act as an MCP (Model Context Protocol) server, exposing its registered tools to external MCP clients such as Claude Code, VS Code, Cursor, and Claude Desktop.

## Configuration

Add the following to your `config.json`:

```json
{
  "tools": {
    "mcp": {
      "expose": {
        "enabled": true,
        "name": "picoclaw",
        "auth_token": "your-secret-token"
      }
    }
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Enable the MCP server endpoint |
| `name` | string | `"picoclaw"` | Server name advertised to clients |
| `auth_token` | string | `""` | Bearer token for authentication. Leave empty to disable auth (not recommended for network-exposed instances) |

Environment variables:
- `PICOCLAW_MCP_EXPOSE_ENABLED=true`
- `PICOCLAW_MCP_EXPOSE_NAME=picoclaw`
- `PICOCLAW_MCP_EXPOSE_AUTH_TOKEN=your-secret-token`

## Endpoint

The MCP server is available at:

```
http://<gateway-host>:<gateway-port>/mcp/
```

Default: `http://localhost:18790/mcp/`

It uses the MCP Streamable HTTP transport (JSON-RPC 2.0 over HTTP POST) in stateless mode.

## Authentication

When `auth_token` is configured, clients must include a Bearer token in every request:

```
Authorization: Bearer your-secret-token
```

Requests without a valid token receive HTTP 401 Unauthorized.

## Client Configuration

### Claude Code

Add to `~/.claude/settings.json`:

```json
{
  "mcpServers": {
    "picoclaw": {
      "type": "streamableHttp",
      "url": "http://localhost:18790/mcp/",
      "headers": {
        "Authorization": "Bearer your-secret-token"
      }
    }
  }
}
```

### VS Code / Cursor

Add to `.vscode/mcp.json` in your workspace:

```json
{
  "servers": {
    "picoclaw": {
      "type": "streamableHttp",
      "url": "http://localhost:18790/mcp/",
      "headers": {
        "Authorization": "Bearer your-secret-token"
      }
    }
  }
}
```

### Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "picoclaw": {
      "url": "http://localhost:18790/mcp/",
      "transport": "streamable-http",
      "headers": {
        "Authorization": "Bearer your-secret-token"
      }
    }
  }
}
```

### ChatGPT (with MCP plugin)

Use the Streamable HTTP endpoint URL with the Bearer token configured in the plugin settings.

## Verification

Test with curl:

```bash
# Initialize (with auth)
curl -s -X POST http://localhost:18790/mcp/ \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer your-secret-token" \
  -d '{
    "jsonrpc": "2.0",
    "id": 1,
    "method": "initialize",
    "params": {
      "protocolVersion": "2025-11-25",
      "capabilities": {},
      "clientInfo": {"name": "curl", "version": "1.0"}
    }
  }'

# List tools (after initialize)
curl -s -X POST http://localhost:18790/mcp/ \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer your-secret-token" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list",
    "params": {}
  }'
```

## Exposed Tools

All tools registered in the default agent's tool registry are exposed, including:

- **Filesystem**: `read_file`, `write_file`, `edit_file`, `append_file`, `list_dir`
- **Shell**: `exec` (with deny-pattern safety still enforced)
- **Web**: `web_search`, `web_fetch`
- **Hardware**: `i2c`, `spi`, `serial` (Linux only)
- **Scheduling**: `cron`
- **Messaging**: `message`, `send_file`

Tool access respects all existing safety constraints (workspace sandbox, deny patterns, etc.).

## Security Considerations

- **Always set `auth_token`** when the gateway is accessible beyond localhost.
- The MCP server shares the same tool registry as the agent loop — all existing safety mechanisms (deny patterns, workspace restriction, file path sandboxing) apply to MCP tool calls.
- Tool results are truncated at 256 KB to prevent memory issues on clients.
- Consider firewall rules to restrict access to the gateway port.

## Limitations

- The MCP server exposes tools only (no MCP Resources or Prompts yet).
- Tool registration is static at gateway startup. Adding/removing tools requires a gateway restart.
- Only the default agent's tools are exposed. Multi-agent configurations expose the default agent only.

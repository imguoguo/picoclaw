# MCP 服务器模式

PicoClaw 可以作为 MCP (Model Context Protocol) 服务器运行，将注册的工具暴露给外部 MCP 客户端，如 Claude Code、VS Code、Cursor 和 Claude Desktop。

## 配置

在 `config.json` 中添加：

```json
{
  "tools": {
    "mcp": {
      "expose": {
        "enabled": true,
        "name": "picoclaw",
        "auth_token": "你的密钥"
      }
    }
  }
}
```

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `enabled` | bool | `false` | 启用 MCP 服务器端点 |
| `name` | string | `"picoclaw"` | 向客户端展示的服务器名称 |
| `auth_token` | string | `""` | Bearer Token 认证密钥。留空不启用认证（不推荐在网络暴露时使用） |

环境变量：
- `PICOCLAW_MCP_EXPOSE_ENABLED=true`
- `PICOCLAW_MCP_EXPOSE_NAME=picoclaw`
- `PICOCLAW_MCP_EXPOSE_AUTH_TOKEN=你的密钥`

## 端点地址

MCP 服务器地址：

```
http://<网关地址>:<网关端口>/mcp/
```

默认：`http://localhost:18790/mcp/`

使用 MCP Streamable HTTP 传输协议（JSON-RPC 2.0 over HTTP POST），无状态模式。

## 认证

配置了 `auth_token` 后，客户端每次请求必须携带 Bearer Token：

```
Authorization: Bearer 你的密钥
```

未携带有效 Token 的请求返回 HTTP 401。

## 客户端接入

### Claude Code

添加到 `~/.claude/settings.json`：

```json
{
  "mcpServers": {
    "picoclaw": {
      "type": "streamableHttp",
      "url": "http://localhost:18790/mcp/",
      "headers": {
        "Authorization": "Bearer 你的密钥"
      }
    }
  }
}
```

### VS Code / Cursor

添加到工作区的 `.vscode/mcp.json`：

```json
{
  "servers": {
    "picoclaw": {
      "type": "streamableHttp",
      "url": "http://localhost:18790/mcp/",
      "headers": {
        "Authorization": "Bearer 你的密钥"
      }
    }
  }
}
```

### Claude Desktop

添加到 `claude_desktop_config.json`：

```json
{
  "mcpServers": {
    "picoclaw": {
      "url": "http://localhost:18790/mcp/",
      "transport": "streamable-http",
      "headers": {
        "Authorization": "Bearer 你的密钥"
      }
    }
  }
}
```

## 验证

使用 curl 测试：

```bash
# 初始化（带认证）
curl -s -X POST http://localhost:18790/mcp/ \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer 你的密钥" \
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

# 列出工具
curl -s -X POST http://localhost:18790/mcp/ \
  -H "Content-Type: application/json" \
  -H "Accept: application/json" \
  -H "Authorization: Bearer 你的密钥" \
  -d '{
    "jsonrpc": "2.0",
    "id": 2,
    "method": "tools/list",
    "params": {}
  }'
```

## 暴露的工具

默认 agent 工具注册表中的所有工具都会被暴露，包括：

- **文件系统**：`read_file`、`write_file`、`edit_file`、`append_file`、`list_dir`
- **Shell**：`exec`（拒绝模式安全规则仍然生效）
- **网络**：`web_search`、`web_fetch`
- **硬件**：`i2c`、`spi`、`serial`（仅 Linux）
- **定时**：`cron`
- **消息**：`message`、`send_file`

工具访问遵守所有现有安全约束（工作区沙箱、拒绝模式等）。

## 安全建议

- 当 gateway 暴露到 localhost 以外时，**务必设置 `auth_token`**。
- MCP 服务器共享 agent loop 的工具注册表——所有安全机制（拒绝模式、工作区限制、文件路径沙箱）对 MCP 调用同样生效。
- 工具结果超过 256 KB 会被截断，防止客户端内存溢出。
- 建议通过防火墙规则限制 gateway 端口的访问范围。

## 限制

- 目前仅暴露工具（Tools），暂不支持 MCP Resources 和 Prompts。
- 工具注册在 gateway 启动时静态完成，增删工具需重启 gateway。
- 仅暴露默认 agent 的工具。多 agent 配置下只暴露默认 agent。

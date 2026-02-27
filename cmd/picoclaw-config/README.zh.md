# PicoClaw Config

PicoClaw 的独立 Web 配置编辑器，提供可视化 JSON 配置编辑和 OAuth Provider 认证管理。

## 功能

- 📝 **配置编辑** — 基于 Web 的 JSON 编辑器，支持实时校验、格式化、`Ctrl+S` 保存
- 🔐 **Provider 认证** — 支持 OpenAI (Device Code)、Anthropic (API Token)、Google Antigravity (Browser OAuth) 登录
- 🌐 **嵌入式前端** — 编译为单一二进制文件，无需额外依赖

## 快速开始

```bash
# 编译
go build -o picoclaw-config ./cmd/picoclaw-config/

# 运行（使用默认配置路径 ~/.picoclaw/config.json）
./picoclaw-config

# 指定配置文件
./picoclaw-config ./config.json

# 允许局域网访问
./picoclaw-config -public
```

启动后在浏览器中打开 `http://localhost:18800`。

## 命令行参数

```
Usage: picoclaw-config [options] [config.json]

Arguments:
  config.json    配置文件路径（默认: ~/.picoclaw/config.json）

Options:
  -public        监听所有网络接口（0.0.0.0），允许局域网设备访问
```

## API 文档

Base URL: `http://localhost:18800`

### Config API

#### GET /api/config

读取当前配置文件内容。

**Response** `200 OK`

```json
{
  "config": { ... },
  "path": "/Users/xiao/.picoclaw/config.json"
}
```

---

#### PUT /api/config

保存配置。请求体为完整的 Config JSON。

**Request Body** — `application/json`

```json
{
  "agents": { "defaults": { "model_name": "gpt-5.2" } },
  "model_list": [
    {
      "model_name": "gpt-5.2",
      "model": "openai/gpt-5.2",
      "auth_method": "oauth"
    }
  ]
}
```

**Response** `200 OK`

```json
{ "status": "ok" }
```

**Error** `400 Bad Request` — 无效 JSON

---

### Auth API

#### GET /api/auth/status

获取所有 Provider 的认证状态和进行中的 Device Code 登录信息。

**Response** `200 OK`

```json
{
  "providers": [
    {
      "provider": "openai",
      "auth_method": "oauth",
      "status": "active",
      "account_id": "user-xxx",
      "expires_at": "2026-03-01T00:00:00Z"
    }
  ],
  "pending_device": {
    "provider": "openai",
    "status": "pending",
    "device_url": "https://auth.openai.com/activate",
    "user_code": "ABCD-1234"
  }
}
```

`status` 可选值: `active` | `expired` | `needs_refresh`

`pending_device` 仅在有进行中的 Device Code 登录时返回。

---

#### POST /api/auth/login

发起 Provider 登录。

**Request Body** — `application/json`

```json
{ "provider": "openai" }
```

支持的 `provider` 值: `openai` | `anthropic` | `google-antigravity`

##### OpenAI (Device Code Flow)

返回 Device Code 信息，后台自动轮询认证结果：

```json
{
  "status": "pending",
  "device_url": "https://auth.openai.com/activate",
  "user_code": "ABCD-1234",
  "message": "Open the URL and enter the code to authenticate."
}
```

用户在浏览器中打开 `device_url` 并输入 `user_code`。认证完成后通过 `GET /api/auth/status` 的 `pending_device.status` 变为 `success` 通知前端。

##### Anthropic (API Token)

需在请求中附带 token：

```json
{ "provider": "anthropic", "token": "sk-ant-xxx" }
```

**Response:**

```json
{ "status": "success", "message": "Anthropic token saved" }
```

##### Google Antigravity (Browser OAuth)

返回授权 URL，前端打开新标签页：

```json
{
  "status": "redirect",
  "auth_url": "https://accounts.google.com/o/oauth2/auth?...",
  "message": "Open the URL to authenticate with Google."
}
```

认证完成后 Google 回调至 `GET /auth/callback`，自动保存凭据并重定向回 picoclaw-config 页面。

---

#### POST /api/auth/logout

登出 Provider。

**Request Body** — `application/json`

```json
{ "provider": "openai" }
```

传空字符串或省略 `provider` 则登出所有 Provider。

**Response** `200 OK`

```json
{ "status": "ok" }
```

---

#### GET /auth/callback

OAuth Browser 回调端点（Google Antigravity 专用），由 OAuth Provider 重定向调用，**非前端直接使用**。

**Query Parameters:**
- `state` — OAuth state 校验
- `code` — 授权码

认证成功后重定向到 `/#auth`。
## 测试

```bash
go test -v ./cmd/picoclaw-config/
```

# PicoClaw Config

PicoClaw 的独立 Web 配置编辑器，提供可视化 JSON 配置编辑和 OAuth Provider 认证管理。

## 功能

- 📝 **配置编辑** — 侧边栏式设置 UI，支持模型管理、通道配置表单和原始 JSON 编辑器（`Ctrl+S` 保存）
- 🤖 **模型管理** — 模型卡片网格，可用性状态显示（无 API Key 时灰色），主模型选择，增删改查，必填/选填字段分离
- 📡 **通道配置** — 12 种通道类型（Telegram、Discord、Slack、企业微信、钉钉、飞书、LINE、WhatsApp、QQ、OneBot、MaixCAM 等）的表单化配置，附带文档链接
- 🔐 **Provider 认证** — 支持 OpenAI (Device Code)、Anthropic (API Token)、Google Antigravity (Browser OAuth) 登录
- 🌐 **嵌入式前端** — 编译为单一二进制文件，无需额外依赖
- 🌍 **国际化** — 中英文切换，首次访问自动检测浏览器语言
- 🎨 **主题** — 亮色 / 暗色 / 跟随系统，偏好保存在 localStorage

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

## 前端

前端是单个 HTML 文件（`internal/ui/index.html`），通过 `//go:embed` 嵌入到二进制中。使用原生 JS，无外部框架依赖。

### 布局

```
┌──────────────────────────────────────────────────────────┐
│  Logo  PicoClaw Config       [🎨] [EN/中] [▶ 启动/停止]  │
├──────────────┬───────────────────────────────────────────┤
│  ▾ 提供商     │   内容面板                                │
│    模型       │   （根据侧边栏选中项渲染）                  │
│    认证       │                                           │
│  ▾ 通道       │                                           │
│    Telegram  │                                           │
│    Discord   │                                           │
│    ...       │                                           │
│  ──────────  │                                           │
│  原始 JSON    │                                           │
├──────────────┴───────────────────────────────────────────┤
│  Footer                                                  │
└──────────────────────────────────────────────────────────┘
```

### 数据流

1. 页面加载 → `GET /api/config` → 存入 JS 全局变量 `configData`
2. 点击侧边栏 → 根据 `configData` 渲染对应面板
3. 用户修改并保存 → 合并表单数据回 `configData` → `PUT /api/config`
4. 认证面板使用 `/api/auth/*` 端点
5. 启动/停止按钮使用 `/api/process/*` 端点

### 国际化

- 翻译字典：`i18nData.en` / `i18nData.zh`
- `t(key, params)` — 运行时翻译查找，支持 `{param}` 参数替换
- 静态 HTML 使用 `data-i18n` 属性，由 `applyI18n()` 更新
- 语言偏好保存在 `localStorage('picoclaw-lang')`，首次访问自动检测浏览器语言

### 主题

通过 Header 按钮循环切换三种模式：**跟随系统**（默认）→ **亮色** → **暗色**

- CSS 变量通过 `[data-theme="light"]` / `[data-theme="dark"]` 选择器按主题定义
- `<head>` 中的内联 `<script>` 在渲染前应用主题，避免闪烁
- 监听 `prefers-color-scheme` 媒体查询，实时响应系统主题变化
- 偏好保存在 `localStorage('picoclaw-theme')`

## API 文档

Base URL: `http://localhost:18800`

### 静态文件

#### GET /

提供嵌入式前端页面（`index.html`）。

---

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
## Process API

#### GET /api/process/status

获取 `picoclaw gateway` 进程的运行状态。

**Response** `200 OK` (运行中)

```json
{
  "process_status": "running",
  "status": "ok",
  "uptime": "1.010814s"
}
```

**Response** `200 OK` (未运行)

```json
{
  "process_status": "stopped",
  "error": "Get \"http://localhost:18790/health\": dial tcp [::1]:18790: connect: connection refused"
}
```

---

#### POST /api/process/start

在后台启动 `picoclaw gateway` 进程。

**Response** `200 OK`

```json
{
  "status": "ok",
  "pid": 12345
}
```

---

#### POST /api/process/stop

停止正在运行的 `picoclaw gateway` 进程。

**Response** `200 OK`

```json
{
  "status": "ok"
}
```

---

## 测试

```bash
go test -v ./cmd/picoclaw-config/
```

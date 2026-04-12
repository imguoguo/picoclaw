package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMClient wraps an OpenAI-compatible chat completion endpoint.
type LLMClient struct {
	BaseURL    string
	Model      string
	APIKey     string
	NoThinking bool // send chat_template_kwargs to disable thinking (llama.cpp specific)
	Client     *http.Client
}

// LLMClientOptions configures the LLM client.
type LLMClientOptions struct {
	BaseURL    string
	Model      string
	APIKey     string
	Timeout    time.Duration
	NoThinking bool
}

// NewLLMClient creates a client for an OpenAI-compatible chat completion API.
func NewLLMClient(opts LLMClientOptions) *LLMClient {
	if opts.Timeout == 0 {
		opts.Timeout = 120 * time.Second
	}
	return &LLMClient{
		BaseURL:    strings.TrimRight(opts.BaseURL, "/"),
		Model:      opts.Model,
		APIKey:     opts.APIKey,
		NoThinking: opts.NoThinking,
		Client: &http.Client{
			Timeout: opts.Timeout,
		},
	}
}

type chatRequest struct {
	Model              string         `json:"model"`
	Messages           []chatMessage  `json:"messages"`
	Temperature        float64        `json:"temperature"`
	MaxTokens          int            `json:"max_tokens"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete sends a chat completion request and returns the assistant's reply.
func (c *LLMClient) Complete(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	messages := []chatMessage{}
	if systemPrompt != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, chatMessage{Role: "user", Content: userPrompt})

	body := chatRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: 0.1,
		MaxTokens:   512,
	}
	if c.NoThinking {
		body.ChatTemplateKwargs = map[string]any{
			"enable_thinking": false,
		}
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	endpoint := c.BaseURL + "/v1/chat/completions"
	if strings.HasSuffix(c.BaseURL, "/v1") || strings.HasSuffix(c.BaseURL, "/v1/") {
		endpoint = strings.TrimRight(c.BaseURL, "/") + "/chat/completions"
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}
	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	// Strip any residual <think>...</think> blocks
	if idx := strings.Index(content, "</think>"); idx >= 0 {
		content = strings.TrimSpace(content[idx+len("</think>"):])
	}
	return content, nil
}

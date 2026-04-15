package commands

import "github.com/sipeed/picoclaw/pkg/config"

// TokenUsage mirrors per-turn LLM token counts for command display.
// Kept local to avoid pulling pkg/providers into command handlers.
type TokenUsage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// ContextStats describes the current session's conversation size and the last
// observed LLM token usage. Returned by Runtime.GetContextStats for /context.
type ContextStats struct {
	SessionKey     string
	MessageCount   int
	EstimatedChars int
	Summary        string
	LastUsage      *TokenUsage
}

// Runtime provides runtime dependencies to command handlers. It is constructed
// per-request by the agent loop so that per-request state (like session scope)
// can coexist with long-lived callbacks (like GetModelInfo).
type Runtime struct {
	Config             *config.Config
	GetModelInfo       func() (name, provider string)
	ListAgentIDs       func() []string
	ListDefinitions    func() []Definition
	ListSkillNames     func() []string
	GetEnabledChannels func() []string
	GetActiveTurn      func() any // Returning any to avoid circular dependency with agent package
	GetContextStats    func() *ContextStats
	SwitchModel        func(value string) (oldModel string, err error)
	SwitchChannel      func(value string) error
	ClearHistory       func() error
	ReloadConfig       func() error
}

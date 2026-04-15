package commands

import (
	"context"
	"strings"
	"testing"
)

func TestContextCommand_NoRuntime(t *testing.T) {
	var reply string
	req := Request{Text: "/context", Reply: func(s string) error {
		reply = s
		return nil
	}}
	if err := contextCommand().Handler(context.Background(), req, nil); err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if reply != unavailableMsg {
		t.Fatalf("reply = %q, want unavailableMsg", reply)
	}
}

func TestContextCommand_WithStats(t *testing.T) {
	var reply string
	req := Request{Text: "/context", Reply: func(s string) error {
		reply = s
		return nil
	}}
	rt := &Runtime{
		GetContextStats: func() *ContextStats {
			return &ContextStats{
				SessionKey:     "agent:main",
				MessageCount:   6,
				EstimatedChars: 4096,
				LastUsage: &TokenUsage{
					PromptTokens:     1200,
					CompletionTokens: 340,
					TotalTokens:      1540,
				},
			}
		},
	}
	if err := contextCommand().Handler(context.Background(), req, rt); err != nil {
		t.Fatalf("handler err: %v", err)
	}
	for _, want := range []string{
		"Messages: 6",
		"4096 chars",
		"prompt=1200",
		"completion=340",
		"total=1540",
	} {
		if !strings.Contains(reply, want) {
			t.Errorf("reply missing %q\n---\n%s", want, reply)
		}
	}
}

func TestContextCommand_NoUsage(t *testing.T) {
	var reply string
	req := Request{Text: "/context", Reply: func(s string) error {
		reply = s
		return nil
	}}
	rt := &Runtime{
		GetContextStats: func() *ContextStats {
			return &ContextStats{MessageCount: 1, EstimatedChars: 10}
		},
	}
	if err := contextCommand().Handler(context.Background(), req, rt); err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if !strings.Contains(reply, "usage not reported") {
		t.Errorf("expected fallback notice, got %q", reply)
	}
}

func TestContextCommand_RegisteredBuiltin(t *testing.T) {
	found := false
	for _, d := range BuiltinDefinitions() {
		if d.Name == "context" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("context command not registered in BuiltinDefinitions()")
	}
}

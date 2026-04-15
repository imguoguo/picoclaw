package commands

import (
	"context"
	"fmt"
	"strings"
)

// charsPerTokenApprox is the rough character-to-token ratio used when a
// provider does not return usage data. Matches the common ~4 chars/token
// heuristic for English + code mixes; it is only a fallback signal.
const charsPerTokenApprox = 4

func contextCommand() Definition {
	return Definition{
		Name:        "context",
		Description: "Show current session context and token usage",
		Usage:       "/context",
		Handler: func(_ context.Context, req Request, rt *Runtime) error {
			if rt == nil || rt.GetContextStats == nil {
				return req.Reply(unavailableMsg)
			}
			stats := rt.GetContextStats()
			if stats == nil {
				return req.Reply("No active session context.")
			}
			return req.Reply(formatContextStats(stats))
		},
	}
}

func formatContextStats(s *ContextStats) string {
	var b strings.Builder
	b.WriteString("Context usage\n")
	if s.SessionKey != "" {
		fmt.Fprintf(&b, "Session: %s\n", s.SessionKey)
	}
	fmt.Fprintf(&b, "Messages: %d\n", s.MessageCount)

	approxTokens := s.EstimatedChars / charsPerTokenApprox
	fmt.Fprintf(&b, "History size: %d chars (~%d tokens est.)\n", s.EstimatedChars, approxTokens)

	if s.Summary != "" {
		fmt.Fprintf(&b, "Summary: %d chars\n", len(s.Summary))
	}

	if s.LastUsage != nil {
		fmt.Fprintf(
			&b,
			"Last turn: prompt=%d completion=%d total=%d",
			s.LastUsage.PromptTokens,
			s.LastUsage.CompletionTokens,
			s.LastUsage.TotalTokens,
		)
	} else {
		b.WriteString("Last turn: usage not reported by provider")
	}
	return b.String()
}

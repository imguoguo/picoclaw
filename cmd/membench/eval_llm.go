package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/sipeed/picoclaw/pkg/seahorse"
)

const answerSystemPrompt = `You are a helpful assistant. Given conversation context, answer the question concisely and accurately. If the answer is not in the context, say "I don't know". Answer in 1-3 sentences maximum.`

const judgeSystemPrompt = `You are an impartial judge evaluating answer quality.
Compare the candidate answer against the reference answer.
Consider semantic equivalence — different wording expressing the same meaning should score high.

Output ONLY a single integer score from 1 to 5:
1 = completely wrong or irrelevant
2 = partially related but mostly incorrect
3 = partially correct, missing key details
4 = mostly correct with minor omissions
5 = fully correct, semantically equivalent

Output ONLY the number, nothing else.`

// generateAnswer asks the LLM to answer a question given retrieved context.
func generateAnswer(ctx context.Context, client *LLMClient, contextText, question string) (string, error) {
	// Truncate context to avoid exceeding model limits while preserving valid UTF-8.
	contextRunes := []rune(contextText)
	if len(contextRunes) > 6000 {
		contextText = string(contextRunes[:6000]) + "\n... [truncated]"
	}

	userPrompt := fmt.Sprintf("## Conversation Context\n\n%s\n\n## Question\n\n%s", contextText, question)
	return client.Complete(ctx, answerSystemPrompt, userPrompt)
}

// judgeAnswer asks the LLM to score the candidate answer vs the gold answer.
// Returns a score from 0.0 to 1.0.
func judgeAnswer(
	ctx context.Context,
	client *LLMClient,
	question, goldAnswer, candidateAnswer string,
) (float64, error) {
	userPrompt := fmt.Sprintf(
		"Question: %s\n\nReference Answer: %s\n\nCandidate Answer: %s\n\nScore:",
		question, goldAnswer, candidateAnswer,
	)

	response, err := client.Complete(ctx, judgeSystemPrompt, userPrompt)
	if err != nil {
		return 0.0, err
	}

	// Parse score from response
	response = strings.TrimSpace(response)
	// Extract first digit found
	for _, ch := range response {
		if ch >= '1' && ch <= '5' {
			score, _ := strconv.Atoi(string(ch))
			return float64(score-1) / 4.0, nil // Normalize 1-5 to 0.0-1.0
		}
	}
	log.Printf("WARNING: could not parse judge score from: %q, defaulting to 0.0", response)
	return 0.0, nil
}

// EvalLegacyLLM evaluates legacy store using LLM generation + LLM-as-Judge.
func EvalLegacyLLM(
	ctx context.Context,
	samples []LocomoSample,
	legacy *LegacyStore,
	budgetTokens int,
	client *LLMClient,
) []EvalResult {
	totalQA := countTotalQA(samples)
	results := make([]EvalResult, 0, len(samples))
	total := 0
	for si := range samples {
		sample := &samples[si]
		history := legacy.GetHistory(sample.SampleID)

		allContent := make([]string, 0, len(history))
		for _, msg := range history {
			allContent = append(allContent, msg.Content)
		}

		qaResults := make([]QAResult, 0, len(sample.QA))
		for qi := range sample.QA {
			qa := &sample.QA[qi]
			total++
			truncated, _ := BudgetTruncate(allContent, budgetTokens)
			contextText := StringListToContent(truncated)

			// Generate answer with LLM
			llmAnswer, err := generateAnswer(ctx, client, contextText, qa.Question)
			if err != nil {
				log.Printf("WARN: LLM generation failed for sample %s Q%d: %v", sample.SampleID, qi, err)
				llmAnswer = ""
			}

			// Judge the answer
			score := 0.0
			if llmAnswer != "" {
				score, err = judgeAnswer(ctx, client, qa.Question, qa.AnswerString(), llmAnswer)
				if err != nil {
					log.Printf("WARN: LLM judge failed for sample %s Q%d: %v", sample.SampleID, qi, err)
				}
			}

			hitRate := RecallHitRate(qa.Evidence, sample, contextText)

			qaResults = append(qaResults, QAResult{
				Question:   qa.Question,
				Category:   qa.Category,
				GoldAnswer: qa.AnswerString(),
				TokenF1:    score,
				HitRate:    hitRate,
			})

			log.Printf("[legacy-llm] sample=%s q=%d/%d score=%.2f answer=%q",
				sample.SampleID, total, totalQA, score, truncateStr(llmAnswer, 80))
		}

		results = append(results, EvalResult{
			Mode:      "legacy-llm",
			SampleID:  sample.SampleID,
			QAResults: qaResults,
			Agg:       aggregateMetrics(qaResults),
		})
	}
	return results
}

// EvalSeahorseLLM evaluates seahorse retrieval using LLM generation + LLM-as-Judge.
func EvalSeahorseLLM(
	ctx context.Context,
	samples []LocomoSample,
	ir *SeahorseIngestResult,
	budgetTokens int,
	client *LLMClient,
) []EvalResult {
	store := ir.Engine.GetRetrieval().Store()
	retrieval := ir.Engine.GetRetrieval()

	totalQA := countTotalQA(samples)
	results := make([]EvalResult, 0, len(samples))
	total := 0
	for si := range samples {
		sample := &samples[si]
		convID, ok := ir.ConvMap[sample.SampleID]
		if !ok {
			log.Printf("WARN: no conversation ID for sample %s", sample.SampleID)
			continue
		}

		qaResults := make([]QAResult, 0, len(sample.QA))
		for qi := range sample.QA {
			qa := &sample.QA[qi]
			total++
			keywords := ExtractKeywords(qa.Question)

			// Search and rank
			bestRank := map[int64]float64{}
			for _, kw := range keywords {
				searchResults, err := store.SearchMessages(ctx, seahorse.SearchInput{
					Pattern:        kw,
					ConversationID: convID,
					Limit:          20,
				})
				if err != nil {
					log.Printf("WARN: search failed for keyword %q: %v", kw, err)
					continue
				}
				for _, sr := range searchResults {
					if sr.MessageID > 0 {
						if prev, ok := bestRank[sr.MessageID]; !ok || sr.Rank < prev {
							bestRank[sr.MessageID] = sr.Rank
						}
					}
				}
			}

			messageIDs := make([]int64, 0, len(bestRank))
			for id := range bestRank {
				messageIDs = append(messageIDs, id)
			}
			sortByRank(messageIDs, bestRank)

			var contentParts []string
			if len(messageIDs) > 0 {
				expandResult, err := retrieval.ExpandMessages(ctx, messageIDs)
				if err == nil {
					for _, msg := range expandResult.Messages {
						contentParts = append(contentParts, msg.Content)
					}
				}
			}

			contextText := ""
			if len(contentParts) > 0 {
				truncated, _ := BudgetTruncate(contentParts, budgetTokens)
				contextText = StringListToContent(truncated)
			}

			// Generate answer with LLM
			llmAnswer := ""
			score := 0.0
			if contextText != "" {
				var err error
				llmAnswer, err = generateAnswer(ctx, client, contextText, qa.Question)
				if err != nil {
					log.Printf("WARN: LLM generation failed for sample %s Q%d: %v", sample.SampleID, qi, err)
				}
			}

			// Judge the answer
			if llmAnswer != "" {
				var err error
				score, err = judgeAnswer(ctx, client, qa.Question, qa.AnswerString(), llmAnswer)
				if err != nil {
					log.Printf("WARN: LLM judge failed for sample %s Q%d: %v", sample.SampleID, qi, err)
				}
			}

			hitRate := RecallHitRate(qa.Evidence, sample, contextText)

			qaResults = append(qaResults, QAResult{
				Question:   qa.Question,
				Category:   qa.Category,
				GoldAnswer: qa.AnswerString(),
				TokenF1:    score,
				HitRate:    hitRate,
			})

			log.Printf("[seahorse-llm] sample=%s q=%d/%d score=%.2f answer=%q",
				sample.SampleID, total, totalQA, score, truncateStr(llmAnswer, 80))
		}

		results = append(results, EvalResult{
			Mode:      "seahorse-llm",
			SampleID:  sample.SampleID,
			QAResults: qaResults,
			Agg:       aggregateMetrics(qaResults),
		})
	}
	return results
}

func countTotalQA(samples []LocomoSample) int {
	n := 0
	for i := range samples {
		n += len(samples[i].QA)
	}
	return n
}

func truncateStr(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// sortByRank sorts message IDs by BM25 rank (more negative = better).
func sortByRank(ids []int64, ranks map[int64]float64) {
	for i := 1; i < len(ids); i++ {
		key := ids[i]
		j := i - 1
		for j >= 0 && ranks[ids[j]] > ranks[key] {
			ids[j+1] = ids[j]
			j--
		}
		ids[j+1] = key
	}
}

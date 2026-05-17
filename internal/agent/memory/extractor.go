package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// Extractor pulls persistent facts from conversations and stores them in user_context.
type Extractor struct {
	client llm.Client
	store  repository.UserContextRepo
	log    logger.Logger
}

// NewExtractor creates a new memory extractor.
func NewExtractor(client llm.Client, store repository.UserContextRepo, log logger.Logger) *Extractor {
	return &Extractor{client: client, store: store, log: log}
}

// ExtractAndSave analyzes a conversation and persists extracted facts.
// It is designed to run in a background goroutine.
func (e *Extractor) ExtractAndSave(ctx context.Context, userID uuid.UUID, history []model.Message) {
	if e.client == nil || e.store == nil {
		return
	}
	if len(history) < 2 {
		return
	}

	// Build conversation text for the prompt
	var convText strings.Builder
	for _, m := range history {
		role := string(m.Role)
		if role == "tool" {
			role = "assistant"
		}
		convText.WriteString(fmt.Sprintf("%s: %s\n", role, m.Content))
	}

	prompt := fmt.Sprintf(`Analyze this conversation and extract persistent facts about the user.
Output ONLY a JSON array, no explanation, no markdown:

[
  {"key": "preferred_language", "value": "TypeScript", "confidence": 0.9},
  {"key": "current_project", "value": "e-commerce with Next.js", "confidence": 0.8}
]

Rules:
- Only extract facts that would be useful in FUTURE conversations.
- Use snake_case for keys.
- Value can be string, number, or boolean.
- Confidence must be between 0.0 and 1.0.
- If nothing useful can be extracted, output an empty array [].

Conversation:
%s`, convText.String())

	extractCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := e.client.Complete(extractCtx, llm.Request{
		Model:       "qwen2.5-coder:3b",
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.0,
		MaxTokens:   500,
		Stream:      false,
	})
	if err != nil {
		e.log.Warn("memory extractor: LLM call failed", logger.Error(err))
		return
	}

	items, err := parseExtractedItems(resp)
	if err != nil {
		e.log.Warn("memory extractor: failed to parse LLM output", logger.Error(err), logger.String("raw", resp))
		return
	}

	for _, item := range items {
		if item.Confidence < 0.6 {
			continue
		}
		if item.Key == "" || item.Value == nil {
			continue
		}

		// Respect explicit sources: don't overwrite explicit with inferred if inferred has lower confidence
		existing, _ := e.store.GetByKey(ctx, userID, item.Key)
		if existing != nil && existing.Source == model.ContextSourceExplicit {
			if item.Source == model.ContextSourceInferred && item.Confidence <= existing.Confidence {
				continue
			}
		}

		item.UserID = userID
		if item.Source == "" {
			item.Source = model.ContextSourceInferred
		}

		if err := e.store.Upsert(ctx, item); err != nil {
			e.log.Error("memory extractor: failed to save item",
				logger.Error(err),
				logger.String("key", item.Key),
				logger.String("user_id", userID.String()))
			continue
		}
		e.log.Info("memory extractor: saved user context",
			logger.String("key", item.Key),
			logger.Any("value", item.Value),
			logger.Any("confidence", item.Confidence))
	}
}

// extractedItem is the raw shape returned by the LLM.
type extractedItem struct {
	Key        string  `json:"key"`
	Value      any     `json:"value"`
	Confidence float64 `json:"confidence"`
}

func parseExtractedItems(raw string) ([]model.UserContextItem, error) {
	// Try to find JSON array in the response
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, fmt.Errorf("no JSON array found in response")
	}
	var items []extractedItem
	if err := json.Unmarshal([]byte(raw[start:end+1]), &items); err != nil {
		return nil, fmt.Errorf("unmarshal extracted items: %w", err)
	}

	result := make([]model.UserContextItem, 0, len(items))
	for _, it := range items {
		result = append(result, model.UserContextItem{
			Key:        it.Key,
			Value:      it.Value,
			Confidence: it.Confidence,
			Source:     model.ContextSourceInferred,
		})
	}
	return result, nil
}

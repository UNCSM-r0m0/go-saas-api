package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/flows"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/quality"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/internal/agent/usage"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

type contextKey string

const (
	contextKeyConversationID contextKey = "conversation_id"
	contextKeyUserID         contextKey = "user_id"
	MaxAgentIterations                  = 10
)

const flowStateMarker = "---FLOW_STATE---"
const flowStateTailLen = len(flowStateMarker) - 1 // 15

// flowStateSanitizer strips the FLOW_STATE block from streaming chunks.
type flowStateSanitizer struct {
	blocked    bool
	buffer     strings.Builder
	streamedUp int
	collecting bool
}

func (s *flowStateSanitizer) Process(chunk string) string {
	if s.blocked {
		return ""
	}
	s.buffer.WriteString(chunk)
	if s.collecting {
		// Buffer everything; emit sanitized content at Flush
		return ""
	}
	full := s.buffer.String()
	if idx := strings.Index(full, flowStateMarker); idx != -1 {
		s.blocked = true
		visible := full[:idx]
		if len(visible) > s.streamedUp {
			out := visible[s.streamedUp:]
			s.streamedUp = len(visible)
			return out
		}
		return ""
	}
	safeLen := len(full) - flowStateTailLen
	if safeLen < 0 {
		safeLen = 0
	}
	if safeLen > s.streamedUp {
		out := full[s.streamedUp:safeLen]
		s.streamedUp = safeLen
		return out
	}
	return ""
}

func (s *flowStateSanitizer) Flush() string {
	if s.blocked && !s.collecting {
		return ""
	}
	full := s.buffer.String()
	var visible string
	if idx := strings.Index(full, flowStateMarker); idx != -1 {
		visible = full[:idx]
	} else {
		visible = full
	}
	if s.collecting {
		visible = flows.EnforceSingleQuestion(visible, "collecting")
	}
	if len(visible) > s.streamedUp {
		out := visible[s.streamedUp:]
		s.streamedUp = len(visible)
		return out
	}
	return ""
}

func (s *flowStateSanitizer) VisibleContent() string {
	full := s.buffer.String()
	var visible string
	if idx := strings.Index(full, flowStateMarker); idx != -1 {
		visible = full[:idx]
	} else {
		visible = full
	}
	if s.collecting {
		visible = flows.EnforceSingleQuestion(visible, "collecting")
	}
	return visible
}

func (s *flowStateSanitizer) RawContent() string {
	return s.buffer.String()
}


// agentLoop runs the ReAct (Reason-Act) loop:
//  1. Send conversation to LLM with tools available
//  2. If LLM returns tool calls → execute them → add results to history → loop
//  3. If LLM returns text only → quality gate (if enabled) → save and finish
//
// All events (content chunks, tool_start, tool_result, done) are sent to outCh.
func (o *Orchestrator) agentLoop(
	ctx context.Context,
	outCh chan<- llm.Chunk,
	agent *model.Agent,
	initialMessages []llm.Message,
	toolDefs []llm.ToolDefinition,
	toolList []tools.Tool,
	conversationID uuid.UUID,
	userID uuid.UUID,
	userMessage string,
	activeFlow flows.Flow,
	metadata map[string]any,
) {
	messages := make([]llm.Message, len(initialMessages))
	copy(messages, initialMessages)

	var allContent strings.Builder
	var allToolCalls []model.ToolCall
	qualityRetryDone := false
	var totalLatencyMs int
	var totalInputTokens int
	var totalOutputTokens int
	var realDeliverables []string

	for i := 0; i < MaxAgentIterations; i++ {
		req := llm.Request{
			Model:       agent.Model,
			Messages:    messages,
			Tools:       toolDefs,
			Temperature: 0.7,
			MaxTokens:   4096,
			Stream:      true,
		}

		start := time.Now()
		llmCh, err := o.llmClient.Stream(ctx, req)
		if err != nil {
			select {
			case outCh <- llm.Chunk{Event: "error", Content: fmt.Sprintf("LLM error: %v", err), Done: true}:
			case <-ctx.Done():
			}
			return
		}

		var sanitizer *flowStateSanitizer
		if activeFlow != nil && activeFlow.Name() == "api_integration" {
			sanitizer = &flowStateSanitizer{
				collecting: flows.GetStatus(metadata) == "collecting",
			}
		}
		var fallbackContent strings.Builder
		var toolCalls []llm.ToolCall
		var chunkUsage *llm.Usage

		for chunk := range llmCh {
			if chunk.Content != "" {
				if sanitizer != nil {
					if out := sanitizer.Process(chunk.Content); out != "" {
						select {
						case outCh <- llm.Chunk{Content: out}:
						case <-ctx.Done():
							return
						}
					}
				} else {
					fallbackContent.WriteString(chunk.Content)
					select {
					case outCh <- llm.Chunk{Content: chunk.Content}:
					case <-ctx.Done():
						return
					}
				}
			}
			if chunk.ToolCall != nil {
				toolCalls = append(toolCalls, *chunk.ToolCall)
			}
			if chunk.Usage != nil {
				chunkUsage = chunk.Usage
			}
			if chunk.Done {
				break
			}
		}

		// Flush any remaining safe tail after stream ends
		if sanitizer != nil {
			if out := sanitizer.Flush(); out != "" {
				select {
				case outCh <- llm.Chunk{Content: out}:
				case <-ctx.Done():
					return
				}
			}
		}

		var visibleContent string
		var rawContent string
		if sanitizer != nil {
			visibleContent = sanitizer.VisibleContent()
			rawContent = sanitizer.RawContent()
		} else {
			visibleContent = fallbackContent.String()
			rawContent = visibleContent
		}

		latencyMs := int(time.Since(start).Milliseconds())
		totalLatencyMs += latencyMs

		if chunkUsage != nil {
			totalInputTokens += chunkUsage.PromptTokens
			totalOutputTokens += chunkUsage.CompletionTokens
		} else {
			totalInputTokens += llm.EstimateRequestTokens(messages)
			totalOutputTokens += llm.EstimateTokens(rawContent)
		}

		if len(toolCalls) == 0 {
			allContent.WriteString(visibleContent)

			// Quality gate: evaluate final response before saving
			if !qualityRetryDone && o.qualityGate != nil && quality.ShouldEvaluate(agent, visibleContent) {
				eval, err := o.qualityGate.Evaluate(ctx, userMessage, visibleContent)
				if err == nil && eval.Regenerate && eval.Reason != "" {
					o.log.Info("quality gate: requesting regeneration", logger.String("reason", eval.Reason))

					// Notify user that we're improving the response
					select {
					case outCh <- llm.Chunk{Event: "progress", Content: "Revisando y mejorando la respuesta..."}:
					case <-ctx.Done():
						return
					}

					// Add system feedback to messages and retry once
					messages = append(messages, llm.Message{
						Role:    "assistant",
						Content: visibleContent,
					})
					messages = append(messages, llm.Message{
						Role:    "system",
						Content: fmt.Sprintf("Quality feedback: %s. Please address this issue and regenerate your response.", eval.Reason),
					})

					qualityRetryDone = true
					continue // Retry one more iteration
				}
			}

			// Enforce single question for API integration flow when collecting
			if activeFlow != nil && activeFlow.Name() == "api_integration" {
				visibleContent = flows.EnforceSingleQuestion(visibleContent, flows.GetStatus(metadata))
			}

			msg := &model.Message{
				ID:             uuid.New(),
				ConversationID: conversationID,
				Role:           model.MessageRoleAssistant,
				Content:        visibleContent,
				Model:          agent.Model,
				ToolCalls:      allToolCalls,
				TokensInput:    totalInputTokens,
				TokensOutput:   totalOutputTokens,
				LatencyMs:      totalLatencyMs,
				CreatedAt:      time.Now(),
			}
			_ = o.sessions.AddMessage(ctx, msg)

			if activeFlow != nil {
				updatedMeta := activeFlow.UpdateState(metadata, rawContent)
				if apiFlow, ok := activeFlow.(*flows.APIIntegrationFlow); ok && len(realDeliverables) > 0 {
					updatedMeta = apiFlow.AddRealDeliverables(updatedMeta, realDeliverables)
				}
				if err := o.sessions.UpdateConversationMetadata(ctx, conversationID, updatedMeta); err != nil {
					o.log.Warn("failed to update conversation metadata", logger.Error(err))
				}
			}

			if o.usageTracker != nil {
				pricing := llm.ResolvePricing(agent.Model)
				cost := pricing.CalculateCost(totalInputTokens, totalOutputTokens)
				_ = o.usageTracker.Record(ctx, usage.Record{
					UserID:         userID,
					ConversationID: conversationID,
					Model:          agent.Model,
					TokensInput:    totalInputTokens,
					TokensOutput:   totalOutputTokens,
					LatencyMs:      totalLatencyMs,
					CostUSD:        cost,
				})
			}

			select {
			case outCh <- llm.Chunk{Done: true}:
			case <-ctx.Done():
			}
			return
		}

		allContent.WriteString(visibleContent)

		var toolResultMessages []llm.Message
		for _, tc := range toolCalls {
			if tc.ID == "" {
				tc.ID = uuid.New().String()
			}

			select {
			case outCh <- llm.Chunk{Event: "tool_start", ToolName: tc.Name, ToolCall: &tc}:
			case <-ctx.Done():
				return
			}

			toolCtx := context.WithValue(ctx, contextKeyConversationID, conversationID)
			toolCtx = context.WithValue(toolCtx, contextKeyUserID, userID)
			result, execErr := o.registry.Execute(toolCtx, tc.Name, tc.Arguments)

			resultContent := result.Content
			if execErr != nil {
				resultContent = fmt.Sprintf("Error: %v", execErr)
			} else if result.Error != "" {
				resultContent = fmt.Sprintf("Error: %s", result.Error)
			}

			select {
			case outCh <- llm.Chunk{Event: "tool_result", ToolName: tc.Name, Content: resultContent}:
			case <-ctx.Done():
				return
			}

			// Track real deliverables from successful file_write tool calls
			if tc.Name == "file_write" {
				if resultContent != "" && !strings.HasPrefix(resultContent, "Error:") {
					if name, ok := tc.Arguments["name"].(string); ok && name != "" {
						realDeliverables = append(realDeliverables, name)
					}
				}
			}

			allToolCalls = append(allToolCalls, model.ToolCall{
				ID:        tc.ID,
				Name:      tc.Name,
				Arguments: tc.Arguments,
			})

			toolResultMessages = append(toolResultMessages, llm.Message{
				Role:       "tool",
				Content:    resultContent,
				ToolCallID: tc.ID,
				Name:       tc.Name,
			})
		}

		// Add assistant message (with tool calls) + tool results to history
		messages = append(messages, llm.Message{
			Role:      "assistant",
			Content:   visibleContent,
			ToolCalls: toolCalls,
		})
		messages = append(messages, toolResultMessages...)

		// Save intermediate messages to DB for conversation continuity
		assistantMsg := &model.Message{
			ID:             uuid.New(),
			ConversationID: conversationID,
			Role:           model.MessageRoleAssistant,
			Content:        visibleContent,
			Model:          agent.Model,
			ToolCalls: []model.ToolCall{{
				ID:        toolCalls[0].ID,
				Name:      toolCalls[0].Name,
				Arguments: toolCalls[0].Arguments,
			}},
			CreatedAt: time.Now(),
		}
		_ = o.sessions.AddMessage(ctx, assistantMsg)

		for i, tc := range toolCalls {
			resultContent := ""
			if i < len(toolResultMessages) {
				resultContent = toolResultMessages[i].Content
			}
			toolMsg := &model.Message{
				ID:             uuid.New(),
				ConversationID: conversationID,
				Role:           model.MessageRoleTool,
				Content:        resultContent,
				ToolCallID:     tc.ID,
				ToolName:       tc.Name,
				CreatedAt:      time.Now(),
			}
			_ = o.sessions.AddMessage(ctx, toolMsg)
		}
	}

	// Max iterations reached
	finalContent := "\n\nI've reached the maximum number of reasoning steps. Please continue the conversation if you need more work done."
	allContent.WriteString(finalContent)

	select {
	case outCh <- llm.Chunk{Content: finalContent, Done: true}:
	case <-ctx.Done():
	}

	// Sanitize max-iterations content too
	visibleFinal := allContent.String()
	if activeFlow != nil {
		if _, ok := activeFlow.(*flows.APIIntegrationFlow); ok {
			visibleFinal = flows.StripFlowStateBlock(visibleFinal)
			visibleFinal = flows.EnforceSingleQuestion(visibleFinal, flows.GetStatus(metadata))
		}
	}

	msg := &model.Message{
		ID:             uuid.New(),
		ConversationID: conversationID,
		Role:           model.MessageRoleAssistant,
		Content:        visibleFinal,
		Model:          agent.Model,
		ToolCalls:      allToolCalls,
		TokensInput:    totalInputTokens,
		TokensOutput:   totalOutputTokens,
		LatencyMs:      totalLatencyMs,
		CreatedAt:      time.Now(),
	}
	_ = o.sessions.AddMessage(ctx, msg)

	if activeFlow != nil {
		updatedMeta := activeFlow.UpdateState(metadata, allContent.String())
		if apiFlow, ok := activeFlow.(*flows.APIIntegrationFlow); ok && len(realDeliverables) > 0 {
			updatedMeta = apiFlow.AddRealDeliverables(updatedMeta, realDeliverables)
		}
		if err := o.sessions.UpdateConversationMetadata(ctx, conversationID, updatedMeta); err != nil {
			o.log.Warn("failed to update conversation metadata", logger.Error(err))
		}
	}

	if o.usageTracker != nil {
		pricing := llm.ResolvePricing(agent.Model)
		cost := pricing.CalculateCost(totalInputTokens, totalOutputTokens)
		_ = o.usageTracker.Record(ctx, usage.Record{
			UserID:         userID,
			ConversationID: conversationID,
			Model:          agent.Model,
			TokensInput:    totalInputTokens,
			TokensOutput:   totalOutputTokens,
			LatencyMs:      totalLatencyMs,
			CostUSD:        cost,
		})
	}
}

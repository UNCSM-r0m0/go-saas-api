# Proposal: Auto-Generated Conversation Titles

## Intent

Replace the current fallback title (first 50 characters of the first message) with a concise, LLM-generated title (3–6 words). This improves UX by giving users meaningful conversation labels without manual effort.

## Scope

### In Scope
- Async title generation triggered after the first user message in a new conversation
- `generateAndSaveTitle()` method on the Orchestrator
- `UpdateConversationTitle()` method on SessionManager
- Title prompt and cleaning logic (trim, limit length)
- Graceful fallback to truncated message if LLM fails

### Out of Scope
- Real-time frontend push (frontend sees title on next poll/refresh)
- Dedicated small model for titling (uses same chat model for MVP)
- Manual title editing UI (already exists via `PATCH /chat/sessions/:id`)

## Capabilities

### New Capabilities
- `conversation-title-generation`: Auto-generate short titles for new conversations using the LLM

### Modified Capabilities
- `agent-runtime`: Add title generation trigger to the chat flow after saving the first user message

## Approach

After saving the first user message in a new conversation, spawn a goroutine with a detached 10s context. Call `llm.Client.Complete()` with a title-generation prompt. Clean the result (trim whitespace, limit to 60 chars). Update `conversations.title` via `SessionManager.UpdateConversationTitle()`. If the LLM call fails or returns empty, keep the existing truncated-message fallback. The chat stream is never blocked.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/agent/runtime/orchestrator.go` | Modified | Add `generateAndSaveTitle()` and trigger after first message save |
| `internal/agent/runtime/session.go` | Modified | Add `UpdateConversationTitle()` method |
| `internal/agent/model/conversation.go` | Modified | Ensure `UpdatedAt` is set on title update |
| `internal/agent/repository/repository.go` | Modified | Add `UpdateConversationTitle` to `ConversationRepo` interface |
| `internal/agent/store/postgres.go` | Modified | Implement title update query |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Context cancellation kills title goroutine | Low | Use detached context with timeout, not request ctx |
| LLM failure leaves no title | Low | Keep existing truncated-message fallback |
| Adds minor DB write load | Low | Only one extra update per new conversation |

## Rollback Plan

Remove the goroutine spawn in `orchestrator.go` and the `UpdateConversationTitle` method. Titles will revert to the existing truncated-message fallback. No DB migration needed.

## Dependencies

- Existing `llm.Client.Complete()` infrastructure
- Existing `ConversationRepo.Update()` or new title-specific update

## Success Criteria

- [ ] New conversations receive an auto-generated title after the first message
- [ ] Chat streaming is not blocked by title generation
- [ ] Fallback truncated title is preserved if LLM fails
- [ ] Existing conversations are unaffected
- [ ] Project compiles without errors

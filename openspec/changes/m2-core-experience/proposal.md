# Proposal: M2 — Core Experience (API Keys + Conversations REST API)

## Intent

Enable real user interaction with the platform by delivering:
1. API key management for third-party integrations
2. Full REST API for conversations and messages so the frontend can render chat history

Without these, users cannot integrate the API into their own apps nor see their past conversations.

## Scope

### In Scope
- API Key lifecycle: create, list, revoke, validate in gateway
- Conversations REST: list, get, rename, archive, delete
- Messages REST: list by conversation (paginated)
- Conversation metadata (title auto-generation, message count, last activity)

### Out of Scope
- File upload (M2.2)
- Real sandbox (M2.3)
- Password reset / email (M2.4)
- WebSocket (M2.5)

## Capabilities

### New Capabilities
- `api-key-management`: CRUD and authentication via API keys
- `conversation-rest-api`: Full REST lifecycle for conversations
- `message-history-api`: Paginated message listing

### Modified Capabilities
- `api-gateway`: Add API key auth middleware alongside JWT

## Approach

Reuse existing patterns: PostgreSQL RLS repos, Gin handlers, JWT middleware pattern for API key auth. API keys stored as SHA-256 hashes (like passwords), prefix shown once on creation. Gateway tries API key header first, falls back to JWT.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/auth` | Modified | Add APIKeyStore + service methods |
| `internal/agent/repository` | Modified | Add ConversationRepo/MessageRepo methods |
| `internal/agent/handler` | New | REST handlers for conversations/messages |
| `cmd/api-gateway` | Modified | API key middleware + routing |
| `cmd/agent-service` | Modified | Register new handlers |
| `migrations/` | New | `005_api_keys.sql` |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| API key leak in logs | Low | Never log full key; only store hash |
| Conversation list slow with many rows | Low | Add DB indexes on `user_id`, `updated_at` |

## Rollback Plan

- Drop `api_keys` table
- Remove API key middleware from gateway
- Revert agent-service router changes

## Dependencies

- Existing auth infrastructure (JWT, bcrypt, Redis)
- Existing agent DB schema (conversations, messages)

## Success Criteria

- [ ] API key can be created and used to authenticate requests through gateway
- [ ] Conversations can be listed, renamed, archived, deleted via REST
- [ ] Messages can be listed paginated by conversation
- [ ] All new endpoints have unit tests

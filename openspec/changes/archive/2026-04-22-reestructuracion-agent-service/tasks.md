# Tasks: Reestructuración a Agent Service

## Phase 1: Foundation

- [x] 1.1 Rename `cmd/chat-service` → `cmd/agent-service`; update all imports
- [x] 1.2 Move `internal/shared/*` → `internal/platform/{config,logger,middleware}`
- [x] 1.3 Create `internal/platform/{postgres,redis,nats}` with connection helpers
- [x] 1.4 Create `pkg/llm/client.go` with `Client` interface and `Request`/`Chunk` types
- [x] 1.5 Create `pkg/llm/ollama.go` with `Stream()` using `http.Client` + `bufio.Scanner`
- [x] 1.6 Create `migrations/002_agent_platform.sql` with `agents`, `conversations`, `messages`, `artifacts`, `memories`, `tool_executions`
- [x] 1.7 Create `migrations/003_rls_policies.sql` with RLS policies for tenant isolation
- [x] 1.8 Consolidate to single root `go.mod`; remove `go.work` and per-cmd `go.mod` files

## Phase 2: Core Implementation

- [x] 2.1 Create `internal/agent/model/` — domain types (Conversation, Message, Agent, Artifact)
- [x] 2.2 Create `internal/agent/repository/` — repository interfaces
- [x] 2.3 Create `internal/agent/tools/registry.go` + `registry_test.go` — Tool interface + registry map
- [x] 2.4 Create `internal/agent/tools/file.go` + `file_test.go` — `file_write` persists to artifacts
- [x] 2.5 Create `internal/agent/tools/code.go` + `code_test.go` — `code_execute` via HTTP sandbox client
- [x] 2.6 Create `internal/agent/prompts/coder.go` + `conversational.go` + tests
- [x] 2.7 Create `internal/agent/runtime/classifier.go` + test — keyword-based intent classification
- [x] 2.8 Create `internal/agent/runtime/dispatcher.go` + test — builds LLM request with system prompt + tools
- [x] 2.9 Create `internal/agent/runtime/orchestrator.go` + test — full chat loop with tool call parsing
- [x] 2.10 Create `internal/agent/runtime/session.go` — conversation CRUD wrapper
- [x] 2.11 Create `internal/agent/store/postgres.go` — RLS-aware Postgres implementations of all repos
- [x] 2.12 Wire `cmd/agent-service/main.go` — Postgres/Redis/NATS/LLM/tools/orchestrator + routes
- [x] 2.13 Update `cmd/sandbox-service/main.go` — `/execute` endpoint accepting code + language

## Phase 3: Integration / Wiring

- [x] 3.1 Refactor `cmd/agent-service/main.go` to `Server` struct for testability
- [x] 3.2 Update `docker-compose.yml`: rename `chat-service` → `agent-service`, add `sandbox-service`
- [x] 3.3 Update `Makefile`: add `build-sandbox`, update `make test` and `make build`
- [x] 3.4 Update `api-gateway` proxy routes: `/api/v1/agent/*` → `agent-service:3002`

## Phase 4: Testing

- [x] 4.1 Unit test: `tool/file_write` with mocked repository — assert artifact created
- [x] 4.2 Unit test: `orchestrator.classify()` — assert "create html" → `coder`, "hello" → `conversational`
- [x] 4.3 Integration test: `POST /agent/chat` returns `Content-Type: text/event-stream` and chunks
- [x] 4.4 Integration test: `POST /artifacts` with auth creates DB row and returns 201
- [x] 4.5 Integration test: `GET /artifacts/:id/preview` returns artifact content
- [x] 4.6 Integration test: auth stubs reject requests without `X-Tenant-ID` / `X-User-ID`

## Phase 5: Cleanup

- [x] 5.1 Remove old stub handlers from renamed chat-service files (strings updated to agent-service)
- [x] 5.2 Update `README.md` with agent-service architecture diagram
- [x] 5.3 Verify `make build` compiles `agent-service` and `sandbox-service` without errors

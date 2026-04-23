# Design: Reestructuración a Agent Service

## Technical Approach

Pivotar `chat-service` a `agent-service` con un **Agent Runtime** interno (monolito modular). El runtime orquesta LLMs, tools, y sub-agentes (goroutines) dentro del mismo proceso. El `sandbox-service` corre separado por seguridad. Comunicación interna vía NATS JetStream. Multi-tenancy vía PostgreSQL RLS.

## Architecture Decisions

| Decision | Choice | Alternatives | Rationale |
|----------|--------|--------------|-----------|
| Agent Service architecture | Monolito modular (1 binario, paquetes internos) | 5 microservicios reales | Early stage: un binario reduce ops. Se puede partir luego sin reescribir dominio |
| Sub-agentes | Goroutines con channels | Procesos Docker separados | Liviano, baja latencia, comparten memoria fácil. El riesgo de crash se aísla con `recover()` |
| Comunicación interna | NATS JetStream | gRPC / HTTP REST | Ya está en docker-compose. Pub/sub natural para eventos de agentes. Desacopla agent de sandbox |
| HTML sandbox | Subdominio aislado + CSP | iframe mismo dominio | Previene XSS entre artefactos de distintos usuarios |
| Code sandbox | Docker containers | WASM / gVisor | Docker es portable y la infra ya existe. gVisor es más seguro pero más complejo (futuro) |
| LLM client | Interface unificada en `pkg/llm` | Cada provider separado | Permite swap de provider sin tocar el runtime |
| Multi-tenancy | Postgres RLS | Schema-per-tenant | RLS es estándar SaaS moderno. Una sola query, planner filtra automáticamente |

## Data Flow

```
Usuario → API Gateway (:3001)
              ↓ proxy
       Agent Service (:3002)
              ↓
    ┌─────────┴──────────┐
    ▼                    ▼
Orchestrator      SubAgent Spawner
    │                    │
    ▼                    ▼
LLM Client ←──→ Goroutines hijas
    │              (coder, writer)
    ▼                    │
Tool Registry ←──────────┘
    │
    ├── file_write ──► Artifact DB + Filesystem
    ├── code_execute ──► NATS ──► Sandbox Service (:3006)
    └── web_search ──► DuckDuckGo
```

## File Changes

| File | Action | Description |
|------|--------|-------------|
| `cmd/chat-service/` | Rename | → `cmd/agent-service/` |
| `cmd/agent-service/main.go` | Modify | Agregar rutas `/agent/chat`, `/artifacts`, `/health` |
| `internal/shared/` | Move | → `internal/platform/` (config, logger, middleware, postgres, redis, nats) |
| `internal/agent/runtime/orchestrator.go` | Create | Orquesta sesiones, enruta a LLM, maneja tool calls |
| `internal/agent/runtime/dispatcher.go` | Create | Envía prompts al LLM con system prompt por agent_type |
| `internal/agent/runtime/session.go` | Create | CRUD de agent_sessions en Postgres + contexto en Redis |
| `internal/agent/types/coder.go` | Create | System prompt y comportamiento del agente coder |
| `internal/agent/types/conversational.go` | Create | System prompt del chat puro |
| `internal/agent/tool/registry.go` | Create | Registro de tools con JSONSchema |
| `internal/agent/tool/file.go` | Create | Implementación de file_write |
| `internal/agent/tool/code.go` | Create | Implementación de code_execute (llama a sandbox vía NATS) |
| `internal/agent/subagent/spawner.go` | Create | Spawn de goroutines hijas con context + cancel |
| `internal/agent/subagent/communicator.go` | Create | Channels Inbox/Outbox padre-hijo |
| `internal/agent/memory/shortterm.go` | Create | Redis: context window |
| `internal/agent/memory/longterm.go` | Create | Postgres: preferencias del usuario |
| `cmd/sandbox-service/main.go` | Create | Servicio mínimo: recibe NATS event, ejecuta Docker, retorna URL |
| `pkg/llm/client.go` | Create | Interface unificada `Stream(ctx, req) (chan Chunk, error)` |
| `pkg/llm/ollama.go` | Create | Cliente Ollama con connection pooling |
| `pkg/llm/openai.go` | Create | Cliente OpenAI compatible |
| `migrations/001_init.sql` | Create | Schema base (users, tenants, conversations, messages) |
| `migrations/002_agent_platform.sql` | Create | Tablas nuevas (agent_sessions, artifacts, tool_calls, sandbox_executions) |
| `migrations/003_rls_policies.sql` | Create | Row Level Security policies |

## Interfaces / Contracts

```go
// pkg/llm/client.go
package llm

type Client interface {
    Stream(ctx context.Context, req Request) (chan Chunk, error)
    HealthCheck(ctx context.Context) error
}

type Request struct {
    Model       string
    Messages    []Message
    Temperature float64
    MaxTokens   int
    Tools       []ToolDefinition // Opcional
}

type Chunk struct {
    Content string
    Done    bool
    ToolCall *ToolCall // Si el LLM emite tool call
}

// internal/agent/tool/registry.go
package tool

type Tool interface {
    Name() string
    Description() string
    Schema() JSONSchema
    Execute(ctx context.Context, args map[string]any) (Result, error)
}

// internal/agent/subagent/spawner.go
package subagent

type SubAgent struct {
    ID     string
    Type   agent.Type
    Inbox  chan Task
    Outbox chan Result
    Cancel context.CancelFunc
}
```

## Testing Strategy

| Layer | What to Test | Approach |
|-------|-------------|----------|
| Unit | Orchestrator routing, tool execution, subagent lifecycle | Mock de llm.Client y repositories. Table-driven tests |
| Integration | HTTP handlers, DB queries con RLS | `httptest` + testcontainers PostgreSQL + Redis |
| E2E | Streaming SSE end-to-end | Script con `curl -N` o Playwright (futuro) |

## Migration / Rollout

No migration de datos (base vacía). Rollout estructural:
1. Detener `chat-service` container
2. Renombrar directorio, actualizar docker-compose.yml
3. Build + run `agent-service` y `sandbox-service`
4. Verificar health checks

## Open Questions

- [ ] ¿Sandbox service corre Docker-in-Docker o usa el Docker host del sistema?
- [ ] ¿El preview de HTML usa subdominio dinámico (ej: `{tenant}.sandbox.local`) o path prefix?
- [ ] ¿El LLM recibe tool definitions en formato OpenAI Functions o formato propio?

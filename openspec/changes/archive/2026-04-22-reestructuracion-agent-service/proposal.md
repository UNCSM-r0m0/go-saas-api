# Proposal: Reestructuración a Agent Service

## Intent

El proyecto actual tiene 5 microservicios stubs con handlers vacíos. La visión del producto es una **Agent Platform** tipo Kimi/Hermes: chat con LLMs + agentes especializados que generan artefactos (HTML, sitios web, código) con memoria persistente y sub-agentes paralelos.

Necesitamos pivotar `chat-service` a `agent-service` con un **Agent Runtime** real: orquestador, tool registry, sub-agent spawner, memory layer y skill registry.

## Scope

### In Scope
- Renombrar `cmd/chat-service` → `cmd/agent-service`
- Reestructurar `internal/shared` → `internal/platform`
- Crear `internal/agent/{runtime,types,memory,skill,tool,subagent}`
- Crear `cmd/sandbox-service` básico (ejecución segura)
- Agregar tablas: `agent_sessions`, `artifacts`, `tool_calls`, `sandbox_executions`
- Implementar `Agent Orchestrator` + `Tool Registry` (file_write, code_execute)
- Implementar `SubAgent Spawner` con goroutines
- Conectar streaming SSE a LLMs reales (Ollama/OpenAI)

### Out of Scope
- Learning loop auto-generador de skills (Fase 3)
- Multi-channel (WhatsApp, Telegram, etc.)
- Voice / Canvas surface
- Kubernetes / producción
- CI/CD pipelines

## Capabilities

### New Capabilities
- `agent-runtime`: Orquestador, dispatcher, gestión de sesiones
- `agent-subagent`: Spawning de sub-agentes paralelos, comunicación padre-hijo, merge de resultados
- `agent-memory`: Short-term (Redis), long-term (Postgres), episodic search (FTS5)
- `agent-skills`: Registry y ejecución de skills (framework, no learning loop)
- `agent-tools`: Tool registry con file_write, code_execute, web_search
- `sandbox-service`: Ejecución aislada de código generado
- `artifact-storage`: CRUD y serving de artefactos generados

### Modified Capabilities
- `chat-streaming`: Pivotar de chat simple a streaming con tool calls integrado
- `api-gateway`: Agregar routing a artifact preview y sandbox-service

## Approach

1. **Monolito modular interno**: `agent-service` alberga el runtime + tipos de agentes + tools + memory + subagent. Los demás servicios (`auth`, `billing`, `usage`) se mantienen separados.
2. **Sub-agentes como goroutines**: Cada sub-agente es una goroutine con canales `Inbox`/`Outbox`, cancelable por `context.Context`.
3. **NATS JetStream** como backbone de eventos entre agent-service y sandbox-service.
4. **Postgres + Redis**: RLS para multi-tenancy, Redis para short-term memory y rate limiting.
5. **Fases**: Fase 1 = MVP coder (genera HTML). Fase 2 = sub-agentes + memoria. Fase 3 = learning loop.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `cmd/chat-service` | Renamed | → `cmd/agent-service/` |
| `internal/shared` | Moved | → `internal/platform/` |
| `internal/agent/*` | New | Runtime, types, memory, skills, tools, subagent |
| `cmd/sandbox-service` | New | Servicio de ejecución segura |
| `migrations/` | New | Tablas agent_sessions, artifacts, tool_calls, sandbox_executions |
| `pkg/llm` | New | Clientes unificados para LLMs |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Sandbox escape (seguridad) | Med | Docker containers con seccomp, no root, network restricted |
| Complejidad del orchestrator | Med | Empezar con 1 agente (coder), agregar tipos gradualmente |
| LLM latency en streaming | Med | Connection pooling, timeout configurable, fallback providers |
| Breaking cambio de estructura | Low | Mantener `internal/shared` funcional durante transición |

## Rollback Plan

1. `git checkout` del commit previo a la reestructuración
2. Los servicios `auth`, `billing`, `usage` no se tocan → siguen funcionando
3. `api-gateway` puede rutear a servicios viejos si existieran

## Dependencies

- Ollama corriendo local o Docker para testing
- golangci-lint para CI local
- Postman/curl para testing manual

## Success Criteria

- [ ] `make build` compila `agent-service` y `sandbox-service`
- [ ] `POST /agent/chat` con prompt "hacé un contador en HTML" retorna streaming + artifact guardado
- [ ] `GET /artifacts/{id}` sirve el HTML generado
- [ ] Sub-agent spawner crea goroutines hijas sin panic
- [ ] Tool `file_write` persiste en DB + filesystem

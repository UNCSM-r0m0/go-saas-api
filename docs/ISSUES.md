# GitHub Issues — go-saas-api

> Repo: `R0LM0/go-saas-api` (o el nombre correcto)
> Total issues: 26 organizadas en 6 milestones

---

## Milestone 1: Fundamentos (Sprint 1-2)

### #1 — [M1] Setup workspace + Go modules + Docker Compose fixes
**Labels:** `milestone-1`, `infrastructure`, `setup`

#### Tareas
- [ ] Reorganizar estructura de directorios (clean architecture)
- [ ] Crear `go.work` para workspace Go 1.24+
- [ ] Agregar `chat-service` y `billing-service` al docker-compose.yml
- [ ] Fix URLs malformadas en docker-compose (REDIS_URL, DATABASE_URL)
- [ ] Eliminar código experimental: `engram/`, `engram-memory/`, `enram-memory/`, `.claude/`, `.gentle-ai/`
- [ ] Crear Makefile con comandos: `dev-up`, `services-up`, `migrate`, `seed`
- [ ] Actualizar README.md con nueva estructura

#### Criterios de aceptación
- `docker-compose up` levanta toda la infraestructura sin errores
- Todos los servicios tienen Dockerfile funcional
- `make dev-up` funciona con un comando

---

### #2 — [M1] Implementar schema SQL completo + migrations
**Labels:** `milestone-1`, `database`, `schema`

#### Tareas
- [ ] Crear schema SQL completo basado en Prisma schema del NestJS
- [ ] Implementar migrations con golang-migrate o similar
- [ ] Tablas: users, tenants, subscriptions, conversations, messages, usage_records
- [ ] Tablas nuevas: ai_providers, ai_models, api_keys
- [ ] Índices optimizados para queries frecuentes
- [ ] Seed script con datos iniciales (providers Ollama, OpenAI, etc.)

#### Schema a migrar desde NestJS/Prisma
- users (con multitenancy)
- tenants
- subscriptions (Stripe integration)
- conversations
- messages
- usage_records (rate limiting)

#### Schema nuevo (escalable)
- ai_providers: providers configurables en BD, no .env
- ai_models: catálogo de modelos por provider
- api_keys: API keys para usuarios (API pública)

#### Criterios de aceptación
- `make migrate` aplica todas las migrations
- `make seed` inserta providers y modelos de ejemplo
- Schema soporta multitenancy y rate limiting

---

### #3 — [M1] Crear `internal/shared` con logger, config, errors
**Labels:** `milestone-1`, `shared`, `infrastructure`

#### Tareas
- [ ] Logger estructurado con Zap (JSON para prod, console para dev)
- [ ] Config loader (Viper o envconfig): soporta .env + variables de entorno
- [ ] Error handling centralizado (custom errors con códigos HTTP)
- [ ] Middleware compartido: request ID, logging, recovery (panic)
- [ ] Validación de inputs (go-playground/validator)
- [ ] Utilidades: JWT, password hashing, UUID

#### Estructura esperada
```
internal/shared/
├── logger/         # Zap logger
├── config/         # Config loader
├── errors/         # Custom errors
├── middleware/     # Gin middlewares
├── validator/      # Input validation
└── utils/          # JWT, crypto, etc.
```

#### Criterios de aceptación
- Todos los servicios pueden importar `internal/shared`
- Logger incluye request ID en cada log
- Config se carga desde .env con defaults
- Middleware recovery captura panics sin crashear el servidor

---

### #4 — [M1] Implementar API Gateway real (reverse proxy)
**Labels:** `milestone-1`, `api-gateway`, `proxy`

#### Tareas
- [ ] Reemplazar stubs por reverse proxy real
- [ ] Routing dinámico basado en path: `/api/v1/chat/*` → chat-service:3002
- [ ] Auth middleware: validar JWT en gateway (no en cada servicio)
- [ ] Rate limiting por IP + por user (Redis)
- [ ] Health check agregado: `/health` consulta todos los servicios
- [ ] Request ID propagation (header X-Request-ID)
- [ ] CORS configurado

#### Endpoints a rutear
| Path | Servicio | Puerto |
|------|----------|--------|
| /api/v1/auth/* | auth-service | 3003 |
| /api/v1/chat/* | chat-service | 3002 |
| /api/v1/billing/* | billing-service | 3004 |
| /api/v1/usage/* | usage-service | 3005 |

#### Criterios de aceptación
- `curl http://localhost:3001/api/v1/chat/health` → proxy a chat-service:3002/health
- JWT inválido → 401 antes de llegar al servicio
- Rate limit excedido → 429 con headers Retry-After
- Request ID se propaga a todos los servicios

---

## Milestone 2: Core Services (Sprint 3-4)

### #5 — [M2] Implementar Auth Service (JWT + OAuth Google/GitHub)
**Labels:** `milestone-2`, `auth`, `jwt`, `oauth`

#### Tareas
- [ ] Register: POST /auth/register (email + password)
- [ ] Login: POST /auth/login (email + password) → JWT + refresh token
- [ ] Refresh: POST /auth/refresh → nuevo access token
- [ ] OAuth Google: GET /auth/google → redirect → callback → JWT
- [ ] OAuth GitHub: GET /auth/github → redirect → callback → JWT
- [ ] Logout: POST /auth/logout → invalidar refresh token (Redis blacklist)
- [ ] Me: GET /auth/me → datos del usuario autenticado
- [ ] Password reset flow

#### Modelos
- User (con tenant_id para multitenancy)
- RefreshToken (almacenado en Redis)
- Password reset tokens

#### Criterios de aceptación
- Register crea usuario con password hasheado (bcrypt)
- Login retorna access token (JWT, 15min) + refresh token (7 días, Redis)
- OAuth funciona con Google y GitHub
- Refresh token rotation (nuevo refresh token en cada uso)
- Logout invalida refresh token inmediatamente

---

### #6 — [M2] Implementar Chat Service con SSE streaming real
**Labels:** `milestone-2`, `chat`, `sse`, `streaming`

#### Tareas
- [ ] POST /chat/completions → respuesta síncrona
- [ ] POST /chat/stream → SSE streaming con datos reales
- [ ] GET /chat/models → listar modelos desde BD (no hardcodeado)
- [ ] GET /chat/history/:conversationId → historial paginado
- [ ] POST /chat/conversations → crear conversación
- [ ] Guardar mensajes en PostgreSQL (solo usuarios registrados)
- [ ] Tracking de tokens usados

#### Streaming SSE
```go
c.Header("Content-Type", "text/event-stream")
c.Header("Cache-Control", "no-cache")
c.Header("Connection", "keep-alive")
```

#### Criterios de aceptación
- Streaming funciona con provider real (Ollama)
- Chunks se envían en tiempo real (< 100ms entre chunks)
- Mensajes se guardan en BD con tokens_used
- Modelos se leen desde tabla ai_models
- Usuarios anónimos pueden chatear sin guardar historial

---

### #7 — [M2] Implementar provider Ollama con Go routines + chunks
**Labels:** `milestone-2`, `provider`, `ollama`, `goroutines`

#### Tareas
- [ ] Cliente HTTP para Ollama con connection pooling
- [ ] Endpoint /api/generate con streaming
- [ ] Parsear chunks SSE de Ollama
- [ ] Go routine por request para no bloquear
- [ ] Cancelación graceful (context.Context)
- [ ] Timeout configurable
- [ ] Retry con backoff para fallos de conexión

#### Configuración en BD (no .env)
```sql
INSERT INTO ai_providers (name, type, base_url, is_active) 
VALUES ('Ollama Local', 'ollama', 'http://host.docker.internal:11434', true);
```

#### Criterios de aceptación
- Conexión a Ollama funciona desde Docker
- Streaming de chunks sin buffering
- Cancelación de request corta la conexión con Ollama
- Retry automático si Ollama no responde
- Métricas: tiempo de respuesta, tokens/segundo

---

### #8 — [M2] Implementar provider OpenAI con streaming
**Labels:** `milestone-2`, `provider`, `openai`, `streaming`

#### Tareas
- [ ] Cliente HTTP para OpenAI API
- [ ] Soportar chat.completions con streaming
- [ ] Parsear formato SSE de OpenAI
- [ ] Manejar errores de rate limit (429) y creditos agotados
- [ ] Configurar modelos: gpt-4, gpt-4-turbo, gpt-3.5-turbo

#### Configuración en BD
```sql
INSERT INTO ai_providers (name, type, base_url, api_key_encrypted, is_active)
VALUES ('OpenAI', 'openai', 'https://api.openai.com', '<encrypted_key>', true);
```

#### Criterios de aceptación
- Streaming funciona con API key real
- Maneja errores 429 con retry exponencial
- Soporta todos los modelos listados en ai_models
- Fallback a otro provider si OpenAI falla

---

## Milestone 3: Providers Escalables (Sprint 5)

### #9 — [M3] Crear tablas ai_providers + ai_models
**Labels:** `milestone-3`, `database`, `providers`, `scalable`

#### Tareas
- [ ] Migration: crear tabla `ai_providers`
- [ ] Migration: crear tabla `ai_models`
- [ ] CRUD API para providers (admin only)
- [ ] CRUD API para models (admin only)
- [ ] Encriptación AES-256 para API keys
- [ ] Sistema de prioridad para fallback

#### Schema ai_providers
```sql
CREATE TABLE ai_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(100) NOT NULL,
    type provider_type NOT NULL,
    base_url TEXT NOT NULL,
    api_key_encrypted TEXT,
    api_key_hash VARCHAR(64),
    is_active BOOLEAN DEFAULT true,
    is_public BOOLEAN DEFAULT true,
    priority INT DEFAULT 0,
    config JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

#### Schema ai_models
```sql
CREATE TABLE ai_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID REFERENCES ai_providers(id),
    name VARCHAR(100) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    max_tokens INT DEFAULT 4096,
    context_window INT DEFAULT 8192,
    supports_streaming BOOLEAN DEFAULT true,
    supports_images BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    is_public BOOLEAN DEFAULT true,
    is_premium BOOLEAN DEFAULT false,
    config JSONB DEFAULT '{}'
);
```

#### Criterios de aceptación
- Admin puede agregar nuevo provider sin deploy
- API keys se encriptan en BD (no plaintext)
- Modelos se filtran por tier del usuario (free vs premium)
- GET /chat/models retorna solo modelos visibles para el usuario

---

### #10 — [M3] Implementar CRUD de providers (admin API)
**Labels:** `milestone-3`, `admin`, `api`, `providers`

#### Tareas
- [ ] POST /admin/providers → crear provider
- [ ] GET /admin/providers → listar todos
- [ ] GET /admin/providers/:id → detalle
- [ ] PATCH /admin/providers/:id → actualizar
- [ ] DELETE /admin/providers/:id → desactivar (soft delete)
- [ ] POST /admin/providers/:id/test → testear conexión
- [ ] Validar que solo super_admin puede acceder

#### Criterios de aceptación
- Crear provider con API key encriptada
- Test de conexión verifica que el provider responde
- Soft delete (is_active = false) en vez de borrar
- Audit log de cambios en providers

---

### #11 — [M3] Implementar provider Gemini
**Labels:** `milestone-3`, `provider`, `gemini`

#### Tareas
- [ ] Cliente HTTP para Google Gemini API
- [ ] Soportar streaming
- [ ] Configurar modelos: gemini-pro, gemini-pro-vision
- [ ] Manejar autenticación con API key

#### Criterios de aceptación
- Streaming funciona con Gemini
- Soporta imágenes (gemini-pro-vision)
- Integrado en el sistema de fallback

---

### #12 — [M3] Implementar provider DeepSeek
**Labels:** `milestone-3`, `provider`, `deepseek`

#### Tareas
- [ ] Cliente HTTP para DeepSeek API
- [ ] Soportar streaming
- [ ] Configurar modelos: deepseek-chat, deepseek-coder
- [ ] Manejar autenticación con API key

#### Criterios de aceptación
- Streaming funciona con DeepSeek
- Integrado en el sistema de fallback

---

### #13 — [M3] Sistema de fallback entre providers
**Labels:** `milestone-3`, `providers`, `fallback`, `resilience`

#### Tareas
- [ ] Ordenar providers por `priority` en BD
- [ ] Si provider A falla, intentar provider B automáticamente
- [ ] Notificar al usuario cuando hay fallback
- [ ] Circuit breaker: desactivar provider tras N fallos consecutivos
- [ ] Health check periódico de providers

#### Lógica de fallback
```
1. Usuario pide modelo X
2. Buscar provider que soporte X con mayor prioridad
3. Intentar provider 1
4. Si falla → intentar provider 2 (siguiente prioridad)
5. Si todos fallan → error 503 con mensaje claro
```

#### Criterios de aceptación
- Fallback automático sin intervención del usuario
- Circuit breaker evita reintentar provider caído
- Health check cada 30 segundos

---

## Milestone 4: Billing & Usage (Sprint 6)

### #14 — [M4] Migrar lógica Stripe desde NestJS
**Labels:** `milestone-4`, `billing`, `stripe`, `migration`

#### Tareas
- [ ] Integrar stripe-go SDK
- [ ] Crear customer en Stripe al registrar usuario
- [ ] GET /billing/plans → listar planes (free, registered, premium)
- [ ] POST /billing/subscribe → crear suscripción Stripe
- [ ] GET /billing/subscription → estado actual del usuario
- [ ] POST /billing/cancel → cancelar suscripción
- [ ] Migrar lógica desde `saas-backend-original/src/stripe/`

#### Datos a migrar desde NestJS
- Planes: FREE (3 msgs), REGISTERED (50 msgs), PREMIUM (1000 msgs)
- Stripe customer ID, subscription ID, price ID
- Webhook handling

#### Criterios de aceptación
- Usuario puede ver planes disponibles
- Suscripción premium se crea en Stripe
- Customer ID se guarda en tabla subscriptions
- Cancelación actualiza estado en BD y Stripe

---

### #15 — [M4] Implementar Usage Service con Redis
**Labels:** `milestone-4`, `usage`, `redis`, `rate-limiting`

#### Tareas
- [ ] Tracking de mensajes por día (user_id + date)
- [ ] Tracking de tokens usados
- [ ] Sliding window rate limiter con Redis Sorted Sets
- [ ] GET /usage/stats → estadísticas del usuario
- [ ] GET /usage/limits → límites actuales y uso

#### Rate limits por tier
| Tier | Mensajes/día | Tokens/mes |
|------|-------------|-----------|
| FREE | 3 | - |
| REGISTERED | 50 | - |
| PREMIUM | 1000 | - |

#### Redis keys
```
usage:daily:{user_id}:{YYYY-MM-DD} → hash {messages, tokens}
rate_limit:{user_id} → sorted set (timestamps)
```

#### Criterios de aceptación
- Rate limit funciona con Redis (no depende de PostgreSQL)
- Sliding window (no reset diario brusco)
- Stats incluyen uso de hoy, esta semana, este mes
- Usuario anónimo se trackea por fingerprint/IP

---

### #16 — [M4] Webhooks Stripe (subscriptions)
**Labels:** `milestone-4`, `stripe`, `webhooks`, `billing`

#### Tareas
- [ ] POST /billing/webhook → endpoint para Stripe
- [ ] Verificar firma del webhook (`stripe-webhook-secret`)
- [ ] Eventos a manejar:
  - `invoice.payment_succeeded` → actualizar periodo
  - `invoice.payment_failed` → marcar como past_due
  - `customer.subscription.deleted` → downgradear a REGISTERED
  - `customer.subscription.updated` → sincronizar estado

#### Criterios de aceptación
- Webhook verifica firma de Stripe
- Suscripción cancelada en Stripe → downgrade automático en BD
- Pago fallido → notificar usuario + grace period
- Idempotencia: mismo evento procesado 2x no causa duplicados

---

### #17 — [M4] Rate limiting por tier (free/registered/premium)
**Labels:** `milestone-4`, `rate-limiting`, `tier`, `middleware`

#### Tareas
- [ ] Middleware de rate limiting en API Gateway
- [ ] Diferentes límites por tier:
  - FREE: 3 mensajes/día, no streaming
  - REGISTERED: 50 mensajes/día, streaming
  - PREMIUM: 1000 mensajes/día, streaming + imágenes
- [ ] Headers de rate limit en respuestas:
  - `X-RateLimit-Limit`
  - `X-RateLimit-Remaining`
  - `X-RateLimit-Reset`
- [ ] Mensaje amigable cuando se alcanza límite

#### Criterios de aceptación
- Usuario free ve mensaje: "Regístrate para más mensajes"
- Usuario registered ve: "Actualiza a Premium"
- Headers presentes en todas las respuestas
- Rate limit no afecta health checks ni auth

---

## Milestone 5: Optimización (Sprint 7)

### #18 — [M5] WebSocket support para streaming bidireccional
**Labels:** `milestone-5`, `websocket`, `streaming`, `performance`

#### Tareas
- [ ] Upgrade de conexión HTTP a WebSocket
- [ ] Protocolo de mensajes: JSON con type (chat, ping, stop)
- [ ] Go routine por conexión WebSocket
- [ ] Broadcast a múltiples clientes del mismo usuario
- [ ] Graceful close con código de cierre

#### Mensajes WebSocket
```json
{"type": "chat", "model": "gpt-4", "messages": [{"role": "user", "content": "Hola"}]}
{"type": "chunk", "content": "Hola", "message_id": "uuid"}
{"type": "done", "message_id": "uuid", "tokens_used": 42}
{"type": "stop"}  // Usuario canceló
```

#### Criterios de aceptación
- Cliente puede enviar mensaje vía WebSocket
- Respuesta streaming en tiempo real
- Usuario puede cancelar generación en curso
- Reconexión automática con resume de conversación

---

### #19 — [M5] Connection pooling HTTP para providers
**Labels:** `milestone-5`, `performance`, `http`, `pooling`

#### Tareas
- [ ] http.Transport compartido por provider
- [ ] MaxIdleConns: 100, MaxIdleConnsPerHost: 10
- [ ] IdleConnTimeout: 90s
- [ ] HTTP/2 support donde esté disponible
- [ ] Métricas de pool: conexiones activas, esperando

#### Criterios de aceptación
- 100 conexiones concurrentes sin crear nuevas TCP
- Métricas exportadas en `/metrics` (Prometheus)
- Timeout configurable por provider

---

### #20 — [M5] Caché de modelos/config en Redis
**Labels:** `milestone-5`, `cache`, `redis`, `performance`

#### Tareas
- [ ] Cachear lista de modelos en Redis (TTL: 5 min)
- [ ] Cachear config de providers (TTL: 1 min)
- [ ] Invalidación manual vía admin API
- [ ] Cachear rate limit status (TTL: 1 min)

#### Redis keys
```
cache:models:{tier} → JSON array de modelos visibles
cache:providers:active → JSON array de providers activos
cache:usage:{user_id} → hash con uso actual
```

#### Criterios de aceptación
- GET /chat/models responde < 10ms (cache hit)
- Cambio en provider se refleja en < 1 minuto
- Cache miss no causa error (fallback a BD)

---

### #21 — [M5] Health checks + graceful shutdown
**Labels:** `milestone-5`, `health`, `shutdown`, `ops`

#### Tareas
- [ ] /health → estado del servicio + dependencias
- [ ] /health/live → liveness probe (siempre 200 si corriendo)
- [ ] /health/ready → readiness probe (200 si BD y Redis OK)
- [ ] Graceful shutdown: stop accepting, drain connections, close resources
- [ ] Timeout de shutdown: 30s

#### Health check de dependencias
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "dependencies": {
    "postgres": "connected",
    "redis": "connected",
    "nats": "connected"
  }
}
```

#### Criterios de aceptación
- Kubernetes puede usar /health/ready para readiness probe
- SIGTERM inicia shutdown graceful
- Requests en curso terminan antes de cerrar
- Timeout de 30s, luego SIGKILL

---

### #22 — [M5] Tests unitarios + integración
**Labels:** `milestone-5`, `testing`, `ci`, `quality`

#### Tareas
- [ ] Tests unitarios para usecases (mock de repositories)
- [ ] Tests de integración para handlers (httptest + test DB)
- [ ] Tests para providers (mock HTTP server)
- [ ] Tests para rate limiting (testcontainers Redis)
- [ ] Coverage > 80% en lógica de negocio
- [ ] CI: GitHub Actions con test + lint + build

#### Criterios de aceptación
- `go test ./...` pasa en local
- CI corre en cada PR
- Coverage report en PRs
- Tests de integración con PostgreSQL en Docker

---

## Milestone 6: Producción (Sprint 8)

### #23 — [M6] Docker Compose producción
**Labels:** `milestone-6`, `docker`, `production`, `deployment`

#### Tareas
- [ ] docker-compose.prod.yml sin volúmenes de desarrollo
- [ ] Multi-stage Dockerfile (builder → runtime)
- [ ] Imágenes basadas en distroless o alpine
- [ ] Secrets via Docker secrets o env files
- [ ] Reverse proxy: Traefik o nginx
- [ ] SSL automático (Let's Encrypt)

#### Criterios de aceptación
- `docker-compose -f docker-compose.prod.yml up` funciona
- Imágenes < 50MB cada una
- No expone puertos de infraestructura al exterior
- SSL configurado automáticamente

---

### #24 — [M6] Kubernetes manifests
**Labels:** `milestone-6`, `kubernetes`, `k8s`, `deployment`

#### Tareas
- [ ] Deployment por servicio
- [ ] Service + ClusterIP
- [ ] ConfigMap para variables no sensibles
- [ ] Secret para API keys, JWT secret, Stripe keys
- [ ] HPA (Horizontal Pod Autoscaler) por CPU/memory
- [ ] Ingress con nginx-controller
- [ ] Cert-manager para SSL

#### Criterios de aceptación
- `kubectl apply -f k8s/` despliega todo
- Pods se escalan automáticamente
- Rolling update sin downtime
- SSL funciona en dominio propio

---

### #25 — [M6] Observabilidad (metrics, tracing)
**Labels:** `milestone-6`, `observability`, `metrics`, `tracing`

#### Tareas
- [ ] Métricas Prometheus: /metrics
  - requests_total, request_duration_seconds
  - active_connections, provider_errors_total
  - rate_limit_hits_total
- [ ] Tracing con OpenTelemetry + Jaeger
- [ ] Logging estructurado con trace_id
- [ ] Alertas: error rate > 5%, p95 latency > 2s

#### Criterios de aceptación
- Dashboard Grafana con métricas clave
- Traces visibles en Jaeger
- Logs correlacionados por trace_id
- Alertas envían a Telegram/Discord

---

### #26 — [M6] Documentación API (OpenAPI/Swagger)
**Labels:** `milestone-6`, `documentation`, `openapi`, `swagger`

#### Tareas
- [ ] Generar OpenAPI spec desde código (swaggo/swag)
- [ ] Endpoint /docs → Swagger UI
- [ ] Documentar todos los endpoints
- [ ] Ejemplos de request/response
- [ ] Documentar errores posibles

#### Criterios de aceptación
- /docs muestra Swagger UI funcional
- Todos los endpoints públicos documentados
- Ejecutar requests desde Swagger UI
- Exportar spec como JSON/YAML

---

*Issues generados: Abril 2026 | Total: 26 issues en 6 milestones*

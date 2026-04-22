# Go SaaS API

Backend SaaS multi-tenant para chat AI — Microservicios en Go 1.24.

## 🏗️ Arquitectura

```
┌─────────────────────────────────────────────────────────┐
│                    API Gateway :3001                     │
│              (Routing, Auth, Rate Limiting)              │
└─────────────────────────────────────────────────────────┘
                            │
        ┌───────────────────┼───────────────────┐
        │                   │                   │
        ▼                   ▼                   ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│ Chat :3002   │  │ Auth :3003   │  │ Billing :3004│
│ Streaming    │  │ JWT + OAuth  │  │ Stripe       │
│ Providers    │  │ Users        │  │ Subscriptions│
└──────────────┘  └──────────────┘  └──────────────┘
                            │
                    ┌───────┴───────┐
                    │ Usage :3005   │
                    │ Tracking      │
                    │ Rate Limits   │
                    └───────────────┘
```

## 🚀 Quick Start

```bash
# 1. Clonar
cd ~/Workspace/GO/go-saas-api

# 2. Configurar
cp .env.example .env
# Editar .env con tus credenciales

# 3. Levantar infraestructura
make dev-up          # Postgres + Redis + NATS

# 4. Levantar servicios
make services-up     # Todos los microservicios

# 5. Verificar
make health
```

## 📦 Stack

| Capa | Tecnología |
|------|-----------|
| Lenguaje | Go 1.24 |
| HTTP Framework | Gin |
| Base de Datos | PostgreSQL 16 |
| Cache | Redis 7 |
| Mensajería | NATS 2.10 |
| Auth | JWT + OAuth 2.0 |
| Pagos | Stripe |
| Container | Docker + Docker Compose |

## 📁 Estructura

```
cmd/                    # Entry points (main.go por servicio)
internal/
  shared/               # Código compartido (logger, config, middleware)
  api-gateway/          # API Gateway
  chat/                 # Chat Service
  auth/                 # Auth Service
  billing/              # Billing Service
  usage/                # Usage Service
pkg/                    # Librerías compartidas
migrations/             # SQL migrations
deployments/
  docker/               # Dockerfiles
  k8s/                  # Kubernetes manifests (futuro)
scripts/                # Scripts de utilidad
```

## 🛠️ Comandos

```bash
make help          # Ver todos los comandos
make up            # Levantar todo
make down          # Detener todo
make build         # Compilar binarios
make test          # Ejecutar tests
make lint          # Ejecutar linter
make migrate       # Aplicar migrations
make seed          # Insertar datos de ejemplo
make health        # Verificar salud de servicios
```

## 🔧 Variables de Entorno

Ver `.env.example` para todas las opciones.

| Variable | Descripción | Default |
|----------|-------------|---------|
| `PORT` | Puerto del servicio | 3001-3005 |
| `DATABASE_URL` | PostgreSQL connection string | - |
| `REDIS_URL` | Redis connection string | - |
| `NATS_URL` | NATS connection string | - |
| `JWT_SECRET` | Secret para JWT | - |
| `STRIPE_SECRET_KEY` | Stripe API key | - |
| `OLLAMA_URL` | Ollama endpoint | http://localhost:11434 |

## 📚 API Endpoints

### API Gateway (3001)
- `GET /health` — Health check
- `/api/v1/auth/*` → Auth Service
- `/api/v1/chat/*` → Chat Service
- `/api/v1/billing/*` → Billing Service
- `/api/v1/usage/*` → Usage Service

### Chat Service (3002)
- `GET /chat/models` — Listar modelos
- `POST /chat/completions` — Chat síncrono
- `POST /chat/stream` — Streaming SSE
- `GET /chat/history/:id` — Historial

### Auth Service (3003)
- `POST /auth/register` — Registro
- `POST /auth/login` — Login
- `POST /auth/refresh` — Refresh token
- `GET /auth/me` — Perfil

## 📝 Roadmap

Ver [PLAN.md](PLAN.md) para el plan completo de desarrollo.

## 📄 Licencia

MIT — R0LM0

# Go SaaS API - Microservicios

Arquitectura de microservicios en Go inspirada en Kimi (Moonshot AI) y Gentle-AI.

## 🚀 Estructura del Proyecto

```
go-saas-api/
├── api-gateway/          # API Gateway (Puerto 3001)
├── chat-service/         # Chat AI con Streaming SSE (Puerto 3002)
├── auth-service/         # Auth JWT + OAuth (Puerto 3003)
├── billing-service/      # Stripe Payments (Puerto 3004)
├── usage-service/        # Tracking de uso (Puerto 3005)
├── docker-compose.yml    # Orquestación completa
└── README.md
```

## 📦 Servicios

| Servicio | Puerto | Tecnología | Descripción |
|----------|--------|------------|-------------|
| API Gateway | 3001 | Go + Gin | Entry point, routing |
| Chat Service | 3002 | Go + SSE | Streaming con Ollama |
| Auth Service | 3003 | Go + JWT | Autenticación |
| Billing Service | 3004 | Go + Stripe | Pagos |
| Usage Service | 3005 | Go | Tracking |

## 🛠️ Stack

- **Go 1.24** - Lenguaje principal
- **Gin** - Framework web
- **NATS** - Mensajería
- **PostgreSQL** - Base de datos
- **Redis** - Cache
- **Docker Compose** - Orquestación

## 🚀 Quick Start

### 1. Configurar variables de entorno

```bash
cp .env.example .env
# Editar .env con tus credenciales
```

### 2. Iniciar con Docker Compose

```bash
docker-compose up --build
```

### 3. Probar endpoints

```bash
# Health check
curl http://localhost:3001/health

# Chat stream (SSE)
curl -X POST http://localhost:3001/api/v1/chat/stream \
  -H "Content-Type: application/json" \
  -d '{"model":"qwen2.5-coder:7b","messages":[{"role":"user","content":"Hello"}]}'
```

## 📚 Endpoints

### API Gateway (Puerto 3001)

- `GET /health` - Health check
- `POST /api/v1/chat/stream` - Chat streaming SSE
- `POST /api/v1/chat/completions` - Chat normal
- `GET /api/v1/chat/models` - Listar modelos

## 🎯 Características

✅ **Streaming SSE** - Respuestas en tiempo real  
✅ **Microservicios** - Escalables independientemente  
✅ **Docker Compose** - Fácil deployment  
✅ **NATS** - Mensajería entre servicios  
✅ **PostgreSQL + Redis** - Persistencia y cache  

## 🔧 Desarrollo

```bash
# Iniciar solo infraestructura
docker-compose up -d postgres redis nats

# Desarrollar un servicio local
cd chat-service
go run cmd/main.go
```

## 📝 TODO

- [ ] Implementar Auth Service completo (JWT + OAuth)
- [ ] Implementar Billing Service (Stripe)
- [ ] Implementar Usage Service
- [ ] Agregar middleware de rate limiting
- [ ] Agregar autenticación en API Gateway
- [ ] Tests unitarios
- [ ] CI/CD pipeline

## 📄 Licencia

MIT - R0LM0

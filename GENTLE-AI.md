# 🎩 Gentle-AI Ecosystem - Configuration

Ecosistema completo de Gentleman Programming adaptado para go-saas-api.

## 📋 Componentes del Ecosistema

### 1. **Engram** - Memoria Persistente
Sistema de memoria para agentes AI con SQLite + FTS5.

### 2. **MCP Servers** - Model Context Protocol
Servidores para extender capacidades del agente AI.

### 3. **Skills** - Habilidades de Código
Skills curadas desde skills.sh para Go y microservicios.

### 4. **SDD Workflow** - Spec-Driven Development
Flujo de trabajo orientado a especificaciones.

### 5. **AI Provider Switcher** - Múltiples Modelos
Soporte para múltiples proveedores de IA.

### 6. **Persona** - Teaching-Oriented
Persona enfocada en enseñar mientras desarrolla.

---

## 🗂️ Estructura del Ecosistema

```
.claude/
├── skills/                    # Skills instalados
│   ├── go-microservices/
│   ├── gin-api-gateway/
│   ├── go-sse-streaming/
│   └── sdd-workflow/
├── engram/                    # Configuración de memoria
│   ├── config.toml
│   └── queries/
├── mcp-servers/              # Servidores MCP
│   ├── filesystem-server/
│   ├── postgres-server/
│   └── git-server/
└── persona.md                # Definición de persona

.gentle-ai/                   # Configuración principal
├── config.yaml
├── workflows/
│   ├── sdd-spec.yaml
│   ├── sdd-arch.yaml
│   ├── sdd-code.yaml
│   ├── sdd-test.yaml
│   └── sdd-deploy.yaml
└── providers/
    ├── kimi.yaml
    ├── openai.yaml
    └── anthropic.yaml
```

---

## 🚀 Quick Start

### 1. Instalar Engram (Memoria)
```bash
# Descargar binario
curl -sSL https://github.com/Gentleman-Programming/engram/releases/latest/download/engram-linux-amd64 -o /usr/local/bin/engram
chmod +x /usr/local/bin/engram

# Iniciar servidor MCP
engram mcp --transport stdio
```

### 2. Configurar MCP Servers
```bash
# Filesystem MCP
npx @modelcontextprotocol/server-filesystem ~/go-saas-api

# PostgreSQL MCP  
npx @modelcontextprotocol/server-postgres postgresql://localhost/saas_db

# Git MCP
npx @modelcontextprotocol/server-git ~/go-saas-api
```

### 3. Iniciar Workflow SDD
```bash
# Fase 1: Specification
claude --workflow .gentle-ai/workflows/sdd-spec.yaml

# Fase 2: Architecture
claude --workflow .gentle-ai/workflows/sdd-arch.yaml

# Fase 3: Implementation
claude --workflow .gentle-ai/workflows/sdd-code.yaml
```

---

## 🧠 Engram - Memoria Persistente

### Configuración
```toml
# ~/.config/engram/config.toml
[database]
path = "~/.local/share/engram/memories.db"
max_size = "1GB"

[search]
fts5_enabled = true
vector_search = false

[mcp]
enabled = true
transport = "stdio"

[memory]
auto_save = true
context_window = 50
similarity_threshold = 0.85
```

### Queries Comunes
```sql
-- Buscar contexto relevante
SELECT content, metadata, rank 
FROM memories 
WHERE memories MATCH ? 
ORDER BY rank 
LIMIT 10;

-- Guardar memoria
INSERT INTO memories (content, metadata, session_id, timestamp)
VALUES (?, ?, ?, datetime('now'));
```

---

## 🔄 SDD Workflow - Spec-Driven Development

### Fase 1: Specification
- **Modelo**: Kimi K2.5 Thinking (razonamiento)
- **Tarea**: Analizar requerimientos y crear especificación
- **Output**: `specs/feature-name.md`

### Fase 2: Architecture
- **Modelo**: Kimi K2.5 (arquitectura)
- **Tarea**: Diseñar arquitectura y componentes
- **Output**: `docs/architecture/feature-arch.md`

### Fase 3: Code
- **Modelo**: Kimi K2 (rápido, código)
- **Tarea**: Implementar código según spec
- **Output**: Código fuente

### Fase 4: Test
- **Modelo**: Kimi K2.5 (testing)
- **Tarea**: Crear tests unitarios e integración
- **Output**: `*_test.go`

### Fase 5: Deploy
- **Modelo**: GPT-4o (DevOps)
- **Tarea**: Configurar deployment
- **Output**: `k8s/`, `docker-compose.yml`

---

## 🛡️ Seguridad - Permisos

### Security-First Approach
- ✅ Solo lectura en filesystem (excepto workspace)
- ✅ No ejecución de comandos destructivos sin confirmación
- ✅ Validación de inputs
- ✅ Sanitización de outputs
- ✅ No envío de datos sensibles

### Reglas de Permisos
```yaml
permissions:
  filesystem:
    read: ["~/go-saas-api", "~/.config"]
    write: ["~/go-saas-api"]
  network:
    allowed: ["localhost", "api.github.com", "api.kimi.ai"]
  commands:
    allowed: ["go", "git", "docker", "kubectl"]
    forbidden: ["rm -rf /", "sudo", "dd"]
```

---

## 📚 Skills Disponibles

### Core Skills
1. **go-microservices-architecture** - Arquitectura de microservicios
2. **gin-api-gateway** - API Gateway con Gin
3. **go-sse-streaming** - Streaming Server-Sent Events
4. **go-testing** - Testing en Go (TDD)
5. **docker-microservices** - Containerización

### Domain Skills
1. **authentication-jwt** - Autenticación JWT
2. **stripe-integration** - Integración de pagos
3. **postgresql-optimization** - Optimización de PostgreSQL
4. **redis-patterns** - Patrones con Redis
5. **nats-messaging** - Mensajería con NATS

---

## 🎯 Comandos Útiles

### Gestión de Skills
```bash
# Listar skills disponibles
gentle-ai skills list

# Instalar skill
gentle-ai skills install go-microservices

# Actualizar skills
gentle-ai skills update --all
```

### Gestión de Memoria
```bash
# Buscar en memoria
engram query "implementación del chat service"

# Guardar contexto
engram save "Decidimos usar NATS para mensajería"

# Ver sesiones recientes
engram sessions --limit 10
```

### Workflow SDD
```bash
# Iniciar nueva feature
gentle-ai workflow start --name "user-authentication"

# Cambiar fase
gentle-ai workflow phase --to code

# Finalizar feature
gentle-ai workflow complete
```

---

## 🔧 Integración con IDEs

### VS Code
```json
{
  "mcp.servers": {
    "engram": {
      "command": "engram",
      "args": ["mcp", "--transport", "stdio"]
    },
    "filesystem": {
      "command": "npx",
      "args": ["@modelcontextprotocol/server-filesystem", "~/go-saas-api"]
    }
  }
}
```

### Cursor
```json
{
  "mcpServers": {
    "engram": {
      "command": "engram",
      "args": ["mcp", "--transport", "stdio"]
    }
  }
}
```

---

## 📖 Recursos

- [Gentle-AI GitHub](https://github.com/Gentleman-Programming/gentle-ai)
- [Engram Documentation](https://github.com/Gentleman-Programming/engram)
- [Skills.sh](https://skills.sh)
- [MCP Protocol](https://modelcontextprotocol.io/)

---

**Configurado por**: Claw 🐾  
**Para**: R0LM0  
**Fecha**: 2026-04-15

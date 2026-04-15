# 🧠 Engram - Sistema de Memoria Persistente

**Engram** es el sistema de memoria persistente principal para Claw, basado en el ecosistema [Gentle-AI](https://github.com/Gentleman-Programming/gentle-ai) de Gentleman Programming.

## 🎯 Características

- ✅ **Memoria persistente** con SQLite + FTS5
- ✅ **Búsqueda full-text** rápida e inteligente
- ✅ **Categorización** de memorias por sesión/tipo
- ✅ **Integración nativa** con OpenClaw
- ✅ **Auto-guardado** de interacciones
- ✅ **Recuperación de contexto** para conversaciones

## 📦 Estructura

```
engram-memory/
├── engram.py              # Módulo principal de memoria
├── cli.py                 # CLI tool
├── engram/                # Implementación en Go (opcional)
│   └── cmd/
│       └── engram-memory/
│           └── main.go
└── README.md
```

## 🚀 Instalación

### 1. Dependencias

```bash
# Python 3.8+
pip install sqlite3  # Usually included in stdlib
```

### 2. Verificar instalación

```bash
cd ~/go-saas-api/engram-memory
python3 cli.py stats
```

## 💻 Uso

### Como CLI Tool

```bash
# Guardar una memoria
python3 cli.py save "El usuario prefiere Go sobre Node.js" --session preferences

# Buscar memorias
python3 cli.py query "Go"

# Ver memorias recientes
python3 cli.py recent --limit 10

# Obtener estadísticas
python3 cli.py stats

# Obtener contexto para conversación
python3 cli.py context --query "arquitectura"
```

### Como Módulo Python (Integración con Claw)

```python
from engram_integration import remember, recall, get_context, save_fact

# Guardar información importante
remember("El usuario usa PostgreSQL", category="tech_stack")

# Guardar un hecho sobre el usuario
save_fact("Usuario prefiere código limpio", category="preference")

# Buscar información relevante
results = recall("base de datos")

# Obtener contexto para la conversación actual
context = get_context("microservicios")
```

## 🔧 Integración con OpenClaw

El archivo `engram_integration.py` en `~/.openclaw/workspace/` proporciona:

- `remember()` - Guardar información
- `recall()` - Recuperar información
- `get_context()` - Obtener contexto de conversación
- `save_fact()` - Guardar hechos importantes
- `save_interaction()` - Guardar interacción completa

### Uso en Claw

```python
# Al inicio de cada sesión, cargar contexto
context = get_context()

# Después de cada respuesta importante
remember("Decisión: usaremos NATS para mensajería", category="architecture")

# Guardar preferencias del usuario
save_fact("Usuario quiere aprender Go", category="learning")
```

## 📊 Base de Datos

**Ubicación:** `~/.local/share/engram/claw-memory.db`

**Estructura:**

```sql
-- Tabla principal
create table memories (
    id integer primary key,
    session_id text,
    content text,
    metadata json,
    timestamp datetime
);

-- Búsqueda full-text (FTS5)
create virtual table memories_fts using fts5(content, metadata);

-- Sesiones
create table sessions (
    id text primary key,
    title text,
    created_at datetime
);
```

## 🔍 Funciones de Búsqueda

### Búsqueda Full-Text (FTS5)

```python
# Buscar palabras específicas
recall("microservicios Go")

# Buscar frases exactas
recall('"Docker Compose"')

# Buscar con prefijos
recall("micro*")
```

### Contexto Inteligente

```python
# Obtener contexto basado en query
get_context("arquitectura")

# Obtener memorias recientes
get_context(recent_limit=20)
```

## 🛡️ Seguridad

- Base de datos local (no en la nube)
- Permisos de archivo restrictivos
- Sin datos sensibles hardcodeados
- Sanitización de inputs

## 📝 Categorías Recomendadas

- `conversation` - Conversaciones generales
- `fact` - Hechos importantes
- `preference` - Preferencias del usuario
- `tech_stack` - Stack tecnológico
- `architecture` - Decisiones de arquitectura
- `project` - Información específica del proyecto

## 🔄 Backup

```bash
# Backup manual
cp ~/.local/share/engram/claw-memory.db ~/backups/engram-$(date +%Y%m%d).db

# O usar sqlite3
sqlite3 ~/.local/share/engram/claw-memory.db ".backup ~/backups/engram-backup.db"
```

## 📈 Monitoreo

```bash
# Estadísticas
python3 cli.py stats

# Tamaño de la base de datos
ls -lh ~/.local/share/engram/claw-memory.db
```

## 🎯 Roadmap

- [ ] Implementación completa en Go para mejor performance
- [ ] Vector search con embeddings
- [ ] MCP server para integración con Claude/Cursor
- [ ] Sincronización entre dispositivos
- [ ] Encriptación de datos sensibles

## 📚 Recursos

- [Engram Original](https://github.com/Gentleman-Programming/engram)
- [Gentle-AI](https://github.com/Gentleman-Programming/gentle-ai)
- [SQLite FTS5](https://www.sqlite.org/fts5.html)

---

**Implementado por:** Claw 🐾  
**Para:** R0LM0  
**Basado en:** Gentleman Programming - Gentle-AI Ecosystem

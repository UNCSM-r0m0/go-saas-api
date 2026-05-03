# go-saas-api — Recomendaciones de Arquitectura
> Análisis basado en el codebase actual | Mayo 2026

---

## 1. Resumen Ejecutivo

Tienes una base sólida: clean architecture, RLS en Postgres, streaming SSE, multi-provider LLM con fallback, WebSocket, tool registry. El problema es que hay **deuda de diseño acumulada** que, si no se corrige antes de escalar, va a costar mucho más después. Este documento cubre:

- Eliminar multitenancy (tenants) sin romper nada
- Simplificar el modelo de usuarios a 3 roles reales
- Modo agéntico con sandbox visual
- Procesamiento de archivos (PDF, imagen, Excel, DOCX)
- Qué construir primero vs. qué puede esperar

---

## 2. Lo que tienes hoy (evaluación honesta)

| Módulo | Estado | Calidad |
|--------|--------|---------|
| `pkg/llm` (multi-provider, fallback, circuit breaker) | ✅ Completo | Excelente |
| `internal/agent/runtime` (orchestrator, classifier, dispatcher) | ✅ Completo | Bueno |
| `internal/agent/tools` (file_write, code_execute) | ✅ Completo | Bueno |
| `internal/agent/websocket` | ✅ Completo | Bueno |
| `internal/auth` (JWT, OAuth, password reset) | ✅ Completo | Muy bueno |
| `internal/billing` (Stripe, webhooks) | ✅ Completo | Bueno |
| `internal/provider` (CRUD providers/models, sync Ollama/LMStudio) | ✅ Completo | Muy bueno |
| `internal/fileupload` | ✅ Completo | Bueno |
| `cmd/api-gateway` (reverse proxy real) | ✅ Completo | Bueno |
| Multitenancy (`tenants` table, RLS por tenant_id) | ⚠️ Activo | **Problema: complejidad sin necesidad** |
| Modo agéntico visual (sandbox → frontend) | ❌ Faltante | — |
| Procesamiento de archivos (parse PDF/Excel/DOCX) | ❌ Faltante | — |

---

## 3. Eliminar Multitenancy (tenants)

### Por qué eliminarlo

Tu producto es un SaaS B2C: los usuarios se registran individualmente. No vendes a empresas que necesiten namespaces aislados. El `tenant_id` agrega:

- Un `set_config('app.current_tenant', ...)` en **cada query**
- Complejidad en cada handler (parse tenant_id del header)
- Políticas RLS que pueden fallar silenciosamente si se omite el set
- Confusión: ¿el tenant es el usuario? ¿la organización? Actualmente es hardcoded a `00000000-0000-0000-0000-000000000001`

Ese UUID hardcodeado en `auth/handler.go` confirma que multitenancy no está realmente en uso.

### Plan de migración (sin romper nada)

**Fase 1: Mantener la columna, dejar de usarla activamente**

```sql
-- NO borres la tabla tenants todavía.
-- Solo inserta un tenant único "sistema" y úsalo siempre.
INSERT INTO tenants (id, slug, name) 
VALUES ('00000000-0000-0000-0000-000000000001', 'system', 'System')
ON CONFLICT DO NOTHING;
```

**Fase 2: Reemplazar tenant_id por user_id en las queries críticas**

En `conversations`, `messages`, `artifacts`: el scope real es `user_id`, no `tenant_id`. Cambia los índices:

```sql
-- Antes (con tenant)
CREATE INDEX idx_conversations_tenant_id ON conversations(tenant_id);

-- Después (por usuario directamente)
CREATE INDEX idx_conversations_user_id ON conversations(user_id);
```

**Fase 3: Simplificar handlers**

Reemplaza el patrón:
```go
// Patrón actual (verboso)
tenantIDStr := c.GetHeader("X-Tenant-ID")
userIDStr   := c.GetHeader("X-User-ID")
if tenantIDStr == "" || userIDStr == "" { ... }
tenantID, _ := uuid.Parse(tenantIDStr)
userID, _   := uuid.Parse(userIDStr)
```

Por contexto JWT ya parseado:
```go
// Patrón simplificado
userID := mustGetUserID(c)  // helper que extrae del JWT ya validado
```

**Fase 4: Eliminar RLS basado en tenant, mantener RLS basado en user**

```sql
-- Nuevo: isolation por user_id directamente
CREATE POLICY user_conversations_isolation ON conversations
    USING (user_id = current_user_id());  -- función que lee el JWT de la sesión

CREATE OR REPLACE FUNCTION current_user_id() RETURNS UUID AS $$
    SELECT nullif(current_setting('app.current_user', true), '')::UUID;
$$ LANGUAGE sql STABLE;
```

**Cuándo borrar la tabla tenants:** Solo cuando no quede ninguna FK activa. Crea una migration `007_drop_tenants.sql` como último paso, después de haber limpiado todas las referencias.

---

## 4. Modelo de Usuarios Simplificado

### Los 3 roles que realmente necesitas

```
┌─────────────────────────────────────────────────────┐
│                    USUARIOS                          │
│                                                      │
│  registered  →  subscribed  →  (admin es un flag)   │
└─────────────────────────────────────────────────────┘
```

| Role | Cómo llega | Límites | Acceso |
|------|------------|---------|--------|
| `registered` | Se registra con email/OAuth | 10 mensajes/mes, solo modelos públicos | Chat básico, historial |
| `subscribed` | Paga suscripción en Stripe | 100 mensajes/mes, todos los modelos, funciones pro | Todo lo anterior + modelos premium, file upload |
| `admin` | Flag `is_admin: bool` en la tabla users | Sin límites | CRUD providers, API keys, modelos, ver métricas |

### Cambios en la tabla users

```sql
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS subscription_status TEXT 
    CHECK (subscription_status IN ('none', 'active', 'canceled', 'past_due'))
    DEFAULT 'none';
ALTER TABLE users ADD COLUMN IF NOT EXISTS messages_used_this_month INT NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN IF NOT EXISTS messages_limit INT NOT NULL DEFAULT 10;
ALTER TABLE users ADD COLUMN IF NOT EXISTS billing_period_start TIMESTAMPTZ DEFAULT now();
```

### Middleware de límites (reemplaza el rate limiter complejo)

```go
// internal/platform/middleware/limits.go

func MessageLimitMiddleware(userRepo UserLimitsRepo) gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := getUserIDFromContext(c)
        if userID == uuid.Nil {
            // Usuario anónimo: sin historial, sin guardar, límite por IP
            c.Next()
            return
        }

        limits, err := userRepo.GetLimits(c.Request.Context(), userID)
        if err != nil {
            c.Next() // fail open
            return
        }

        if limits.MessagesUsed >= limits.MessagesLimit {
            c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error":   "monthly message limit reached",
                "used":    limits.MessagesUsed,
                "limit":   limits.MessagesLimit,
                "tier":    limits.Tier,
                "upgrade": "https://tu-app.com/upgrade",
            })
            return
        }

        c.Next()
    }
}
```

### Reset mensual (cron job simple)

```go
// cmd/cron/main.go (nuevo binario pequeño, o usar NATS scheduler)
func resetMonthlyLimits(pool *pgxpool.Pool) {
    _, err := pool.Exec(ctx, `
        UPDATE users 
        SET messages_used_this_month = 0,
            billing_period_start = now()
        WHERE billing_period_start < date_trunc('month', now())
    `)
}
```

---

## 5. Modo Agéntico con Sandbox Visual

### Visión del flujo

```
Usuario escribe: "crea una landing page para una cafetería"
        ↓
Orchestrator classifica → RoleCoder
        ↓
LLM genera código (HTML/CSS/JS) + llama a file_write tool
        ↓
artifact guardado en DB con type="html"
        ↓
SSE emite: { type: "artifact_ready", artifact_id: "uuid", preview_url: "/artifacts/{id}/preview" }
        ↓
Frontend abre panel lateral con iframe que carga /artifacts/{id}/preview
        ↓
Usuario ve el resultado en tiempo real
```

### Lo que necesitas agregar en backend

**a) Endpoint de preview ya existe** — `/artifacts/:id/preview` en `agent-service`. Solo necesita servir con el Content-Type correcto según el tipo:

```go
func (s *Server) handlePreviewArtifact(c *gin.Context) {
    // ... código actual ...
    
    // Mejorar: detectar tipo y servir correctamente
    switch art.Type {
    case "html":
        c.Header("Content-Type", "text/html; charset=utf-8")
    case "svg":
        c.Header("Content-Type", "image/svg+xml")
    case "markdown":
        // Convertir a HTML si el frontend lo pide
        c.Header("Content-Type", "text/plain; charset=utf-8")
    default:
        c.Header("Content-Type", "text/plain; charset=utf-8")
    }
    
    // Seguridad: CSP para iframes de artifacts
    c.Header("Content-Security-Policy", 
        "default-src 'self' 'unsafe-inline' 'unsafe-eval' cdn.tailwindcss.com unpkg.com")
    c.Header("X-Frame-Options", "SAMEORIGIN")
    
    c.String(http.StatusOK, art.Content)
}
```

**b) SSE debe emitir eventos tipados de artifacts**

Modifica el stream en el orchestrator para que cuando un tool call de `file_write` termine, emita un chunk especial:

```go
// En orchestrator.go, después de ejecutar el tool
if toolResult.Name == "file_write" && res.Error == "" {
    // Extraer artifact_id del resultado
    artifactChunk := llm.Chunk{
        Content: res.Content,
        Done:    false,
        ToolCall: &llm.ToolCall{
            Name: "artifact_created",
            Arguments: map[string]any{
                "artifact_id": extractArtifactID(res.Content),
                "type":        args["type"],
                "name":        args["name"],
            },
        },
    }
    outCh <- artifactChunk
}
```

**c) Mejorar el sandbox para ejecutar y retornar output en tiempo real**

El `sandbox-service` actual ya ejecuta código. Solo necesita mejorar:

```go
// cmd/sandbox-service/main.go — agregar streaming de output
http.HandleFunc("/execute/stream", func(w http.ResponseWriter, r *http.Request) {
    // Configurar SSE
    w.Header().Set("Content-Type", "text/event-stream")
    
    // Ejecutar con cmd.Start() en lugar de cmd.Run()
    // Leer stdout línea por línea y enviar como SSE
    scanner := bufio.NewScanner(cmd.Stdout)
    for scanner.Scan() {
        fmt.Fprintf(w, "data: %s\n\n", scanner.Text())
        flusher.Flush()
    }
})
```

**d) Soporte para código React/Vue (no solo HTML)**

```go
// En sandbox-service, agregar soporte para bundling básico
case "react":
    // Usar esbuild (Go nativo) para transpilar JSX
    // github.com/evanw/esbuild/pkg/api
    result := api.Transform(code, api.TransformOptions{
        Loader: api.LoaderJSX,
        Target: api.ES2020,
    })
    // Envolver en HTML con React CDN
    html := wrapInHTML(result.Code)
    filename = "index.html"
    args = []string{"serve"} // servir estático
```

---

## 6. Procesamiento de Archivos

### Archivos que quieres soportar

| Tipo | Parser | Uso en LLM |
|------|--------|------------|
| PDF | `pdfcpu` o `ledongthuc/pdfparser` | Extraer texto → contexto |
| Imagen (PNG/JPG/WEBP) | Pasar directo al LLM | Modelos con visión (Gemini, GPT-4V) |
| DOCX | `unidoc/unioffice` | Extraer texto → contexto |
| XLSX/CSV | `tealeg/xlsx` o `encoding/csv` | Extraer datos → contexto estructurado |
| Código (.go, .py, etc.) | Texto plano | Contexto directo |

### Arquitectura de procesamiento

```
Usuario sube archivo
        ↓
fileupload.Service.Save() — ya existe, guarda en disco
        ↓
[NUEVO] fileupload.Processor.Extract(upload) → ExtractedContent
        ↓
ExtractedContent{Text, Images, Tables, Metadata}
        ↓
Se adjunta al chat como contexto adicional
```

**Nueva interfaz de procesamiento:**

```go
// internal/fileupload/processor.go (NUEVO)

type ExtractedContent struct {
    Text     string          // texto plano extraído
    Tables   []Table         // para Excel/CSV
    Images   []ImageData     // para PDFs con imágenes / imágenes directas
    Metadata map[string]any  // autor, páginas, etc.
    MimeType string
}

type Table struct {
    Name    string
    Headers []string
    Rows    [][]string
}

type ImageData struct {
    Base64   string
    MimeType string
}

type Processor interface {
    Extract(ctx context.Context, upload *Upload) (*ExtractedContent, error)
}

// Dispatcher que elige el procesador correcto
type FileProcessor struct {
    processors map[string]Processor
}

func NewFileProcessor() *FileProcessor {
    fp := &FileProcessor{
        processors: make(map[string]Processor),
    }
    fp.processors["application/pdf"]    = &PDFProcessor{}
    fp.processors["text/csv"]           = &CSVProcessor{}
    fp.processors["application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"] = &XLSXProcessor{}
    fp.processors["application/vnd.openxmlformats-officedocument.wordprocessingml.document"] = &DOCXProcessor{}
    fp.processors["image/png"]  = &ImageProcessor{}
    fp.processors["image/jpeg"] = &ImageProcessor{}
    fp.processors["image/webp"] = &ImageProcessor{}
    // texto plano: ya lo tienes con ReadText()
    return fp
}
```

**Implementación PDF (más común):**

```go
// internal/fileupload/processors/pdf.go
import "github.com/ledongthuc/pdf"

type PDFProcessor struct{}

func (p *PDFProcessor) Extract(ctx context.Context, upload *Upload) (*ExtractedContent, error) {
    f, r, err := pdf.Open(upload.StoragePath)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    var text strings.Builder
    totalPages := r.NumPage()
    
    // Limitar a las primeras 50 páginas para no explotar el contexto del LLM
    pages := min(totalPages, 50)
    
    for pageIndex := 1; pageIndex <= pages; pageIndex++ {
        p, err := r.Page(pageIndex)
        if err != nil || p.V.IsNull() {
            continue
        }
        t, _ := p.GetPlainText(nil)
        text.WriteString(t)
    }

    return &ExtractedContent{
        Text:     text.String(),
        MimeType: "application/pdf",
        Metadata: map[string]any{
            "pages":       totalPages,
            "pages_read":  pages,
            "truncated":   totalPages > 50,
        },
    }, nil
}
```

**Implementación imágenes (para modelos multimodales):**

```go
// internal/fileupload/processors/image.go
type ImageProcessor struct{}

func (p *ImageProcessor) Extract(ctx context.Context, upload *Upload) (*ExtractedContent, error) {
    data, err := os.ReadFile(upload.StoragePath)
    if err != nil {
        return nil, err
    }
    
    // Redimensionar si es muy grande (para no explotar el contexto)
    resized, err := resizeIfNeeded(data, 1024) // max 1024px
    if err != nil {
        resized = data
    }
    
    return &ExtractedContent{
        MimeType: upload.ContentType,
        Images: []ImageData{
            {
                Base64:   base64.StdEncoding.EncodeToString(resized),
                MimeType: upload.ContentType,
            },
        },
    }, nil
}
```

**Integrar en el orchestrator:**

```go
// En orchestrator.go, reemplazar buildFileContext()
func (o *Orchestrator) buildFileContext(ctx context.Context, tenantID uuid.UUID, fileIDs []uuid.UUID) (string, []ImageData, error) {
    var textParts []string
    var images []ImageData
    
    for _, id := range fileIDs {
        upload, err := o.fileSvc.Get(ctx, tenantID, id)
        if err != nil {
            continue
        }
        
        extracted, err := o.fileProcessor.Extract(ctx, upload)
        if err != nil {
            continue
        }
        
        if extracted.Text != "" {
            // Limitar a 8000 tokens aprox (~32K chars)
            text := extracted.Text
            if len(text) > 32000 {
                text = text[:32000] + "\n...[archivo truncado por tamaño]"
            }
            textParts = append(textParts, fmt.Sprintf(
                "--- Archivo: %s (%s) ---\n%s",
                upload.OriginalName, upload.ContentType, text,
            ))
        }
        
        images = append(images, extracted.Images...)
    }
    
    return strings.Join(textParts, "\n\n"), images, nil
}
```

**Pasar imágenes al LLM (para providers que soportan visión):**

```go
// pkg/llm/client.go — extender Message para soportar imágenes
type MessageContent struct {
    Type     string `json:"type"` // "text" o "image_url"
    Text     string `json:"text,omitempty"`
    ImageURL *ImageURL `json:"image_url,omitempty"`
}

type ImageURL struct {
    URL    string `json:"url"` // "data:image/png;base64,..."
    Detail string `json:"detail"` // "auto", "low", "high"
}

type Message struct {
    Role    string           `json:"role"`
    Content interface{}      `json:"content"` // string o []MessageContent
}
```

### Dependencias a agregar en go.mod

```
github.com/ledongthuc/pdf v0.0.0-20240201131950-da5d75209b16
github.com/tealeg/xlsx v1.0.5
github.com/evanw/esbuild v0.24.0
```

Para DOCX la opción más simple sin licencias complejas:

```
github.com/nguyenthenguyen/docx v0.0.0-20230621112118-9c8e795a11db
```

---

## 7. Admin: Gestión de Providers y API Keys

### Lo que ya tienes y está bien

- CRUD completo de providers (`internal/provider/handler.go`)
- Sync automático de modelos Ollama y LMStudio
- Circuit breaker y health checks en MultiClient

### Lo que falta

**a) Encriptación real de API keys de providers**

Actualmente guardas la API key en texto plano con un comentario `// TODO: encrypt`. Esto es un riesgo de seguridad real.

```go
// internal/platform/crypto/aes.go (NUEVO)
import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
)

type AESEncryptor struct {
    key []byte // 32 bytes para AES-256
}

func NewAESEncryptor(keyHex string) (*AESEncryptor, error) {
    key, err := hex.DecodeString(keyHex)
    if err != nil || len(key) != 32 {
        return nil, fmt.Errorf("key must be 32 bytes hex-encoded")
    }
    return &AESEncryptor{key: key}, nil
}

func (e *AESEncryptor) Encrypt(plaintext string) (string, error) {
    block, _ := aes.NewCipher(e.key)
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    rand.Read(nonce)
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (e *AESEncryptor) Decrypt(encoded string) (string, error) {
    data, _ := base64.StdEncoding.DecodeString(encoded)
    block, _ := aes.NewCipher(e.key)
    gcm, _ := cipher.NewGCM(block)
    nonceSize := gcm.NonceSize()
    plaintext, err := gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
    return string(plaintext), err
}
```

Agrega `ENCRYPTION_KEY` (32 bytes hex) a tus variables de entorno.

**b) Panel de admin simplificado**

Los endpoints `/admin/providers/*` ya existen. Solo necesitas protegerlos con un middleware de admin:

```go
// internal/platform/middleware/admin.go (NUEVO)
func AdminOnly() gin.HandlerFunc {
    return func(c *gin.Context) {
        role, exists := c.Get("role")
        if !exists || role != "admin" {
            c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
                "error": "admin access required",
            })
            return
        }
        c.Next()
    }
}
```

Y en el router del agent-service:

```go
// Solo admins pueden gestionar providers
admin := r.Group("/admin")
admin.Use(middleware.JWTAuth(jwtMgr))
admin.Use(middleware.AdminOnly())
providerHandler.RegisterRoutes(admin)
```

**c) Compatibilidad Anthropic**

Ya tienes el cliente OpenAI-compatible. Anthropic usa un formato diferente. Agregar:

```go
// pkg/llm/anthropic.go (NUEVO)
type AnthropicClient struct {
    baseURL string
    apiKey  string
    client  *http.Client
}

func NewAnthropicClient(apiKey string) *AnthropicClient {
    return &AnthropicClient{
        baseURL: "https://api.anthropic.com",
        apiKey:  apiKey,
        client:  &http.Client{Timeout: 120 * time.Second},
    }
}

func (c *AnthropicClient) Stream(ctx context.Context, req Request) (<-chan Chunk, error) {
    // Formato Anthropic: messages API con SSE
    body, _ := json.Marshal(map[string]any{
        "model":      req.Model,
        "max_tokens": req.MaxTokens,
        "messages":   convertToAnthropicMessages(req.Messages),
        "stream":     true,
    })
    
    httpReq, _ := http.NewRequestWithContext(ctx, "POST", 
        c.baseURL+"/v1/messages", bytes.NewReader(body))
    httpReq.Header.Set("x-api-key", c.apiKey)
    httpReq.Header.Set("anthropic-version", "2023-06-01")
    httpReq.Header.Set("Content-Type", "application/json")
    
    // Parsear SSE de Anthropic (formato diferente a OpenAI)
    // evento: content_block_delta con delta.text
    // ...
}
```

Agrega `ProviderAnthropic` en `internal/provider/model.go` ya está en el enum, solo falta el cliente.

---

## 8. Orden de Implementación Recomendado

```
SEMANA 1 — Limpiar la base
├── [ ] Eliminar tenant activo (hardcode a UUID único)
├── [ ] Simplificar handlers (quitar X-Tenant-ID, usar JWT directo)
├── [ ] Agregar campo is_admin a users
├── [ ] Middleware AdminOnly
└── [ ] Encriptación AES-256 para API keys de providers

SEMANA 2 — Modelo de límites
├── [ ] Agregar messages_used_this_month y messages_limit a users
├── [ ] Middleware MessageLimitMiddleware (reemplaza rate limiter Redis complejo)
├── [ ] Cron job de reset mensual (o trigger de Stripe webhook)
├── [ ] Webhook Stripe: activar subscribed, incrementar límite a 100
└── [ ] Tests unitarios de límites

SEMANA 3 — Procesamiento de archivos
├── [ ] Interfaz Processor + dispatcher
├── [ ] PDFProcessor (ledongthuc/pdf)
├── [ ] CSVProcessor (encoding/csv estándar)
├── [ ] XLSXProcessor (tealeg/xlsx)
├── [ ] ImageProcessor (base64 pass-through)
├── [ ] DOCXProcessor
└── [ ] Integrar en buildFileContext() del orchestrator

SEMANA 4 — Modo agéntico visual
├── [ ] Mejorar handlePreviewArtifact con CSP headers
├── [ ] SSE emite eventos tipados: artifact_created, artifact_updated
├── [ ] Sandbox: agregar /execute/stream con SSE output
├── [ ] Soporte React en sandbox (esbuild)
└── [ ] Cliente Anthropic

SEMANA 5 — Limpieza de multitenancy
├── [ ] Migration: quitar tenant_id de conversations, messages, artifacts
├── [ ] Agregar user_id directo donde faltaba
├── [ ] Actualizar RLS policies a current_user_id()
└── [ ] Remover tabla tenants (si no queda nada apuntando)
```

---

## 9. Decisiones de Arquitectura Claves

### ¿Microservicios o monolito modular?

Para donde estás ahora, **el monolito modular es la decisión correcta**. Tienes 6 binarios pero la mayor parte del valor está en `agent-service`. Los demás son relativamente pequeños.

Considera colapsar `usage-service` dentro de `billing-service` — hacen lo mismo con datos relacionados. Te ahorras un servicio sin perder nada.

### ¿Redis para rate limiting o DB?

Con el nuevo modelo de límites mensuales por usuario, Redis es overkill. Una columna `messages_used_this_month` en `users` + un índice + un UPDATE atómico es más simple, más durable y suficientemente rápido:

```sql
UPDATE users 
SET messages_used_this_month = messages_used_this_month + 1
WHERE id = $1 
  AND messages_used_this_month < messages_limit
RETURNING messages_used_this_month;
-- Si no retorna fila: límite alcanzado
```

Esto reemplaza todo el sistema de Redis Sorted Sets para rate limiting. Más simple, más correcto.

### ¿NATS es necesario?

En tu uso actual, NATS solo conecta `agent-service` → `sandbox-service`. Podrías reemplazarlo por una llamada HTTP directa (ya existe `HTTPSandboxClient`). NATS agrega valor cuando tienes eventos que múltiples servicios consumen. Si no llegas a ese punto, es complejidad innecesaria.

Mantén NATS en docker-compose por si acaso, pero no inviertas en construir más sobre él hasta que lo necesites.

---

## 10. Seguridad: Puntos Críticos

1. **API keys de providers en plaintext** — Prioridad alta. Implementar AES-256 esta semana.

2. **Sandbox sin aislamiento real** — El sandbox actual ejecuta código en el host con `exec.Command`. Para producción necesitas Docker containers con límites de red y CPU. Mientras tanto, limitar a lenguajes seguros y agregar timeout estricto (ya tienes 10s, está bien).

3. **Content-Security-Policy en artifact preview** — Los HTML generados por el LLM podrían contener código malicioso. El header CSP en el iframe mitiga esto.

4. **Validación de archivos** — El `fileupload.Service` ya valida content-type. Agregar validación de tamaño por tipo: PDF max 20MB, imagen max 5MB, Excel max 10MB.

5. **Admin hardcodeado** — El primer admin deberá setearse manualmente en DB:
   ```sql
   UPDATE users SET role = 'owner', is_admin = true WHERE email = 'tu@email.com';
   ```
   Documentar esto claramente. No necesitas un endpoint de "crear admin".

---

## 11. Frontend: Lo que necesita del backend

Para el modo agéntico visual, el frontend necesita recibir eventos SSE tipados:

```typescript
// Eventos SSE del backend
type SSEEvent = 
  | { type: "chunk"; content: string; done: false }
  | { type: "chunk"; done: true }
  | { type: "artifact_created"; artifact_id: string; artifact_type: "html" | "react" | "svg"; name: string }
  | { type: "tool_call"; tool: string; status: "running" | "done" | "error"; result?: string }
  | { type: "error"; message: string }
```

El frontend puede:
1. Renderizar chunks de texto en tiempo real (ya funciona)
2. Cuando recibe `artifact_created`, abrir un panel lateral con un `<iframe>` apuntando a `/artifacts/{id}/preview`
3. Cuando recibe `tool_call` con status "running", mostrar un spinner con el nombre del tool

---

## 12. Librerías a Agregar (evaluadas)

| Librería | Para qué | Licencia | Peso |
|----------|----------|----------|------|
| `github.com/ledongthuc/pdf` | Parsear PDFs | MIT | Liviana |
| `github.com/tealeg/xlsx` | Parsear Excel | BSD | Liviana |
| `github.com/evanw/esbuild/pkg/api` | Transpilar JSX/TSX en sandbox | MIT | Pesada pero buena |
| `github.com/nguyenthenguyen/docx` | Parsear DOCX | MIT | Liviana |
| `github.com/disintegration/imaging` | Redimensionar imágenes | MIT | Liviana |

**No agregar por ahora:**
- pgvector / embeddings — Complejidad alta, bajo ROI hasta tener usuarios reales
- Kafka / RabbitMQ — NATS ya está, y probablemente tampoco lo necesitas
- ElasticSearch — Postgres FTS es suficiente para tu escala inicial

---

*Documento generado: Mayo 2026*  
*Próximo paso recomendado: Semana 1 — Limpiar la base (tenant simplification + AES)*

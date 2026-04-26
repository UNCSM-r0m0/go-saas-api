# Guía de Pruebas Manual — API SaaS (Español)

> Estado actual: **6 servicios corriendo, backend 100% funcional para pruebas**

---

## 1. Servicios corriendo ahora

| Servicio | Puerto directo | Puerto vía Gateway | Estado |
|----------|---------------|-------------------|--------|
| API Gateway | — | **13000** | ✅ Healthy |
| Auth | 13001 | /api/v1/auth/* | ✅ Healthy |
| Agent | 13002 | /api/v1/agent/* | ✅ Healthy |
| Billing | 13004 | /api/v1/billing/* | ✅ Healthy |
| Usage | 13005 | /api/v1/usage/* | ✅ Healthy |
| Sandbox | 13006 | — | ✅ Healthy |

**Base de datos:** `saas_db` en `localhost:5433` (Docker)
**Tenant por defecto:** `00000000-0000-0000-0000-000000000001`

---

## 2. Flujo de prueba paso a paso

### Swagger UI (documentación interactiva)

En modo development, el API Gateway expone Swagger UI:

**URL:** `http://localhost:13000/swagger/index.html`

Desde ahí puedes explorar todos los endpoints, ver schemas de request/response, y ejecutar peticiones directamente.

---

### Paso 0 — Regístrate (vía Gateway)

**POST** `http://localhost:13000/api/v1/auth/register`

```json
{
  "email": "tu@email.com",
  "password": "TuClaveSegura123!",
  "name": "Tu Nombre"
}
```

> **Nota:** No necesitas enviar `tenant_id` — el servicio usa el tenant por defecto.

**Respuesta esperada (201):**
```json
{
  "user": {
    "id": "...uuid...",
    "tenant_id": "00000000-0000-0000-0000-000000000001",
    "email": "tu@email.com",
    "name": "Tu Nombre",
    "role": "member"
  }
}
```

---

### Paso 1 — Login y obtener token

**POST** `http://localhost:13000/api/v1/auth/login`

```json
{
  "email": "tu@email.com",
  "password": "TuClaveSegura123!"
}
```

**Respuesta:**
```json
{
  "token": {
    "access_token": "eyJhbG...",
    "refresh_token": "...",
    "expires_in": 900
  },
  "user": { ... }
}
```

**Guarda el `access_token`** — lo necesitas para todo lo demás.

---

### Paso 2 — Ver tu perfil

**GET** `http://localhost:13000/api/v1/auth/me`

**Header:** `Authorization: Bearer <tu_access_token>`

---

### Paso 3 — Listar modelos de IA

**GET** `http://localhost:13000/api/v1/chat/models`

**Respuesta esperada:**
```json
{
  "models": [
    { "id": "gemma4:31b-cloud", "provider": "Ollama Local" },
    { "id": "gpt-oss:20b-cloud", "provider": "Ollama Local" },
    { "id": "minimax-m2.7:cloud", "provider": "Ollama Local" },
    { "id": "qwen3.5:397b-cloud", "provider": "Ollama Local" }
  ]
}
```

---

### Paso 4 — Admin: ver proveedores

**GET** `http://localhost:13002/admin/providers`

**Header:** `Authorization: Bearer <tu_access_token>`

**Respuesta:** Lista con Kimi Code, Ollama Local, LM Studio.

---

### Paso 5 — Admin: sincronizar modelos desde LM Studio

1. Obtén el ID del proveedor LM Studio del paso anterior.
2. **POST** `http://localhost:13002/admin/providers/<id_lmstudio>/sync-models`

**Header:** `Authorization: Bearer <tu_access_token>`

Esto lee los modelos de tu LM Studio (`192.168.1.13:1234`) y los guarda en la BD.

---

### Paso 6 — Chat con el agente (SSE streaming)

**POST** `http://localhost:13002/agent/chat`

**Headers:**
- `Content-Type: application/json`
- `X-Tenant-ID: 00000000-0000-0000-0000-000000000001`
- `X-User-ID: <tu_user_id>`

**Body:**
```json
{
  "message": "Hola, ¿cómo estás?"
}
```

> ⚠️ La respuesta es **SSE** (Server-Sent Events). En Postman se verá como texto plano con líneas `data: {...}`.

---

### Paso 7 — Conversaciones

**Listar:** **GET** `http://localhost:13002/conversations`

**Headers:** `X-Tenant-ID` + `X-User-ID`

**Ver mensajes:** **GET** `http://localhost:13002/conversations/<id>/messages`

---

### Paso 8 — Billing / Planes

**GET** `http://localhost:13004/billing/plans`

**Respuesta:** Free, Registered, Premium con sus límites.

---

### Paso 9 — Sandbox: ejecutar Python

**POST** `http://localhost:13006/execute`

**Headers:**
- `Content-Type: application/json`
- `X-Tenant-ID: 00000000-0000-0000-0000-000000000001`
- `X-User-ID: <tu_user_id>`

**Body:**
```json
{
  "language": "python",
  "code": "print(2 + 2)"
}
```

---

## 3. Configuración rápida de Postman / Bruno

### Variables de colección

| Variable | Valor |
|----------|-------|
| `base_url` | `http://localhost:13000/api/v1` |
| `token` | (vacía, se llena automático) |

### Test script para login (pegar en pestaña Tests del login)

```javascript
var jsonData = pm.response.json();
pm.collectionVariables.set("token", jsonData.token.access_token);
pm.collectionVariables.set("user_id", jsonData.user.id);
```

### Authorization automática

En la colección → **Authorization** → Type `Bearer Token` → Token: `{{token}}`

---

## 4. Errores comunes y soluciones

| Error | Causa | Solución |
|-------|-------|----------|
| `failed to list providers` (500) | Campos NULL en BD escaneados como `string` | ✅ Ya arreglado — `APIKeyEncrypted` y `APIKeyHash` ahora son `*string` |
| `failed to list plans` (500) | `StripePriceID` NULL en BD | ✅ Ya arreglado — ahora es `*string` |
| `missing X-Tenant-ID or X-User-ID` | Faltan headers | Agrega ambos headers |
| `invalid token` (401) | Token expirado (15 min) o mal copiado | Renueva con `/auth/login` |
| `connection refused` | Servicio no corre | Verifica health checks |
| Puerto 3000-3006 ocupado | Stack Docker viejo | Usamos puertos 13000-13006 |
| Models solo fallback | Provider loader no cargó BD | ✅ Arreglado — ahora carga de `saas_db` en 5433 |

---

## 5. Cambios de código aplicados en esta sesión

1. **`.env`** creado con puertos alternativos y URLs de servicios
2. **`internal/provider/model.go`** — `APIKeyEncrypted` / `APIKeyHash` cambiados a `*string`
3. **`internal/provider/service.go`** — Asignaciones actualizadas a punteros
4. **`cmd/agent-service/provider_loader.go`** — Manejo de punteros nil en API keys
5. **`internal/billing/domain.go`** — `StripePriceID` cambiado a `*string`
6. **`internal/billing/service.go`** — Manejo de punteros nil
7. **`internal/billing/service_test.go`** — Tests actualizados para punteros
8. **`cmd/api-gateway/main.go`** — Agregada ruta `/chat/models`
9. **`bin/run-service.ps1`** — Script launcher para levantar servicios con `.env`

---

## 6. Comandos útiles

### Verificar health de todos los servicios
```powershell
13000,13001,13002,13004,13005,13006 | ForEach-Object { try { $r = iwr "http://localhost:$_/health" -TimeoutSec 3; "Port $_`: $($r.StatusCode)" } catch { "Port $_`: ERROR" } }
```

### Matar todos los servicios
```powershell
Get-Process auth-service,agent-service,billing-service,usage-service,api-gateway,sandbox-service -ErrorAction SilentlyContinue | Stop-Process -Force
```

### Recompilar y lanzar un servicio
```powershell
go build -o bin/auth-service.exe ./cmd/auth-service
# Luego usar bin/run-service.ps1
```

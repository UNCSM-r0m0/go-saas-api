# Guía de Pruebas — Flujo Completo con Swagger UI

> **Límites actualizados:**
> - Free: 3 msg/día (anónimos, sin login)
> - **Registered: 10 msg/día** (usuarios logueados)
> - **Premium: 100 msg/día** (usuarios con suscripción Stripe)

---

## 1. Cómo funciona el Chat con el Agente

### Arquitectura del chat

```
Usuario → API Gateway → Agent Service → Orchestrator
                                           ↓
                                    LLM (Ollama/Kimi/etc)
                                           ↓
                                    ¿Necesita tool?
                                           ↓
                              ┌──────────────────────┐
                              │  code_execute        │ → Sandbox Service
                              │  file_write          │ → Artifact DB
                              └──────────────────────┘
```

### Flujo paso a paso

1. **El usuario envía un mensaje** → `POST /api/v1/agent/chat`
2. **El Orchestrator** guarda el mensaje, carga el historial, y clasifica la intención
3. **Construye un prompt de sistema** que incluye las tools disponibles:
   ```
   You have access to the following tools:
   - code_execute: Execute code in an isolated sandbox...
   - file_write: Write a file with the given name...

   To use a tool, respond exactly with:
   TOOL_CALL: {"tool":"name","args":{...}} :END_TOOL_CALL
   ```
4. **El LLM responde** con texto plano o con un `TOOL_CALL`
5. **Si hay tool call**, el Orchestrator la ejecuta y adjunta el resultado
6. **Todo se guarda** como mensajes en la conversación

### Server-Sent Events (SSE)

El chat usa **SSE**, no JSON normal. La respuesta es un stream de texto:

```
data: {"content":"Hola","done":false}

data: {"content":"! ¿Cómo","done":false}

data: {"content":" estás?","done":true}

```

> ⚠️ En Swagger UI se verá como texto plano concatenado. Para ver el stream en tiempo real, usa Postman o `curl`.

---

## 2. Prueba 1 — Registro y Login (vía Swagger UI)

### Paso 1: Registrar usuario

1. Abre `http://localhost:13000/swagger/index.html`
2. Busca **POST /api/v1/auth/register**
3. Haz clic en **"Try it out"**
4. En el body, pon:
   ```json
   {
     "email": "tu@email.com",
     "password": "Password123!",
     "name": "Tu Nombre"
   }
   ```
5. **Execute** → Debería devolver `201 Created` con el usuario

### Paso 2: Login

1. Busca **POST /api/v1/auth/login**
2. Body:
   ```json
   {
     "email": "tu@email.com",
     "password": "Password123!"
   }
   ```
3. **Execute** → Copia el `access_token` de la respuesta

### Paso 3: Autorizar en Swagger

1. Arriba a la derecha en Swagger UI, haz clic en **"Authorize"**
2. Escribe: `Bearer eyJhbG...` (tu token)
3. Haz clic en **Authorize** y cierra el diálogo

> Ahora todos los endpoints protegidos enviarán el token automáticamente.

---

## 3. Prueba 2 — Chat Básico

### POST /api/v1/agent/chat

1. Busca el endpoint **POST /api/v1/agent/chat**
2. Completa los headers:
   - `X-Tenant-ID`: `00000000-0000-0000-0000-000000000001`
   - `X-User-ID`: El ID de tu usuario (lo obtuviste en el login)
3. Body:
   ```json
   {
     "message": "Hola, ¿cómo estás?"
   }
   ```
4. **Execute**

**Respuesta esperada:** Texto plano con el stream SSE (el agente responde saludando).

---

## 4. Prueba 3 — Chat en Modo Sandbox (ejecutar código)

El agente puede ejecutar código Python si le pides algo que requiera cálculo o prueba.

### Prompt para activar sandbox

Envía este mensaje:

```
Calcula la suma de los números del 1 al 100 usando Python y ejecútalo en el sandbox.
```

**Lo que debe pasar:**
1. El LLM recibe el prompt con las tools disponibles
2. Genera una respuesta que incluye:
   ```
   TOOL_CALL: {"tool":"code_execute","args":{"code":"sum(range(1, 101))","language":"python"}} :END_TOOL_CALL
   ```
3. El Orchestrator detecta el `TOOL_CALL`, ejecuta el código en el Sandbox Service
4. El resultado (5050) se adjunta a la respuesta final

### Otros prompts para probar sandbox

| Prompt | Tool usada | Resultado esperado |
|--------|-----------|-------------------|
| "Escribe un script Python que imprima 'Hola Mundo' y ejecútalo" | `code_execute` | Output: `Hola Mundo` |
| "Calcula el factorial de 5 en Python" | `code_execute` | Output: `120` |
| "Crea un archivo HTML llamado 'hola.html' con un título rojo" | `file_write` | Artifact creado |

---

## 5. Prueba 4 — Crear un Artifact (file_write)

Pídele al agente que cree un archivo. El usará la tool `file_write`.

### Prompt

```
Crea un archivo llamado "styles.css" con estilos para un botón azul con bordes redondeados.
```

### Verificar el artifact

1. Busca **POST /api/v1/artifacts**
2. O revisa la conversación con **GET /api/v1/conversations/{id}/messages**
3. El mensaje del asistente mostrará algo como:
   ```
   [Tool file_write result: Artifact a1b2c3d4-... created successfully]
   ```

---

## 6. Prueba 5 — Modo Premium (Stripe Checkout)

Para probar el flujo premium necesitas crear una suscripción de pago.

### Crear checkout

1. **POST /api/v1/billing/subscribe**
2. Headers: `X-Tenant-ID` + `X-User-ID`
3. Body:
   ```json
   {
     "email": "tu@email.com",
     "plan_slug": "premium"
   }
   ```
4. Obtendrás una `checkout_url` de Stripe
5. Ábrela en el navegador y completa el pago de prueba con:
   - Tarjeta: `4242 4242 4242 4242`
   - Fecha: cualquiera futura
   - CVC: cualquiera
   - ZIP: cualquiera
6. Stripe redirigirá al `FRONTEND_URL`

### Verificar suscripción

1. **GET /api/v1/billing/subscription**
2. Debería mostrar `plan: premium` y `status: active`

---

## 7. Prueba 6 — Límite de mensajes (Rate Limit)

Los usuarios registrados tienen **10 mensajes por día**.

### Cómo probar

1. Envía 10 mensajes de chat consecutivos
2. El 11º mensaje debería devolver:
   ```json
   {
     "error": "rate limit exceeded",
     "retry_after": 3600
   }
   ```

> Para resetear el contador, reinicia Redis o espera al día siguiente.

---

## 8. Colección Postman / Bruno

Si prefieres Postman sobre Swagger, aquí tienes el flujo:

### Variables de entorno

| Variable | Valor inicial |
|----------|--------------|
| `base_url` | `http://localhost:13000/api/v1` |
| `token` | (vacía) |
| `user_id` | (vacía) |
| `tenant_id` | `00000000-0000-0000-0000-000000000001` |

### Script de login (Tests tab)

```javascript
var jsonData = pm.response.json();
pm.environment.set("token", jsonData.token.access_token);
pm.environment.set("user_id", jsonData.user.id);
```

### Headers automáticos

En tu colección, agrega estos headers que se envíen en cada request:

```
Authorization: Bearer {{token}}
X-Tenant-ID: {{tenant_id}}
X-User-ID: {{user_id}}
```

### Flujo de requests

1. `POST {{base_url}}/auth/register`
2. `POST {{base_url}}/auth/login` → guarda token
3. `GET {{base_url}}/auth/me` → verifica
4. `GET {{base_url}}/chat/models` → lista modelos
5. `POST {{base_url}}/agent/chat` → chat SSE
6. `GET {{base_url}}/conversations` → ver conversaciones
7. `GET {{base_url}}/billing/plans` → ver planes
8. `POST {{base_url}}/billing/subscribe` → checkout premium

---

## 9. Comandos útiles para pruebas

### Probar chat con curl (ver SSE en tiempo real)

```bash
curl -N -X POST http://localhost:13000/api/v1/agent/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer TU_TOKEN" \
  -H "X-Tenant-ID: 00000000-0000-0000-0000-000000000001" \
  -H "X-User-ID: TU_USER_ID" \
  -d '{"message":"Hola"}'
```

> Usa `-N` para no bufferizar la respuesta y ver el stream en tiempo real.

### Resetear contador de rate limit (Redis)

```bash
docker exec saas-redis redis-cli DEL "ratelimit:TU_USER_ID"
```

### Ver logs del agente

```bash
# Si usas el launcher de PowerShell:
Get-Content log_agent.out -Tail 20
```

---

## 10. Resumen de tools disponibles

| Tool | Descripción | Ejemplo de uso |
|------|-------------|----------------|
| `code_execute` | Ejecuta código en sandbox | Python, JS, Go, Bash |
| `file_write` | Crea un artifact/archivo | HTML, CSS, JS, código |

### Formato interno de tool call

El LLM debe responder exactamente así:

```
TOOL_CALL: {"tool":"code_execute","args":{"code":"print(2+2)","language":"python"}} :END_TOOL_CALL
```

El Orchestrator detecta esto, ejecuta la tool, y devuelve el resultado al usuario.

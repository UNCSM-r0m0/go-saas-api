# Website Agent - Roadmap a Multi-File Professional Builder

## Estado Actual (Fase 2.5 - Single File HTML)
- LLM genera 1 archivo HTML con CSS/JS embebido
- Artifact guarda: `index.html` (string content)
- Preview: iframe con `srcdoc`
- UI: file tree de 1 archivo, code viewer sin syntax highlighting

## Fase 3: Multi-File Professional Builder

### 3.1 Pulir UI/UX del SandboxPanel (Inmediato)
**Objetivo**: Panel profesional, responsive, con UX de editor de código real.

**Tasks:**
1. **Responsive Layout**
   - Collapsible file tree (toggle con hamburguesa en mobile)
   - Resizable panels (drag divider entre file tree y editor/preview)
   - Full-screen mode para preview
   - Stack layout en mobile: tabs (Preview | Code) con swipe

2. **Editor de Código Mejorado**
   - Integrar `react-syntax-highlighter` (ya está en dependencias)
   - Syntax highlighting por lenguaje (html, css, js, tsx)
   - Line numbers
   - Copy button funcional
   - Word wrap toggle
   - Minimap (opcional)

3. **File Tree Profesional**
   - Iconos por tipo de archivo (React, CSS, HTML, JSON)
   - Carpeta src/ con subdirectorios
   - Nuevo archivo / Nueva carpeta (botones)
   - Breadcrumb navigation
   - File search/filter

4. **Preview Mejorado**
   - Device frames (desktop, tablet, mobile)
   - Refresh button
   - URL bar (para navegación dentro del iframe)
   - Zoom in/out
   - Inspector mode (overlay para ver elementos)
   - CORS fix: usar blob URL en vez de srcdoc para soporte de recursos externos

5. **Header del Panel**
   - Título del proyecto (editable)
   - Botón "Descargar ZIP"
   - Botón "Full Screen"
   - Botón "Deploy" (futuro)
   - Tabs: Preview | Code | Console | Network (futuro)

### 3.2 Backend: Multi-File Artifact Model
**Objetivo**: Guardar proyectos completos, no solo 1 archivo.

**Tasks:**
1. **Nuevo Modelo `ArtifactProject`**
   ```go
   type ArtifactProject struct {
       ID             uuid.UUID
       ConversationID uuid.UUID
       Name           string    // "Landing Page Jabones"
       Type           string    // "website" | "react-vite"
       Framework      string    // "html" | "react-vite"
       Version        int
       Files          []ArtifactFile
       EntryFile      string    // "index.html" | "src/main.tsx"
       IsComplete     bool      // true cuando todos los archivos están generados
       CreatedAt      time.Time
       UpdatedAt      time.Time
   }
   
   type ArtifactFile struct {
       Path     string // "src/App.tsx"
       Language string // "typescript"
       Content  string
       Order    int    // para ordenar en el file tree
   }
   ```

2. **Nueva Tabla `artifact_files`**
   ```sql
   CREATE TABLE artifact_files (
       id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
       artifact_id UUID NOT NULL REFERENCES artifacts(id) ON DELETE CASCADE,
       path TEXT NOT NULL,
       language TEXT,
       content TEXT NOT NULL,
       file_order INT DEFAULT 0,
       created_at TIMESTAMP DEFAULT NOW(),
       updated_at TIMESTAMP DEFAULT NOW(),
       UNIQUE(artifact_id, path)
   );
   ```

3. **Parser Multi-File**
   - El LLM genera output con delimitadores especiales:
   ```
   === FILE: index.html ===
   <html>...</html>
   
   === FILE: src/App.tsx ===
   import React...
   
   === FILE: src/main.tsx ===
   ...
   
   === FILE: package.json ===
   {...}
   
   === END ===
   ```
   - Backend parsea con regex y crea múltiples `ArtifactFile`
   - Valida estructura mínima (package.json, index.html o main.tsx)

4. **API Endpoints**
   - `GET /artifacts/:id` → devuelve ArtifactProject con files[]
   - `POST /artifacts` → crea proyecto completo (array de files)
   - `GET /artifacts/:id/files/:path` → devuelve 1 archivo (para lazy load)
   - `PATCH /artifacts/:id/files/:path` → edita 1 archivo (futuro)

### 3.3 Frontend: Multi-File Store & UI
**Objetivo**: Store y UI que manejen proyectos multi-archivo.

**Tasks:**
1. **Actualizar `ArtifactProject` interface**
   ```typescript
   interface ArtifactProject {
       id: string;
       conversation_id: string;
       name: string;
       type: string;
       framework: string;
       version: number;
       entry_file: string;
       is_complete: boolean;
       files: ArtifactFile[];
       created_at: string;
       updated_at: string;
   }
   ```

2. **Nuevo Componente `FileTree`**
   - Renderiza jerarquía de directorios (src/components/...)
   - Expansible/collapsible folders
   - Drag & drop para reordenar (futuro)
   - Context menu: rename, delete, new file/folder

3. **Nuevo Componente `CodeEditor`**
   - Usar `react-syntax-highlighter` con tema oscuro/claro
   - Soportar múltiples lenguajes
   - Opcional: integrar Monaco Editor (más pesado pero mejor UX)

4. **Preview con Bundler**
   - **Opción A**: WebContainer (StackBlitz) → corre Node.js en browser
     - Pros: Vite real, npm install, hot reload
     - Cons: ~2MB bundle, lento en mobile, no todos los browsers
   - **Opción B**: iframe con bundler server-side
     - Pros: más rápido, funciona en todos lados
     - Cons: requiere servicio backend de bundling
   - **Opción C**: iframe con srcdoc mejorado (intermedio)
     - Inyectar `<script type="module">` con imports resueltos
     - Transformar JSX/TSX a JS en cliente (usando @babel/standalone)
     - Tailwind CDN para estilos
     - Pros: no requiere backend extra, funciona offline
     - Cons: lento para proyectos grandes, no soporta todos los imports

### 3.4 Website Agent: Prompt Multi-File
**Objetivo**: El LLM genera un proyecto completo, no solo 1 archivo.

**Tasks:**
1. **Nuevo System Prompt**
   ```
   Sos un arquitecto web senior. Generá un proyecto completo con React + Vite + Tailwind.
   
   REGLAS:
   - Usá el formato de archivos delimitado === FILE: path ===
   - Separá lógicamente: componentes, estilos, configuración
   - Index.html debe tener el DOCTYPE y script src="/src/main.tsx"
   - package.json debe incluir react, react-dom, tailwindcss
   - Tailwind config debe estar presente
   - Todos los componentes en src/components/
   - App.tsx como entry point
   - main.tsx como bootstrap
   
   FORMATO DE SALIDA:
   === FILE: index.html ===
   <!DOCTYPE html>
   ...
   
   === FILE: package.json ===
   {"name": "landing-page", ...}
   
   === FILE: vite.config.ts ===
   ...
   
   === FILE: tailwind.config.js ===
   ...
   
   === FILE: src/main.tsx ===
   ...
   
   === FILE: src/App.tsx ===
   ...
   
   === FILE: src/components/Hero.tsx ===
   ...
   
   === END ===
   ```

2. **Continuación de Generación**
   - Si el LLM corta (max tokens), pedir continuación:
   - "Continuá desde el archivo X, línea Y"
   - Marcar proyecto como `is_complete: false` hasta que termine
   - Mostrar progress: "Generando archivo 5/12..."

3. **Validación Post-Generación**
   - Verificar que todos los archivos requeridos estén presentes
   - Si falta package.json → agregar default
   - Si falta tailwind.config → agregar default
   - Si falta index.html → crear con entry point correcto

### 3.5 Bundler Client-Side (Opción C - Intermedia)
**Objetivo**: Preview profesional sin backend de bundling.

**Tasks:**
1. **Crear `bundler.ts` service**
   ```typescript
   // Transforma un proyecto multi-archivo en un HTML ejecutable
   export function bundleProject(files: ArtifactFile[]): string {
       // 1. Encontrar entry point (index.html o main.tsx)
       // 2. Inyectar React + ReactDOM + Babel desde CDN
       // 3. Transformar TSX/JSX a JS con Babel standalone
       // 4. Resolver imports relativos (./components/Hero → inline)
       // 5. Inyectar Tailwind CDN
       // 6. Generar HTML final con todos los scripts inline
       return html;
   }
   ```

2. **CDN Dependencies**
   - React 19: `https://esm.sh/react@19`
   - ReactDOM: `https://esm.sh/react-dom@19/client`
   - Babel: `https://unpkg.com/@babel/standalone/babel.min.js`
   - Tailwind: `https://cdn.tailwindcss.com`

3. **Import Resolution**
   - Interceptar imports en el código
   - Resolver `./Component` → buscar en files[]
   - Inyectar como módulo inline

### 3.6 Deploy & Export
**Objetivo**: Exportar proyecto como ZIP o deployar.

**Tasks:**
1. **Exportar ZIP**
   - Botón "Descargar ZIP"
   - Generar ZIP con estructura de carpetas
   - Usar librería `jszip`

2. **Deploy a Static Hosting (futuro)**
   - Integrar Vercel/Netlify API
   - Botón "Deploy"
   - Generar URL pública

## Plan de Implementación Paso a Paso

### Sprint 1: UI Polish (1-2 días)
- [ ] Responsive SandboxPanel
- [ ] Collapsible file tree
- [ ] Syntax highlighting con react-syntax-highlighter
- [ ] Device frames en preview
- [ ] Refresh button
- [ ] Mejorar empty/loading/error states

### Sprint 2: Backend Multi-File (2-3 días)
- [ ] Migrar schema: nueva tabla artifact_files
- [ ] Crear ArtifactProject model
- [ ] Parser multi-file con delimitadores === FILE: ===
- [ ] Actualizar API endpoints
- [ ] Actualizar websiteAgentLoop para generar multi-file

### Sprint 3: Frontend Multi-File (2-3 días)
- [ ] Actualizar ArtifactProject interface
- [ ] Crear componente FileTree con directorios
- [ ] Actualizar CodeEditor con syntax highlighting real
- [ ] Actualizar preview para soportar multi-file (srcdoc mejorado)

### Sprint 4: Bundler Client-Side (3-4 días)
- [ ] Crear service bundler.ts
- [ ] Integrar Babel standalone para JSX/TSX
- [ ] Resolver imports relativos
- [ ] Inyectar Tailwind CDN
- [ ] Soporte para React hooks y JSX

### Sprint 5: Polish & Testing (2 días)
- [ ] Edge cases: proyectos incompletos, errores de bundling
- [ ] Performance: lazy load de archivos grandes
- [ ] UX: progress bars, toast notifications
- [ ] Testing E2E

## Decisiones Técnicas Pendientes

1. **Monaco Editor vs react-syntax-highlighter**
   - Monaco: mejor UX (IntelliSense, minimap) pero +2MB bundle
   - SyntaxHighlighter: más liviano, suficiente para preview
   - **Recomendación**: SyntaxHighlighter ahora, Monaco como upgrade futuro

2. **Bundler: Client-side vs Server-side vs WebContainer**
   - Client-side (Babel + iframe): más simple, funciona offline, suficiente para landing pages
   - Server-side (Vite API): más robusto pero requiere infraestructura
   - WebContainer: experiencia premium pero pesado y lento
   - **Recomendación**: Client-side primero, WebContainer como upgrade premium

3. **Formato de salida del LLM**
   - === FILE: path === delimiter
   - XML tags <file path="...">
   - JSON array
   - **Recomendación**: === FILE: === por simplicidad y legibilidad

## Métricas de Éxito
- [ ] Website Agent genera proyectos con mínimo 5 archivos
- [ ] Preview carga en <3 segundos
- [ ] File tree muestra jerarquía real de carpetas
- [ ] Code editor tiene syntax highlighting para HTML/CSS/JS/TSX
- [ ] Exportar ZIP funciona
- [ ] Responsive en mobile (file tree colapsable)

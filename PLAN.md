# Norfrig Photo Hub — Plan de Proyecto

## Objetivo
Hub local (después cloud) para buscar y acceder rápidamente a las ~10k fotos de productos de Norfrig, distribuidas en 3 carpetas raíz del servidor (`LAUTARO`, `PABLO_Recursos`, `RESPALDOS`). Búsqueda principal por SKU, con respuestas en milisegundos. MVP corre en una PC Windows; futuro: Mac y cloud.

---

## Decisiones técnicas

| Componente | Tecnología | Razón |
|---|---|---|
| Backend | Go | Binario único, cross-compile a Mac trivial, concurrencia nativa para indexar, deploy cloud directo |
| DB | SQLite + FTS5 | Cero setup, full-text search nativo para SKUs caóticos, 10k registros sobran |
| File watching | `fsnotify` | Estándar de facto en Go |
| Thumbnails | `github.com/disintegration/imaging` | Simple, suficiente para JPG/PNG |
| Frontend | React + Vite + TS + TailwindCSS + shadcn/ui | UI linda con componentes listos, Claude Code lo maneja muy bien |
| Empaquetado | `//go:embed` del frontend buildeado | Un solo binario sirve todo |

---

## Estructura del repo

```
norfrig-hub/
├── cmd/hub/main.go              # entry point del binario
├── internal/
│   ├── indexer/                 # walk filesystem + worker pool
│   ├── sku/                     # extracción y normalización de SKU
│   ├── db/                      # capa SQLite + migraciones
│   ├── watcher/                 # fsnotify
│   ├── thumbnails/              # generación y caché
│   ├── api/                     # HTTP handlers
│   └── config/                  # carga de config.yaml
├── web/                         # frontend (React + Vite)
│   ├── src/
│   └── package.json
├── web/dist/                    # build output (embed-eado por Go)
├── data/                        # DB + thumbs (gitignored)
├── testdata/                    # ~50 fotos reales para tests
├── config.yaml                  # paths a las 3 carpetas raíz
├── go.mod
├── README.md
└── PLAN.md
```

---

## Estrategia de extracción de SKU

El SKU se encuentra de forma más confiable en la **ruta del archivo** (como nombre de carpeta) que en el nombre del archivo.

1. **Path-first:** recorrer las carpetas de la ruta de adentro hacia afuera; la carpeta más profunda que matchee patrón SKU es el SKU canónico.
2. **Filename fallback:** si no hay match en path, parsear desde nombre (ej: `3eu102_01.jpg` → `3EU102`).
3. **Normalización:** todos los SKUs en mayúsculas. Mantener también el "raw" original.
4. **Patrón inicial:** `^[A-Z0-9]{3,15}$` (regex base, ajustable contra `testdata/`).
5. **Múltiples candidatos:** si hay más de un match, guardar todos en `photo_skus` (relación N a N).
6. **Falsos positivos a excluir:** palabras como `JPG`, `PNG`, `WEB`, `COMBOS`, nombres propios de carpetas raíz, etc. Mantener una blacklist configurable.

Comando de debug: `hub debug-sku <ruta>` muestra qué SKU se extrajo y de dónde.

---

## Schema SQLite

```sql
CREATE TABLE photos (
  id INTEGER PRIMARY KEY,
  path TEXT NOT NULL UNIQUE,
  filename TEXT NOT NULL,
  root TEXT NOT NULL,           -- 'LAUTARO' | 'PABLO_Recursos' | 'RESPALDOS'
  sku_primary TEXT,
  size_bytes INTEGER,
  mtime DATETIME,
  taken_at DATETIME,            -- de EXIF si existe
  width INTEGER,
  height INTEGER,
  hash TEXT,                    -- para deduplicación
  indexed_at DATETIME
);

CREATE TABLE photo_skus (
  photo_id INTEGER REFERENCES photos(id) ON DELETE CASCADE,
  sku TEXT NOT NULL,
  source TEXT,                  -- 'path' | 'filename'
  PRIMARY KEY (photo_id, sku)
);

CREATE INDEX idx_photos_root ON photos(root);
CREATE INDEX idx_photos_mtime ON photos(mtime);
CREATE INDEX idx_photo_skus_sku ON photo_skus(sku);

CREATE VIRTUAL TABLE photos_fts USING fts5(
  filename, sku_primary, path,
  content='photos', content_rowid='id'
);
```

---

## API (boceto)

```
GET  /api/search?q=3EU102&root=&from=&to=&limit=50
GET  /api/photo/:id
GET  /api/photo/:id/thumb        # 256x256 WEBP
GET  /api/photo/:id/full         # archivo original (stream)
GET  /api/stats                  # totales, últimos indexados, salud del watcher
POST /api/reindex                # re-scan completo manual
POST /api/photo/:id/rename       # fase 6
POST /api/photo/:id/tags         # fase 6
```

---

## Plan por fases

> **Cómo usar con Claude Code:** cada fase es una "sesión". Abrí Claude Code en la raíz del repo y pegale el bloque de la fase + las secciones de SKU/Schema/API relevantes. No saltes fases. Pedile que escriba tests primero cuando aplique.

### Fase 0 — Scaffold y datos de muestra
**Goal:** repo inicializado, samples para testear.

Tareas:
- `go mod init` con la estructura de carpetas de arriba.
- Crear `config.yaml` con los paths de las 3 raíces configurables.
- Copiar ~50 fotos reales a `testdata/` (mezclando casos limpios y caóticos).
- README mínimo con cómo correr.

**Aceptación:** `go run ./cmd/hub --version` corre sin errores.

---

### Fase 1 — Indexer y extractor de SKU
**Goal:** CLI que recorre las 3 raíces y puebla la DB.

Tareas:
- `internal/sku` con tests unitarios (mínimo 20 casos cubriendo `LAUTARO/COMBOS/3527/JPG/SKU_ (1).jpg`, `RESPALDOS/.../3EU102/3eu102_01.jpg`, casos sin SKU, casos con múltiples candidatos, etc.).
- `internal/indexer` con worker pool sobre `filepath.WalkDir`.
- `internal/db` con migraciones embebidas.
- Comandos: `hub index --full` y `hub debug-sku <path>`.

**Aceptación:**
- 10k fotos indexan en < 30s desde disco local.
- `SELECT COUNT(*) FROM photos` matchea con `find ... | wc -l` real.
- `hub debug-sku testdata/RESPALDOS/.../3eu102_01.jpg` devuelve `3EU102` con `source=path`.
- Todos los tests de `internal/sku` pasan.

---

### Fase 2 — API HTTP + búsqueda
**Goal:** endpoint de búsqueda rápido sobre la DB.

Tareas:
- `internal/api` con `net/http` standard.
- Implementar search con FTS5 + filtros por root y rango de fechas.
- Tests de integración con DB real.

**Aceptación:**
- `curl localhost:8080/api/search?q=3EU102` devuelve JSON en < 50ms.
- Búsqueda parcial (`q=3EU`) también matchea.
- Búsqueda case-insensitive.

---

### Fase 3 — Thumbnails con caché
**Goal:** servir miniaturas rápido.

Tareas:
- `internal/thumbnails`: generar 256x256 WEBP y guardar en `data/thumbs/<id>.webp`.
- Endpoint `GET /api/photo/:id/thumb` que genera on-demand y sirve del caché.
- Comando opcional `hub thumbs --pregenerate` para precalentar.

**Aceptación:** thumb se sirve en < 20ms tras la primera generación.

---

### Fase 4 — Frontend
**Goal:** UI linda con búsqueda + grid.

Tareas:
- Setup Vite + React + TS + Tailwind + shadcn/ui en `web/`.
- Pantalla principal: search bar grande, grid de thumbnails responsive.
- Modal/sidebar al click: foto full, metadata (ruta, root = quién la subió, fecha, dimensiones), botones "abrir carpeta" y "copiar ruta".
- Filtros: por root, por rango de fechas.
- Atajos: `/` para focus search, flechas para navegar resultados, `Esc` para cerrar modal.
- Build a `web/dist` + `//go:embed` desde `cmd/hub`.

**Aceptación:** abrir `localhost:8080`, buscar "3EU102", ver thumbnails con metadata visible. Estética oscura, limpia, foco visual en las fotos.

---

### Fase 5 — Watcher
**Goal:** índice se mantiene fresco sin re-scan manual.

Tareas:
- `internal/watcher` con fsnotify recursivo.
- Debounce (cambios masivos no spamean la DB).
- Reflejar create/rename/delete en la DB.

**Aceptación:** copiar una foto nueva al server aparece en search en < 5s.

---

### Fase 6 — Edición
**Goal:** renombrar, mover, taggear desde la UI.

Tareas:
- Endpoints POST para rename/move (con validación de paths dentro de las raíces).
- Confirmación visual en UI antes de aplicar.
- Tabla `change_log` para historial / undo.
- Tabla `tags` y `photo_tags`.

**Aceptación:** renombrar desde la UI mueve el archivo en disco y actualiza la DB. Undo del último cambio funciona.

---

### Fase 7 — Mac y cloud (futuro)
- Cross-compile: `GOOS=darwin GOARCH=arm64 go build`.
- Cloud: decidir cuando lleguemos (VPS con SMB montado, o sync a S3 + worker).

---

## Reglas para Claude Code

1. **Una fase por sesión.** No pedir "implementá todo".
2. **Tests primero** en el extractor de SKU. Es la pieza más frágil.
3. **`testdata/` con casos reales** para iterar contra ejemplos concretos.
4. **Commits por fase** con mensaje claro tipo `feat(indexer): walk + SKU extraction`.
5. **No tocar archivos del server real** hasta fase 6 y solo con confirmación.
6. **Pegarle el schema y la sección de SKU** cada vez que la fase los toque, para que tenga el contexto a mano.

# Norfrig Photo Hub

Buscador local de fotos de productos por SKU. ~10k imágenes indexadas en SQLite con búsqueda en milisegundos.

## Requisitos

- Go 1.21+
- Node.js 18+ (para el frontend, Fase 4)

## Configuración

Editá `config.yaml` y apuntá los paths a las 3 carpetas raíz del servidor:

```yaml
roots:
  - name: LAUTARO
    path: "\\\\servidor\\LAUTARO"
  - name: PABLO_Recursos
    path: "\\\\servidor\\PABLO_Recursos"
  - name: RESPALDOS
    path: "\\\\servidor\\RESPALDOS"
```

## Cómo correr

```bash
# Verificar versión
go run ./cmd/hub --version

# Indexar (Fase 1)
go run ./cmd/hub index --full

# Debug de SKU en un archivo (Fase 1)
go run ./cmd/hub debug-sku testdata/ruta/foto.jpg

# Servidor (Fase 2+)
go run ./cmd/hub serve
```

## Estructura

```
cmd/hub/          — entry point
internal/
  sku/            — extracción y normalización de SKU
  indexer/        — walk filesystem + worker pool
  db/             — SQLite + migraciones
  watcher/        — fsnotify para cambios en tiempo real
  thumbnails/     — generación y caché de miniaturas
  api/            — HTTP handlers
  config/         — carga de config.yaml
web/              — frontend React + Vite
data/             — DB y thumbs (gitignored)
testdata/         — fotos de muestra para tests
```

## Fases

| Fase | Estado | Descripción |
|------|--------|-------------|
| 0    | ✅     | Scaffold + datos de muestra |
| 1    | ⬜     | Indexer + extractor de SKU |
| 2    | ⬜     | API HTTP + búsqueda |
| 3    | ⬜     | Thumbnails con caché |
| 4    | ⬜     | Frontend |
| 5    | ⬜     | File watcher |
| 6    | ⬜     | Edición desde UI |
| 7    | ⬜     | Mac + cloud |

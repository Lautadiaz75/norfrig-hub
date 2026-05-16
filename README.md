# Norfrig Photo Hub

Buscador local de fotos de productos por SKU. 137k imágenes indexadas en SQLite con búsqueda en milisegundos. Un único ejecutable que incluye el servidor, la base de datos y el frontend.

## Stack

| Capa | Tecnología |
|------|-----------|
| Backend | Go — servidor HTTP estándar, sin frameworks |
| Base de datos | SQLite + FTS5 (búsqueda fulltext) |
| Thumbnails | `disintegration/imaging` — JPEG 256×256, pure Go |
| Frontend | React 19 + Vite 6 + TypeScript + Tailwind CSS v3 |
| Embed | `//go:embed` — el frontend vive dentro del binario |
| Watcher | `fsnotify` — detecta cambios en disco en tiempo real |

## Requisitos

- Go 1.21+
- Node.js 18+ (solo para modificar el frontend; el binario ya lo incluye compilado)

## Instalación

```bash
# 1. Clonar
git clone https://github.com/Lautadiaz75/norfrig-hub.git
cd norfrig-hub

# 2. Configurar paths
cp config.yaml.example config.yaml
# editar config.yaml con las rutas reales del servidor

# 3. Compilar
go build -o hub.exe ./cmd/hub   # Windows
go build -o hub ./cmd/hub       # Mac/Linux
```

## Uso

```bash
# Indexar las carpetas raíz por primera vez (tarda según el tamaño)
hub.exe index

# Iniciar el servidor
hub.exe serve
# → http://localhost:8080

# Pre-generar todos los thumbnails (opcional, mejora velocidad inicial)
hub.exe thumbs

# Debug: ver qué SKU se extrae de una ruta
hub.exe debug-sku "M:\LAUTARO\COM0531\JPG\foto.jpg"

# Versión
hub.exe --version
```

## Autoarranque en Windows

Para que el Hub inicie automáticamente al encender la PC:

```powershell
# Ejecutar como Administrador
.\autostart-install.ps1

# Para desinstalar
.\autostart-uninstall.ps1
```

## Acceso remoto (Tailscale)

1. Instalar [Tailscale](https://tailscale.com/download) en la PC del servidor y en la PC remota
2. Iniciar sesión con la misma cuenta en ambas
3. Acceder desde la PC remota: `http://<ip-tailscale>:8080`

## Compilar para Mac

```bash
# Apple Silicon (M1/M2/M3)
GOOS=darwin GOARCH=arm64 go build -o hub-mac ./cmd/hub

# Intel
GOOS=darwin GOARCH=amd64 go build -o hub-mac-intel ./cmd/hub
```

No requiere CGO — funciona sin toolchain adicional.

## Estructura

```
cmd/hub/            — entry point y comandos CLI
internal/
  sku/              — extracción de SKU desde rutas y nombres de archivo
  indexer/          — worker pool sobre filepath.WalkDir
  db/               — capa SQLite: schema, upsert, búsqueda FTS5
  watcher/          — fsnotify: sincroniza DB ante cambios en disco
  thumbnails/       — generación y caché de miniaturas JPEG
  api/              — HTTP handlers (search, thumb, full, stats, reindex)
  config/           — carga de config.yaml
web/src/            — frontend React (SearchBar, PhotoGrid, PhotoModal, FilterBar)
data/               — DB y thumbnails en caché (gitignored)
testdata/           — fotos de muestra para tests
config.yaml.example — plantilla de configuración
```

## Cómo funciona la búsqueda

1. El indexer recorre las carpetas raíz y extrae el SKU de cada foto a partir de la ruta (la carpeta más profunda que matchee el patrón gana)
2. Guarda en SQLite: path, filename, root, sku, tamaño, fecha
3. Construye un índice FTS5 para búsqueda fulltext instantánea
4. La búsqueda combina FTS5 MATCH + LIKE por prefijo — resultados en < 20ms sobre 137k fotos
5. El watcher mantiene el índice fresco sin necesidad de re-indexar manualmente

## Comandos disponibles

| Comando | Descripción |
|---------|-------------|
| `hub serve` | Inicia el servidor HTTP en el puerto configurado |
| `hub index` | Indexa todas las carpetas raíz desde cero |
| `hub thumbs` | Pre-genera todos los thumbnails faltantes |
| `hub debug-sku <ruta>` | Muestra el SKU extraído de una ruta |
| `hub --version` | Muestra la versión |

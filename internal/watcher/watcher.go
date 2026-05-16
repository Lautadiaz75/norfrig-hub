package watcher

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/norfrig/hub/internal/config"
	"github.com/norfrig/hub/internal/db"
	"github.com/norfrig/hub/internal/sku"
)

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true,
	".webp": true, ".tif": true, ".tiff": true, ".bmp": true,
}

// Watcher vigila las raíces configuradas y sincroniza la DB ante cambios en disco.
type Watcher struct {
	fsw       *fsnotify.Watcher
	db        *db.DB
	extractor *sku.Extractor
	roots     []config.Root
	debounce  time.Duration

	mu      sync.Mutex
	pending map[string]fsnotify.Event
	timer   *time.Timer
}

func New(database *db.DB, extractor *sku.Extractor, roots []config.Root) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		fsw:       fsw,
		db:        database,
		extractor: extractor,
		roots:     roots,
		debounce:  2 * time.Second,
		pending:   make(map[string]fsnotify.Event),
	}, nil
}

// Start lanza el watcher en background sin bloquear el arranque del servidor.
func (w *Watcher) Start() {
	go w.loop()
	go func() {
		for _, root := range w.roots {
			if err := w.watchRecursive(root.Path); err != nil {
				log.Printf("watcher: no se pudo iniciar en %s: %v", root.Path, err)
			} else {
				log.Printf("watcher: vigilando %s", root.Path)
			}
		}
	}()
}

func (w *Watcher) Close() error {
	return w.fsw.Close()
}

func (w *Watcher) watchRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // ignorar errores de acceso
		}
		if d.IsDir() {
			_ = w.fsw.Add(path)
		}
		return nil
	})
}

func (w *Watcher) loop() {
	for {
		select {
		case event, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	// carpeta nueva → agregarla al watcher recursivamente
	if event.Has(fsnotify.Create) {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			_ = w.watchRecursive(event.Name)
			return
		}
	}

	// solo nos interesan archivos de imagen
	if !imageExts[strings.ToLower(filepath.Ext(event.Name))] {
		return
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Remove tiene prioridad sobre cualquier evento anterior para el mismo path
	if prev, exists := w.pending[event.Name]; !exists || !prev.Has(fsnotify.Remove) {
		w.pending[event.Name] = event
	}

	if w.timer != nil {
		w.timer.Reset(w.debounce)
	} else {
		w.timer = time.AfterFunc(w.debounce, w.flush)
	}
}

func (w *Watcher) flush() {
	w.mu.Lock()
	events := w.pending
	w.pending = make(map[string]fsnotify.Event)
	w.timer = nil
	w.mu.Unlock()

	var upserted, deleted int

	for path, event := range events {
		if event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
			if err := w.db.DeletePhoto(path); err != nil {
				log.Printf("watcher: delete %s: %v", path, err)
			} else {
				deleted++
			}
			continue
		}

		info, err := os.Stat(path)
		if err != nil {
			continue
		}

		root := w.rootFor(path)
		if root == nil {
			continue
		}

		res := w.extractor.Primary(path)
		photo := db.Photo{
			Path:       path,
			Filename:   filepath.Base(path),
			Root:       root.Name,
			SKUPrimary: res.SKU,
			SizeBytes:  info.Size(),
			Mtime:      info.ModTime(),
			IndexedAt:  time.Now().UTC(),
		}
		if res.SKU != "" {
			photo.SKUs = []db.SKUEntry{{SKU: res.SKU, Source: res.Source}}
		}

		if err := w.db.UpsertPhoto(photo); err != nil {
			log.Printf("watcher: upsert %s: %v", path, err)
		} else {
			upserted++
		}
	}

	if upserted+deleted > 0 {
		if err := w.db.RebuildFTS(); err != nil {
			log.Printf("watcher: FTS rebuild: %v", err)
		}
		log.Printf("watcher: +%d -%d fotos sincronizadas", upserted, deleted)
	}
}

func (w *Watcher) rootFor(path string) *config.Root {
	lpath := strings.ToLower(path)
	for i, r := range w.roots {
		if strings.HasPrefix(lpath, strings.ToLower(r.Path)) {
			return &w.roots[i]
		}
	}
	return nil
}

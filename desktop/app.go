package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/norfrig/hub/internal/config"
	"github.com/norfrig/hub/internal/db"
	"github.com/norfrig/hub/internal/indexer"
	"github.com/norfrig/hub/internal/sku"
	"github.com/norfrig/hub/internal/thumbnails"
	"github.com/norfrig/hub/internal/watcher"
)

type App struct {
	ctx          context.Context
	db           *db.DB
	thumbs       *thumbnails.Cache
	extractor    *sku.Extractor
	cfg          *config.Config
	watch        *watcher.Watcher
	assetBaseURL string
}

func NewApp() *App {
	return &App{}
}

// findConfig busca config.yaml en el directorio actual y en el padre.
// Cambia el working directory al directorio donde lo encuentra para que
// los paths relativos del config (data/hub.db, data/thumbs) resuelvan bien.
func findConfig() string {
	for _, p := range []string{"config.yaml", "../config.yaml"} {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			os.Chdir(filepath.Dir(abs)) //nolint:errcheck
			return abs
		}
	}
	return ""
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	configPath := findConfig()
	if configPath == "" {
		return
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return
	}
	a.cfg = cfg
	a.extractor = sku.New(cfg.SKU.Blacklist)

	database, err := db.Open(cfg.DB.Path)
	if err != nil {
		return
	}
	a.db = database

	thumbs, err := thumbnails.NewCache(cfg.Thumbnails.Dir, cfg.Thumbnails.Size)
	if err != nil {
		return
	}
	a.thumbs = thumbs

	w, err := watcher.New(database, a.extractor, cfg.Roots)
	if err == nil {
		w.Start()
		a.watch = w
	}

	// system tray
	a.startTray()

	// servidor HTTP interno para servir thumbnails y fotos originales.
	// El AssetsHandler de Wails solo funciona en producción, no en dev mode.
	listener, err := net.Listen("tcp", ":0")
	if err == nil {
		port := listener.Addr().(*net.TCPAddr).Port
		a.assetBaseURL = fmt.Sprintf("http://localhost:%d", port)
		go http.Serve(listener, a.assetHandler()) //nolint:errcheck
	}

	go func() {
		type entry struct {
			id   int64
			path string
		}
		var all []entry
		database.EachPhoto(func(id int64, path string) error { //nolint:errcheck
			all = append(all, entry{id, path})
			return nil
		})
		for _, e := range all {
			thumbs.Get(e.id, e.path) //nolint:errcheck
		}
	}()
}

func (a *App) shutdown(_ context.Context) {
	if a.watch != nil {
		a.watch.Close()
	}
	if a.db != nil {
		a.db.Close()
	}
}

// ---- tipos expuestos al frontend ----

type SearchResult struct {
	Results []PhotoItem `json:"results"`
	Total   int         `json:"total"`
	TookMs  int64       `json:"took_ms"`
}

type PhotoItem struct {
	ID         int64  `json:"id"`
	Path       string `json:"path"`
	Filename   string `json:"filename"`
	Root       string `json:"root"`
	SKUPrimary string `json:"sku_primary"`
	SizeBytes  int64  `json:"size_bytes"`
	Mtime      string `json:"mtime"`
	ThumbURL   string `json:"thumb_url"`
	FullURL    string `json:"full_url"`
}

// ---- métodos bound al frontend ----

func (a *App) Search(q, root string, limit int) SearchResult {
	if a.db == nil {
		return SearchResult{Results: []PhotoItem{}}
	}
	start := time.Now()
	rows, err := a.db.Search(db.SearchParams{Q: q, Root: root, Limit: limit})
	if err != nil {
		return SearchResult{Results: []PhotoItem{}}
	}
	items := make([]PhotoItem, len(rows))
	for i, r := range rows {
		idStr := strconv.FormatInt(r.ID, 10)
		items[i] = PhotoItem{
			ID:         r.ID,
			Path:       r.Path,
			Filename:   r.Filename,
			Root:       r.Root,
			SKUPrimary: r.SKUPrimary,
			SizeBytes:  r.SizeBytes,
			Mtime:      r.Mtime.Format(time.RFC3339),
			ThumbURL:   a.assetBaseURL + "/thumb/" + idStr,
			FullURL:    a.assetBaseURL + "/full/" + idStr,
		}
	}
	return SearchResult{Results: items, Total: len(items), TookMs: time.Since(start).Milliseconds()}
}

type StatsResult struct {
	TotalPhotos int64    `json:"total_photos"`
	Roots       []string `json:"indexed_roots"`
}

func (a *App) Stats() StatsResult {
	if a.db == nil {
		return StatsResult{Roots: []string{}}
	}
	stats, err := a.db.Stats()
	if err != nil {
		return StatsResult{Roots: []string{}}
	}
	roots := stats.Roots
	if roots == nil {
		roots = []string{}
	}
	return StatsResult{TotalPhotos: stats.TotalPhotos, Roots: roots}
}

func (a *App) OpenFolder(id int64) {
	if a.db == nil {
		return
	}
	photo, err := a.db.GetPhoto(id)
	if err != nil || photo == nil {
		return
	}
	folder := filepath.Dir(photo.Path)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer.exe", folder)
	case "darwin":
		cmd = exec.Command("open", folder)
	default:
		cmd = exec.Command("xdg-open", folder)
	}
	cmd.Start() //nolint:errcheck
}

func (a *App) OpenFile(id int64) {
	if a.db == nil {
		return
	}
	photo, err := a.db.GetPhoto(id)
	if err != nil || photo == nil {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", photo.Path)
	case "darwin":
		cmd = exec.Command("open", photo.Path)
	default:
		cmd = exec.Command("xdg-open", photo.Path)
	}
	cmd.Start() //nolint:errcheck
}

func (a *App) Reindex() {
	if a.db == nil || a.extractor == nil || a.cfg == nil {
		return
	}
	go func() {
		idx := indexer.New(a.db, a.extractor, a.cfg.Indexer.Workers)
		idx.IndexAll(a.cfg.Roots)  //nolint:errcheck
		a.db.RebuildFTS()          //nolint:errcheck
	}()
}

// assetHandler sirve thumbnails y fotos originales al WebView
func (a *App) assetHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if strings.HasPrefix(path, "/thumb/") {
			id, err := strconv.ParseInt(strings.TrimPrefix(path, "/thumb/"), 10, 64)
			if err != nil || a.db == nil || a.thumbs == nil {
				http.NotFound(w, r)
				return
			}
			photo, err := a.db.GetPhoto(id)
			if err != nil || photo == nil {
				http.NotFound(w, r)
				return
			}
			thumbPath, err := a.thumbs.Get(id, photo.Path)
			if err != nil {
				http.Error(w, "error generando thumbnail", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "public, max-age=86400")
			http.ServeFile(w, r, thumbPath)
			return
		}

		if strings.HasPrefix(path, "/full/") {
			id, err := strconv.ParseInt(strings.TrimPrefix(path, "/full/"), 10, 64)
			if err != nil || a.db == nil {
				http.NotFound(w, r)
				return
			}
			photo, err := a.db.GetPhoto(id)
			if err != nil || photo == nil {
				http.NotFound(w, r)
				return
			}
			http.ServeFile(w, r, photo.Path)
			return
		}

		http.NotFound(w, r)
	})
}

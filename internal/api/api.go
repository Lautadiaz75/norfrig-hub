package api

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/norfrig/hub/internal/db"
	"github.com/norfrig/hub/internal/thumbnails"
)

// Server maneja las peticiones HTTP.
type Server struct {
	db      *db.DB
	thumbs  *thumbnails.Cache // nil si no está configurado
	reindex func()            // nil si no está disponible
}

// New devuelve un http.Handler con todas las rutas de la API.
// staticFS es el FS con el frontend buildeado (puede ser nil en desarrollo).
// thumbs puede ser nil. reindex puede ser nil.
func New(database *db.DB, thumbs *thumbnails.Cache, staticFS fs.FS, reindex func()) http.Handler {
	s := &Server{db: database, thumbs: thumbs, reindex: reindex}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/search", s.handleSearch)
	mux.HandleFunc("/api/photo/", s.handlePhoto)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/reindex", s.handleReindex)

	// servir frontend embebido en todas las rutas no-API
	if staticFS != nil {
		fileServer := http.FileServer(http.FS(staticFS))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			// si el archivo no existe, devolver index.html (SPA fallback)
			_, err := staticFS.Open(strings.TrimPrefix(r.URL.Path, "/"))
			if err != nil {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				fileServer.ServeHTTP(w, r2)
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	return corsMiddleware(mux)
}

// ---- response types ----

type searchResponse struct {
	Results []photoJSON `json:"results"`
	Total   int         `json:"total"`
	TookMs  int64       `json:"took_ms"`
}

type photoJSON struct {
	ID         int64     `json:"id"`
	Path       string    `json:"path"`
	Filename   string    `json:"filename"`
	Root       string    `json:"root"`
	SKUPrimary string    `json:"sku_primary"`
	SizeBytes  int64     `json:"size_bytes"`
	Mtime      time.Time `json:"mtime"`
	IndexedAt  time.Time `json:"indexed_at"`
	ThumbURL   string    `json:"thumb_url"`
	FullURL    string    `json:"full_url"`
}

// ---- handlers ----

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query().Get("q")
	if strings.TrimSpace(q) == "" {
		jsonOK(w, searchResponse{Results: []photoJSON{}, Total: 0, TookMs: 0})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

	start := time.Now()
	rows, err := s.db.Search(db.SearchParams{
		Q:     q,
		Root:  r.URL.Query().Get("root"),
		From:  r.URL.Query().Get("from"),
		To:    r.URL.Query().Get("to"),
		Limit: limit,
	})
	if err != nil {
		jsonError(w, "error en búsqueda", http.StatusInternalServerError)
		return
	}

	results := make([]photoJSON, len(rows))
	for i, row := range rows {
		results[i] = toPhotoJSON(row)
	}

	jsonOK(w, searchResponse{
		Results: results,
		Total:   len(results),
		TookMs:  time.Since(start).Milliseconds(),
	})
}

func (s *Server) handlePhoto(w http.ResponseWriter, r *http.Request) {
	// Parsear /api/photo/{id}  o  /api/photo/{id}/thumb  o  /api/photo/{id}/full
	tail := strings.TrimPrefix(r.URL.Path, "/api/photo/")
	parts := strings.SplitN(tail, "/", 2)
	if parts[0] == "" {
		jsonError(w, "id requerido", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		jsonError(w, "id inválido", http.StatusBadRequest)
		return
	}

	sub := ""
	if len(parts) == 2 {
		sub = parts[1]
	}

	switch sub {
	case "":
		s.handleGetPhoto(w, r, id)
	case "thumb":
		s.handleThumb(w, r, id)
	case "full":
		s.handleFull(w, r, id)
	case "open-folder":
		s.handleOpenFolder(w, r, id)
	default:
		jsonError(w, "not found", http.StatusNotFound)
	}
}

func (s *Server) handleGetPhoto(w http.ResponseWriter, _ *http.Request, id int64) {
	photo, err := s.db.GetPhoto(id)
	if err != nil || photo == nil {
		jsonError(w, "foto no encontrada", http.StatusNotFound)
		return
	}
	jsonOK(w, toPhotoJSON(*photo))
}

func (s *Server) handleThumb(w http.ResponseWriter, r *http.Request, id int64) {
	if s.thumbs == nil {
		jsonError(w, "thumbnails no configurados", http.StatusNotImplemented)
		return
	}
	photo, err := s.db.GetPhoto(id)
	if err != nil || photo == nil {
		jsonError(w, "foto no encontrada", http.StatusNotFound)
		return
	}
	thumbPath, err := s.thumbs.Get(id, photo.Path)
	if err != nil {
		jsonError(w, "error generando thumbnail", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeFile(w, r, thumbPath)
}

func (s *Server) handleFull(w http.ResponseWriter, r *http.Request, id int64) {
	photo, err := s.db.GetPhoto(id)
	if err != nil || photo == nil {
		jsonError(w, "foto no encontrada", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, photo.Path)
}

func (s *Server) handleOpenFolder(w http.ResponseWriter, r *http.Request, id int64) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	photo, err := s.db.GetPhoto(id)
	if err != nil || photo == nil {
		jsonError(w, "foto no encontrada", http.StatusNotFound)
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
	jsonOK(w, map[string]string{"status": "ok"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats, err := s.db.Stats()
	if err != nil {
		jsonError(w, "error al obtener stats", http.StatusInternalServerError)
		return
	}
	if stats.Roots == nil {
		stats.Roots = []string{}
	}
	jsonOK(w, stats)
}

func (s *Server) handleReindex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.reindex == nil {
		jsonError(w, "reindex no disponible", http.StatusServiceUnavailable)
		return
	}
	go s.reindex()
	w.WriteHeader(http.StatusAccepted)
	jsonOK(w, map[string]string{"status": "reindex iniciado"})
}

// ---- helpers ----

func toPhotoJSON(p db.PhotoRow) photoJSON {
	idStr := strconv.FormatInt(p.ID, 10)
	return photoJSON{
		ID:         p.ID,
		Path:       p.Path,
		Filename:   p.Filename,
		Root:       p.Root,
		SKUPrimary: p.SKUPrimary,
		SizeBytes:  p.SizeBytes,
		Mtime:      p.Mtime,
		IndexedAt:  p.IndexedAt,
		ThumbURL:   "/api/photo/" + idStr + "/thumb",
		FullURL:    "/api/photo/" + idStr + "/full",
	}
}

func corsMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

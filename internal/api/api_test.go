package api_test

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/norfrig/hub/internal/api"
	"github.com/norfrig/hub/internal/db"
	"github.com/norfrig/hub/internal/thumbnails"
)

func setupServer(t *testing.T) *httptest.Server {
	t.Helper()

	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	ahora := time.Now().UTC()
	fotos := []db.Photo{
		{
			Path:       `M:\LAUTARO\COM0531\JPG\COM0531_ (1).jpg`,
			Filename:   "COM0531_ (1).jpg",
			Root:       "LAUTARO",
			SKUPrimary: "COM0531",
			SizeBytes:  102400,
			Mtime:      ahora.Add(-24 * time.Hour),
			IndexedAt:  ahora,
			SKUs:       []db.SKUEntry{{SKU: "COM0531", Source: "path"}},
		},
		{
			Path:       `M:\PABLO_Recursos\3EU107\3EU107_1.jpg`,
			Filename:   "3EU107_1.jpg",
			Root:       "PABLO_Recursos",
			SKUPrimary: "3EU107",
			SizeBytes:  204800,
			Mtime:      ahora.Add(-48 * time.Hour),
			IndexedAt:  ahora,
			SKUs:       []db.SKUEntry{{SKU: "3EU107", Source: "path"}},
		},
		{
			Path:       `M:\RESPALDOS\1013\1013_1.jpg`,
			Filename:   "1013_1.jpg",
			Root:       "RESPALDOS",
			SKUPrimary: "1013",
			SizeBytes:  51200,
			Mtime:      ahora.Add(-72 * time.Hour),
			IndexedAt:  ahora,
			SKUs:       []db.SKUEntry{{SKU: "1013", Source: "path"}},
		},
		{
			Path:       `M:\LAUTARO\COM0039\JPG\COM0039_ (1).jpg`,
			Filename:   "COM0039_ (1).jpg",
			Root:       "LAUTARO",
			SKUPrimary: "COM0039",
			SizeBytes:  76800,
			Mtime:      ahora.Add(-96 * time.Hour),
			IndexedAt:  ahora,
			SKUs:       []db.SKUEntry{{SKU: "COM0039", Source: "path"}},
		},
	}

	for _, p := range fotos {
		if err := database.UpsertPhoto(p); err != nil {
			t.Fatalf("UpsertPhoto(%s): %v", p.SKUPrimary, err)
		}
	}
	if err := database.RebuildFTS(); err != nil {
		t.Fatalf("RebuildFTS: %v", err)
	}

	return httptest.NewServer(api.New(database, nil, nil, nil))
}

func TestSearchExacto(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/search?q=COM0531")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Results []struct {
			SKUPrimary string `json:"sku_primary"`
			ThumbURL   string `json:"thumb_url"`
			FullURL    string `json:"full_url"`
		} `json:"results"`
		Total int `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 1 {
		t.Errorf("total = %d, want 1", body.Total)
	}
	if body.Results[0].SKUPrimary != "COM0531" {
		t.Errorf("sku_primary = %q, want COM0531", body.Results[0].SKUPrimary)
	}
	if body.Results[0].ThumbURL == "" {
		t.Error("thumb_url vacía")
	}
	if body.Results[0].FullURL == "" {
		t.Error("full_url vacía")
	}
}

func TestSearchParcial(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// "COM" debería matchear COM0531 y COM0039
	resp, err := http.Get(srv.URL + "/api/search?q=COM")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var body struct {
		Total int `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Total < 2 {
		t.Errorf("búsqueda parcial 'COM' debería dar >= 2, got %d", body.Total)
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// minúsculas deben encontrar lo mismo que mayúsculas
	resp, _ := http.Get(srv.URL + "/api/search?q=com0531")
	defer resp.Body.Close()

	var body struct {
		Total int `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Total != 1 {
		t.Errorf("búsqueda case-insensitive 'com0531' debería dar 1, got %d", body.Total)
	}
}

func TestSearchFiltrarPorRoot(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	get := func(url string) int {
		resp, _ := http.Get(url)
		defer resp.Body.Close()
		var body struct{ Total int `json:"total"` }
		json.NewDecoder(resp.Body).Decode(&body)
		return body.Total
	}

	// COM aparece en LAUTARO (2 fotos)
	if n := get(srv.URL + "/api/search?q=COM&root=LAUTARO"); n != 2 {
		t.Errorf("root=LAUTARO + q=COM: want 2, got %d", n)
	}
	// COM no aparece en RESPALDOS
	if n := get(srv.URL + "/api/search?q=COM&root=RESPALDOS"); n != 0 {
		t.Errorf("root=RESPALDOS + q=COM: want 0, got %d", n)
	}
}

func TestSearchVacio(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/search")
	defer resp.Body.Close()

	var body struct {
		Results []any `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Results) != 0 {
		t.Error("búsqueda sin q debería devolver results vacío")
	}
}

func TestSearchSinResultados(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/search?q=NOEXISTE999")
	defer resp.Body.Close()

	var body struct{ Total int `json:"total"` }
	json.NewDecoder(resp.Body).Decode(&body)
	if body.Total != 0 {
		t.Errorf("SKU inexistente debería dar 0, got %d", body.Total)
	}
}

func TestStats(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/stats")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		TotalPhotos int      `json:"total_photos"`
		Roots       []string `json:"indexed_roots"`
	}
	json.NewDecoder(resp.Body).Decode(&body)

	if body.TotalPhotos != 4 {
		t.Errorf("total_photos = %d, want 4", body.TotalPhotos)
	}
	if len(body.Roots) != 3 {
		t.Errorf("indexed_roots len = %d, want 3 (%v)", len(body.Roots), body.Roots)
	}
}

func TestGetPhoto(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// buscar para obtener un ID real
	resp, _ := http.Get(srv.URL + "/api/search?q=COM0531")
	var search struct {
		Results []struct {
			ID int64 `json:"id"`
		} `json:"results"`
	}
	json.NewDecoder(resp.Body).Decode(&search)
	resp.Body.Close()

	if len(search.Results) == 0 {
		t.Fatal("sin resultados para obtener ID")
	}

	id := search.Results[0].ID
	resp2, err := http.Get(fmt.Sprintf("%s/api/photo/%d", srv.URL, id))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/photo/%d status = %d, want 200", id, resp2.StatusCode)
	}

	var photo struct {
		SKUPrimary string `json:"sku_primary"`
	}
	json.NewDecoder(resp2.Body).Decode(&photo)
	if photo.SKUPrimary != "COM0531" {
		t.Errorf("sku_primary = %q, want COM0531", photo.SKUPrimary)
	}
}

func TestGetPhotoNoExiste(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/photo/99999")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("foto inexistente debería dar 404, got %d", resp.StatusCode)
	}
}

func TestCORS(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	resp, _ := http.Get(srv.URL + "/api/stats")
	defer resp.Body.Close()

	if resp.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Error("header CORS faltante")
	}
}

func TestSearchLimite(t *testing.T) {
	srv := setupServer(t)
	defer srv.Close()

	// limit=1 debería devolver solo 1 resultado aunque haya más
	resp, _ := http.Get(srv.URL + "/api/search?q=COM&limit=1")
	defer resp.Body.Close()

	var body struct {
		Results []any `json:"results"`
		Total   int   `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Results) > 1 {
		t.Errorf("con limit=1 debería devolver máx 1 resultado, got %d", len(body.Results))
	}
}

// setupServerConThumbs crea un servidor con thumbnails cache real y una foto real en disco.
func setupServerConThumbs(t *testing.T) (*httptest.Server, int64) {
	t.Helper()

	tmpDir := t.TempDir()
	database, err := db.Open(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	// crear imagen JPEG real en disco
	imgPath := filepath.Join(tmpDir, "foto.jpg")
	func() {
		img := image.NewRGBA(image.Rect(0, 0, 400, 300))
		azul := color.RGBA{R: 50, G: 100, B: 200, A: 255}
		for y := range img.Bounds().Max.Y {
			for x := range img.Bounds().Max.X {
				img.Set(x, y, azul)
			}
		}
		f, _ := os.Create(imgPath)
		defer f.Close()
		jpeg.Encode(f, img, nil) //nolint:errcheck
	}()

	foto := db.Photo{
		Path: imgPath, Filename: "foto.jpg", Root: "TEST",
		SKUPrimary: "TEST001", SizeBytes: 1024,
		Mtime: time.Now(), IndexedAt: time.Now(),
		SKUs: []db.SKUEntry{{SKU: "TEST001", Source: "path"}},
	}
	if err := database.UpsertPhoto(foto); err != nil {
		t.Fatalf("UpsertPhoto: %v", err)
	}

	// buscar el ID que le asignó la DB
	rows, _ := database.Search(db.SearchParams{Q: "TEST001", Limit: 1})
	if len(rows) == 0 {
		t.Fatal("foto no encontrada en DB")
	}
	photoID := rows[0].ID

	thumbsDir := filepath.Join(tmpDir, "thumbs")
	cache, err := thumbnails.NewCache(thumbsDir, 256)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	srv := httptest.NewServer(api.New(database, cache, nil, nil))
	t.Cleanup(srv.Close)
	return srv, photoID
}

func TestThumbGeneraWEBP(t *testing.T) {
	srv, id := setupServerConThumbs(t)

	resp, err := http.Get(fmt.Sprintf("%s/api/photo/%d/thumb", srv.URL, id))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("thumb status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
	if resp.ContentLength == 0 {
		t.Error("thumbnail vacío")
	}
}

func TestThumbCacheControl(t *testing.T) {
	srv, id := setupServerConThumbs(t)

	resp, _ := http.Get(fmt.Sprintf("%s/api/photo/%d/thumb", srv.URL, id))
	defer resp.Body.Close()

	if cc := resp.Header.Get("Cache-Control"); cc == "" {
		t.Error("Cache-Control header faltante en thumbnail")
	}
}

func TestThumbFotoInexistente(t *testing.T) {
	srv, _ := setupServerConThumbs(t)

	resp, _ := http.Get(srv.URL + "/api/photo/99999/thumb")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("thumb de foto inexistente debería dar 404, got %d", resp.StatusCode)
	}
}

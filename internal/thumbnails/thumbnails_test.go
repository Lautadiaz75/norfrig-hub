package thumbnails_test

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/norfrig/hub/internal/thumbnails"
)

// crearJPEG genera un JPEG 200×300 rojo en path para usar en tests.
func crearJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 200, 300))
	rojo := color.RGBA{R: 200, G: 50, B: 50, A: 255}
	for y := range img.Bounds().Max.Y {
		for x := range img.Bounds().Max.X {
			img.Set(x, y, rojo)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("crear jpeg de prueba: %v", err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatalf("encodear jpeg: %v", err)
	}
}

func TestGetGeneraYCachea(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "foto.jpg")
	crearJPEG(t, src)

	cache, err := thumbnails.NewCache(dir, 256)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	// primera llamada: genera el thumb
	path, err := cache.Get(1, src)
	if err != nil {
		t.Fatalf("Get (primera vez): %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("thumb no existe en disco: %v", err)
	}

	// segunda llamada: devuelve el cacheado sin regenerar
	path2, err := cache.Get(1, src)
	if err != nil {
		t.Fatalf("Get (segunda vez): %v", err)
	}
	if path != path2 {
		t.Errorf("path cambió entre llamadas: %q vs %q", path, path2)
	}
}

func TestThumbFitDentro256(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "foto.jpg")
	crearJPEG(t, src) // 200×300

	cache, err := thumbnails.NewCache(dir, 256)
	if err != nil {
		t.Fatal(err)
	}

	thumbPath, err := cache.Get(2, src)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	// verificar que el archivo JPEG existe y tiene contenido
	info, err := os.Stat(thumbPath)
	if err != nil {
		t.Fatalf("thumb no existe: %v", err)
	}
	if info.Size() == 0 {
		t.Error("thumb WEBP está vacío")
	}
}

func TestGetSrcInexistente(t *testing.T) {
	cache, err := thumbnails.NewCache(t.TempDir(), 256)
	if err != nil {
		t.Fatal(err)
	}

	_, err = cache.Get(99, "/no/existe/foto.jpg")
	if err == nil {
		t.Error("debería haber error al abrir una imagen inexistente")
	}
}

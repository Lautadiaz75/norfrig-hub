package thumbnails

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/disintegration/imaging"
)

// Cache genera y almacena miniaturas JPEG en disco.
type Cache struct {
	dir  string
	size int
}

// NewCache crea un Cache que guarda thumbs en dir.
// Crea el directorio si no existe.
func NewCache(dir string, size int) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("crear directorio de thumbs: %w", err)
	}
	if size <= 0 {
		size = 256
	}
	return &Cache{dir: dir, size: size}, nil
}

// Path devuelve la ruta del thumbnail de una foto (exista o no).
func (c *Cache) Path(id int64) string {
	return filepath.Join(c.dir, strconv.FormatInt(id, 10)+".jpg")
}

// Get devuelve la ruta del thumbnail, generándolo si no existe aún.
func (c *Cache) Get(id int64, srcPath string) (string, error) {
	dst := c.Path(id)
	if _, err := os.Stat(dst); err == nil {
		return dst, nil // cache hit
	}
	if err := c.generate(srcPath, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// generate lee srcPath, redimensiona a size×size (fit) y guarda como JPEG en dst.
func (c *Cache) generate(src, dst string) error {
	img, err := imaging.Open(src, imaging.AutoOrientation(true))
	if err != nil {
		return fmt.Errorf("decodificar %s: %w", src, err)
	}
	thumb := imaging.Fit(img, c.size, c.size, imaging.Lanczos)
	return imaging.Save(thumb, dst, imaging.JPEGQuality(82))
}

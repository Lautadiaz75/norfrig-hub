package indexer

import (
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/norfrig/hub/internal/config"
	"github.com/norfrig/hub/internal/db"
	"github.com/norfrig/hub/internal/sku"
)

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true,
	".webp": true, ".gif": true, ".tiff": true, ".tif": true, ".bmp": true,
}

type Indexer struct {
	db        *db.DB
	extractor *sku.Extractor
	workers   int
}

func New(database *db.DB, extractor *sku.Extractor, workers int) *Indexer {
	return &Indexer{db: database, extractor: extractor, workers: workers}
}

type fileJob struct {
	path string
	root string
	info fs.FileInfo
}

// IndexAll recorre todas las raíces y persiste cada imagen en la DB.
// Devuelve el total de archivos procesados.
func (idx *Indexer) IndexAll(roots []config.Root) (int, error) {
	jobs := make(chan fileJob, 500)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		total   int
		errored int
	)

	for i := 0; i < idx.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := idx.processFile(job); err != nil {
					log.Printf("error procesando %s: %v", job.path, err)
					mu.Lock()
					errored++
					mu.Unlock()
					continue
				}
				mu.Lock()
				total++
				mu.Unlock()
			}
		}()
	}

	for _, root := range roots {
		rootPath := root.Path
		rootName := root.Name
		err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				log.Printf("error al caminar %s: %v", path, walkErr)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !imageExts[strings.ToLower(filepath.Ext(path))] {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			jobs <- fileJob{path: path, root: rootName, info: info}
			return nil
		})
		if err != nil {
			log.Printf("error al recorrer raíz %s: %v", rootPath, err)
		}
	}

	close(jobs)
	wg.Wait()

	if errored > 0 {
		log.Printf("indexación completa: %d OK, %d con errores", total, errored)
	}
	return total, nil
}

func (idx *Indexer) processFile(job fileJob) error {
	results := idx.extractor.Extract(job.path)

	var primarySKU string
	var skuEntries []db.SKUEntry
	for _, r := range results {
		if primarySKU == "" {
			primarySKU = r.SKU
		}
		skuEntries = append(skuEntries, db.SKUEntry{SKU: r.SKU, Source: r.Source})
	}

	return idx.db.UpsertPhoto(db.Photo{
		Path:       job.path,
		Filename:   filepath.Base(job.path),
		Root:       job.root,
		SKUPrimary: primarySKU,
		SizeBytes:  job.info.Size(),
		Mtime:      job.info.ModTime(),
		IndexedAt:  time.Now(),
		SKUs:       skuEntries,
	})
}

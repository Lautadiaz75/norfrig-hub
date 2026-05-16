package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/norfrig/hub/internal/api"
	"github.com/norfrig/hub/internal/config"
	"github.com/norfrig/hub/internal/db"
	"github.com/norfrig/hub/internal/indexer"
	"github.com/norfrig/hub/internal/sku"
	"github.com/norfrig/hub/internal/thumbnails"
	"github.com/norfrig/hub/internal/watcher"
	hubweb "github.com/norfrig/hub/web"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "--version", "version":
		fmt.Printf("hub v%s\n", version)

	case "index":
		cmdIndex()

	case "serve":
		cmdServe()

	case "debug-sku":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "uso: hub debug-sku <ruta>")
			os.Exit(1)
		}
		cmdDebugSKU(os.Args[2])

	case "thumbs":
		cmdThumbs()

	default:
		fmt.Fprintf(os.Stderr, "comando desconocido: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "uso: hub <comando>")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "comandos:")
	fmt.Fprintln(os.Stderr, "  index          recorre las raíces e indexa las fotos en la DB")
	fmt.Fprintln(os.Stderr, "  serve          inicia el servidor HTTP")
	fmt.Fprintln(os.Stderr, "  thumbs         pre-genera todos los thumbnails en caché")
	fmt.Fprintln(os.Stderr, "  debug-sku      muestra el SKU extraído de una ruta de archivo")
	fmt.Fprintln(os.Stderr, "  --version      muestra la versión")
}

func cmdIndex() {
	cfg := loadConfig()
	extractor := sku.New(cfg.SKU.Blacklist)

	database, err := db.Open(cfg.DB.Path)
	if err != nil {
		log.Fatalf("no se pudo abrir la DB: %v", err)
	}
	defer database.Close()

	idx := indexer.New(database, extractor, cfg.Indexer.Workers)

	fmt.Printf("indexando %d raíces con %d workers...\n", len(cfg.Roots), cfg.Indexer.Workers)
	start := time.Now()

	total, err := idx.IndexAll(cfg.Roots)
	if err != nil {
		log.Fatalf("error durante la indexación: %v", err)
	}

	elapsed := time.Since(start)

	fmt.Print("reconstruyendo índice de búsqueda FTS... ")
	if err := database.RebuildFTS(); err != nil {
		fmt.Printf("advertencia: %v\n", err)
	} else {
		fmt.Println("listo")
	}

	count, _ := database.Count()
	fmt.Printf("indexación completa: %d archivos procesados en %s (total en DB: %d)\n",
		total, elapsed.Round(time.Millisecond), count)
}

func cmdServe() {
	cfg := loadConfig()
	extractor := sku.New(cfg.SKU.Blacklist)

	database, err := db.Open(cfg.DB.Path)
	if err != nil {
		log.Fatalf("no se pudo abrir la DB: %v", err)
	}
	defer database.Close()

	thumbs, err := thumbnails.NewCache(cfg.Thumbnails.Dir, cfg.Thumbnails.Size)
	if err != nil {
		log.Fatalf("no se pudo crear caché de thumbnails: %v", err)
	}

	reindex := func() {
		idx := indexer.New(database, extractor, cfg.Indexer.Workers)
		n, err := idx.IndexAll(cfg.Roots)
		if err != nil {
			log.Printf("reindex error: %v", err)
			return
		}
		if err := database.RebuildFTS(); err != nil {
			log.Printf("FTS rebuild error: %v", err)
		}
		log.Printf("reindex completo: %d fotos", n)
	}

	distFS, err := fs.Sub(hubweb.FS, "dist")
	if err != nil {
		log.Fatalf("no se pudo crear sub-FS del frontend: %v", err)
	}

	// pre-generar thumbnails faltantes en background sin bloquear el arranque
	go func() {
		// cargar todos los IDs/paths en memoria antes de generar
		// para no mantener la conexión DB abierta durante la generación
		type entry struct {
			id   int64
			path string
		}
		var all []entry
		database.EachPhoto(func(id int64, path string) error { //nolint:errcheck
			all = append(all, entry{id, path})
			return nil
		})
		var ok, errs int64
		for _, e := range all {
			if _, err := thumbs.Get(e.id, e.path); err != nil {
				errs++
			} else {
				ok++
			}
		}
		log.Printf("thumbnails: %d listos, %d errores", ok, errs)
	}()

	w, err := watcher.New(database, extractor, cfg.Roots)
	if err != nil {
		log.Printf("watcher no disponible: %v", err)
	} else {
		w.Start()
		defer w.Close()
	}

	handler := api.New(database, thumbs, distFS, reindex)
	addr := fmt.Sprintf(":%d", cfg.Server.Port)

	log.Printf("hub v%s corriendo en http://localhost%s", version, addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func cmdThumbs() {
	cfg := loadConfig()

	database, err := db.Open(cfg.DB.Path)
	if err != nil {
		log.Fatalf("no se pudo abrir la DB: %v", err)
	}
	defer database.Close()

	thumbs, err := thumbnails.NewCache(cfg.Thumbnails.Dir, cfg.Thumbnails.Size)
	if err != nil {
		log.Fatalf("no se pudo crear caché de thumbnails: %v", err)
	}

	type job struct {
		id   int64
		path string
	}
	jobs := make(chan job, 200)

	var (
		done    atomic.Int64
		errors  atomic.Int64
		wg      sync.WaitGroup
	)

	for i := 0; i < cfg.Indexer.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if _, err := thumbs.Get(j.id, j.path); err != nil {
					errors.Add(1)
				} else {
					done.Add(1)
				}
				if n := done.Load() + errors.Load(); n%1000 == 0 {
					fmt.Printf("\r  %d generados...", n)
				}
			}
		}()
	}

	count, _ := database.Count()
	fmt.Printf("pre-generando thumbnails para %d fotos con %d workers...\n", count, cfg.Indexer.Workers)
	start := time.Now()

	database.EachPhoto(func(id int64, path string) error { //nolint:errcheck
		jobs <- job{id: id, path: path}
		return nil
	})
	close(jobs)
	wg.Wait()

	fmt.Printf("\rthumbnails listos: %d OK, %d errores en %s\n",
		done.Load(), errors.Load(), time.Since(start).Round(time.Second))
}

func cmdDebugSKU(path string) {
	cfg := loadConfig()
	extractor := sku.New(cfg.SKU.Blacklist)

	results := extractor.Extract(path)
	if len(results) == 0 {
		fmt.Printf("sin SKU encontrado en: %s\n", path)
		return
	}

	fmt.Printf("ruta:         %s\n", path)
	fmt.Printf("SKU primario: %s  (fuente: %s)\n", results[0].SKU, results[0].Source)
	if len(results) > 1 {
		fmt.Println("candidatos adicionales:")
		for _, r := range results[1:] {
			fmt.Printf("  - %s  (fuente: %s, raw: %q)\n", r.SKU, r.Source, r.Raw)
		}
	}
}

func loadConfig() *config.Config {
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("no se pudo cargar config.yaml: %v", err)
	}
	return cfg
}

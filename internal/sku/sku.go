package sku

import (
	"path/filepath"
	"regexp"
	"strings"
)

// Result holds an extracted SKU and its provenance.
type Result struct {
	SKU    string // normalizado en mayúsculas
	Source string // "path" | "filename"
	Raw    string // segmento original antes de normalizar
}

// skuRe acepta strings normalizados (uppercase) que podrían ser un SKU:
// empieza y termina con alfanumérico, el medio puede incluir guiones, 3-20 chars.
var skuRe = regexp.MustCompile(`^[A-Z0-9][A-Z0-9\-]{1,18}[A-Z0-9]$`)

// looksLikeSKU devuelve true si s matchea el patrón Y tiene al menos un dígito.
// Exigir un dígito filtra palabras de ruido puras como JPG, WEB, TESTDATA, PUBLICACIONES.
func looksLikeSKU(s string) bool {
	if !skuRe.MatchString(s) {
		return false
	}
	for _, c := range s {
		if c >= '0' && c <= '9' {
			return true
		}
	}
	return false
}

// DefaultBlacklist contiene nombres de carpeta conocidos que no son SKUs.
var DefaultBlacklist = []string{
	"JPG", "PNG", "WEB", "WEBP", "COMBOS",
	"LAUTARO", "PABLO_RECURSOS", "RESPALDOS", "FOTOS", "IMAGENES",
}

// Extractor extrae SKUs de rutas de archivo.
type Extractor struct {
	blacklist map[string]bool
}

// New crea un Extractor con el blacklist por defecto más entradas extra opcionales.
func New(extra []string) *Extractor {
	bl := make(map[string]bool)
	for _, s := range DefaultBlacklist {
		bl[strings.ToUpper(s)] = true
	}
	for _, s := range extra {
		bl[strings.ToUpper(s)] = true
	}
	return &Extractor{blacklist: bl}
}

// Extract devuelve todos los SKU candidatos encontrados en la ruta.
// Los candidatos del path (carpeta más profunda primero) preceden al fallback de filename.
func (e *Extractor) Extract(path string) []Result {
	var results []Result
	seen := make(map[string]bool)

	add := func(sku, source, raw string) {
		if !seen[sku] {
			seen[sku] = true
			results = append(results, Result{SKU: sku, Source: source, Raw: raw})
		}
	}

	// path-first: recorrer carpetas de más profunda a más alta
	parts := splitPath(filepath.Dir(path))
	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		norm := strings.ToUpper(part)
		if e.blacklist[norm] {
			continue
		}
		if looksLikeSKU(norm) {
			add(norm, "path", part)
			continue
		}
		// si el nombre de carpeta tiene espacios, probar cada token
		if strings.ContainsRune(norm, ' ') {
			for _, tok := range strings.Fields(norm) {
				if looksLikeSKU(tok) && !e.blacklist[tok] {
					add(tok, "path", part)
					break
				}
			}
		}
	}

	// filename fallback: solo si el path no dio ningún resultado
	if len(results) == 0 {
		stem := trimExt(filepath.Base(path))
		norm := strings.ToUpper(stem)
		// extraer prefijo antes del primer _, espacio o (
		if idx := strings.IndexAny(norm, "_ ("); idx > 0 {
			norm = norm[:idx]
		}
		if looksLikeSKU(norm) && !e.blacklist[norm] {
			add(norm, "filename", stem)
		}
	}

	return results
}

// Primary devuelve el SKU más probable (el primero de Extract), o Result vacío si no hay.
func (e *Extractor) Primary(path string) Result {
	results := e.Extract(path)
	if len(results) == 0 {
		return Result{}
	}
	return results[0]
}

func splitPath(p string) []string {
	return strings.FieldsFunc(p, func(r rune) bool { return r == '/' || r == '\\' })
}

func trimExt(name string) string {
	return strings.TrimSuffix(name, filepath.Ext(name))
}

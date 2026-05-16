package db

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS photos (
  id          INTEGER PRIMARY KEY,
  path        TEXT NOT NULL UNIQUE,
  filename    TEXT NOT NULL,
  root        TEXT NOT NULL,
  sku_primary TEXT,
  size_bytes  INTEGER,
  mtime       INTEGER,
  taken_at    INTEGER,
  width       INTEGER,
  height      INTEGER,
  hash        TEXT,
  indexed_at  INTEGER
);

CREATE TABLE IF NOT EXISTS photo_skus (
  photo_id INTEGER REFERENCES photos(id) ON DELETE CASCADE,
  sku      TEXT NOT NULL,
  source   TEXT,
  PRIMARY KEY (photo_id, sku)
);

CREATE INDEX IF NOT EXISTS idx_photos_root    ON photos(root);
CREATE INDEX IF NOT EXISTS idx_photos_mtime   ON photos(mtime);
CREATE INDEX IF NOT EXISTS idx_photo_skus_sku ON photo_skus(sku);

CREATE VIRTUAL TABLE IF NOT EXISTS photos_fts USING fts5(
  filename, sku_primary, path,
  content='photos', content_rowid='id'
);
`

// DB envuelve la conexión SQLite.
type DB struct {
	sql *sql.DB
}

// Photo es la fila a insertar o actualizar.
type Photo struct {
	Path       string
	Filename   string
	Root       string
	SKUPrimary string
	SizeBytes  int64
	Mtime      time.Time
	IndexedAt  time.Time
	SKUs       []SKUEntry
}

// SKUEntry es un par SKU+fuente para la tabla photo_skus.
type SKUEntry struct {
	SKU    string
	Source string
}

// PhotoRow es el resultado de una consulta de foto.
type PhotoRow struct {
	ID         int64     `json:"id"`
	Path       string    `json:"path"`
	Filename   string    `json:"filename"`
	Root       string    `json:"root"`
	SKUPrimary string    `json:"sku_primary"`
	SizeBytes  int64     `json:"size_bytes"`
	Mtime      time.Time `json:"mtime"`
	IndexedAt  time.Time `json:"indexed_at"`
}

// SearchParams agrupa los parámetros de búsqueda.
type SearchParams struct {
	Q     string
	Root  string
	From  string // fecha ISO "2006-01-02", filtro por mtime >=
	To    string // fecha ISO "2006-01-02", filtro por mtime <=
	Limit int
}

// Stats resume el estado de la DB.
type Stats struct {
	TotalPhotos int64     `json:"total_photos"`
	Roots       []string  `json:"indexed_roots"`
	LastIndexed time.Time `json:"last_indexed"`
}

// Open abre (o crea) la base de datos en path y aplica el schema.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// Una sola conexión evita SQLITE_BUSY cuando múltiples goroutines escriben en paralelo.
	// database/sql serializa las llamadas a nivel Go antes de llegar a SQLite.
	conn.SetMaxOpenConns(1)

	if _, err := conn.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, err
	}
	if _, err := conn.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		return nil, err
	}
	// Esperar hasta 10s si por alguna razón externa hay lock (ej: otro proceso)
	if _, err := conn.Exec("PRAGMA busy_timeout=10000"); err != nil {
		return nil, err
	}
	if _, err := conn.Exec(schema); err != nil {
		conn.Close()
		return nil, err
	}
	return &DB{sql: conn}, nil
}

// Close cierra la conexión.
func (d *DB) Close() error {
	return d.sql.Close()
}

// UpsertPhoto inserta o actualiza una foto y sus SKUs en una sola transacción.
func (d *DB) UpsertPhoto(p Photo) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(`
		INSERT INTO photos (path, filename, root, sku_primary, size_bytes, mtime, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			filename    = excluded.filename,
			root        = excluded.root,
			sku_primary = excluded.sku_primary,
			size_bytes  = excluded.size_bytes,
			mtime       = excluded.mtime,
			indexed_at  = excluded.indexed_at
	`, p.Path, p.Filename, p.Root, nullStr(p.SKUPrimary), p.SizeBytes,
		p.Mtime.Unix(), p.IndexedAt.Unix())
	if err != nil {
		return err
	}

	var photoID int64
	if err := tx.QueryRow(`SELECT id FROM photos WHERE path = ?`, p.Path).Scan(&photoID); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM photo_skus WHERE photo_id = ?`, photoID); err != nil {
		return err
	}
	for _, s := range p.SKUs {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO photo_skus (photo_id, sku, source) VALUES (?, ?, ?)`,
			photoID, s.SKU, s.Source,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// RebuildFTS reconstruye el índice FTS5 desde la tabla photos.
func (d *DB) RebuildFTS() error {
	_, err := d.sql.Exec(`INSERT INTO photos_fts(photos_fts) VALUES('rebuild')`)
	return err
}

// Search busca fotos por SKU (prefix) y texto libre (FTS5).
func (d *DB) Search(p SearchParams) ([]PhotoRow, error) {
	q := strings.TrimSpace(strings.ToUpper(p.Q))
	if q == "" {
		return nil, nil
	}
	limit := p.Limit
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	skuLike := q + "%"
	// FTS5: prefijo entre comillas para tratar literalmente, * para prefix match
	ftsTerm := `"` + strings.ReplaceAll(q, `"`, `""`) + `"*`

	query := `
		SELECT DISTINCT p.id, p.path, p.filename, p.root,
			COALESCE(p.sku_primary, ''), p.size_bytes, p.mtime, p.indexed_at
		FROM photos p
		WHERE p.id IN (
			SELECT photo_id FROM photo_skus WHERE sku LIKE ?
			UNION
			SELECT rowid FROM photos_fts WHERE photos_fts MATCH ?
		)
	`
	args := []any{skuLike, ftsTerm}

	if p.Root != "" {
		query += " AND p.root = ?"
		args = append(args, p.Root)
	}
	if p.From != "" {
		// comparar como Unix timestamp: convertir fecha a inicio del día UTC
		if t, err := time.Parse("2006-01-02", p.From); err == nil {
			query += " AND p.mtime >= ?"
			args = append(args, t.UTC().Unix())
		}
	}
	if p.To != "" {
		if t, err := time.Parse("2006-01-02", p.To); err == nil {
			query += " AND p.mtime <= ?"
			args = append(args, t.UTC().Add(24*time.Hour-time.Second).Unix())
		}
	}
	query += " ORDER BY p.mtime DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.sql.Query(query, args...)
	if err != nil {
		// FTS puede fallar si el índice no fue construido aún; fallback a solo SKU
		return d.searchSKUOnly(skuLike, p, limit)
	}
	defer rows.Close()
	return scanRows(rows)
}

// searchSKUOnly busca solo en photo_skus (sin FTS), usado como fallback.
func (d *DB) searchSKUOnly(skuLike string, p SearchParams, limit int) ([]PhotoRow, error) {
	query := `
		SELECT DISTINCT p.id, p.path, p.filename, p.root,
			COALESCE(p.sku_primary, ''), p.size_bytes, p.mtime, p.indexed_at
		FROM photos p
		JOIN photo_skus ps ON ps.photo_id = p.id
		WHERE ps.sku LIKE ?
	`
	args := []any{skuLike}
	if p.Root != "" {
		query += " AND p.root = ?"
		args = append(args, p.Root)
	}
	query += " ORDER BY p.mtime DESC LIMIT ?"
	args = append(args, limit)

	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRows(rows)
}

// GetPhoto devuelve una foto por ID.
func (d *DB) GetPhoto(id int64) (*PhotoRow, error) {
	rows, err := d.sql.Query(`
		SELECT id, path, filename, root,
			COALESCE(sku_primary, ''), size_bytes, mtime, indexed_at
		FROM photos WHERE id = ?
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	photos, err := scanRows(rows)
	if err != nil || len(photos) == 0 {
		return nil, err
	}
	return &photos[0], nil
}

// Stats devuelve totales y estado general de la DB.
func (d *DB) Stats() (*Stats, error) {
	var s Stats

	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM photos`).Scan(&s.TotalPhotos); err != nil {
		return nil, err
	}

	rows, err := d.sql.Query(`SELECT DISTINCT root FROM photos ORDER BY root`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var root string
		if err := rows.Scan(&root); err == nil {
			s.Roots = append(s.Roots, root)
		}
	}

	var lastUnix sql.NullInt64
	d.sql.QueryRow(`SELECT MAX(indexed_at) FROM photos`).Scan(&lastUnix) //nolint:errcheck
	if lastUnix.Valid {
		s.LastIndexed = time.Unix(lastUnix.Int64, 0).UTC()
	}

	return &s, nil
}

// EachPhoto llama a fn para cada foto en la DB (id, path).
// Útil para pregenerar thumbnails sin cargar todo en memoria.
func (d *DB) EachPhoto(fn func(id int64, path string) error) error {
	rows, err := d.sql.Query(`SELECT id, path FROM photos ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var path string
		if err := rows.Scan(&id, &path); err != nil {
			return err
		}
		if err := fn(id, path); err != nil {
			return err
		}
	}
	return rows.Err()
}

// DeletePhoto elimina una foto de la DB por path (los SKUs se borran en cascade).
func (d *DB) DeletePhoto(path string) error {
	_, err := d.sql.Exec(`DELETE FROM photos WHERE path = ?`, path)
	return err
}

// Count devuelve el total de fotos indexadas.
func (d *DB) Count() (int64, error) {
	var n int64
	err := d.sql.QueryRow(`SELECT COUNT(*) FROM photos`).Scan(&n)
	return n, err
}

func scanRows(rows *sql.Rows) ([]PhotoRow, error) {
	var result []PhotoRow
	for rows.Next() {
		var (
			r              PhotoRow
			mtimeUnix      int64
			indexedAtUnix  int64
		)
		if err := rows.Scan(
			&r.ID, &r.Path, &r.Filename, &r.Root,
			&r.SKUPrimary, &r.SizeBytes, &mtimeUnix, &indexedAtUnix,
		); err != nil {
			return nil, err
		}
		r.Mtime = time.Unix(mtimeUnix, 0).UTC()
		r.IndexedAt = time.Unix(indexedAtUnix, 0).UTC()
		result = append(result, r)
	}
	return result, rows.Err()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}

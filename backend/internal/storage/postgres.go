package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

type DB struct {
	db *sql.DB
}

type Classification struct {
	DriveFileID string
	FileName    string
	MIMEType    string
	Path        string
	Tags        map[string]any
	Source      string
	ConfirmedAt time.Time
}

func Open(databaseURL string) (*DB, error) {
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return nil, nil
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &DB{db: db}
	if err := store.EnsureSchema(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *DB) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *DB) Enabled() bool {
	return s != nil && s.db != nil
}

func (s *DB) EnsureSchema(ctx context.Context) error {
	if !s.Enabled() {
		return nil
	}

	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS document_classifications (
  id BIGSERIAL PRIMARY KEY,
  drive_file_id TEXT,
  file_name TEXT NOT NULL,
  file_name_normalized TEXT,
  mime_type TEXT,
  path TEXT,
  source TEXT NOT NULL,
  confidence DOUBLE PRECISION,
  tags_json JSONB NOT NULL,
  keywords_json JSONB,
  justification TEXT,
  confirmed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE document_classifications
  ADD COLUMN IF NOT EXISTS drive_file_id TEXT,
  ADD COLUMN IF NOT EXISTS file_name TEXT,
  ADD COLUMN IF NOT EXISTS file_name_normalized TEXT,
  ADD COLUMN IF NOT EXISTS mime_type TEXT,
  ADD COLUMN IF NOT EXISTS path TEXT,
  ADD COLUMN IF NOT EXISTS source TEXT,
  ADD COLUMN IF NOT EXISTS confidence DOUBLE PRECISION,
  ADD COLUMN IF NOT EXISTS tags_json JSONB,
  ADD COLUMN IF NOT EXISTS keywords_json JSONB,
  ADD COLUMN IF NOT EXISTS justification TEXT,
  ADD COLUMN IF NOT EXISTS confirmed_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE document_classifications
SET source = 'usuario_validado'
WHERE source IS NULL OR source = '';

UPDATE document_classifications
SET tags_json = '{}'::jsonb
WHERE tags_json IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_document_classifications_drive_file_id_unique
ON document_classifications (drive_file_id)
WHERE drive_file_id IS NOT NULL AND drive_file_id <> '';

CREATE INDEX IF NOT EXISTS idx_document_classifications_file_name_normalized
ON document_classifications (file_name_normalized);
`)
	return err
}

func (s *DB) SaveConfirmedClassification(ctx context.Context, c Classification) error {
	if !s.Enabled() {
		return nil
	}

	if c.Source == "" {
		c.Source = "usuario_validado"
	}
	if c.ConfirmedAt.IsZero() {
		c.ConfirmedAt = time.Now().UTC()
	}

	tagsJSON, err := json.Marshal(c.Tags)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO document_classifications (
  drive_file_id,
  file_name,
  file_name_normalized,
  mime_type,
  path,
  source,
  tags_json,
  confirmed_at,
  updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, NOW())
ON CONFLICT (drive_file_id) WHERE drive_file_id IS NOT NULL AND drive_file_id <> ''
DO UPDATE SET
  file_name = EXCLUDED.file_name,
  file_name_normalized = EXCLUDED.file_name_normalized,
  mime_type = EXCLUDED.mime_type,
  path = EXCLUDED.path,
  source = EXCLUDED.source,
  tags_json = EXCLUDED.tags_json,
  confirmed_at = EXCLUDED.confirmed_at,
  updated_at = NOW();
`, c.DriveFileID, c.FileName, normalizeName(c.FileName), c.MIMEType, c.Path, c.Source, string(tagsJSON), c.ConfirmedAt)

	return err
}

func (s *DB) CountClassifications(ctx context.Context) (int, error) {
	if !s.Enabled() {
		return 0, nil
	}

	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM document_classifications`).Scan(&count)
	return count, err
}

func normalizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	value = strings.Join(strings.Fields(value), " ")
	return value
}

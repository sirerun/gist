package gist

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStore implements the Store interface using PostgreSQL with tsvector
// for full-text search and pg_trgm for trigram substring search.
type PostgresStore struct {
	pool *pgxpool.Pool
}

// NewPostgresStore creates a new PostgresStore connected to the given DSN.
// It creates the required extensions, tables, and indexes if they do not exist.
func NewPostgresStore(ctx context.Context, dsn string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	s := &PostgresStore{pool: pool}
	if err := s.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *PostgresStore) migrate(ctx context.Context) error {
	ddl := `
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE IF NOT EXISTS sources (
	id SERIAL PRIMARY KEY,
	label TEXT NOT NULL,
	created_at TIMESTAMPTZ DEFAULT NOW(),
	bytes_indexed BIGINT DEFAULT 0
);

CREATE TABLE IF NOT EXISTS chunks (
	id SERIAL PRIMARY KEY,
	source_id INT REFERENCES sources(id),
	content TEXT NOT NULL,
	heading_path TEXT,
	content_type TEXT,
	start_byte INT,
	end_byte INT,
	chunk_index INT,
	tsv TSVECTOR
);

CREATE INDEX IF NOT EXISTS idx_chunks_tsv ON chunks USING GIN (tsv);
CREATE INDEX IF NOT EXISTS idx_chunks_trgm ON chunks USING GIN (content gin_trgm_ops);
`
	_, err := s.pool.Exec(ctx, ddl)
	return err
}

// SaveSource creates a new source record.
func (s *PostgresStore) SaveSource(ctx context.Context, label string, format Format) (Source, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO sources (label) VALUES ($1) RETURNING id`,
		label,
	).Scan(&id)
	if err != nil {
		return Source{}, fmt.Errorf("insert source: %w", err)
	}
	return Source{
		ID:     id,
		Label:  label,
		Format: format,
	}, nil
}

// SaveChunk persists a chunk and updates the tsvector and source byte count.
func (s *PostgresStore) SaveChunk(ctx context.Context, chunk Chunk) (Chunk, error) {
	var id int
	err := s.pool.QueryRow(ctx,
		`INSERT INTO chunks (source_id, content, heading_path, content_type, start_byte, end_byte, chunk_index, tsv)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, to_tsvector('english', $2))
		 RETURNING id`,
		chunk.SourceID, chunk.Content, chunk.HeadingPath, chunk.ContentType,
		chunk.ByteStart, chunk.ByteEnd, 0,
	).Scan(&id)
	if err != nil {
		return Chunk{}, fmt.Errorf("insert chunk: %w", err)
	}
	chunk.ID = id

	_, err = s.pool.Exec(ctx,
		`UPDATE sources SET bytes_indexed = bytes_indexed + $1 WHERE id = $2`,
		int64(len(chunk.Content)), chunk.SourceID,
	)
	if err != nil {
		return Chunk{}, fmt.Errorf("update source bytes: %w", err)
	}
	return chunk, nil
}

// SearchPorter performs full-text search using tsvector with porter stemming.
func (s *PostgresStore) SearchPorter(ctx context.Context, params SearchParams) ([]SearchMatch, error) {
	query := `
		SELECT c.id, c.source_id, c.heading_path, c.content, c.content_type,
		       ts_rank(c.tsv, plainto_tsquery('english', $1)) AS score
		FROM chunks c`
	args := []any{params.Query}
	argIdx := 2

	if params.SourceFilter != "" {
		query += fmt.Sprintf(` JOIN sources s ON s.id = c.source_id AND s.label = $%d`, argIdx)
		args = append(args, params.SourceFilter)
		argIdx++
	}

	query += ` WHERE c.tsv @@ plainto_tsquery('english', $1)
		ORDER BY score DESC`

	if params.Limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, params.Limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search porter: %w", err)
	}
	defer rows.Close()

	var matches []SearchMatch
	for rows.Next() {
		var m SearchMatch
		if err := rows.Scan(&m.ChunkID, &m.SourceID, &m.HeadingPath, &m.Content, &m.ContentType, &m.Score); err != nil {
			return nil, fmt.Errorf("scan porter result: %w", err)
		}
		m.MatchLayer = "porter"
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

// SearchTrigram performs substring search using pg_trgm similarity scoring.
func (s *PostgresStore) SearchTrigram(ctx context.Context, params SearchParams) ([]SearchMatch, error) {
	query := `
		SELECT c.id, c.source_id, c.heading_path, c.content, c.content_type,
		       similarity(c.content, $1) AS score
		FROM chunks c`
	args := []any{params.Query}
	argIdx := 2

	if params.SourceFilter != "" {
		query += fmt.Sprintf(` JOIN sources s ON s.id = c.source_id AND s.label = $%d`, argIdx)
		args = append(args, params.SourceFilter)
		argIdx++
	}

	query += ` WHERE c.content ILIKE '%' || $1 || '%'
		ORDER BY score DESC`

	if params.Limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, params.Limit)
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("search trigram: %w", err)
	}
	defer rows.Close()

	var matches []SearchMatch
	for rows.Next() {
		var m SearchMatch
		if err := rows.Scan(&m.ChunkID, &m.SourceID, &m.HeadingPath, &m.Content, &m.ContentType, &m.Score); err != nil {
			return nil, fmt.Errorf("scan trigram result: %w", err)
		}
		m.MatchLayer = "trigram"
		matches = append(matches, m)
	}
	return matches, rows.Err()
}

// VocabularyTerms returns distinct words from all indexed chunk content using ts_stat.
func (s *PostgresStore) VocabularyTerms(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT word FROM ts_stat('SELECT tsv FROM chunks')`)
	if err != nil {
		return nil, fmt.Errorf("vocabulary terms: %w", err)
	}
	defer rows.Close()

	var terms []string
	for rows.Next() {
		var term string
		if err := rows.Scan(&term); err != nil {
			return nil, fmt.Errorf("scan term: %w", err)
		}
		terms = append(terms, term)
	}
	return terms, rows.Err()
}

// Sources returns all indexed sources.
func (s *PostgresStore) Sources(ctx context.Context) ([]Source, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, label, bytes_indexed FROM sources ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}
	defer rows.Close()

	var sources []Source
	for rows.Next() {
		var src Source
		if err := rows.Scan(&src.ID, &src.Label, &src.BytesIndexed); err != nil {
			return nil, fmt.Errorf("scan source: %w", err)
		}
		sources = append(sources, src)
	}
	return sources, rows.Err()
}

// Stats returns aggregate statistics about the store contents.
func (s *PostgresStore) Stats(ctx context.Context) (StoreStats, error) {
	var stats StoreStats
	err := s.pool.QueryRow(ctx,
		`SELECT
			COALESCE((SELECT COUNT(*) FROM sources), 0),
			COALESCE((SELECT COUNT(*) FROM chunks), 0),
			COALESCE((SELECT SUM(bytes_indexed) FROM sources), 0)`).
		Scan(&stats.SourceCount, &stats.ChunkCount, &stats.BytesIndexed)
	if err != nil {
		return StoreStats{}, fmt.Errorf("stats: %w", err)
	}
	return stats, nil
}

// Close releases the connection pool.
func (s *PostgresStore) Close() error {
	s.pool.Close()
	return nil
}

// Compile-time check that PostgresStore implements Store.
var _ Store = (*PostgresStore)(nil)

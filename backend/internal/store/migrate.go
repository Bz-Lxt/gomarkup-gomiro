package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func (d *DB) Migrate(ctx context.Context, dir string) error {
	conn, err := d.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(72618431)`); err != nil {
		return fmt.Errorf("advisory lock: %w", err)
	}
	defer func() { _, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(72618431)`) }()

	if err := d.ensureTable(ctx); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)
	applied, err := d.applied(ctx)
	if err != nil {
		return err
	}
	for _, name := range files {
		if applied[name] {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		up := extractUp(string(raw))
		if _, err := d.Pool.Exec(ctx, up); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := d.Pool.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES ($1) ON CONFLICT DO NOTHING`, name); err != nil {
			return fmt.Errorf("record %s: %w", name, err)
		}
	}
	return nil
}

func (d *DB) ensureTable(ctx context.Context) error {
	_, err := d.Pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)
	return err
}

func (d *DB) applied(ctx context.Context) (map[string]bool, error) {
	rows, err := d.Pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func extractUp(sql string) string {
	const markerDown = "-- +migrate Down"
	if i := strings.Index(sql, markerDown); i >= 0 {
		sql = sql[:i]
	}
	return strings.TrimSpace(strings.ReplaceAll(sql, "-- +migrate Up", ""))
}

func extractDown(sql string) string {
	const markerDown = "-- +migrate Down"
	i := strings.Index(sql, markerDown)
	if i < 0 {
		return ""
	}
	return strings.TrimSpace(sql[i+len(markerDown):])
}

func (d *DB) AppliedVersions(ctx context.Context) ([]string, error) {
	m, err := d.applied(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}

func (d *DB) HasVersion(ctx context.Context, name string) (bool, error) {
	m, err := d.applied(ctx)
	if err != nil {
		return false, err
	}
	return m[name], nil
}

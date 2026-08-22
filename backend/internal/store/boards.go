package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"gomiro/internal/model"
	"gomiro/internal/timeutil"
)

var ErrNotFound = errors.New("not found")

func (d *DB) ListBoards(ctx context.Context) ([]model.Board, error) {
	rows, err := d.Pool.Query(ctx, `
SELECT id, title, pass_hash, thumbnail, created_at, updated_at
FROM boards ORDER BY updated_at DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Board
	for rows.Next() {
		var b model.Board
		if err := rows.Scan(&b.ID, &b.Title, &b.PassHash, &b.Thumbnail, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		b.HasPass = b.PassHash != ""
		out = append(out, b)
	}
	return out, rows.Err()
}

func (d *DB) GetBoard(ctx context.Context, id string) (model.Board, error) {
	var b model.Board
	err := d.Pool.QueryRow(ctx, `
SELECT id, title, pass_hash, thumbnail, created_at, updated_at
FROM boards WHERE id=$1`, id).Scan(&b.ID, &b.Title, &b.PassHash, &b.Thumbnail, &b.CreatedAt, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrNotFound
	}
	b.HasPass = b.PassHash != ""
	return b, err
}

func (d *DB) InsertBoard(ctx context.Context, b model.Board) error {
	_, err := d.Pool.Exec(ctx, `
INSERT INTO boards(id, title, pass_hash, thumbnail, created_at, updated_at)
VALUES ($1,$2,$3,$4,$5,$6)`, b.ID, b.Title, b.PassHash, b.Thumbnail, b.CreatedAt, b.UpdatedAt)
	return err
}

func (d *DB) UpdateBoard(ctx context.Context, id, title, passHash, thumbnail string) error {
	now := timeutil.Now()
	tag, err := d.Pool.Exec(ctx, `
UPDATE boards SET
  title = COALESCE(NULLIF($2,''), title),
  pass_hash = CASE WHEN $3 = '__keep__' THEN pass_hash ELSE $3 END,
  thumbnail = COALESCE(NULLIF($4,''), thumbnail),
  updated_at = $5
WHERE id=$1`, id, title, passHash, thumbnail, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) DeleteBoard(ctx context.Context, id string) error {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM boards WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) TouchBoard(ctx context.Context, id string) error {
	_, err := d.Pool.Exec(ctx, `UPDATE boards SET updated_at=$2 WHERE id=$1`, id, timeutil.Now())
	return err
}

func (d *DB) InsertUpload(ctx context.Context, hash, mime string, bytes int) error {
	_, err := d.Pool.Exec(ctx, `
INSERT INTO uploads(hash, mime, bytes, created_at)
VALUES ($1,$2,$3,$4)
ON CONFLICT (hash) DO NOTHING`, hash, mime, bytes, timeutil.Now())
	if err != nil {
		return fmt.Errorf("insert upload: %w", err)
	}
	return nil
}

func (d *DB) GetUpload(ctx context.Context, hash string) (mime string, ok bool, err error) {
	err = d.Pool.QueryRow(ctx, `SELECT mime FROM uploads WHERE hash=$1`, hash).Scan(&mime)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return mime, err == nil, err
}

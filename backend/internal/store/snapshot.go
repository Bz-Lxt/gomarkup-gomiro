package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"gomiro/internal/engine"
	"gomiro/internal/model"
	"gomiro/internal/timeutil"
)

func (d *DB) SaveSnapshot(ctx context.Context, snap *model.Snapshot) error {
	if snap == nil {
		return nil
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	_, err = d.Pool.Exec(ctx, `
INSERT INTO board_snapshots(board_id, server_seq, payload, updated_at)
VALUES ($1,$2,$3,$4)
ON CONFLICT (board_id) DO UPDATE SET
  server_seq = EXCLUDED.server_seq,
  payload = EXCLUDED.payload,
  updated_at = EXCLUDED.updated_at`,
		snap.BoardID, int64(snap.ServerSeq), raw, timeutil.Now())
	return err
}

func (d *DB) LoadSnapshot(ctx context.Context, boardID string) (*model.Snapshot, error) {
	var raw []byte
	err := d.Pool.QueryRow(ctx, `SELECT payload FROM board_snapshots WHERE board_id=$1`, boardID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return &model.Snapshot{BoardID: boardID, Shapes: map[string]*model.Shape{}}, nil
	}
	if err != nil {
		return nil, err
	}
	return engine.UnmarshalSnapshot(raw)
}

func (d *DB) RebuildDocument(ctx context.Context, boardID string) (*engine.Document, error) {
	doc := engine.NewDocument(boardID)
	snap, err := d.LoadSnapshot(ctx, boardID)
	if err != nil {
		return nil, err
	}
	doc.Restore(snap)
	ops, err := d.OpsAfter(ctx, boardID, doc.ServerSeq, 10000)
	if err != nil {
		return nil, err
	}
	for _, op := range ops {
		if err := doc.ApplyRemote(op); err != nil {
			// If a hole is found, prefer snapshot-only rather than a corrupt doc.
			return doc, err
		}
	}
	doc.Dirty = false
	return doc, nil
}

func (d *DB) RebuildReport(ctx context.Context, boardID string) (*engine.Document, engine.RebuildReport, error) {
	doc, err := d.RebuildDocument(ctx, boardID)
	if err != nil && doc != nil {
		return doc, engine.RebuildReport{BoardID: boardID, Hole: true, ToSeq: doc.ServerSeq, Live: doc.LiveCount()}, err
	}
	if err != nil {
		return nil, engine.RebuildReport{BoardID: boardID}, err
	}
	return doc, engine.RebuildReport{
		BoardID: boardID,
		ToSeq:   doc.ServerSeq,
		Live:    doc.LiveCount(),
	}, nil
}

func (d *DB) SnapshotSeq(ctx context.Context, boardID string) (uint64, error) {
	var seq int64
	err := d.Pool.QueryRow(ctx, `SELECT server_seq FROM board_snapshots WHERE board_id=$1`, boardID).Scan(&seq)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return uint64(seq), err
}

package store

import (
	"context"
	"encoding/json"
	"time"

	"gomiro/internal/protocol"
	"gomiro/internal/timeutil"
)

type OpRow struct {
	BoardID   string
	ServerSeq uint64
	AuthorID  string
	Kind      string
	Payload   []byte
	CreatedAt time.Time
}

func (d *DB) AppendOp(ctx context.Context, boardID string, seq uint64, author, kind string, payload []byte) error {
	_, err := d.Pool.Exec(ctx, `
INSERT INTO op_logs(board_id, server_seq, author_id, kind, payload, created_at)
VALUES ($1,$2,$3,$4,$5,$6)
ON CONFLICT (board_id, server_seq) DO NOTHING`,
		boardID, int64(seq), author, kind, payload, timeutil.Now())
	return err
}

func (d *DB) OpsAfter(ctx context.Context, boardID string, after uint64, limit int) ([]protocol.OpBroadcast, error) {
	if limit <= 0 || limit > 5000 {
		limit = 2000
	}
	rows, err := d.Pool.Query(ctx, `
SELECT server_seq, author_id, kind, payload
FROM op_logs
WHERE board_id=$1 AND server_seq>$2
ORDER BY server_seq ASC
LIMIT $3`, boardID, int64(after), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []protocol.OpBroadcast
	for rows.Next() {
		var (
			seq    int64
			author string
			kind   string
			raw    []byte
		)
		if err := rows.Scan(&seq, &author, &kind, &raw); err != nil {
			return nil, err
		}
		var env protocol.OpBroadcast
		if json.Unmarshal(raw, &env) != nil {
			env = protocol.OpBroadcast{Patch: raw, Kind: kind, AuthorID: author, ServerSeq: uint64(seq)}
		}
		env.ServerSeq = uint64(seq)
		env.AuthorID = author
		env.Kind = kind
		out = append(out, env)
	}
	return out, rows.Err()
}

func (d *DB) CompactOps(ctx context.Context, boardID string, upto uint64) error {
	_, err := d.Pool.Exec(ctx, `DELETE FROM op_logs WHERE board_id=$1 AND server_seq<=$2`, boardID, int64(upto))
	return err
}

func (d *DB) OpStats(ctx context.Context, boardID string) (count int, minSeq, maxSeq uint64, err error) {
	var c, mn, mx int64
	err = d.Pool.QueryRow(ctx, `
SELECT COUNT(*), COALESCE(MIN(server_seq),0), COALESCE(MAX(server_seq),0)
FROM op_logs WHERE board_id=$1`, boardID).Scan(&c, &mn, &mx)
	return int(c), uint64(mn), uint64(mx), err
}

func (d *DB) LatestOp(ctx context.Context, boardID string) (*protocol.OpBroadcast, error) {
	var seq int64
	var author, kind string
	var raw []byte
	err := d.Pool.QueryRow(ctx, `
SELECT server_seq, author_id, kind, payload
FROM op_logs WHERE board_id=$1
ORDER BY server_seq DESC LIMIT 1`, boardID).Scan(&seq, &author, &kind, &raw)
	if err != nil {
		return nil, err
	}
	var env protocol.OpBroadcast
	_ = json.Unmarshal(raw, &env)
	env.ServerSeq = uint64(seq)
	env.AuthorID = author
	env.Kind = kind
	return &env, nil
}

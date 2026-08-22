package protocol

const (
	ProtoVersion = 1

	MaxNicknameRunes  = 24
	MaxTitleRunes     = 80
	MaxTextRunes      = 4000
	MaxPoints         = 4000
	MaxPatchFields    = 32
	MaxSelection      = 256
	MaxCreateBatch    = 64
	MaxCoordAbs       = 1e7
	MaxSize           = 1e6
	MinSize           = 0.1
	MaxStrokeW        = 64
	MaxFontSize       = 400
	MaxGroupMembers   = 256
	MaxJSONBytes      = 256 << 10

	TypeJoin          = "join"
	TypeOp            = "op"
	TypeCursor        = "cursor"
	TypeSelection     = "selection"
	TypePing          = "ping"

	TypeJoined        = "joined"
	TypeOpAck         = "op_ack"
	TypeOpReject      = "op_reject"
	TypeOpBcast       = "op_bcast"
	TypeCursorBcast   = "cursor_bcast"
	TypeMemberJoin    = "member_join"
	TypeMemberLeave   = "member_leave"
	TypeResync        = "resync_required"
	TypeError         = "error"
	TypePong          = "pong"
	TypeShutdown      = "server_shutdown"

	OpCreate          = "shape.create"
	OpUpdate          = "shape.update"
	OpDelete          = "shape.delete"
	OpReorder         = "shape.reorder"
	OpGroup           = "shapes.group"
	OpUngroup         = "shapes.ungroup"

	RejectStale       = "stale_base"
	RejectDeleted     = "deleted"
	RejectUnknown     = "unknown_shape"
	RejectInvalid     = "invalid"
	RejectDuplicate   = "duplicate"

	BinCursorC2S byte = 0x01
	BinCursorS2C byte = 0x02
	BinCursorSize     = 1 + 4 + 4 // type + x + y (client)
)

var AllowedOpKinds = map[string]struct{}{
	OpCreate: {}, OpUpdate: {}, OpDelete: {},
	OpReorder: {}, OpGroup: {}, OpUngroup: {},
}

func KnownOpKind(k string) bool {
	_, ok := AllowedOpKinds[k]
	return ok
}

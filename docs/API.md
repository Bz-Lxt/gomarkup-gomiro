# GoMiro API

时区：GMT+8。JSON 日期字段对外展示为 `yyyy-MM-dd HH:mm:ss`。
错误体统一：`{"code":"string","message":"string"}`。

## 1. HTTP

基址：`http://localhost:18431`（经 nginx）或 `http://localhost:18432`（api-1）。

### GET /healthz

存活探针，不探测依赖。

```json
{"status":"ok","time":"2026-08-22 23:20:00"}
```

### GET /readyz

真实 ping PostgreSQL 与 Redis。任一失败返回 503。

```json
{"status":"ok","db":true,"redis":true,"time":"2026-08-22 23:20:00"}
```

| 错误码 | HTTP | 含义 |
|---|---|---|
| — | 503 | db 或 redis 不可达 |

### GET /metrics

Prometheus 文本。指标：`gomiro_connections` `gomiro_rooms` `gomiro_ops_total` `gomiro_ops_rejected` `gomiro_cursors_dropped` `gomiro_slow_disconnects` `gomiro_broadcast_*`。

### GET /api/v1/boards

```json
{"items":[{"id":"b_ab12","title":"架构评审","hasPass":false,"thumbnail":"","createdAt":"2026-08-22 23:00:00","updatedAt":"2026-08-22 23:10:00"}]}
```

### POST /api/v1/boards

请求：`{"title":"架构评审","passcode":""}`  
响应 201：`{"id":"b_ab12","title":"架构评审","hasPass":false,"createdAt":"...","updatedAt":"..."}`

| 错误码 | HTTP | 含义 |
|---|---|---|
| bad_json | 400 | 体无法解析 |
| bad_field | 400 | 标题过长 |

### GET /api/v1/boards/{id}

`{id}` 不得含 `/`（ServeMux 通配符仅允许出现在 pattern 末尾）。

404：`{"code":"not_found","message":"board not found"}`

### PATCH /api/v1/boards/{id}

`{"title":"新名","passcode":"secret","clearPass":false,"thumbnail":""}`

### DELETE /api/v1/boards/{id}

204 无体。

### POST /api/v1/boards/{id}/unlock

`{"passcode":"secret"}` → `{"ok":true,"id":"b_ab12"}`  
口令错误 403 `forbidden`。

### POST /api/v1/uploads

`multipart/form-data` 字段 `file`。MIME 白名单 png/jpeg/webp/gif，上限 5MB。按 SHA-256 去重。

```json
{"hash":"…64 hex…","mime":"image/png","bytes":1024,"url":"/api/v1/files/<hash>"}
```

### GET /api/v1/files/{hash}

不可变缓存。hash 必须 64 位小写十六进制。

---

## 2. WebSocket `/ws`

升级后**首条必须为 JSON join**。其后 JSON 文本或二进制光标。

### Envelope

```json
{"v":1,"type":"join","id":"optional","payload":{}}
```

`v` 必须为 `1`，否则 `bad_version`。

### C→S

| type | payload | 说明 |
|---|---|---|
| join | boardId, nickname, color, passcode?, lastSeq, protoVersion | 首包 |
| op | clientOpId, lamport, baseVersion, opKind, targetId, patch | 增量 |
| selection | shapeIds[] | 远端选中 |
| ping | — | 应用层心跳 |

opKind：`shape.create` `shape.update` `shape.delete` `shape.reorder` `shapes.group` `shapes.ungroup`。

create patch：`{"shape":{...}}`  
update patch：只含变更字段  
delete patch：`{"ids":["shp_…"]}`

### S→C

| type | 要点 |
|---|---|
| joined | selfId, userIdx, members, snapshot, serverSeq, missed? |
| op_ack | clientOpId, serverSeq, acceptedVersion |
| op_reject | clientOpId, reason(`stale_base`/`deleted`/`unknown_shape`/`invalid`/`duplicate`), authoritativeShape? |
| op_bcast | serverSeq, authorId, opKind, targetId, patch, lamport, version |
| member_join / member_leave | member |
| selection_bcast | clientId, shapeIds |
| resync_required | reason, snapshot, serverSeq |
| error | code, message |
| pong | — |
| server_shutdown | at |

### 二进制光标

C→S：`0x01` + `float32le x` + `float32le y`（9 字节）  
S→C：`0x02` + `uint8 count` + N × (`uint32 userIdx` + `f32 x` + `f32 y`)

NaN/Inf 被拒绝。客户端发送 ≤ 30Hz，服务端按 tick 合并广播。

### 冲突

属性级 LWW：无字段交集则 rebase 接受；有交集则 `stale_base` 并回传权威 shape。已删除图形的 update 一律 `deleted`（Delete Wins）。同一 `clientOpId` 幂等。

### 限制

单消息 256KB；单连接 200 op/s；坐标绝对值 ≤ 1e7；涂鸦点 ≤ 4000。

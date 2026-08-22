# GoMiro 交付路线图

> **权威性**：本文件定义 **WHEN**。WHAT 以 `docs/Requirements.md` 为准。
> **版本**：v1.0 ｜ **日期**：2026-08-22 (GMT+8)

---

## Phase Order Decision

**采用 Logic-First：交换 SOP 默认的 Phase 2（UI）与 Phase 3（Logic）。**

理由：画布是 SOP v13 明列的 editor / canvas 情形——工具栏、选中态、渲染分层、Undo 栈全部从 `Shape` schema 与增量协议派生。先冻结协议与冲突引擎，再写渲染层，避免前端按臆测字段返工。

执行顺序：**Phase 1 Architect → Phase 3 Logic → Phase 2 UI → Phase 4 QA → Phase 5 Audit**。

不创建 `frontend-admin` / `frontend-mp`：Requirements 未定义管理端或小程序，创建空壳属于 Scope Drift。

---

## 分期边界

| 阶段 | 范围 | 状态 |
|---|---|---|
| **MVP** | 单节点房间、双通道协议、冲突引擎、8 类图元、无限画布、光标/成员、快照+Op 日志、Undo、compose 一键启动 | 本期必须 |
| **V1** | Redis 房间所有权 + Pub/Sub 双节点、图片图元、对齐/成组/层级、白板 CRUD/口令/缩略图、导出、metrics、压测脚本 | 本期完成 |
| **V2** | CRDT、富文本、历史回放 UI、完整用户体系、Follow/音视频 | **明确不做** |

---

## 目录骨架

```
backend/                 Go 服务（25–35 生产文件，5500–8000 行）
frontend-user/           Vue 3 + Vite + Tailwind + Pinia
tests/                   API smoke + Playwright E2E + Go 压测客户端
docs/                    SSOT 与审计记录
docker-compose.yml       随机高位端口（开发）
```

---

## 开发端口（已探测空闲）

| 服务 | 宿主机 | 容器 |
|---|---|---|
| 用户前端 (nginx) | **18431** | 80 |
| API 节点 1 | **18432** | 8080 |
| API 节点 2 | **18433** | 8080 |
| PostgreSQL 16 | **15432** | 5432 |
| Redis 7 | **16379** | 6379 |

`/deploy` 阶段再改为 8081+。1848x 段按知识库规避。

---

## 任务清单

### A. 基础设施
- [x] A1 git init + .gitignore
- [x] A2 docker-compose.yml（2 backend + pg + redis + nginx）
- [x] A3 多阶段 Dockerfile（CN 镜像源、跨平台）
- [x] A4 版本化 SQL 迁移
- [x] A5 `.env.example` + 统一 slog

### B. 协议与冲突引擎（Logic-First 核心）
- [x] B1 Shape / Board / Member 模型
- [x] B2 JSON Envelope + 二进制光标帧
- [x] B3 入站强校验（NaN/长度/类型）
- [x] B4 属性级 LWW + Delete Wins + 幂等
- [x] B5 分数 z-index、几何 AABB

### C. 房间集群
- [x] C1 Hub / Room / Client + 单写协程
- [x] C2 心跳、慢客户端、空房回收、优雅停机
- [x] C3 Redis 房间所有权 + inbox/out + 光标总线
- [x] C4 快照 + Op 日志 + 压实 + 重连补齐

### D. HTTP / 上传
- [x] D1 Board CRUD + 口令 bcrypt
- [x] D2 上传（5MB / MIME / 内容哈希）
- [x] D3 /healthz /readyz /metrics

### E. 前端画布
- [x] E1 DesignSpec + 首页白板列表
- [x] E2 无限画布（平移/缩放/裁剪/网格/小地图）
- [x] E3 图元绘制与编辑（含成组、对齐、层级）
- [x] E4 双通道 WS、光标、成员、远端选中、Undo
- [x] E5 导出 PNG/SVG/JSON、连接态、主题

### F. 质量
- [x] F1 引擎/协议/房间单元测试
- [x] F2 API smoke + Playwright 双窗口协同
- [x] F3 压测脚本（1000 连接）
- [x] F4 docs/API.md · QA_Record · AuditReport

---

## 后端生产文件规划（32 个）

`cmd/server/main.go`  
`internal/config/config.go` · `internal/logx/logger.go` · `internal/timeutil/beijing.go`  
`internal/model/{ids,shape,board,member}.go`  
`internal/protocol/{envelope,binary,validate,limits}.go`  
`internal/engine/{document,conflict,apply,zorder,geometry}.go`  
`internal/ws/{hub,room,client,upgrader}.go`  
`internal/httpx/{router,board,health,metrics,upload,middleware}.go`  
`internal/store/{db,migrate,boards,oplog,snapshot}.go`  
`internal/cluster/bus.go`

测试文件（`*_test.go`）与 `cmd/loadtest` **不计入** 25–35 预算。

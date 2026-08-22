# GoMiro 需求规格说明书（SSOT）

> **文档权威性**：本文件定义 **WHAT**（做什么）。`docs/Roadmap.md` 定义 **WHEN**（何时做）。
> **版本**：v1.0 ｜ **冻结时间**：2026-08-22 14:13 (GMT+8) ｜ **状态**：🔒 FROZEN
> **PM 判定**：ACCEPT（规模 10k–40k LoC 区间，分期 Roadmap 为强制前置条件）

---

## 1. 项目定位

GoMiro 是一个**实时多人协同白板与流程图绘制系统**（Mini Miro / Excalidraw 形态）。
核心工程价值不在"画得多好看"，而在于**高并发连接管理、增量同步协议设计、并发冲突收敛**三件事。

**一句话验收标准**：两个浏览器窗口打开同一白板，A 拖动一个矩形，B 在 100ms 内看到该矩形移动；A 和 B 在同一毫秒拖动同一矩形，松手后两端画布状态**完全一致**。

---

## 2. 技术栈裁决（Decision Record）

原始需求存在 6 处"二选一"表述，此处逐条锁定。**后续 Phase 不得推翻本节裁决**（Anti-Flip-Flop）。

| 维度 | 裁决 | 理由 |
|---|---|---|
| 后端语言 | **Go 1.24** | 需求指定 |
| WebSocket 库 | **github.com/gorilla/websocket** | 生态最成熟、连接生命周期 API 清晰；`coder/websocket` 作为备选但不在本期采用 |
| HTTP 路由 | **标准库 net/http (Go 1.22+ ServeMux)** | 零依赖；注意 `{name...}` 通配符只能位于 pattern 末尾（见 §11 已知陷阱） |
| 前端框架 | **Vue 3 + TypeScript + Vite** | 需求给出 "Vue 3 / Svelte" 二选一，取 Vue 3 |
| 渲染层 | **Canvas 2D 分层渲染**（非 SVG） | SVG 在 5000+ 节点时 DOM 开销不可接受；分层 = 静态图形层 + 交互覆盖层 |
| UI 组件 | **Tailwind CSS**（自建轻组件） | 满足"Dribbble 标准"，避免重型组件库拖慢画布页 |
| 前端状态 | **Pinia** | 白板文档状态 + 在线成员状态分 store |
| 同步协议 | **双通道**：结构化 Op 用 **JSON**，高频光标/拖拽预览用 **紧凑二进制帧** | 需求为"二进制 **或** JSON"，二者不互斥；JSON 保可调试性，二进制保高频吞吐 |
| 冲突判定 | **服务端单调序列号（权威全序）+ Lamport 时钟（客户端因果序）+ 属性级 LWW** | 需求为"版本号 **或** 时间戳"；纯客户端墙钟时间因时钟漂移不可信，必须以服务端序列号为最终仲裁 |
| 持久化 | **PostgreSQL 16** | 白板元数据、图形快照、Op 日志 |
| 集群总线 | **Redis 7 Pub/Sub** | 跨节点房间广播；MVP 阶段可单节点降级（见 §7 分期） |
| 时区 | **Asia/Shanghai (GMT+8)** | 全局规范；容器 `TZ=Asia/Shanghai`，Go 侧统一 `time.Local` 且入库用带时区类型 |

---

## 3. 功能需求

### 3.1 无限画布（Infinite Canvas）— P0

| ID | 需求 | 验收方式 |
|---|---|---|
| FC-01 | 平移：鼠标中键拖拽 / 空格+拖拽 / 触控板双指 | 平移后视口坐标正确，图形不抖动 |
| FC-02 | 缩放：滚轮缩放，以**光标位置为锚点**，范围 10%–500% | 缩放后光标下的图形点保持在光标处（锚点不漂移） |
| FC-03 | 世界坐标系与屏幕坐标系解耦 | 所有图形以世界坐标存储，渲染时经 viewport 变换 |
| FC-04 | 视口裁剪（Viewport Culling） | 仅渲染与视口 AABB 相交的图形；5000 图形下平移 FPS ≥ 50 |
| FC-05 | 网格背景随缩放自适应（点阵/线网格切档） | 缩放极值下网格不产生摩尔纹或过密 |
| FC-06 | 小地图 / 「回到内容」按钮 | 视口远离所有内容时可一键归位 |

### 3.2 图形绘制与编辑 — P0

支持图元类型（**最小集，不可再裁**）：

| 类型 | 说明 |
|---|---|
| `rect` | 矩形（含圆角） |
| `ellipse` | 椭圆/圆 |
| `diamond` | 菱形（流程图判定节点） |
| `line` | 直线 |
| `arrow` | 箭头（支持**吸附到图形锚点**，端点随图形移动） |
| `freedraw` | 自由涂鸦笔迹（点序列 + 压感宽度模拟） |
| `text` | 文字（字号、颜色、对齐） |
| `sticky` | 便签（背景色 + 内嵌文字 + 自动换行） |
| `image` | 图片（本地上传，存 Drive/对象目录，仅存引用） |

编辑能力：

| ID | 需求 |
|---|---|
| SH-01 | 选中（单选/框选/Shift 多选）、移动、缩放（8 handle）、旋转 |
| SH-02 | 样式面板：描边色、填充色、线宽、线型（实线/虚线）、透明度、字号 |
| SH-03 | 层级：置顶/置底/上移一层/下移一层（z-index 为分数索引，避免整表重排） |
| SH-04 | 删除、复制粘贴、`Ctrl+D` 原位复制 |
| SH-05 | **本地 Undo/Redo**：撤销栈仅回滚"本人"的操作，不得撤销他人操作 |
| SH-06 | 成组 / 解组 |
| SH-07 | 对齐辅助线（拖拽时吸附到邻近图形边/中心线） |
| SH-08 | 快捷键：`V`选择 `R`矩形 `O`椭圆 `D`菱形 `L`线 `A`箭头 `P`画笔 `T`文字 `N`便签 |

### 3.3 多人协同 — P0（本项目核心）

| ID | 需求 | 验收方式 |
|---|---|---|
| CO-01 | **实时光标同步**：显示他人光标位置 + 昵称标签 + 用户色 | 双窗口对比，光标延迟 P95 < 80ms |
| CO-02 | 光标发送节流：客户端 ≤ 30Hz 采样，服务端按 tick 合并广播 | 抓包确认单用户光标消息 ≤ 30 msg/s |
| CO-03 | **在线成员列表**：头像/首字母、昵称、颜色，加入/离开实时增删 | 关闭一个窗口，另一窗口 3s 内移除该成员 |
| CO-04 | **远端选中框**：他人正在选中/拖拽的图形显示其用户色虚框 | 视觉上可区分"谁在动谁" |
| CO-05 | **增量广播**：只广播变更 Op，禁止全量画布 | 抓包确认单次 move 消息体 ≤ 120 bytes(JSON) |
| CO-06 | **乐观本地应用 + 服务端权威回执**：本地立即渲染，服务端拒绝时回滚 | 人为注入冲突后画布收敛 |
| CO-07 | **断线重连增量补齐**：客户端记录 `lastServerSeq`，重连时请求 `seq >` 的 Op | 断网 10s 后恢复，画布与对端一致 |
| CO-08 | **序号空洞检测**：客户端发现 seq 不连续，主动触发全量重同步 | 人为丢帧后能自愈 |
| CO-09 | 跟随模式（Follow）：点击某成员，视口跟随其视口 | P1，可延后 |

### 3.4 白板管理 — P1

| ID | 需求 |
|---|---|
| BD-01 | 白板列表：创建、重命名、删除、缩略图 |
| BD-02 | 匿名身份：首次进入输入昵称，随机分配用户色，存 localStorage |
| BD-03 | 房间口令（可选）：设置了口令的白板需校验后才能加入 |
| BD-04 | 分享链接：`/board/{boardId}` 直达 |
| BD-05 | 导出：PNG / SVG / JSON |

> **Scope Drift 防线**：**不做**完整用户注册登录体系、不做权限 RBAC、不做评论、不做视频通话、不做模板市场。原始 Prompt 未提及这些。

---

## 4. 后端架构需求（Go）

### 4.1 房间集群模型 — P0

| ID | 需求 |
|---|---|
| BE-01 | **Hub / Room / Client 三层模型**：Hub 管理 Room 索引，Room 持有成员集合，Client 封装单连接 |
| BE-02 | **每 Room 一个 goroutine 串行处理**：所有 Op 进入房间的单一 channel，天然获得全序，避免图形状态锁竞争 |
| BE-03 | **每 Client 单写协程（single writer）**：`writePump` 独占 `conn.WriteMessage`，禁止多 goroutine 并发写同一连接（gorilla/websocket 非并发安全） |
| BE-04 | **慢客户端策略**：发送缓冲 channel 满时按策略处置（丢弃低优先级光标帧 → 仍满则断开并要求重连），禁止阻塞房间主循环 |
| BE-05 | **连接生命周期**：Ping/Pong 心跳（30s ping / 60s read deadline）、优雅关闭、panic recover 不影响其他连接 |
| BE-06 | **房间懒创建 + 空房回收**：最后一人离开后 N 秒（可配）落盘快照并释放内存 |
| BE-07 | **跨节点广播**：Redis Pub/Sub，channel 按 `board:{id}`；本节点内直接内存广播，跨节点经 Redis，需**过滤自身回环** |
| BE-08 | 优雅停机：`SIGTERM` 后停止接受新连接、广播 `server_shutdown`、落盘全部脏房间、超时强制退出 |

### 4.2 增量同步协议 — P0

**协议分层**：`Frame(传输层) → Envelope(路由层) → Op(业务层)`

**消息方向与类型（最小集）**：

| 方向 | 类型 | 载荷要点 |
|---|---|---|
| C→S | `join` | boardId, nickname, color, passcode?, lastSeq? |
| C→S | `op` | clientOpId, lamport, baseVersion, opKind, targetId, patch |
| C→S | `cursor` | x, y, viewport?（**二进制帧**） |
| C→S | `selection` | shapeIds[] |
| C→S | `ping` | — |
| S→C | `joined` | selfId, members[], snapshot(全量，仅此一次), serverSeq |
| S→C | `op_ack` | clientOpId, serverSeq, acceptedVersion |
| S→C | `op_reject` | clientOpId, reason, authoritativeShape |
| S→C | `op_bcast` | serverSeq, authorId, opKind, targetId, patch |
| S→C | `cursor_bcast` | 批量（**二进制帧**，一个 tick 合并多人） |
| S→C | `member_join` / `member_leave` | member |
| S→C | `resync_required` | reason |
| S→C | `error` | code, message |

**Op 种类**：`shape.create` / `shape.update` / `shape.delete` / `shape.reorder` / `shapes.group` / `shapes.ungroup`

| ID | 需求 |
|---|---|
| PR-01 | **只传增量**：`shape.update` 的 `patch` 仅含变更字段，禁止整对象回传 |
| PR-02 | **服务端单调序列号** `serverSeq`：房间级 uint64，每个被接受的 Op +1，作为全序权威 |
| PR-03 | **Lamport 时钟**：客户端维护逻辑时钟，随 Op 上报，用于同 seq 下的因果判定与 tie-break（tie-break 次序：`lamport` → `clientId` 字典序） |
| PR-04 | **二进制光标帧**：固定头（1B type + 1B count）+ 每人 (4B userIdx + 4B float x + 4B float y)，单人 ≤ 40 bytes |
| PR-05 | **协议版本协商**：`join` 携带 `protoVersion`，不匹配返回明确错误码而非静默失败 |
| PR-06 | **反序列化强校验**：所有入站消息必须校验字段存在性、类型、数值边界（坐标 NaN/Inf、数组长度上限、字符串长度上限），非法输入返回 `error` 且不得 panic ⟵ *global.md [Robustness]* |
| PR-07 | 单消息体积上限（默认 256KB）与单连接速率限制（默认 200 op/s），超限断连 |

### 4.3 并发冲突剪裁 — P0（核心难点）

| ID | 需求 |
|---|---|
| CF-01 | 每个 shape 持有 `version uint64` + `lastWriterId` + `updatedAt` |
| CF-02 | **属性级 LWW（Last-Write-Wins per property）**：两人同毫秒分别改同一 shape 的 `x` 和 `fill`，两项变更**都应保留**，不得整体覆盖 |
| CF-03 | 同一属性冲突时以 `serverSeq` 较大者胜；`serverSeq` 由房间 goroutine 串行分配，故不存在真正的"同毫秒"歧义 |
| CF-04 | `baseVersion` 落后于当前 `version` 时：若无属性交集 → **接受并 rebase**；有属性交集 → **拒绝并回传权威 shape** |
| CF-05 | **删除优先（Delete Wins）**：对已删除 shape 的 update 直接丢弃并回执 `op_reject`，禁止"复活"图形 |
| CF-06 | **收敛性测试（必须有）**：N 个并发 client 对同一 shape 随机发 M 个 op，结束后所有 client 与服务端状态**逐字节一致** |
| CF-07 | 幂等：同一 `clientOpId` 重复到达（重连重发）只生效一次 |

### 4.4 持久化与可靠性 — P0

| ID | 需求 |
|---|---|
| PS-01 | 白板状态**内存为主**（房间内 `map[shapeId]*Shape`），周期性/事件触发快照落库（默认 5s 脏检查 + 空房时强制） |
| PS-02 | **Op 日志表**：`(board_id, server_seq, author_id, kind, payload, created_at)`，支持重连增量补齐与历史回放 |
| PS-03 | 进程重启后从"最近快照 + 后续 Op 日志"重建房间状态 ⟵ *server.md [Cache] 禁止 raw Map 无快照* |
| PS-04 | Op 日志压实（compaction）：快照成功后清理该 seq 之前的日志，防止无界增长 |
| PS-05 | 数据库迁移：版本化 SQL 迁移文件，启动时自动执行且幂等 |
| PS-06 | 图片上传：大小上限（默认 5MB）、MIME 白名单、内容哈希命名（避免重复存储） |

### 4.5 工程规范 — P0

| ID | 需求 | 来源 |
|---|---|---|
| EN-01 | **统一 Logger**：结构化日志（`log/slog`），带 level 控制与 `board_id`/`client_id` 上下文字段；**禁止散落 `fmt.Println`**；前端禁止散落 `console.log`，须统一 logger 且生产屏蔽 debug | global.md [Logging] |
| EN-02 | **`docs/API.md`**：每个 HTTP 端点与每个 WS 消息类型都要有请求/响应示例、字段类型表、错误码表 | global.md [Documentation] |
| EN-03 | **测试**：后端单元测试覆盖 CRUD + 冲突引擎 + 协议编解码；E2E 覆盖双客户端协同关键路径 | global.md [Testing] |
| EN-04 | 配置全部走环境变量，含默认值，`.env.example` 齐备 | — |
| EN-05 | Prometheus 风格 `/metrics`：在线连接数、房间数、op 吞吐、广播延迟直方图、丢弃帧数 | — |
| EN-06 | `/healthz`（存活）与 `/readyz`（DB/Redis 就绪，**禁止硬编码 true**） | server.md [Kafka] 同源教训 |
| EN-07 | Go 后端文件数 **25–35**，代码量 **5500–8000 行**（不含测试与前端） | 原始需求硬约束 |

---

## 5. 前端需求（Vue 3）

| ID | 需求 | 来源 |
|---|---|---|
| FE-01 | 分层 Canvas：`静态层`(图形) + `覆盖层`(选中框/光标/辅助线)，仅重绘变动层 | 性能 |
| FE-02 | 渲染循环用 `requestAnimationFrame`，脏矩形合并，禁止每个 op 触发全量 repaint | 性能 |
| FE-03 | HiDPI：按 `devicePixelRatio` 缩放画布后备缓冲，文字与线条不模糊 | 美观 |
| FE-04 | **禁止原生 `alert/confirm/prompt`**，统一自定义 Modal/Dialog | client.md |
| FE-05 | **禁止无功能按钮**：工具栏每个入口都必须有实现，否则移除 | client.md |
| FE-06 | 错误提示支持 × 手动关闭 + 5s 自动消失；删除等危险操作走确认弹窗 | client.md |
| FE-07 | 用户可见日期时间统一 `yyyy-MM-dd HH:mm:ss`（GMT+8） | client.md |
| FE-08 | 响应式：覆盖 768px 与 480px 两级断点；窄屏工具栏收纳为抽屉，属性面板转底部 sheet | client.md |
| FE-09 | 页面级容器 `w-full` 撑满，禁止硬性 `max-w-*`（登录卡片/Modal 除外） | client.md |
| FE-10 | 原生 `select` 必须重置 `appearance: none` + 自绘 SVG 箭头 | client.md |
| FE-11 | 连接状态指示器：已连接 / 重连中 / 已断开，重连自动退避重试 | 可用性 |
| FE-12 | Pinia `watch` 若依赖派生 id（如 `activeBoardId → activeShapeId`），必须同时 watch 派生值，避免空态渲染 | client.md [Vue][Pinia] |

---

## 6. 可测量验收基线（Acceptance Baselines）

> 以下为**硬性数字指标**，QA Phase 必须逐条产出实测值写入 `docs/QA_Record.md`。不接受"运行正常"这类散文式结论。

### 6.1 性能

| 指标 | 基线 | 测量方法 |
|---|---|---|
| 单节点并发 WS 连接 | **≥ 1000** | 压测客户端（Go 编写）建立 1000 连接，稳定 60s 无掉线 |
| 单房间并发编辑者 | **≥ 50** | 50 连接同房间持续发 op |
| Op 端到端广播延迟 | **P95 < 50ms，P99 < 120ms**（容器网络内） | 客户端打时间戳，服务端 echo 回执，统计直方图 |
| 光标同步延迟 | **P95 < 80ms** | 同上 |
| 光标广播频率 | 服务端合并后 **≤ 30 tick/s** | `/metrics` 计数 |
| 单 `shape.update` 消息体 | **JSON ≤ 120 bytes**，**二进制光标 ≤ 40 bytes/人** | 抓包实测 |
| 前端渲染 | **5000 图形下平移/缩放 FPS ≥ 50** | Performance 面板录制 |
| 首屏进入白板（100 图形） | **< 1.5s** | Lighthouse / 手工计时 |
| 单连接内存占用 | **< 64KB**（读写缓冲合计） | pprof heap |

### 6.2 正确性

| 指标 | 基线 |
|---|---|
| 冲突收敛 | 20 客户端 × 500 随机 op 后，所有客户端与服务端 shape 表**完全一致**，0 分歧 |
| 消息丢失 | `serverSeq` 连续无空洞；有空洞则必须触发 resync（不允许静默不一致） |
| 幂等 | 同一 `clientOpId` 重复 3 次，最终只产生 1 次状态变更 |
| 断线重连恢复 | 断网 10s 后 **≤ 3s** 内恢复一致状态 |
| 进程重启恢复 | 重启后房间状态与重启前一致（快照 + Op 日志重放） |
| 删除优先 | 对已删 shape 的 100 次 update 全部被拒，图形不复活 |

### 6.3 健壮性

| 指标 | 基线 |
|---|---|
| 恶意/畸形输入 | 模糊测试 1000 条畸形帧（NaN 坐标、超长字符串、类型错乱、截断二进制），服务端 **0 panic**，全部返回明确错误码 |
| 慢客户端 | 人为让 1 个客户端不读取，其余客户端**不受影响**（房间主循环不阻塞） |
| 超限保护 | 超过 200 op/s 或 256KB 单消息，连接被限流/断开且日志可追溯 |

### 6.4 交付

| 指标 | 基线 |
|---|---|
| 启动 | `docker compose up --build -d` 一条命令，无需任何手工步骤 |
| 跨平台 | 所有镜像 `linux/arm64` 与 `linux/amd64` 均可拉取构建 |
| 访问 | 浏览器 `localhost` 可直达白板页面 |
| 测试成本 | QA 每轮实际外部 API 花费 **¥0**（本项目无外部计费 API） |

---

## 7. 分期边界（10k–40k LoC 强制要求）

> 详细排期见 `docs/Roadmap.md`。此处仅冻结**范围边界**。

### MVP（必须交付，缺一即 FAIL）
- 单节点 Hub/Room/Client 模型 + 连接生命周期
- JSON 增量协议 + 二进制光标帧
- 冲突剪裁引擎（serverSeq + Lamport + 属性级 LWW + Delete Wins）
- 无限画布：平移/缩放/视口裁剪
- 图元：rect / ellipse / diamond / line / arrow / freedraw / text / sticky
- 多人光标 + 昵称 + 在线成员列表 + 远端选中框
- Postgres 快照 + Op 日志 + 重连增量补齐
- 本地 Undo/Redo
- docker compose 一键启动 + 单元测试 + E2E

### V1（本期目标，Roadmap 内完成）
- Redis Pub/Sub 多节点集群（compose 内 2 副本 + 反向代理）
- 图片上传图元
- 对齐辅助线、成组/解组、层级调整
- 白板列表 / 重命名 / 删除 / 缩略图 / 口令
- 导出 PNG / SVG / JSON
- `/metrics` + 压测脚本 + 性能报告

### V2（明确标注为"未来工作"，不在本期交付）
- CRDT（Yjs 风格）替代 LWW，支持完全离线编辑合并
- 富文本便签、Markdown 渲染
- 版本历史时间轴回放（Op 日志已具备数据基础）
- 完整用户体系 / 团队 / 权限
- 跟随模式（Follow）、语音/视频

---

## 8. 非功能需求

| 类别 | 要求 |
|---|---|
| 美学（Redline 2） | 现代深色/浅色双主题，Tailwind 设计系统，间距/字阶/圆角成体系；工具栏为浮动 pill 形态；无"工程师 UI"、无错位 |
| 交付（Redline 1） | `docker compose up` 单命令；镜像支持 ARM64 + AMD64；多阶段构建（Go distroless/alpine，前端 nginx 托管） |
| 文档（Redline 3） | `docs/Requirements.md`(本文) → `docs/Roadmap.md` → `docs/DesignSpec.md` → `docs/API.md` → `docs/QA_Record.md` → `docs/AuditReport.md` → `docs/SelfTestReport.md` → `README.md` |
| 时区 | 容器 `TZ=Asia/Shanghai`；Go 侧 GMT+8；前端展示 GMT+8 |
| 安全 | WS Origin 校验；房间口令用 bcrypt 存储不明文；上传 MIME 白名单；SQL 全参数化；无硬编码密钥（全走 env） |
| 端口 | 开发期用**随机高位端口**；`/deploy` 阶段统一改为 8081+ |

---

## 9. 外部依赖评估

| 依赖 | 类别 | 判定 |
|---|---|---|
| PostgreSQL | 自托管容器 | ✅ 无需 Mock |
| Redis | 自托管容器 | ✅ 无需 Mock |
| 第三方 API | — | ✅ **无任何外部计费/实时事实数据 API** |

**结论**：本项目**不触发 Mock Provider 要求**，无 Contract Gate 需求（Phase 3 无需真实外部调用验证）。`README.md` §7 将明确声明"本项目无外部 API 依赖，无 Mock/Real 切换"，而非留空。

---

## 10. Phase Order 建议（供 Chief Architect 决策参考）

**建议采用 Logic-First（交换 Phase 2 与 Phase 3）**。

理由一句话：画布 UI 的组件结构（图元渲染器、Op 应用器、选中态管理）是**从 shape 数据模型与同步协议派生出来的**，属于 SOP v13 明列的 "editor / canvas" 情形；先定 schema 与协议再写渲染层，可避免前端返工。

最终决策权归 Phase 1 Chief Architect，须在 `docs/Roadmap.md` 中显式记录。

---

## 11. 已知技术陷阱（从知识库预注入，Phase 3/4 必须规避）

| 来源 | 陷阱 → 规避动作 |
|---|---|
| server.md `[Go][ServeMux]` | `{name...}` 通配符只能在 pattern 末尾，否则启动即 panic → 路由不得写 `/boards/{id...}/ops` |
| server.md `[Go][WAL]` | `Close()` 关闭 stop channel 必须用 `sync.Once`，否则测试里重复 Close 会 `close of closed channel` → Room/Hub 的 Close 一律 `sync.Once` |
| server.md `[Cache]` | 内存 Map 重启丢数据 → 必须有周期快照（PS-01/PS-03） |
| server.md `[Kafka]` 同源 | 健康检查禁止硬编码 true → `/readyz` 必须真实 ping DB/Redis |
| global.md `[Robustness]` | 反序列化必须校验结构完整性 → PR-06 |
| client.md `[Vue][Pinia]` | 只 watch 父 id 会渲染空态 → FE-12 |
| gorilla/websocket 特性 | `Conn` 写操作非并发安全 → BE-03 单写协程 |

---

## 12. 冲突与假设记录（Contradiction Log）

| # | 原始表述 | 冲突性质 | 裁决 |
|---|---|---|---|
| 1 | "成百上千人同时在线" vs `docker compose` 单机交付 | 规模与交付形态张力 | compose 内 2 个 backend 副本 + Redis 演示真实集群路径；压测基线锁定**单节点 1000 连接**，并在 `SelfTestReport` 中说明水平扩展外推依据 |
| 2 | "自定义二进制 **或** JSON" | 二选一表述 | 非互斥 → 双通道协议（§2） |
| 3 | "Vue 3 / Svelte" | 二选一 | Vue 3 + TS |
| 4 | "Canvas/SVG" | 二选一 | Canvas 2D 分层 |
| 5 | "基于版本号 **或** 时间戳" | 二选一，且时间戳方案有缺陷 | serverSeq 权威 + Lamport 因果 + 属性级 LWW；**不采信客户端墙钟** |
| 6 | "25~35 个 Go 文件，5500–8000 行" | 未说明是否含前端/测试 | **假设**：仅指 backend 生产代码，不含 `_test.go` 与前端 |
| 7 | 未提及认证/权限 | 需求缺口 | **假设**：匿名昵称 + 可选房间口令；不做用户中心（避免 Scope Drift） |
| 8 | "同一毫秒修改同一图形" | 表述隐含客户端时间同序 | 澄清：房间 goroutine 串行化后**不存在真正同毫秒歧义**，冲突本质是 `baseVersion` 过期，按 CF-04 处理 |

---

**🔒 需求已冻结。输入 `/auto` 开始自动化交付流程（Phase 1–5）。**

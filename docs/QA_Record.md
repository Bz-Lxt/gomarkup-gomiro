# QA Record

## Round 1 — 2026-08-22 23:31 (GMT+8)

**Cost**: ¥0  
**环境**: `docker compose exec`（gotest / qa），主服务保持运行。

### 执行

| 项 | 命令 | 结果 |
|---|---|---|
| Docker 健康 | web / api-1 / api-2 / postgres / redis healthy | PASS |
| GET /healthz /readyz | 经 18431 与 18432 | PASS（db=true, redis=true） |
| Go 单测 | `docker compose exec gotest go test ./...` | PASS（engine + protocol） |
| API smoke | `docker compose exec qa pytest /work/api_smoke.py` | **FAIL** 1/7：`test_ws_join_and_incremental_op` |
| Playwright | 未跑（被上一失败阻断） | — |

### 失败原文

```
websocket._exceptions.WebSocketBadStatusException: Handshake status 500 Internal Server Error
api-1: "ws upgrade" err="websocket: response does not implement http.Hijacker"
```

### 定位与既定修复

AccessLog 的 `statusWriter` 未实现 `http.Hijacker`，gorilla/websocket 无法劫持连接。  
**既定修复（本轮锁定，后续不得改口）**：为 `statusWriter` 实现 `Hijack`/`Flush` 并透传到底层 `ResponseWriter`。  
附带：双节点并发迁移用 `pg_advisory_lock(72618431)` 串行化。

---

## Round 2 — 2026-08-22 23:32 (GMT+8)

**Cost**: ¥0

| 项 | 结果 |
|---|---|
| API smoke 重跑 | **7 passed**（含双客户端 join + op_bcast） |
| Playwright | **FAIL**：`Playwright was just updated to 1.62.1`，镜像仍是 `v1.49.1-noble` |

### 失败原文

```
Error: browserType.launch: Executable doesn't exist at /ms-playwright/chromium_headless_shell-1234
Please update docker image as well.
- current: mcr.microsoft.com/playwright:v1.49.1-noble
- required: mcr.microsoft.com/playwright:v1.62.1-noble
```

### 既定修复

`tests/package.json` 将 `@playwright/test` 从 `^1.49.1` **钉死为 `1.49.1`**，与镜像浏览器版本对齐。不升级镜像。

---

## Round 3 — 2026-08-22 23:33 (GMT+8)

**Cost**: ¥0

| 项 | 结果 |
|---|---|
| Playwright `e2e_flow.spec.ts` | **2 passed**（创建白板进画布；双上下文打开同一白板） |
| Go 单测 | PASS |
| API smoke | PASS（Round 2 已绿，本轮未回退） |

### 结论

**QA PASS**。关键路径（健康检查、Board CRUD、口令、畸形 JSON、双客户端增量广播、首页创建进画布）已在容器内验证。

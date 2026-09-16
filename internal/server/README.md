# internal/server

HTTP API 与内嵌前端。

- `server.go`：`Server` 组装、路由表、鉴权中间件、请求日志。`streamTurn` 把 agent 事件
  以 NDJSON 流式写出（`Content-Type: application/x-ndjson`）。
- `handlers.go`：config / models / tools / sessions CRUD、发消息、开场、重 roll、图片上传。
  重 roll 会先备份整轮，若模型在产出任何内容前失败就整轮回滚。
- `stories.go`：预设 CRUD 与预设资源。
- `speech.go`：TTS 合成 + 内容寻址缓存（模型|音色|格式|语速|文本 → 会话 assets）。
- `auth.go`：登录鉴权。

事件类型定义在 `internal/agent/context.go`；图片通过
`/api/{sessions|stories}/{id}/assets/{name}` 读取。

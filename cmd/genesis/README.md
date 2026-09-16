# cmd/genesis

程序入口，只做三件事：解析参数、组装依赖、启动 HTTP 服务。

启动顺序：

1. 读取 `.env`（不覆盖已存在的环境变量）。
2. 加载配置：默认值 → `config.json` → 环境变量。
3. 打开 `sessions/` 与 `stories/` 两个仓库。
4. 迁移旧版 `llm_sessions/` 到 `sessions/`。
5. 首次启动写入内置预设「灰烬王冠 · Emberfall」。
6. 注册工具，用 `GENESIS_PASSWORD` 构造鉴权器。
7. 启动 HTTP 服务。

参数：`-addr`、`-data`、`-config`、`-version`。
版本号通过 `-ldflags "-X main.version=..."` 注入（见 Makefile）。

依赖的其余组件分别在 `internal/config`、`internal/auth`、`internal/store`、
`internal/tools`、`internal/server`。

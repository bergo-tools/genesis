# internal/store

数据模型与持久化。一个记录一个目录，方便整体备份和删除。

- `model.go`：`Session`（一次对话）、`Story`（可复用预设），以及 `Settings`（预设与会话唯一
  的设置项：故事指令；模型与生成参数在 `internal/config`）、`Character`、`Message`、`Scene`、
  `Choice`、`TokenStats`、`TurnSnapshot`。
- `store.go`：
  - 布局：`stories/<id>/story.json + assets/`、`sessions/<id>/session.json + assets/`；
  - 原子写入（临时文件 + rename）；id 与资源名都做校验，禁止 `..` 与路径分隔符；
  - `Store` 额外维护一份**内存摘要缓存**供侧栏使用：列表只做一次 `ReadDir` 对账，
    不解析未变动的会话历史，手动删掉目录也能立刻反映；
  - `TurnLock(id)`：每个会话一把锁，串行化生成回合；删除会话时也用它等待在跑的回合。
- `builtin.go`：内置预设「灰烬王冠 · Emberfall」，首次启动写入。
- `migrate.go`：把旧版 `llm_sessions/` 中带对话内容的预设迁进 `sessions/`。

资源限制：单文件 10 MiB，允许 png / jpg / jpeg / gif / webp。

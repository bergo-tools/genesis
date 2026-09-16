# internal/config

全局配置的加载、保存与 `.env` 解析。

- `Config`：与具体会话无关的配置——API key、默认模型、生成参数、TTS、默认工具开关。
- `Load(path)`：默认值 → `config.json` → 环境变量，逐层覆盖。
- `Store`：并发安全的持有者，`Get` / `Update` / `Save`；
  `Public()` 是发给浏览器的脱敏副本（只给 `hasKey`，绝不返回 key）。
- `LoadDotEnv(path)`：无依赖的 `KEY=VALUE` 解析，已存在的环境变量不覆盖。

支持的环境变量：`OPENROUTER_API_KEY`、`OPENROUTER_BASE_URL`、`GENESIS_MODEL`、
`GENESIS_ADDR`、`GENESIS_DATA_DIR`、`GENESIS_REASONING_EFFORT`、
`GENESIS_SPEECH_MODEL`、`GENESIS_SPEECH_VOICE`。

`GENESIS_PASSWORD` 不在这里：密码只由入口直接读环境变量，避免被写进 `config.json`。

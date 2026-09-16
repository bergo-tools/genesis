# internal/llm

与 provider 无关的模型接口，以及 OpenRouter 适配。

- `types.go`：`Message` / `Request` / `Response` / `Usage` / `ToolDef` / `ToolChoice`。
  `Message` 的 `Images` 只存文件名，`DataURL` 由 agent 在发请求时补齐。
- `openrouter.go`：用官方 SDK 发 chat 请求。
  - 多模态 user message 打成 `text + image_url` 内容块；
  - 映射 `tool_choice`、`parallel_tool_calls`、`reasoning_effort`（`off` → `none`）；
  - `usageFromSDK` 摊平 `cached_tokens` / `reasoning_tokens` / `cost`。
- `speech.go`：`/audio/speech` 的 TTS 调用，归一化输出格式与扩展名。

因为走的是 OpenAI 兼容协议，换后端只需改 Base URL。

# internal/agent

agent 循环：模型的一切输出都是工具调用，这里负责把它们跑起来。

- `registry.go`：`Tool`（Name / Description / Parameters / Handler / Category / Terminal / Query）
  与有序的 `Registry`。增加一个能力 = 一次 `Register`。
- `context.go`：`Event`（发给浏览器的 NDJSON 事件）和 `TurnContext`（工具运行时上下文：
  读写会话、emit 事件、`Show` 追加并广播消息）。
- `agent.go`：`Continue` 主循环。每个 step：组请求 → 执行工具 → 判断是否收尾。
  - 生成参数（模型、温度、token 上限、tool choice、思考强度、工具开关）全部来自全局 config，
    会话不存这些值，所以改 Settings 对进行中的会话也立即生效。
  - `activeTools`：按全局 `disabledTools` 与 `choicesEnabled` 过滤。被禁用的工具不会执行，
    也不能用 `last_call` / `terminal` / `choices` 结束回合。
  - choices 启用时强制在回合末尾调用；缺失就继续追加 `[system]` 提醒，直到步数预算用完
    （不再向用户报错）。
  - choices 未启用时，`last_call=true` 结束回合。
  - 历史整段发送：不做滑动窗口、不裁剪。每次请求都是上一次请求再加上新回合，provider 的前缀缓存
    因而能覆盖除最新回合以外的全部内容。
- `prompt.go`：每次请求动态拼 system prompt。固定 preamble 里除了工具协议，还有一份
  **写作 brief**（长度、具体感、五感、节奏、潜台词、收尾方式，`prompt_test.go` 会守住它），
  之后依次拼接工具列表、收尾规则、全局/故事指令、玩家、cast、世界状态。

扩展方式：在 `internal/tools` 新增工具并注册，prompt 与请求会自动带上。

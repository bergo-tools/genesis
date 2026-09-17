# internal/tools

三个内置工具，刻意保持精简。

- `message.go`：一个场景节拍。`text`（对白 / 动作 / 旁白）+ 可选 `thought`（同气泡、暗色显示）、
  以及 `speaker`、`kind`、`mood`、`last_call`。
- `narrator.go`：旁白与环境描写，必填 `text`。以 Narrator 身份留一条 narration 消息，
  用来铺陈场景、推进情节（取代了原来结构化的 scene 工具，那个像填表格，效果不好）。
- `choices.go`：terminal 工具。可选 narration + 2–4 个短选项（只取 `choices[].text`，不需要描述），
  持久化到会话，刷新后仍在；玩家也可以直接在输入框写自己的行动。
- `schema.go`：`object` / `stringProp` / `enumProp` / `arrayProp` / `lastCallProp` 等
  JSON Schema 小工具，以及宽松的 `decode`。

新增工具：写一个返回 `*agent.Tool` 的函数，在 `tools.go` 的 `RegisterBuiltins` 里注册即可。

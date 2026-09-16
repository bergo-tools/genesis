# Genesis · 单文件 Agentic 角色扮演引擎

Genesis 是一个用 Go 写的、类 SillyTavern 的角色扮演应用，但它是 **agentic** 的：
模型**不直接输出文本**，它的一切行为——对白、动作、旁白、场景切换、角色状态变更、
记忆、骰子判定、给玩家的选项、结束回合——都是**一次 function/tool call**。
因此新增一个能力只需要在 Go 里注册一个工具，不需要改动任何 prompt 拼装或协议代码。

最终产物是**一个二进制文件**：前端（原生 JS，无构建步骤）通过 `go:embed` 打包进去，
OpenRouter 的 chat 接口由官方 SDK [`github.com/OpenRouterTeam/go-sdk`](https://github.com/OpenRouterTeam/go-sdk) 调用。

## 特性

- **一切皆工具调用**：内置 12 个工具，覆盖叙事、世界状态、记忆、骰子、回合控制。
- **工具注册表**：`agent.Registry` + 单一 `Register` 调用即可扩展；`GET /api/tools` 会实时暴露给 UI。
- **流式 agent 循环**：`POST /api/sessions/{id}/messages` 返回 NDJSON 事件流，
  逐步推送 thinking 状态、工具调用、消息、场景、状态、选项、用量。
- **多步推理**：模型可以连续「思考 → 查询 → 行动 → 再行动」，直到调用终结工具
  （`await_player` / `end_turn`）或达到步数上限。
- **持久化**：会话以 JSON 原子写入磁盘；`config.json` 保存配置（API key 可来自 `.env`）。
- **单二进制 + 嵌入式前端**：自适应、移动端可用、跟随系统深浅色。
- **可替换后端**：默认 OpenRouter；因为 SDK 讲的是 OpenAI 兼容协议，
  在 Settings 里改 Base URL 就能接到任何兼容端点（例如本地或第三方）。

## 目录结构

```
cmd/genesis/main.go          程序入口、flag、组装
internal/config/             配置加载/保存、.env 解析
internal/llm/                中立的 Chat/工具类型 + OpenRouter SDK 适配
internal/store/              会话数据模型与文件持久化
internal/agent/              工具注册表、事件、agent 循环、system prompt
internal/tools/              内置工具（新增工具只需加一个文件 + 一行注册）
internal/server/             HTTP 路由、NDJSON 流式接口
web/                         原生 JS 前端（embed 进二进制）
```

## 快速开始

```bash
# 1. 准备 key
cp .env.example .env        # 填入 OPENROUTER_API_KEY
#   也可以启动后在网页 Settings 里填，会写入 config.json

# 2. 构建单文件
make build                  # 产物：dist/genesis

# 3. 运行
./dist/genesis                         # 默认 http://127.0.0.1:8080
./dist/genesis -addr :9000 -data /var/lib/genesis
```

打开浏览器访问提示的地址即可。首次使用点 **Settings** 填 API key 与模型；
点 **New story** 创建角色卡后开始。

### CLI 参数

| 参数 | 说明 |
| --- | --- |
| `-addr` | 监听地址，如 `127.0.0.1:8080` |
| `-data` | 数据目录，内含 `config.json` 与 `llm_sessions/` |
| `-config` | 指定 config.json 路径 |
| `-version` | 打印版本 |

### 环境变量

`OPENROUTER_API_KEY`、`OPENROUTER_BASE_URL`、`GENESIS_MODEL`、
`GENESIS_ADDR`、`GENESIS_DATA_DIR`。环境变量优先于 `config.json`。

## 添加一个工具（核心）

```go
// internal/tools/weather.go
package tools

import (
	"context"
	"encoding/json"

	"github.com/zp/genesis/internal/agent"
)

func weatherTool() *agent.Tool {
	return &agent.Tool{
		Name:        "set_weather",
		Category:    "world",
		Description: "改变当前天气。",
		Parameters: object(map[string]any{
			"value": stringProp("新的天气描述。"),
		}, "value"),
		Handler: func(_ context.Context, tc *agent.TurnContext, args json.RawMessage) (any, error) {
			var a struct {
				Value string `json:"value"`
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			tc.Session.Scene.Weather = a.Value
			tc.Emit(agent.Event{Type: agent.EventScene, Scene: &tc.Session.Scene})
			return map[string]any{"ok": true}, nil
		},
	}
}
```

然后在 `internal/tools/tools.go` 的 `RegisterBuiltins` 里加一行：

```go
r.Register(weatherTool())
```

`Tool` 的字段含义：

| 字段 | 作用 |
| --- | --- |
| `Handler` | 执行逻辑，返回值会 JSON 序列化后作为 tool result 回喂给模型 |
| `Terminal` | 该工具一旦被调用，本轮 agent 循环结束（如 `await_player`） |
| `Query` | 只读信息类工具（如 `roll` / `recall`），仅用于文档标注 |
| `Category` | 分组标签，显示在 `/api/tools` |
| `Parameters` | JSON Schema，直接传给模型做 function calling |

> 想让模型「必须先查询再行动」，用 `Query` 工具返回结构化数据即可；
> 想让一回合结束，把它标成 `Terminal`。

## 内置工具

| 工具 | 类别 | 说明 |
| --- | --- | --- |
| `send_message` | narration | 输出对白/动作/旁白/心理/场外，可多次调用组成多个气泡 |
| `think` | meta | 记录私有推理，展示在 Agent 活动面板 |
| `set_scene` | world | 更新地点/时间/天气/背景 |
| `update_character` | world | 新建或更新角色卡与 `state`（心情、好感等） |
| `update_state` | world | 用点路径修改世界状态（set/add/append/delete/toggle） |
| `get_state` | info(query) | 读取世界状态 |
| `remember` | memory | 写入长期记忆 |
| `recall` | info(query) | 检索长期记忆 |
| `roll` | info(query) | 骰子判定，支持 `NdM±K`、优势/劣势、DC |
| `offer_choices` | flow | 给玩家 2–4 个建议行动 |
| `await_player` | flow(terminal) | 结束回合，交还控制权 |
| `end_turn` | flow(terminal) | 场景自然延续时结束回合 |

## HTTP API

| 方法 & 路径 | 说明 |
| --- | --- |
| `GET /api/config` / `PUT /api/config` | 读取（key 脱敏）/ 保存配置 |
| `GET /api/models` | 代理拉取可用模型列表 |
| `GET /api/tools` | 列出已注册工具及 JSON Schema |
| `GET /api/sessions` | 会话列表 |
| `POST /api/sessions` | 新建会话（含角色卡、开场白） |
| `GET/PATCH/DELETE /api/sessions/{id}` | 读取 / 更新 / 删除 |
| `POST /api/sessions/{id}/messages` | 发送玩家消息，返回 NDJSON 事件流 |
| `POST /api/sessions/{id}/opening` | 让 agent 无输入地开场 |
| `POST /api/sessions/{id}/regenerate` | 丢弃上一轮 AI 输出并重写 |

流事件类型：`status`、`user_message`、`message`、`scene`、`character`、
`memory`、`state`、`choices`、`reasoning`、`tool_start`、`tool_end`、
`usage`、`notice`、`error`、`turn_end`、`title`。

## 模型侧协议

- 模型被要求「永不输出纯文本」，一律用工具表达。
- 默认 `tool_choice=auto`；如果模型偶尔输出文本，服务端会把它兜底渲染成旁白并发出
  `notice`，不会丢内容。追求严格约束可在 Settings 里改成 `required`。
- system prompt 每次请求根据角色卡/场景/状态/记忆重建，历史只存 `user/assistant/tool` 消息。

## 开发

```bash
make test     # go test ./...
make vet      # go vet ./...
make fmt      # gofmt
make dev ARGS="-addr 127.0.0.1:8080"
```

测试覆盖 agent 循环（工具执行、终结、协议兜底、历史裁剪）、工具行为与持久化。

### 关于本机 Go 环境

如果 `GOMODCACHE` / `GOCACHE` 位于沙箱可写范围之外，可在工作区内重定向：

```bash
export GOPATH="$PWD/.gopath" GOMODCACHE="$PWD/.gopath/pkg/mod" GOCACHE="$PWD/.gocache"
```

## 说明

- 后端只做角色扮演与工具编排，不包含内容审核；请遵守所用模型的条款。
- `config.json` 与 `llm_sessions/` 已在 `.gitignore` 中，不会提交。

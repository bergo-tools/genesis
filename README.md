# Genesis · 单文件 Agentic 角色扮演引擎

Genesis 是一个用 Go 写的、类 SillyTavern 的角色扮演应用，但它是 **agentic** 的：
模型**不直接输出文本**，它的一切行为都是**一次 function/tool call**。
最终产物是**一个二进制文件**：前端（Preact + htm，无构建步骤）通过 `go:embed` 打包进去，
OpenRouter 的 chat 接口由官方 SDK [`github.com/OpenRouterTeam/go-sdk`](https://github.com/OpenRouterTeam/go-sdk) 调用。

## 工具集（刻意保持精简）

只有 4 个工具，后续扩展也只需注册即可：

| 工具 | 作用 | 是否结束回合 |
| --- | --- | --- |
| `message` | 角色发声/行动，或旁白场景（kind 为 speech / action / narration） | 否 |
| `think` | 角色**内心想法**，以暗淡气泡展示 | 否 |
| `update_state` | 记录玩家要求记住的信息（物品、数值、旗标、关系、场景事实） | 否 |
| `choices` | 输出场景描述 + 2–4 个后续选项，把控制权交还玩家 | **是（terminal）** |

**choices 强制机制**：当 story 开启 choices 后，system prompt 要求「每回合必须以 choices 结束」；
如果模型某一轮结束后没有调用 choices，**agent loop 会自动追加一条提醒消息并继续**（最多 2 次），
仍失败才会结束并给出提示。玩家除了点选项，也可以直接在输入框里写自己的行动。

## 特性

- **一切皆工具调用**，只有 4 个工具：message / think / update_state / choices。
- **多角色 cast**：一个 story 可配置任意多个角色，每个角色有名字、设定、性格与头像；
  模型用 `message` 的 `speaker` 指定是谁在说话。
- **图片**：story 头像、角色头像、聊天中发图都支持；聊天图片会以多模态（base64 data URL）
  一起发给模型，具备视觉能力的模型可以直接看到。
- **重新 roll**：一键丢弃上一轮 AI 输出并重写，换一个走向。
- **思考强度可调**：off / minimal / low / medium / high / max（关闭思考即 off）。
- **语音（TTS）**：通过 OpenRouter 合成语音，可配置 speech 模型与音色，逐块朗读任意文本。
- **每个 story 一个目录**，删掉目录即彻底清理，便于备份与批量删除。
- **流式 agent 循环**：NDJSON 事件流实时推送 thinking、工具调用、消息、状态、选项与用量。
- **持久化**：story 以 JSON 原子写入；配置在 `config.json`（API key 可来自 `.env`）。
- **可替换后端**：默认 OpenRouter；因为 SDK 讲 OpenAI 兼容协议，改 Base URL 即可接入任意兼容端点。

## 存储布局

每个 story 独占一个目录，方便清理：

```
<dataDir>/
  config.json
  stories/
    <story-id>/
      story.json           # 角色卡、消息、世界状态、模型侧 history
      assets/
        <random>.png       # 头像与聊天图片
```

删除某个 story 时服务端直接 `os.RemoveAll` 整个目录；上传文件名经过校验，禁止 `..` 与路径分隔符。

## 目录结构

```
cmd/genesis/main.go          程序入口、flag、组装
internal/config/             配置加载/保存、.env 解析
internal/llm/                中立的 Chat/工具类型 + 多模态 + OpenRouter SDK 适配
internal/store/              数据模型、按 story 分目录的持久化与资源存储
internal/agent/              工具注册表、事件、agent 循环、system prompt
internal/tools/              message / think / update_state / choices
internal/server/             HTTP 路由、NDJSON 流、图片上传/读取
web/                         Preact + htm 前端（Vendor ESM，embed 进二进制）
```

## 快速开始

```bash
cp .env.example .env        # 填入 OPENROUTER_API_KEY
make build                  # 产物：dist/genesis
./dist/genesis              # 默认 http://127.0.0.1:8080
./dist/genesis -addr :9000 -data /var/lib/genesis
```

在网页里 **Settings** 填 API key 与模型；**New story** 里可以加多个角色、上传故事/角色头像、
选择思考强度与是否强制 choices。

### CLI 参数

| 参数 | 说明 |
| --- | --- |
| `-addr` | 监听地址 |
| `-data` | 数据目录（内含 `config.json` 与 `stories/`） |
| `-config` | 指定 config.json 路径 |
| `-version` | 打印版本 |

### 环境变量

`OPENROUTER_API_KEY`、`OPENROUTER_BASE_URL`、`GENESIS_MODEL`、`GENESIS_ADDR`、
`GENESIS_DATA_DIR`、`GENESIS_REASONING_EFFORT`、`GENESIS_SPEECH_MODEL`、
`GENESIS_SPEECH_VOICE`。环境变量优先于 `config.json`。

## 添加一个工具

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
				Value string $json:"value"$
			}
			if err := decode(args, &a); err != nil {
				return nil, err
			}
			if tc.Session.State == nil {
				tc.Session.State = map[string]any{}
			}
			tc.Session.State["weather"] = a.Value
			tc.Emit(agent.Event{Type: agent.EventState, State: tc.Session.State})
			return map[string]any{"ok": true}, nil
		},
	}
}
```

然后在 `internal/tools/tools.go` 的 `RegisterBuiltins` 里加一行 `r.Register(weatherTool())`。

`Tool` 字段：`Handler` 执行逻辑（返回值会作为 tool result 回喂模型）；`Terminal` 表示调用后结束回合；
`Query` 标注只读信息类工具；`Category` 用于 `/api/tools` 分组；`Parameters` 是 JSON Schema。

## HTTP API

| 方法 & 路径 | 说明 |
| --- | --- |
| `GET/PUT /api/config` | 读取（key 脱敏）/ 保存配置 |
| `GET /api/models` | 代理拉取模型列表 |
| `GET /api/tools` | 列出已注册工具及 schema |
| `GET /api/sessions` | story 列表 |
| `POST /api/sessions` | 新建（persona、characters[]、greeting、settings） |
| `GET/PATCH/DELETE /api/sessions/{id}` | 读取 / 更新（角色、头像、标题）/ 删除整个目录 |
| `POST /api/sessions/{id}/messages` | 发送消息（`{text, images: [资源名]}`），返回 NDJSON |
| `POST /api/sessions/{id}/opening` | 让 agent 无输入地开场 |
| `POST /api/sessions/{id}/regenerate` | 重新 roll：丢弃上一轮 AI 输出并重写 |
| `POST /api/sessions/{id}/assets` | 上传图片（multipart `file`），返回 `{name, url}` |
| `GET /api/sessions/{id}/assets/{name}` | 读取图片或合成音频 |
| `POST /api/sessions/{id}/speech` | 朗读文本块：`{text, speaker}` 或 `{messageId}`，返回音频 |

事件类型：`status`、`user_message`、`message`、`state`、`choices`、`reasoning`、
`tool_start`、`tool_end`、`usage`、`notice`、`error`、`turn_end`、`title`。

## 模型侧协议

- 模型被要求「永不输出纯文本」，一律用工具表达；若它仍输出文本，会被兜底渲染成旁白。
- **choices**：开启时强制在回合末尾调用；缺失则 agent loop 自动提醒并继续（最多 2 次）。
- **思考强度**：以 `reasoning_effort` 透传；`off` 映射为 `none`（关闭）。
- **多模态**：聊天图片在构造请求时读取资源并转成 `data:image/...;base64` 内容块；
  持久化只存文件名，不会把 base64 写进 `story.json`。
- system prompt 每次请求根据角色卡、世界状态与提示重建；history 只存 user/assistant/tool。

## 语音（TTS）

通过 OpenRouter 的 `/audio/speech` 合成语音，用来朗读**指定的文本块**：

- **可配置 speech 模型**：默认 `openai/gpt-4o-mini-tts`，在 Settings 中修改；输出格式支持
  mp3 / wav / opus / aac / flac / pcm，并支持播放速度。
- **逐块朗读**：每条消息旁有 🔊 按钮，点击朗读该块，再点停止。
- **音色**：角色卡可单独设置 `voice`（如 `alloy` / `shimmer`）；朗读时优先使用角色音色，
  否则回退到全局默认音色，旁白使用全局音色。
- **可选自动朗读**：Settings 打开 `autoSpeak` 后，新消息按顺序自动朗读。
- **缓存**：合成结果按 `模型|音色|格式|文本` 内容寻址缓存到该 story 的 `assets/` 下，
  同一文本块重复朗读不会再次调用模型。

端点：`POST /api/sessions/{id}/speech`，请求体 `{ text, speaker }` 或 `{ messageId }`，
也可显式传 `voice` / `force`。

## 前端技术栈

- **Preact 10 + htm**，以 **Vendor ESM**（`web/vendor/*.module.js`）提交，
  **没有 npm install、没有 bundler、没有构建步骤**；只把裸导入 `"preact"` 改成相对路径。
- 模块划分：`api.js`（网络 + 资源上传 + NDJSON 流解析）、`format.js`（转义与轻量 markdown）、
  `components.js`（纯展示组件与编辑器）、`app.js`（根组件、hooks 状态、流事件、`mount()`）。
- 图片选择、预览、上传在 `Composer`；多角色与头像编辑在 `NewStoryModal` / `CastModal`。
- 无浏览器冒烟测试：`web/test/render.test.mjs` 用 vendored 的 `preact-render-to-string`
  渲染整棵组件树并校验文案，由 `make test-web` 运行。

## 开发

```bash
make test     # go test ./... + Preact SSR 冒烟测试
make test-web # 仅前端 SSR 冒烟测试
make vet      # go vet ./...
make fmt      # gofmt
make dev ARGS="-addr 127.0.0.1:8080"
```

测试覆盖 agent 循环（工具执行、choices 强制提醒、终结、历史裁剪）、四个内置工具、
多模态与 reasoning 映射、按目录持久化与资源安全、以及前端组件树的 SSR 渲染。

### 本机 Go 环境

若 `GOMODCACHE` / `GOCACHE` 在沙箱可写范围之外，可在工作区内重定向：

```bash
export GOPATH="$PWD/.gopath" GOMODCACHE="$PWD/.gopath/pkg/mod" GOCACHE="$PWD/.gocache"
```

## 说明

- 后端只做角色扮演与工具编排，不包含内容审核；请遵守所用模型的条款。
- `config.json` 与 `stories/` 已在 `.gitignore` 中，不会提交。

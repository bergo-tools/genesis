# Genesis · 单文件 Agentic 角色扮演引擎

Genesis 是一个用 Go 写的、类 SillyTavern 的角色扮演应用，但它是 **agentic** 的：
模型**不直接输出文本**，它的一切行为都是**一次 function/tool call**。
最终产物是**一个二进制文件**：前端（Preact + htm，无构建步骤）通过 `go:embed` 打包进去，
OpenRouter 的 chat 接口由官方 SDK [`github.com/OpenRouterTeam/go-sdk`](https://github.com/OpenRouterTeam/go-sdk) 调用。

## 工具集（刻意保持精简）

只有 3 个工具，后续扩展也只需注册即可：

| 工具 | 作用 | 是否结束回合 |
| --- | --- | --- |
| `message` | 一个场景节拍：`text`（对白/动作/旁白）+ 可选 `thought`（角色内心想法，同块显示、颜色区分） | 否 |
| `scene` | 场景描写：location / time / weather / background / notes，只更新传入字段 | 否 |
| `choices` | 输出场景描述 + 2–4 个后续选项，把控制权交还玩家；选项会持久保存，刷新后仍在 | **是（terminal）** |

每个工具都带一个可选参数 `last_call`：模型在自己认为是本轮最后一次调用时置为 `true`。
当 choices 未启用时，`last_call` 会结束本轮；当 choices 启用时，它被忽略，模型仍必须调用
`choices` 收尾（缺失则由 agent loop 提醒，最多 2 次）。工具可在全局 Settings 与每个
story 的 Story settings 里逐个开关。

**choices 强制机制**：当 story 开启 choices 后，system prompt 要求「每回合必须以 choices 结束」；
如果模型某一轮结束后没有调用 choices，**agent loop 会自动追加一条提醒消息并继续**（最多 2 次），
仍失败才会结束并给出提示。玩家除了点选项，也可以直接在输入框里写自己的行动。

## 特性

- **登录鉴权**：在 `.env` 里设置 `GENESIS_PASSWORD` 后，进入 UI 需先输入密码；密码只从环境变量读取，
  登录会话**只存在内存中**（进程重启即全部失效），连续输错会被短暂锁定。
- **一切皆工具调用**：当前工具集为 message / scene / choices（`update_state` 暂缓，设计待定）。
- **一段式消息**：`message` 的 text 与 thought 合并在同一个气泡里，内心想法用不同颜色与斜体区分。
- **场景栏**：`scene` 维护地点/时间/天气/背景，顶部场景栏实时更新。
- **会话管理**：侧栏每个 session 都有删除按钮；`choices` 持久化在会话上，刷新页面不丢。
- **Story 预设 + Session 会话**：Story 是可复用的预设（cast、开场、默认设置、种子状态），
  每个 session 从预设开启并拥有独立对话；内置中世纪魔法预设「灰烬王冠 · Emberfall」。
- **多角色 cast**：一个 story 可配置任意多个角色，每个角色有名字、设定、性格、头像与 TTS 音色；
  模型用 `message` 的 `speaker` 指定是谁在说话。
- **图片**：story 头像、角色头像、聊天中发图都支持；聊天图片会以多模态（base64 data URL）
  一起发给模型，具备视觉能力的模型可以直接看到。
- **按回合重 roll / 编辑输入**：每个 AI 回合末尾都有 ↻，可从任意回合重写；用户消息可 ✎ 编辑，
  保存后从该处重跑（会丢弃其后的所有回合）。
- **OOC 场外指令**：输入框旁的 `OOC` 按钮展开第二个输入框，内容会随本轮回复一起送给模型
  （作为 `[OOC] ...` 指令，跳出剧情直接驱动 LLM），界面上以独立标签块显示；也可只发 OOC。
- **可打断输出**：生成中 Send 变为 **Cancel**，点击即中断本次生成（已生成的部分会保留）。
- **思考强度可调**：off / minimal / low / medium / high / max（关闭思考即 off）。Settings 里设全局
  默认，**每个 story 也能在 Story settings 里单独改**（含关闭）。
- **语音（TTS）**：通过 OpenRouter 合成语音，可配置 speech 模型与音色，逐块朗读任意文本。
- **模型列表自动加载**：Settings、New story、Story settings 的文字模型输入都是从 OpenRouter
  拉取的 datalist，标注上下文长度与是否支持 tools（支持工具调用的排在前面）；选中模型时会
  **把 max tokens 自动填成 OpenRouter 返回的该模型最大输出**。
- **工具可开关**：每个 story 可单独启用/禁用工具；所有工具都带 `last_call` 参数，模型用它表示
  「这是本轮最后一次调用」——除非 choices 已启用（那时必须以 choices 收尾）。
- **每个 story 一个目录**，删掉目录即彻底清理，便于备份与批量删除。
- **流式 agent 循环**：NDJSON 事件流实时推送 thinking、工具调用、消息、状态、选项与用量。
- **持久化**：story 以 JSON 原子写入；配置在 `config.json`（API key 可来自 `.env`）。
- **可替换后端**：默认 OpenRouter；因为 SDK 讲 OpenAI 兼容协议，改 Base URL 即可接入任意兼容端点。

## Story 与 Session

- **Story = 预设**：可复用的模板，包含角色卡（cast）、开场白、默认生成设置、种子世界状态、
  故事级指令，以及故事自己的头像/角色头像。内置了一个中世纪魔法主题预设
  **「灰烬王冠 · Emberfall」**（3 个角色 + 开场 + 种子状态），首次启动自动写入。
- **Session = 一次对话**：从某个 Story 预设开启。创建时会把预设的 cast、默认设置、
  种子状态**快照**进这个 session（之后改预设不会影响已开始的对话），并拥有独立的
  消息记录、世界状态与图片。每个 session 可再单独调模型/思考强度/工具/温度等。

## 存储布局

每个 session / preset 独占一个目录，方便清理：

```
<dataDir>/
  config.json
  stories/                   # 预设
    <story-id>/
      story.json             # cast、开场、默认设置、种子状态
      assets/<random>.png    # 预设头像与角色头像
  sessions/                  # 对话实例
    <session-id>/
      session.json           # 消息、世界状态、模型侧 history、生效设置
      assets/<random>.png    # 聊天图片与合成语音
```

删除某个 session 或 preset 时服务端直接 `os.RemoveAll` 整个目录；上传文件名经过校验，禁止 `..` 与路径分隔符。

## 目录结构

```
cmd/genesis/main.go          程序入口、flag、组装
internal/config/             配置加载/保存、.env 解析
internal/auth/               共享密码鉴权与内存会话
internal/llm/                中立的 Chat/工具类型 + 多模态 + OpenRouter SDK 适配
internal/store/              数据模型、按 story 分目录的持久化与资源存储
internal/agent/              工具注册表、事件、agent 循环、system prompt
internal/tools/              message / scene / choices
internal/server/             HTTP 路由、NDJSON 流、图片上传/读取
web/                         Preact + htm 前端（components / story / library / vendor）
```

## 快速开始

```bash
cp .env.example .env        # 填入 OPENROUTER_API_KEY
make build                  # 产物：dist/genesis
./dist/genesis              # 默认 http://127.0.0.1:8080
./dist/genesis -addr :9000 -data /var/lib/genesis
```

在网页里 **Settings** 填 API key 与模型；**Stories** 管理预设（含内置的「灰烬王冠」），
**New session** 选一个预设即可开始对话；预设与单个 session 都能设置角色、头像、思考强度与工具开关。

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

### 登录鉴权

设置 `GENESIS_PASSWORD`（推荐写在 `.env`）后，Web UI 需要先输入密码：

- 密码只从环境变量读取，**不会写进 `config.json`**；留空或不设置则不启用鉴权（保持原有开放行为，启动日志会提示）。
- 登录成功后下发 `genesis_session` cookie（`HttpOnly` + `SameSite=Lax`，HTTPS 下加 `Secure`）；
  **会话只保存在内存中**，进程重启即全部失效，每次访问滑动续期（默认 30 天）。
- 静态资源、`/api/auth/*` 与 `/api/health` 保持公开；其余 `/api/*` 未登录一律返回 401，
  前端收到 401 会自动回到登录页。
- 同一 IP 连续输错 5 次锁定 30 秒（返回 `429` + `Retry-After`）。

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
| `GET /api/auth/status` | 是否启用鉴权、当前是否已登录（无需登录） |
| `POST /api/auth/login` | 提交 `{password}`，成功后种下会话 cookie |
| `POST /api/auth/logout` | 退出登录并作废内存中的会话 |
| `GET/PUT /api/config` | 读取（key 脱敏）/ 保存配置 |
| `GET /api/models` | 代理拉取文本模型列表（含 `tools` 标记，支持工具调用的排前面） |
| `GET /api/tools` | 列出已注册工具及 schema |
| `GET /api/stories` / `POST /api/stories` | Story 预设列表 / 新建预设 |
| `GET/PATCH/DELETE /api/stories/{id}` | 读取 / 更新 / 删除预设（含 cast、开场、默认设置、种子状态） |
| `POST /api/stories/{id}/assets` · `GET .../assets/{name}` | 预设图片上传 / 读取 |
| `GET /api/sessions` | session 列表 |
| `POST /api/sessions` | 新建（persona、characters[]、greeting、settings） |
| `GET/PATCH/DELETE /api/sessions/{id}` | 读取 / 更新（角色、头像、标题）/ 删除整个目录 |
| `POST /api/sessions/{id}/messages` | 发送消息（`{text, images: [资源名]}`），返回 NDJSON |
| `POST /api/sessions/{id}/opening` | 让 agent 无输入地开场 |
| `POST /api/sessions/{id}/regenerate` | 重新 roll：丢弃上一轮 AI 输出并重写 |
| `POST /api/sessions/{id}/assets` | 上传图片（multipart `file`），返回 `{name, url}` |
| `GET /api/sessions/{id}/assets/{name}` | 读取图片或合成音频 |
| `POST /api/sessions/{id}/speech` | 朗读文本块：`{text, speaker}` 或 `{messageId}`，返回音频 |
| `GET /api/speech/models` | 列出 TTS 模型及其音色目录（`supported_voices`） |

事件类型：`status`、`user_message`、`message`、`state`、`choices`、`reasoning`、
`tool_start`、`tool_end`、`usage`、`notice`、`error`、`turn_end`、`title`。

## 模型侧协议

- 模型被要求「永不输出纯文本」，一律用工具表达；若它仍输出文本，会被兜底渲染成旁白。
- **choices**：开启时强制在回合末尾调用；缺失则 agent loop 自动提醒并继续（最多 2 次）。
- **last_call**：所有工具都接受该参数；choices 未启用时用于结束本轮，启用时被忽略。
- **工具开关**：被禁用的工具不会出现在 system prompt、也不会出现在请求的 tools 列表里。
- **思考强度**：以 `reasoning_effort` 透传；`off` 映射为 `none`（关闭）。
- **多模态**：聊天图片在构造请求时读取资源并转成 `data:image/...;base64` 内容块；
  持久化只存文件名，不会把 base64 写进 `story.json`。
- system prompt 每次请求根据角色卡、世界状态与提示重建；history 只存 user/assistant/tool。

## 语音（TTS）

通过 OpenRouter 的 `/audio/speech` 合成语音，用来朗读**指定的文本块**：

- **可配置 speech 模型**：默认 `hexgrad/kokoro-82m`（音色 `af_heart`），在 Settings 中修改；
  输出格式支持 mp3 / wav / opus / aac / flac / pcm，并支持播放速度。
  已实测可返回音频的模型：`hexgrad/kokoro-82m`、`deepgram/flux-tts:free`（免费）。
- **逐块朗读**：每条消息旁有 🔊 按钮，点击朗读该块，再点停止。
- **音色按模型不同**：OpenRouter 在模型对象上给出 `supported_voices`，**每个 TTS 模型一套**
  （例如 Kokoro 54 个、Deepgram Flux 36 个，命名风格完全不同：`af_heart` vs `flux-alexis-en`）。
  后端 `GET /api/speech/models` 返回 `{id, name, voices}`；前端把「默认音色」和「角色音色」
  都做成 **datalist 自动补全**，选中某个 speech 模型后只提示该模型支持的音色
  （少数模型没有音色目录，此时自由输入，用错会返回 provider 的具体报错）。
- **音色覆盖顺序**：请求显式 `voice` > 角色卡 `voice`（按 `speaker` 匹配）> 全局默认音色；
  旁白使用全局默认音色。
- **可选自动朗读**：Settings 打开 `autoSpeak` 后，新消息按顺序自动朗读。
- **缓存**：合成结果按 `模型|音色|格式|语速|文本` 内容寻址缓存到该 story 的 `assets/` 下，
  同一文本块重复朗读不会再次调用模型。

端点：`POST /api/sessions/{id}/speech`，请求体 `{ text, speaker }` 或 `{ messageId }`，
也可显式传 `voice` / `force`。

## 前端技术栈

- **Preact 10 + htm**，以 **Vendor ESM**（`web/vendor/*.module.js`）提交，
  **没有 npm install、没有 bundler、没有构建步骤**；只把裸导入 `"preact"` 改成相对路径。
- 模块划分：`api.js`（网络 + 资源上传 + NDJSON 流解析 + 401 处理）、`format.js`（转义与轻量 markdown）、
  `components.js`（纯展示组件与编辑器）、`login.js`（密码登录门）、`app.js`（根组件、hooks 状态、流事件、`mount()`）。
- 图片选择、预览、上传在 `Composer`；多角色与头像编辑在 `NewSessionModal` / `PresetModal` / `StoryModal`（共用 `CharacterEditor`）。
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

测试覆盖 agent 循环（工具执行、choices 强制提醒、终结、历史裁剪）、三个内置工具、
多模态与 reasoning 映射、按目录持久化与资源安全、以及前端组件树的 SSR 渲染。

### 本机 Go 环境

若 `GOMODCACHE` / `GOCACHE` 在沙箱可写范围之外，可在工作区内重定向：

```bash
export GOPATH="$PWD/.gopath" GOMODCACHE="$PWD/.gopath/pkg/mod" GOCACHE="$PWD/.gocache"
```

## 说明

- 后端只做角色扮演与工具编排，不包含内容审核；请遵守所用模型的条款。
- `config.json` 与 `stories/` 已在 `.gitignore` 中，不会提交。

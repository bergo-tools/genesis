# Genesis · 单文件 Agentic 角色扮演引擎

用 Go 写的类 SillyTavern 角色扮演应用，但它是 **agentic** 的：模型不直接输出文本，
每一句台词、旁白、场景、选项都是**一次 tool call**。前端（Preact + htm，无构建步骤）
通过 `go:embed` 打包进二进制，最终只有一个可执行文件。
模型接口走 OpenRouter 官方 SDK（OpenAI 兼容协议，改 Base URL 即可换端点）。

## 功能

- **一切皆工具调用**：内置 `writing_block`（一个角色或世界的一个节拍，`blocks` 是 `text` / `thought`
  的有序列表，内心想法可以插在两句对白之间；`speaker` 写 Narrator 就是环境 / 氛围 / 情节推进）、
  `choices`（回合末尾给出 2–4 个选项）两个工具，全部带 `last_call`；工具开关是全局的，
  禁用的工具绝不会执行。
- **Story 预设 + Session 会话**：预设是可复用模板（cast、开场、故事指令、种子状态）；
  模型与生成参数只有一份，放在全局 Settings 里，改完全部会话（含进行中的）下一回合即生效；
  内置「灰烬王冠 · Emberfall」。
- **PresetGen**：预设编辑器顶部有一栏，写一句描述就让模型起草标题、开场、cast 与故事指令，
  直接填进下面的表单；不点 Save 不会落盘。侧栏 `New story` 旁边的 ✨ 随时开一个空白预设来生成。
- **多角色 cast**：每个角色有名字、描述、头像与 TTS 音色。
- **点选 vs 手打**：回合末尾的 `choices` 是左右翻的选项卡片。点选出来的玩家消息带 `choice` 徽章、
  边框样式也不同；模型会收到 `[choice]` 标记，于是 Narrator 会先把这一步的实际效果展开描写
  （选了「施放咒术」，就先写咒术具体发生了什么），再让角色反应。手打的回复不带这个标记。
- **图片**：预设 / 会话头像、角色头像、聊天配图；聊天图片作为多模态输入发给模型。
- **按回合重 roll / 编辑**：每个 AI 回合末尾有 ↻（先确认），用户消息可 ✎ 编辑并重跑；
  重 roll 会回滚该回合改过的世界状态，失败时整轮恢复。
- **OOC 场外指令**：输入框旁的 OOC 按钮，内容以 `[OOC]` 指令随本轮一起发给模型。
- **可打断**：生成中 Send 变为 Cancel。
- **思考强度**：off / minimal / low / medium / high / max，全局设置。
- **Token 状态**：顶栏上下文占用条 + 面板明细（占用、缓存 token、累计用量、费用），
  按会话持久化。
- **语音（TTS）**：OpenRouter 合成语音，可配置模型 / 音色 / 格式 / 语速，逐块朗读并做缓存。
- **模型列表自动加载**：从 OpenRouter 拉取，标注上下文长度与是否支持 tools，自动填 max tokens。
- **登录鉴权**：用 `GENESIS_PASSWORD` 设密码，登录会话只存在内存里。

## 目录结构

| 目录 | 作用 |
| --- | --- |
| `cmd/genesis/` | 程序入口、命令行参数、组件组装 |
| `internal/config/` | 配置加载 / 保存、`.env` 解析 |
| `internal/auth/` | 共享密码鉴权与内存会话 |
| `internal/llm/` | provider 中立的 Chat 类型 + OpenRouter 适配 + TTS |
| `internal/store/` | 数据模型、按目录持久化、资源与摘要缓存 |
| `internal/agent/` | 工具注册表、agent 循环、事件、system prompt |
| `internal/tools/` | writing_block / choices 两个内置工具 |
| `internal/server/` | HTTP 路由、NDJSON 流、鉴权中间件 |
| `web/` | Preact + htm 前端（无构建，`go:embed` 打包） |

每个目录下都有自己的 `README.md`，说明该模块的结构与扩展点。

## 配置

复制 `.env.example` 为 `.env`（已被 gitignore）：

```bash
OPENROUTER_API_KEY=sk-or-...
GENESIS_PASSWORD=choose-a-password   # 留空则不启用鉴权
```

环境变量优先于 `config.json`；不写 `.env` 也可以在网页 **Settings** 里填 key。

| 变量 | 说明 |
| --- | --- |
| `OPENROUTER_API_KEY` | OpenRouter key |
| `GENESIS_PASSWORD` | 进入界面的密码；留空 = 不鉴权 |
| `OPENROUTER_BASE_URL` | 自定义端点 |
| `GENESIS_MODEL` | 默认模型 |
| `GENESIS_ADDR` | 监听地址 |
| `GENESIS_DATA_DIR` | 数据目录（默认 `.`） |
| `GENESIS_REASONING_EFFORT` | 默认思考强度 |
| `GENESIS_SPEECH_MODEL` / `GENESIS_SPEECH_VOICE` | 默认 TTS 模型 / 音色 |

其余设置（温度、max tokens、TTS 格式与语速等）在界面 Settings 里改，写入 `config.json`。

## 编译与运行

```bash
cp .env.example .env         # 填 key（可选：填密码）
make build                   # 产物 dist/genesis
./dist/genesis               # 默认 http://127.0.0.1:8080
./dist/genesis -addr :9000 -data /var/lib/genesis
```

| 参数 | 说明 |
| --- | --- |
| `-addr` | 监听地址 |
| `-data` | 数据目录 |
| `-config` | 指定 config.json |
| `-version` | 打印版本 |

| make | 说明 |
| --- | --- |
| `make build` | 编译单二进制 |
| `make run` | 编译并运行 |
| `make dev` | `go run` |
| `make test` | Go 测试 + 前端 SSR 冒烟 |
| `make vet` / `make fmt` / `make tidy` / `make clean` | 常规 |

## 数据存放

```
<dataDir>/
  config.json
  stories/<story-id>/story.json + assets/
  sessions/<session-id>/session.json + assets/
```

删除某个会话或预设会连同它的图片与语音一起删掉（整个目录移除）。
`stories/`、`sessions/`、`config.json`、`.env` 都在 `.gitignore` 里。

## 说明

- 后端只做角色扮演与工具编排，不含内容审核；请遵守所用模型的服务条款。
- 前端没有构建步骤：改完 `web/` 重新 `make build` 即可。

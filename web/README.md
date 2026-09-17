# web

前端：Preact 10 + htm，vendor 成普通 ESM 提交，**没有 npm、没有构建步骤**，
由 `go:embed` 打进二进制。

- `app.js`：根组件。状态、NDJSON 事件处理、流的中止与恢复、设置保存等。
- `components.js`：纯展示组件（消息、输入框、侧栏、世界面板、各类设置弹窗）。
- `tokens.js`：顶栏的上下文占用条与面板里的 Tokens 明细。
- `picker.js`：可搜索的模型/音色选择器（移动端弹层）。原生的 `<datalist>` 在 iOS Safari 上完全不显示、在 Android 上也很挤，所以所有模型选择都走这里。
- `story.js`：当前会话的 Story settings 弹窗（标题、头像、故事指令、cast；模型参数走全局 Settings）。
- `library.js`：New session / Stories / Preset 弹窗。PresetGen 内嵌在 Preset 编辑器顶部（写一句描述 → 模型填表，不点 Save 不落盘）；预设只编辑内容，模型与生成参数走全局 Settings。
- `login.js`：全屏密码登录页。
- `api.js`：fetch 封装、资源 URL、NDJSON 解析、401 时退回登录页。
- `format.js`：HTML 转义 + 轻量 markdown + token 格式化。
- `app.css`：全部样式（含浅色/深色与移动端）。
- `test/render.test.mjs`：用 vendored 的 `preact-render-to-string` 做 SSR 冒烟测试。

注意：htm 模板里组件必须写成 `<${Name} .../>`，直接写 `<Name/>` 会渲染成未知元素。

验证：`make test-web`（等价于 `cd web && node test/render.test.mjs`）。
改动后重新 `make build` 才会进二进制。

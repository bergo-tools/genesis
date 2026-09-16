# internal/auth

单个共享密码的登录门，密码来自 `GENESIS_PASSWORD`。

- 密码为空 = 整体禁用，应用保持开放（启动日志会提示）。
- 登录成功签发随机 token，只存在内存 map 里，不落盘：**进程重启即全部登出**。
- cookie 名 `genesis_session`：HttpOnly + SameSite=Lax，HTTPS 下加 Secure；默认 30 天滑动续期。
- 密码用常量时间比较；同一 IP 连续错 5 次锁定 30 秒。

HTTP 层见 `internal/server/auth.go`：`/api/auth/status|login|logout` 和 `requireAuth` 中间件。
静态文件、`/api/auth/*`、`/api/health` 保持公开，其余 `/api/*` 未登录返回 401。

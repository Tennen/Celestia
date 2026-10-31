# 花园账号自动化菜单

业务模块位于 `internal/core/project/garden/accounts`，与 Codex 告警/升级动作分开。通过 garden 现有管理接口读取账号并更新 `schedule.enabled`，不直接修改 garden 数据库、不回写整个账号配置，也不调用 Codex。

## 菜单和同步

- Celestia 启动后立即检查一次，此后每小时检查一次；在 Celestia 内检测账号增减。
- 从既有 `project/garden/codex` 文档的 `cwd` 定位 garden 项目，读取 `.local/admin/service.json` 并核对本机服务实例。需要认证时使用 `.local/admin/access-key` 登录，口令不进入菜单、日志或 Celestia 持久化状态。
- 比较排序后的账号 ID 与显示名。只切换开关或调整账号顺序不会发布菜单；ID 集合变化（包括同数量替换）或显示名变化才同步。重启后沿用 SQLite 中的最近成功名单，不会无条件重新发布。
- 菜单保留既有「花园」动作。账号不超过 5 个时使用「花园开关」父菜单；6–10 个时分成「账号开关1」「账号开关2」，每组最多 5 个。子项使用 garden 管理页的角色显示名，缺少角色名时使用配置名。
- 企微接口只支持两级菜单、最多 3 个父菜单，每组最多 5 个子项。超过剩余容量时不截断名单、不覆盖原菜单，错误写入同步状态并在下个检测周期重试。账号名称不能超过子菜单的 40 字节限制。
- Core 文档 `project/garden/account-menu` 保存最近成功名单、检查时间、发布时间和同步错误。账号减少到零时移除账号分组，保留其他菜单。

## 点击开关

Click key 为 `garden:account-toggle:<账号 UUID>`。仅当前企微应用经过桥验签的 `event/click` 事件可执行，发送者必须是已启用并正确绑定的 `Frypan` 或 `Jellyfish`。账号开关白名单和收件人绑定按 UserID 不区分大小写匹配，兼容真实回调中的 `jellyfish`；回复使用匹配到的已配置收件人 ID，不直接依赖回调字符串查找收件人。原来的 Codex 告警/升级动作仍只允许 Frypan。

点击时重新读取账号当前开关，并通过 garden 的 `PUT /api/accounts/{id}/execution` 只提交 `{"enabled": <相反值>}`。账号已删除或服务不可用时明确失败；暂停沿用 garden 现有的安全停止流程，当前操作及回执核对按 garden 规则完成，不强杀任务。

`project/garden/account-toggles` 在调用前持久化发送者、消息 ID、账号和目标状态，最多保留最近 200 条。SSE 重放和服务重启不会重复处理相同点击；结果不明时不自动重试 toggle。不同的新点击可以继续切换，不受两个 Codex 动作的每日限额影响。

处理结果仅回复点击者。普通 XML callback/ingress 不会分派 `garden:` 动作，不能通过伪造 UserID 绕过验签入口。未新增账号切换 HTTP API 或 Agent 工具。

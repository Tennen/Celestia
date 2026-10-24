# 花园企微菜单

在现有企微应用菜单中增加「花园」分组，使用两个固定 click key：

| 菜单 | Key | 发送给 Codex 的一句话 |
| --- | --- | --- |
| 检查告警 | `garden:check-alerts` | 检查花园脚本最近的告警，判断是游戏问题还是脚本问题，需要修复的话请修复。 |
| 处理游戏升级 | `garden:game-upgrade` | 检查花园游戏是否更新，核对接口协议，必要时更新脚本适配。 |

通过现有 `PUT /api/v1/touchpoints/wecom/menu` 发布。当前应用仅保留「花园」分组；「市场分析」「家居」菜单按用户要求移除，需要时可重新配置。

业务入口只接受 Celestia 主动连接到已配置 WeCom bridge 的 SSE 点击事件。Bridge 验证原始企微回调的签名并解密；Celestia 再校验事件类型为 `event/click`、企业 ID、应用 ID、非空消息 ID，以及启用的收件人 `id=frypan, wecom_user=Frypan`。发送者必须精确等于 `Frypan`。其它用户的点击只记录为 `denied`，不会触发 Codex 或通用 Agent。

保留整个 `garden:` key 命名空间。普通 XML `POST /api/v1/touchpoints/wecom/callback` 和 `/ingress` 即使声称发送者是 Frypan，也只记录 `blocked`，不分派这些动作。不存在 HTTP 花园任务执行 API 或 Agent 花园工具。

Core 的 SQLite `agent_documents` 存储项目文档 `project/garden/codex`（domain 为 `project.garden`），内容为 `{"thread_id":"<现有花园脚本会话 ID>","cwd":"<花园项目绝对路径>"}`。这是现有会话绑定，缺少配置时明确失败，不创建或搜索替代会话。

客户端通过当前用户的 `$CODEX_HOME/app-server-control/app-server-control.sock`（默认 `~/.codex/...`）连接运行中的 Codex app-server，使用 Unix WebSocket JSON-RPC。先初始化、读取并核对会话与目录，再 `thread/resume`、`turn/start`。不启动 daemon，不创建/分叉会话，不覆盖模型、权限或原指令，不自动接受审批。会话忙碌时拒绝提交，避免干扰已有任务。

`project/garden/requests` 保存最近 100 次请求，RPC 发送前持久化消息 ID、北京时间日期 `day` 和该动作当天计数 `daily_count`。两个动作分别按北京时间（UTC+8）自然日限额，每项每天最多提交一次，00:00 切换到新日期；重复点击不增加计数，不调用 Codex。授权失败、未知动作、缺少配置不会占次数；进入 Codex 提交阶段即占用次数，提交失败或结果不明也不返还，防止重试导致重复执行业务。计数从持久化请求记录计算，服务重启仍有效，升级前当天的记录也计入。

提交确认和超限提示都会显示当日次数及北京时间日期。任务进度、结果及需要人工审批的操作继续在「花园脚本」原会话中处理。

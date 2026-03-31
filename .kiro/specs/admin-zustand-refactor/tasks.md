# Tasks: Admin Console Zustand Refactor

## Task List

- [x] 1 安装 Zustand 依赖
  - [x] 1.1 在 web/admin 工作区安装 zustand ^5.x
  - [x] 1.2 创建 web/admin/src/stores/ 目录

- [x] 2 创建 adminStore
  - [x] 2.1 创建 web/admin/src/stores/adminStore.ts，包含后端数据状态、refreshAll action、reportError action
  - [x] 2.2 在 adminStore 中实现 initPolling（轮询定时器 + SSE EventSource），返回 cleanup 函数
  - [x] 2.3 将 useAdminConsole 中的 selectedPluginId/selectedDeviceId 自动选中逻辑迁移到 adminStore.refreshAll 内部（通过读取 pluginStore/deviceStore 当前值判断）

- [x] 3 创建 pluginStore
  - [x] 3.1 创建 web/admin/src/stores/pluginStore.ts，包含 selectedPluginId、installDrafts、configDrafts、pluginLogs、busy、xiaomiVerifyTicket 状态
  - [x] 3.2 实现 initDraftsFromCatalog action（从 catalog 初始化 installDrafts，从已安装 plugins 初始化 configDrafts）
  - [x] 3.3 实现 installPlugin / enablePlugin / disablePlugin / discoverPlugin / deletePlugin / saveConfig actions（含 busy 管理、refreshAll 调用、错误上报）
  - [x] 3.4 实现 reloadPluginLogs action
  - [x] 3.5 实现 setDraft action（根据 isInstalled 更新 installDrafts 或 configDrafts）
  - [x] 3.6 实现 retryXiaomiVerification action（合并 verify ticket 到 configDraft 并调用 updatePluginConfig）

- [x] 4 创建 deviceStore
  - [x] 4.1 创建 web/admin/src/stores/deviceStore.ts，包含 selectedDeviceId、deviceSearch、selectedAction、commandParams、actor、commandResult、busy、toggleOverrides、togglePending 状态
  - [x] 4.2 实现 sendCommand action（读取当前 selectedAction/commandParams/actor，调用 API，更新 commandResult，调用 refreshAll）
  - [x] 4.3 实现 onToggleControl action（乐观更新 + 失败回滚，从 useToggleControlActions 迁移逻辑）
  - [x] 4.4 实现 onActionControl / onValueControl actions
  - [x] 4.5 实现 updateDevicePreference / updateControlPreference actions
  - [x] 4.6 实现 pruneOverrides action（清理已不存在设备的 override key）
  - [x] 4.7 实现 applySuggestion action（同时更新 selectedAction 和 commandParams）

- [x] 5 适配 useXiaomiOAuth hook
  - [x] 5.1 修改 useXiaomiOAuth，移除 installDrafts/configDrafts/setInstallDrafts/setConfigDrafts/plugins 参数，改为直接读写 pluginStore 和 adminStore
  - [x] 5.2 验证 oauthBanner 和 oauthActive 的返回接口不变，App.tsx 调用处只需移除传参

- [x] 6 重构 App.tsx
  - [x] 6.1 移除 App.tsx 中所有业务 state（selectedAction、commandParams、actor、xiaomiVerifyTicket、installDrafts、configDrafts、commandResult、busy）
  - [x] 6.2 移除 useAdminConsole 和 useToggleControlActions 的调用，改为在 useEffect 中调用 adminStore.initPolling()
  - [x] 6.3 移除 PluginWorkspace 的所有业务 props，移除 DeviceWorkspace 的所有业务 props
  - [x] 6.4 sidebar 和 header 中的计数/状态从 useAdminStore 读取
  - [x] 6.5 在 App.tsx 挂载时调用 pluginStore.initDraftsFromCatalog（通过订阅 adminStore.catalog/plugins 变化触发）

- [x] 7 重构 PluginWorkspace
  - [x] 7.1 移除 PluginWorkspace 的 Props 类型中所有业务字段，改为无 props 或仅保留必要的布局 props
  - [x] 7.2 在组件内通过 useAdminStore 订阅 catalog、plugins
  - [x] 7.3 在组件内通过 usePluginStore 订阅 selectedPluginId、pluginDraft、pluginLogs、busy、xiaomiVerifyTicket 等
  - [x] 7.4 将所有 onXxx 回调替换为直接调用 pluginStore actions
  - [x] 7.5 保留 detailMode 本地 state，useEffect 监听 selectedPluginId 重置 detailMode

- [x] 8 重构 DeviceWorkspace
  - [x] 8.1 移除 DeviceWorkspace 的 Props 类型中所有业务字段
  - [x] 8.2 在组件内通过 useAdminStore 订阅 devices
  - [x] 8.3 在组件内通过 useDeviceStore 订阅 selectedDeviceId、selectedAction、commandParams、actor、commandResult、busy、toggleOverrides、togglePending
  - [x] 8.4 将所有 onXxx 回调替换为直接调用 deviceStore actions
  - [x] 8.5 将 commandSuggestions 的 useMemo 从 App.tsx 移入 DeviceWorkspace，依赖 deviceStore.selectedDevice
  - [x] 8.6 保留 deviceAliasDraft、editingDeviceAlias、aliasDrafts、controlDrafts 本地 state

- [x] 9 清理废弃文件
  - [x] 9.1 删除 web/admin/src/hooks/useAdminConsole.ts
  - [x] 9.2 删除 web/admin/src/hooks/useToggleControlActions.ts
  - [x] 9.3 检查并清理所有文件中对已删除 hooks 的导入引用

- [x] 10 验证构建和测试
  - [x] 10.1 运行 TypeScript 编译（tsc --noEmit）确认无类型错误
  - [x] 10.2 运行 vitest --run 确认现有测试通过
  - [x] 10.3 检查所有 store 文件和重构后的组件文件均不超过 500 行

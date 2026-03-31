# Requirements: Admin Console Zustand Refactor

## Introduction

本需求文档描述将 Admin Console 前端重构为 Zustand 全局状态管理的功能需求。重构目标是消除 `App.tsx` 中大量 props 透传，将后端数据集中到全局 store，将各工作区 UI 状态下沉到对应子组件或专属 store，使代码结构清晰、可维护性提升。

所有变更限定在 `web/admin/src/` 目录内，不涉及后端 API、Go 代码或插件协议。

---

## Requirements

### Requirement 1: 安装并集成 Zustand

**User Story**: 作为前端开发者，我需要在 admin 工作区安装 Zustand，以便使用其 store 机制管理全局状态。

#### Acceptance Criteria

1. WHEN 执行 `npm install zustand --workspace web/admin` THEN `web/admin/package.json` 的 `dependencies` 中包含 `zustand ^5.x`
2. WHEN 构建 admin 工作区 THEN TypeScript 编译无错误，Zustand 类型可正常使用
3. WHEN 创建 store 文件 THEN 文件位于 `web/admin/src/stores/` 目录下，每个 store 独立一个文件

---

### Requirement 2: 创建 adminStore 管理后端数据

**User Story**: 作为前端开发者，我需要一个全局 adminStore，集中管理来自后端的 catalog、plugins、devices、events、audits、dashboard 数据，以及全局 loading/error 状态。

#### Acceptance Criteria

1. WHEN adminStore 初始化 THEN 其状态包含 `dashboard`、`catalog`、`plugins`、`devices`、`events`、`audits`、`loading`、`error` 字段，初始值与现有 `emptyLoadState()` 一致
2. WHEN 调用 `adminStore.refreshAll()` THEN 并发调用所有 fetch API，成功后更新全部数据字段，`loading` 置为 `false`，`error` 置为 `null`
3. WHEN `refreshAll()` 抛出异常 THEN `loading` 置为 `false`，`error` 设置为错误消息
4. WHEN 调用 `adminStore.initPolling()` THEN 启动 `POLL_MS` 间隔的轮询和 SSE EventSource 监听，返回 cleanup 函数
5. WHEN cleanup 函数被调用 THEN 轮询定时器和 EventSource 均被清理
6. WHEN 调用 `adminStore.reportError(message)` THEN `error` 字段被设置为该消息

---

### Requirement 3: 创建 pluginStore 管理 Plugin 工作区状态

**User Story**: 作为前端开发者，我需要一个 pluginStore，集中管理 Plugin 工作区的选中状态、draft 配置、日志、busy 标志和所有 plugin 操作，消除 App.tsx 中对应的 state 和 PluginWorkspace 的业务 props。

#### Acceptance Criteria

1. WHEN pluginStore 初始化 THEN 其状态包含 `selectedPluginId`、`installDrafts`、`configDrafts`、`pluginLogs`、`busy`、`xiaomiVerifyTicket`，初始值均为空
2. WHEN 调用 `pluginStore.initDraftsFromCatalog(catalog, plugins)` THEN `installDrafts` 中每个 catalog plugin id 都有对应的默认配置 draft，`configDrafts` 中每个已安装 plugin 都有对应的当前配置 draft
3. WHEN 调用任意 plugin 操作（install / enable / disable / discover / delete / saveConfig）THEN `busy` 在操作期间设置为对应标识字符串，操作完成后清空
4. WHEN plugin 操作成功 THEN 调用 `adminStore.getState().refreshAll()`
5. WHEN plugin 操作失败 THEN 调用 `adminStore.getState().reportError(message)`，`busy` 清空
6. WHEN 调用 `pluginStore.reloadPluginLogs(pluginId)` THEN `pluginLogs` 更新为该 plugin 的最新日志
7. WHEN `selectedPluginId` 变化 THEN 自动触发对应 plugin 的日志加载（与现有 `useAdminConsole` 行为一致）

---

### Requirement 4: 创建 deviceStore 管理 Device 工作区状态

**User Story**: 作为前端开发者，我需要一个 deviceStore，集中管理 Device 工作区的选中状态、搜索、命令参数、toggle 乐观更新和所有 device 操作，消除 App.tsx 中对应的 state 和 DeviceWorkspace 的业务 props。

#### Acceptance Criteria

1. WHEN deviceStore 初始化 THEN 其状态包含 `selectedDeviceId`、`deviceSearch`、`selectedAction`、`commandParams`、`actor`、`commandResult`、`busy`、`toggleOverrides`、`togglePending`，初始值与现有代码一致
2. WHEN 调用 `deviceStore.sendCommand(device)` THEN 使用当前 `selectedAction`、`commandParams`、`actor` 发送命令，成功后 `commandResult` 设置为格式化的响应 JSON，并调用 `adminStore.refreshAll()`
3. WHEN 调用 `deviceStore.onToggleControl(device, controlId, on)` THEN 立即设置乐观 override，发送 toggle 请求，成功后调用 `refreshAll()`，失败后回滚 override 并上报错误
4. WHEN `adminStore.devices` 更新 THEN `deviceStore.pruneOverrides(devices)` 被调用，清理已不存在设备的 override key
5. WHEN 调用任意 device 操作 THEN `busy` 在操作期间设置为对应标识字符串，操作完成后清空
6. WHEN device 操作失败 THEN 调用 `adminStore.getState().reportError(message)`，`busy` 清空

---

### Requirement 5: 重构 App.tsx 移除业务状态和 props 透传

**User Story**: 作为前端开发者，我需要 App.tsx 只负责布局骨架和 section 切换，不再持有任何业务状态，也不再向子组件透传业务 props。

#### Acceptance Criteria

1. WHEN App.tsx 渲染 THEN 其本地 state 只包含 `activeSection`，不包含任何业务数据或操作状态
2. WHEN App.tsx 挂载 THEN 调用 `adminStore.initPolling()` 并在卸载时执行 cleanup
3. WHEN App.tsx 渲染 `PluginWorkspace` THEN 不传递任何业务 props（catalog、plugins、selectedPluginId、busy、onXxx 等均不出现）
4. WHEN App.tsx 渲染 `DeviceWorkspace` THEN 不传递任何业务 props（devices、selectedDeviceId、busy、onXxx 等均不出现）
5. WHEN App.tsx 渲染 sidebar THEN 从 `adminStore` 读取 `catalog.length`、`devices.length`、`events.length`、`audits.length`、`loading`、`error` 用于显示计数和状态徽章
6. WHEN App.tsx 渲染 error banner THEN 从 `adminStore.error` 读取
7. WHEN App.tsx 渲染 oauth banner THEN 从 `useXiaomiOAuth` hook 读取（hook 内部改为读写 `pluginStore`）

---

### Requirement 6: 重构 PluginWorkspace 直接消费 store

**User Story**: 作为前端开发者，我需要 PluginWorkspace 直接从 pluginStore 和 adminStore 读取数据并调用 actions，不再依赖父组件传入的业务 props。

#### Acceptance Criteria

1. WHEN PluginWorkspace 渲染 THEN 其 Props 类型不包含任何业务数据字段（catalog、plugins、selectedPluginId、busy、onXxx 等）
2. WHEN PluginWorkspace 需要 catalog 或 plugins 数据 THEN 通过 `useAdminStore` 订阅 `adminStore`
3. WHEN PluginWorkspace 需要 selectedPluginId、drafts、busy 等 UI 状态 THEN 通过 `usePluginStore` 订阅 `pluginStore`
4. WHEN 用户点击 Install / Enable / Disable 等按钮 THEN 直接调用 `pluginStore` 对应 action，不通过 props 回调
5. WHEN `selectedPluginId` 变化 THEN `detailMode` 重置为 `'runtime'`（保留现有本地 state 行为）

---

### Requirement 7: 重构 DeviceWorkspace 直接消费 store

**User Story**: 作为前端开发者，我需要 DeviceWorkspace 直接从 deviceStore 和 adminStore 读取数据并调用 actions，不再依赖父组件传入的业务 props。

#### Acceptance Criteria

1. WHEN DeviceWorkspace 渲染 THEN 其 Props 类型不包含任何业务数据字段（devices、selectedDeviceId、busy、onXxx 等）
2. WHEN DeviceWorkspace 需要 devices 列表 THEN 通过 `useAdminStore` 订阅 `adminStore`
3. WHEN DeviceWorkspace 需要 selectedDeviceId、commandParams、actor 等 UI 状态 THEN 通过 `useDeviceStore` 订阅 `deviceStore`
4. WHEN 用户发送命令或操作 toggle THEN 直接调用 `deviceStore` 对应 action，不通过 props 回调
5. WHEN selectedDevice 变化 THEN `deviceAliasDraft`、`editingDeviceAlias`、`aliasDrafts`、`controlDrafts` 重置（保留现有本地 state 行为）
6. WHEN DeviceWorkspace 计算 `commandSuggestions` THEN 使用 `useMemo` 依赖 `deviceStore.selectedDevice`，逻辑与现有 App.tsx 中一致

---

### Requirement 8: 适配 useXiaomiOAuth hook

**User Story**: 作为前端开发者，我需要 useXiaomiOAuth hook 改为读写 pluginStore，而不是依赖 App.tsx 传入的 installDrafts/configDrafts setter。

#### Acceptance Criteria

1. WHEN useXiaomiOAuth 需要读取 installDrafts / configDrafts THEN 直接从 `pluginStore` 读取，不再通过参数传入
2. WHEN useXiaomiOAuth 需要更新 installDrafts / configDrafts THEN 直接调用 `pluginStore` 的 setter，不再通过参数传入的 Dispatch
3. WHEN useXiaomiOAuth 需要读取 plugins THEN 直接从 `adminStore` 读取，不再通过参数传入
4. WHEN useXiaomiOAuth 的 hook 参数简化后 THEN App.tsx 中调用该 hook 时不再需要传入 state 和 setter

---

### Requirement 9: 清理废弃的 hooks

**User Story**: 作为前端开发者，我需要在重构完成后删除已被 store 替代的 hooks，保持代码库整洁。

#### Acceptance Criteria

1. WHEN 重构完成 THEN `web/admin/src/hooks/useAdminConsole.ts` 被删除，无任何文件引用它
2. WHEN 重构完成 THEN `web/admin/src/hooks/useToggleControlActions.ts` 被删除，无任何文件引用它
3. WHEN 重构完成 THEN TypeScript 编译无错误，无未使用的导入

---

### Requirement 10: 代码文件行数约束

**User Story**: 作为前端开发者，我需要确保重构后的每个文件不超过 500 行，符合项目代码规范。

#### Acceptance Criteria

1. WHEN 重构完成 THEN `web/admin/src/stores/adminStore.ts` 不超过 500 行
2. WHEN 重构完成 THEN `web/admin/src/stores/pluginStore.ts` 不超过 500 行
3. WHEN 重构完成 THEN `web/admin/src/stores/deviceStore.ts` 不超过 500 行
4. WHEN 重构完成 THEN `web/admin/src/App.tsx` 不超过 500 行
5. WHEN 重构完成 THEN `web/admin/src/components/admin/PluginWorkspace.tsx` 不超过 500 行
6. WHEN 重构完成 THEN `web/admin/src/components/admin/DeviceWorkspace.tsx` 不超过 500 行

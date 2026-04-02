# Requirements Document

## Introduction

本次重构将 `plugins/haier`、`plugins/hikvision`、`plugins/petkit` 三个插件的 `internal/app` 目录按职责拆分为两个子包：

- `internal/client`：vendor HTTP/WebSocket/MQTT 客户端相关文件（认证、传输、设备 API 调用、连接管理）
- `internal/app`：插件运行时相关文件（`Plugin` 结构体、生命周期、命令分发、状态同步、事件发射、配置持久化）

`xiaomi` 插件已符合目标结构，不做改动。

## Glossary

- **Plugin**: 实现 `internal/pluginapi` 接口的 vendor 插件进程，负责 vendor 认证、设备发现、状态翻译、命令执行和事件摄取。
- **internal/app**: 每个插件内保留的包，包含 `Plugin` 结构体及其运行时逻辑。
- **internal/client**: 每个插件内新建的包，包含 vendor 客户端实现（HTTP/WebSocket/MQTT）。
- **Refactor_Tool**: 执行本次重构的自动化工具或开发者操作。
- **Import_Path**: Go 模块内的完整包导入路径，如 `github.com/chentianyu/celestia/plugins/haier/internal/client`。
- **package_declaration**: Go 源文件顶部的 `package <name>` 声明。
- **Build_System**: Go 工具链（`go build`、`go test`）。

## Requirements

### Requirement 1: haier 插件 — 创建 internal/client 包

**User Story:** As a developer, I want haier's vendor client code separated into its own package, so that the plugin runtime and client responsibilities are clearly bounded.

#### Acceptance Criteria

1. THE Refactor_Tool SHALL create the directory `plugins/haier/internal/client/`.
2. THE Refactor_Tool SHALL move `client.go` (containing `haierClient` struct, constructor `newHaierClient`, and `requestJSON`) to `plugins/haier/internal/client/client.go` with package_declaration `package client`.
3. THE Refactor_Tool SHALL move `client_auth.go` to `plugins/haier/internal/client/auth.go` with package_declaration `package client`.
4. THE Refactor_Tool SHALL move `client_devices.go` to `plugins/haier/internal/client/devices.go` with package_declaration `package client`.
5. THE Refactor_Tool SHALL move `client_wss.go` to `plugins/haier/internal/client/wss.go` with package_declaration `package client`.
6. THE Refactor_Tool SHALL move `client_helpers.go` to `plugins/haier/internal/client/helpers.go` with package_declaration `package client`.
7. THE Refactor_Tool SHALL move `wss_listener.go` to `plugins/haier/internal/client/wss_listener.go` with package_declaration `package client`.
8. WHEN the above files are moved, THE Refactor_Tool SHALL remove the original files from `plugins/haier/internal/app/`.

### Requirement 2: haier 插件 — 更新 internal/app 包

**User Story:** As a developer, I want haier's plugin runtime files to have the `plugin_` prefix stripped and all client references updated to use the new import path, so that the app package is clean and self-consistent.

#### Acceptance Criteria

1. THE Refactor_Tool SHALL rename `plugin_commands.go` to `commands.go` within `plugins/haier/internal/app/`.
2. THE Refactor_Tool SHALL rename `plugin_controls.go` to `controls.go` within `plugins/haier/internal/app/`.
3. THE Refactor_Tool SHALL rename `plugin_devices.go` to `devices.go` within `plugins/haier/internal/app/`.
4. THE Refactor_Tool SHALL rename `plugin_helpers.go` to `helpers.go` within `plugins/haier/internal/app/`.
5. THE Refactor_Tool SHALL rename `plugin_refresh.go` to `refresh.go` within `plugins/haier/internal/app/`.
6. THE Refactor_Tool SHALL rename `plugin_wss.go` to `wss.go` within `plugins/haier/internal/app/`.
7. WHEN any file in `plugins/haier/internal/app/` references `haierClient`, `wssListener`, `newHaierClient`, `newWSSListener`, `parseWSSDeviceUpdate`, `getWSSGatewayURL`, `stringFromAny`, or `applianceOptions`, THE Refactor_Tool SHALL qualify those references with the `client.` prefix and add the import `github.com/chentianyu/celestia/plugins/haier/internal/client`.
8. THE Refactor_Tool SHALL retain `plugin.go`, `types.go` in `plugins/haier/internal/app/` with package_declaration `package app`.

### Requirement 3: hikvision 插件 — 创建 internal/client 包

**User Story:** As a developer, I want hikvision's camera client code separated into its own package, so that CGo and SDK dependencies are isolated from the plugin runtime.

#### Acceptance Criteria

1. THE Refactor_Tool SHALL create the directory `plugins/hikvision/internal/client/`.
2. THE Refactor_Tool SHALL move `client.go` (containing `cameraClient` interface and `newCameraClient` factory) to `plugins/hikvision/internal/client/client.go` with package_declaration `package client`.
3. THE Refactor_Tool SHALL move `client_stub.go` to `plugins/hikvision/internal/client/stub.go` with package_declaration `package client`.
4. THE Refactor_Tool SHALL move `client_unsupported.go` to `plugins/hikvision/internal/client/unsupported.go` with package_declaration `package client`.
5. THE Refactor_Tool SHALL move `hcnet_client_linux.go` to `plugins/hikvision/internal/client/hcnet_client_linux.go` with package_declaration `package client`.
6. THE Refactor_Tool SHALL move `hcnet_cgo_linux.go` to `plugins/hikvision/internal/client/hcnet_cgo_linux.go` with package_declaration `package client`.
7. THE Refactor_Tool SHALL move `hcnet_types_compat.h` to `plugins/hikvision/internal/client/hcnet_types_compat.h` (C header, no package_declaration change needed).
8. WHEN the above files are moved, THE Refactor_Tool SHALL remove the original files from `plugins/hikvision/internal/app/`.

### Requirement 4: hikvision 插件 — 更新 internal/app 包

**User Story:** As a developer, I want hikvision's plugin runtime to import the new client package and have no remaining client implementation files, so that the app package only contains plugin lifecycle and command logic.

#### Acceptance Criteria

1. WHEN `plugins/hikvision/internal/app/plugin.go` references `cameraClient`, `newCameraClient`, or `cameraStatus`, THE Refactor_Tool SHALL qualify those references with the `client.` prefix and add the import `github.com/chentianyu/celestia/plugins/hikvision/internal/client`.
2. THE Refactor_Tool SHALL retain `plugin.go`, `commands.go`, `stream_commands.go`, `config.go`, `device.go`, and `helpers.go` in `plugins/hikvision/internal/app/` with package_declaration `package app`.
3. IF `plugins/hikvision/internal/app/` contains any file whose sole purpose is a client implementation after the move, THEN THE Refactor_Tool SHALL remove that file.

### Requirement 5: petkit 插件 — 创建 internal/client 包

**User Story:** As a developer, I want petkit's vendor client code separated into its own package, so that HTTP transport, auth, and MQTT logic are isolated from the plugin runtime.

#### Acceptance Criteria

1. THE Refactor_Tool SHALL create the directory `plugins/petkit/internal/client/`.
2. THE Refactor_Tool SHALL move `client.go` (containing `Client` struct and `NewClient`) to `plugins/petkit/internal/client/client.go` with package_declaration `package client`.
3. THE Refactor_Tool SHALL move `client_auth.go` to `plugins/petkit/internal/client/auth.go` with package_declaration `package client`.
4. THE Refactor_Tool SHALL move `client_commands.go` to `plugins/petkit/internal/client/commands.go` with package_declaration `package client`.
5. THE Refactor_Tool SHALL move `client_controls.go` to `plugins/petkit/internal/client/controls.go` with package_declaration `package client`.
6. THE Refactor_Tool SHALL move `client_feeder.go` to `plugins/petkit/internal/client/feeder.go` with package_declaration `package client`.
7. THE Refactor_Tool SHALL move `client_mapping.go` to `plugins/petkit/internal/client/mapping.go` with package_declaration `package client`.
8. THE Refactor_Tool SHALL move `client_paths.go` to `plugins/petkit/internal/client/paths.go` with package_declaration `package client`.
9. THE Refactor_Tool SHALL move `client_sync.go` to `plugins/petkit/internal/client/sync.go` with package_declaration `package client`.
10. THE Refactor_Tool SHALL move `client_transport.go` to `plugins/petkit/internal/client/transport.go` with package_declaration `package client`.
11. THE Refactor_Tool SHALL move `mqtt.go` to `plugins/petkit/internal/client/mqtt.go` with package_declaration `package client`.
12. WHEN the above files are moved, THE Refactor_Tool SHALL remove the original files from `plugins/petkit/internal/app/`.

### Requirement 6: petkit 插件 — 更新 internal/app 包

**User Story:** As a developer, I want petkit's plugin runtime files to have the `plugin_` prefix stripped and all client references updated to use the new import path, so that the app package is clean and self-consistent.

#### Acceptance Criteria

1. THE Refactor_Tool SHALL rename `plugin_runtime.go` to `runtime.go` within `plugins/petkit/internal/app/`.
2. THE Refactor_Tool SHALL rename `plugin_sync.go` to `sync.go` within `plugins/petkit/internal/app/`.
3. THE Refactor_Tool SHALL rename `plugin_mqtt.go` to `mqtt.go` within `plugins/petkit/internal/app/`.
4. THE Refactor_Tool SHALL rename `plugin_events.go` to `events.go` within `plugins/petkit/internal/app/`.
5. THE Refactor_Tool SHALL rename `plugin_config.go` to `config.go` within `plugins/petkit/internal/app/`.
6. WHEN any file in `plugins/petkit/internal/app/` references `Client`, `NewClient`, `mqttListener`, `newMQTTListener`, `iotMQTTConfig`, `sessionInfo`, `petkitDeviceInfo`, or `petkitRequestError`, THE Refactor_Tool SHALL qualify those references with the `client.` prefix and add the import `github.com/chentianyu/celestia/plugins/petkit/internal/client`.
7. THE Refactor_Tool SHALL retain `plugin.go`, `normalize.go` in `plugins/petkit/internal/app/` with package_declaration `package app`.

### Requirement 7: 测试文件迁移

**User Story:** As a developer, I want test files to follow their subject code into the correct package, so that tests compile and run against the right package.

#### Acceptance Criteria

1. THE Refactor_Tool SHALL move `plugins/petkit/internal/app/client_feeder_test.go` to `plugins/petkit/internal/client/feeder_test.go` and update its package_declaration to `package client` or `package client_test`.
2. THE Refactor_Tool SHALL move `plugins/petkit/internal/app/client_test.go` to `plugins/petkit/internal/client/client_test.go` and update its package_declaration to `package client` or `package client_test`.
3. THE Refactor_Tool SHALL retain `plugins/petkit/internal/app/plugin_test.go` in `plugins/petkit/internal/app/` and update its package_declaration to `package app` or `package app_test` if not already correct.
4. THE Refactor_Tool SHALL retain `plugins/hikvision/internal/app/config_test.go` in `plugins/hikvision/internal/app/` with package_declaration `package app` or `package app_test`.
5. THE Refactor_Tool SHALL retain `plugins/hikvision/internal/app/stream_commands_test.go` in `plugins/hikvision/internal/app/` with package_declaration `package app` or `package app_test`.

### Requirement 8: 构建与测试验证

**User Story:** As a developer, I want the entire module to build and all tests to pass after the refactor, so that no regressions are introduced.

#### Acceptance Criteria

1. WHEN the refactor is complete, THE Build_System SHALL compile all three plugins (`haier`, `hikvision`, `petkit`) without errors.
2. WHEN the refactor is complete, THE Build_System SHALL compile the root module without errors.
3. WHEN the refactor is complete, THE Build_System SHALL run all existing tests in `plugins/haier/...`, `plugins/hikvision/...`, and `plugins/petkit/...` without failures.
4. IF any import path in the module references a moved symbol without the correct package qualifier, THEN THE Build_System SHALL report a compile error that the Refactor_Tool SHALL resolve before the task is considered complete.

### Requirement 9: xiaomi 插件不变

**User Story:** As a developer, I want the xiaomi plugin to remain untouched, so that its already-correct structure is not accidentally broken.

#### Acceptance Criteria

1. THE Refactor_Tool SHALL NOT modify any file under `plugins/xiaomi/`.
2. WHEN the refactor is complete, THE Build_System SHALL compile `plugins/xiaomi/...` without errors.

# Tasks: plugin-dir-refactor

## Task List

- [x] 1. haier — 创建 internal/client 包并迁移文件
  - [x] 1.1 创建目录 `plugins/haier/internal/client/`
  - [x] 1.2 将 `internal/app/client.go` 复制为 `internal/client/client.go`，修改 package 声明为 `package client`
  - [x] 1.3 将 `internal/app/client_auth.go` 复制为 `internal/client/auth.go`，修改 package 声明为 `package client`
  - [x] 1.4 将 `internal/app/client_devices.go` 复制为 `internal/client/devices.go`，修改 package 声明为 `package client`
  - [x] 1.5 将 `internal/app/client_wss.go` 复制为 `internal/client/wss.go`，修改 package 声明为 `package client`
  - [x] 1.6 将 `internal/app/client_helpers.go` 复制为 `internal/client/helpers.go`，修改 package 声明为 `package client`
  - [x] 1.7 将 `internal/app/wss_listener.go` 复制为 `internal/client/wss_listener.go`，修改 package 声明为 `package client`
  - [x] 1.8 删除 `internal/app/` 中已迁移的原始文件（client.go, client_auth.go, client_devices.go, client_wss.go, client_helpers.go, wss_listener.go）

- [x] 2. haier — 更新 internal/app 包
  - [x] 2.1 将 `plugin_commands.go` 重命名为 `commands.go`
  - [x] 2.2 将 `plugin_controls.go` 重命名为 `controls.go`
  - [x] 2.3 将 `plugin_devices.go` 重命名为 `devices.go`
  - [x] 2.4 将 `plugin_helpers.go` 重命名为 `helpers.go`
  - [x] 2.5 将 `plugin_refresh.go` 重命名为 `refresh.go`
  - [x] 2.6 将 `plugin_wss.go` 重命名为 `wss.go`
  - [x] 2.7 在 `internal/app/` 所有文件中，将对 `haierClient`、`wssListener`、`newHaierClient`、`newWSSListener`、`parseWSSDeviceUpdate`、`getWSSGatewayURL`、`stringFromAny`、`applianceOptions` 等符号的引用加上 `client.` 前缀，并添加 import `github.com/chentianyu/celestia/plugins/haier/internal/client`
  - [x] 2.8 确认 `plugin.go` 和 `types.go` 保留在 `internal/app/`，package 声明为 `package app`

- [x] 3. hikvision — 创建 internal/client 包并迁移文件
  - [x] 3.1 创建目录 `plugins/hikvision/internal/client/`
  - [x] 3.2 将 `internal/app/client.go` 复制为 `internal/client/client.go`，修改 package 声明为 `package client`
  - [x] 3.3 将 `internal/app/client_stub.go` 复制为 `internal/client/stub.go`，修改 package 声明为 `package client`
  - [x] 3.4 将 `internal/app/client_unsupported.go` 复制为 `internal/client/unsupported.go`，修改 package 声明为 `package client`
  - [x] 3.5 将 `internal/app/hcnet_client_linux.go` 复制为 `internal/client/hcnet_client_linux.go`，修改 package 声明为 `package client`
  - [x] 3.6 将 `internal/app/hcnet_cgo_linux.go` 复制为 `internal/client/hcnet_cgo_linux.go`，修改 package 声明为 `package client`
  - [x] 3.7 将 `internal/app/hcnet_types_compat.h` 复制为 `internal/client/hcnet_types_compat.h`
  - [x] 3.8 删除 `internal/app/` 中已迁移的原始文件（client.go, client_stub.go, client_unsupported.go, hcnet_client_linux.go, hcnet_cgo_linux.go, hcnet_types_compat.h）

- [x] 4. hikvision — 更新 internal/app 包
  - [x] 4.1 在 `internal/app/plugin.go` 中，将对 `cameraClient`、`newCameraClient`、`cameraStatus` 的引用加上 `client.` 前缀，并添加 import `github.com/chentianyu/celestia/plugins/hikvision/internal/client`
  - [x] 4.2 确认 `plugin.go`、`commands.go`、`stream_commands.go`、`config.go`、`device.go`、`helpers.go` 保留在 `internal/app/`，package 声明为 `package app`

- [x] 5. petkit — 创建 internal/client 包并迁移文件
  - [x] 5.1 创建目录 `plugins/petkit/internal/client/`
  - [x] 5.2 将 `internal/app/client.go` 复制为 `internal/client/client.go`，修改 package 声明为 `package client`
  - [x] 5.3 将 `internal/app/client_auth.go` 复制为 `internal/client/auth.go`，修改 package 声明为 `package client`
  - [x] 5.4 将 `internal/app/client_commands.go` 复制为 `internal/client/commands.go`，修改 package 声明为 `package client`
  - [x] 5.5 将 `internal/app/client_controls.go` 复制为 `internal/client/controls.go`，修改 package 声明为 `package client`
  - [x] 5.6 将 `internal/app/client_feeder.go` 复制为 `internal/client/feeder.go`，修改 package 声明为 `package client`
  - [x] 5.7 将 `internal/app/client_mapping.go` 复制为 `internal/client/mapping.go`，修改 package 声明为 `package client`
  - [x] 5.8 将 `internal/app/client_paths.go` 复制为 `internal/client/paths.go`，修改 package 声明为 `package client`
  - [x] 5.9 将 `internal/app/client_sync.go` 复制为 `internal/client/sync.go`，修改 package 声明为 `package client`
  - [x] 5.10 将 `internal/app/client_transport.go` 复制为 `internal/client/transport.go`，修改 package 声明为 `package client`
  - [x] 5.11 将 `internal/app/mqtt.go` 复制为 `internal/client/mqtt.go`，修改 package 声明为 `package client`
  - [x] 5.12 删除 `internal/app/` 中已迁移的原始文件（client.go, client_auth.go, client_commands.go, client_controls.go, client_feeder.go, client_mapping.go, client_paths.go, client_sync.go, client_transport.go, mqtt.go）

- [x] 6. petkit — 更新 internal/app 包
  - [x] 6.1 将 `plugin_runtime.go` 重命名为 `runtime.go`
  - [x] 6.2 将 `plugin_sync.go` 重命名为 `sync.go`
  - [x] 6.3 将 `plugin_mqtt.go` 重命名为 `mqtt.go`
  - [x] 6.4 将 `plugin_events.go` 重命名为 `events.go`
  - [x] 6.5 将 `plugin_config.go` 重命名为 `config.go`
  - [x] 6.6 在 `internal/app/` 所有文件中，将对 `Client`、`NewClient`、`mqttListener`、`newMQTTListener`、`iotMQTTConfig`、`sessionInfo`、`petkitDeviceInfo`、`petkitRequestError` 的引用加上 `client.` 前缀，并添加 import `github.com/chentianyu/celestia/plugins/petkit/internal/client`
  - [x] 6.7 确认 `plugin.go` 和 `normalize.go` 保留在 `internal/app/`，package 声明为 `package app`

- [x] 7. 测试文件迁移
  - [x] 7.1 将 `plugins/petkit/internal/app/client_feeder_test.go` 移动到 `plugins/petkit/internal/client/feeder_test.go`，更新 package 声明
  - [x] 7.2 将 `plugins/petkit/internal/app/client_test.go` 移动到 `plugins/petkit/internal/client/client_test.go`，更新 package 声明
  - [x] 7.3 确认 `plugins/petkit/internal/app/plugin_test.go` 的 package 声明为 `package app` 或 `package app_test`
  - [x] 7.4 确认 `plugins/hikvision/internal/app/config_test.go` 的 package 声明为 `package app` 或 `package app_test`
  - [x] 7.5 确认 `plugins/hikvision/internal/app/stream_commands_test.go` 的 package 声明为 `package app` 或 `package app_test`

- [x] 8. 构建与测试验证
  - [x] 8.1 运行 `go build ./plugins/haier/...` 确认无编译错误
  - [x] 8.2 运行 `go build ./plugins/hikvision/...` 确认无编译错误
  - [x] 8.3 运行 `go build ./plugins/petkit/...` 确认无编译错误
  - [x] 8.4 运行 `go build ./...` 确认根模块无编译错误
  - [x] 8.5 运行 `go test ./plugins/haier/... ./plugins/hikvision/... ./plugins/petkit/...` 确认所有测试通过

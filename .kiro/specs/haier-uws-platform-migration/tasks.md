# 任务列表：Haier UWS 平台迁移

## 任务

- [x] 1. 精简 AccountConfig 并新建签名模块
  - [x] 1.1 替换 `client/types.go`：移除 Email/Password/MobileID/Timezone 字段，保留 Name/ClientID/RefreshToken，更新 NormalizedName() 和 HasCredentials()
  - [x] 1.2 新建 `client/signer.go`：实现 `Sign(urlPath, bodyStr, timestamp string) string`，常量 uwsAppID/uwsAppKey，SHA256 十六进制小写输出
  - [x] 1.3 新建 `client/signer_test.go`：固定向量单元测试 + 属性 1（确定性与格式，rapid，100 次迭代）

- [x] 2. 替换认证模块
  - [x] 2.1 替换 `client/auth.go`：移除所有 hOn 认证函数（loginWithPassword/introduce/handleRedirects/followOnce/loadLoginPage/submitLogin/loadToken/extractTokens/extractTokensFromText/apiLogin），实现 `Authenticate(ctx)`（调用 refreshAccessToken）和 `refreshAccessToken(ctx)`（POST zj.haier.net/api-gw/oauthserver/account/v1/refreshToken）
  - [x] 2.2 新建 `client/auth_test.go`：httptest mock 刷新接口的单元测试 + 属性 2（token 解析往返，rapid，100 次迭代）

- [x] 3. 替换 HTTP 客户端核心
  - [x] 3.1 替换 `client/client.go`：移除 hOn 常量（haierAuthAPI/haierClientID/haierScope/haierAPIKey/haierOSVersion/haierOS/haierDeviceModel/haierUserAgent）和 haierAuthState 中的 IDToken/CognitoToken 字段；重命名结构体为 UWSClient；实现携带完整 UWS 签名头的 `requestJSON`（401 时自动刷新一次后重试）
  - [x] 3.2 新建 `client/types_test.go`：属性 6（空白凭证拒绝，rapid，100 次迭代）

- [x] 4. 替换设备 API
  - [x] 4.1 替换 `client/devices.go`：移除 LoadCommands/LoadAttributes/LoadStatistics/LoadMaintenance/SendCommand（HTTP 版本）；实现 `LoadAppliances(ctx)`（GET /uds/v1/protected/deviceinfos，检查 retCode）和 `LoadDigitalModels(ctx, deviceIDs)`（POST /shadow/v1/devdigitalmodels，解析 attributes 数组）
  - [x] 4.2 新建 `client/devices_test.go`：httptest 单元测试 + 属性 3（设备列表解析完整性）+ 属性 4（非零 retCode 返回错误）+ 属性 5（数字模型属性解析往返），均使用 rapid，100 次迭代

- [x] 5. 替换 WebSocket 模块
  - [x] 5.1 替换 `client/wss.go`：更新 `getWSSGatewayURL` 请求体（clientId + accessToken 替代 CognitoToken），更新 WSS 连接 URL 格式（`{agAddr}/userag?token={accessToken}&agClientId={clientId}`）
  - [x] 5.2 更新 `client/wss_listener.go`：`connect()` 中使用 accessToken 替代 CognitoToken；新增 `SendCommand(ctx, deviceID, params)` 方法（topic: BatchCmdReq，连接不可用时返回错误）；重连前调用 `Authenticate` 刷新 token
  - [x] 5.3 新建 `client/wss_test.go`：属性 7（WSS 消息解码往返，rapid，100 次迭代）

- [x] 6. 更新 app 层辅助函数和类型
  - [x] 6.1 更新 `app/helpers.go`：精简 `parseAccountConfig`（移除 email/password/mobile_id/timezone/token 字段解析，改为解析 clientId/client_id 和 refresh_token/refreshToken）；移除 `firstNonEmpty` 对 email/username 的处理
  - [x] 6.2 更新 `app/types.go`：`accountRuntime.Config` 类型随 AccountConfig 精简自动更新，确认无遗留 hOn 字段引用

- [x] 7. 更新刷新编排逻辑
  - [x] 7.1 更新 `app/refresh.go`：替换 `refreshAccount` 中的 hOn 调用链（LoadCommands/LoadAttributes/LoadStatistics/LoadMaintenance/buildCapabilities）为 UWS 调用链（LoadAppliances → LoadDigitalModels → buildCapabilitiesFromDigitalModel → buildDevice → buildStateSnapshot）
  - [x] 7.2 更新 `app/refresh.go`：`syncAccountConfig` 中持久化逻辑保持不变（已正确实现），确认仅持久化 refreshToken 不持久化 accessToken
  - [x] 7.3 新建 `app/refresh_test.go`：属性 9（持久化幂等性，mock PersistPluginConfig，rapid，100 次迭代）

- [x] 8. 更新设备模型构建
  - [x] 8.1 更新 `app/devices.go`：新增 `buildCapabilitiesFromDigitalModel(attrs map[string]string) (map[string]string, map[string]bool)` 函数，根据数字模型属性键推断能力集（替代基于 hOn commands 的 buildCapabilities）；更新 `buildDevice` 使用 deviceId 替代 macAddress 作为 VendorDeviceID
  - [x] 8.2 更新 `app/devices.go`：更新 `buildStateSnapshot` 数据来源（从 hOn API 响应改为 UWS 数字模型属性 map[string]string），保持函数签名不变
  - [x] 8.3 新建 `app/devices_test.go`：属性 8（状态映射确定性，machMode 映射，rapid，100 次迭代）

- [x] 9. 更新命令发送路径
  - [x] 9.1 更新 `app/plugin.go`：`ExecuteCommand` 中将命令发送从 `account.Client.SendCommand(HTTP)` 改为 `account.WSS.SendCommand(WSS)`；更新 `HealthCheck` 返回消息从 `"hOn sessions active"` 改为 `"UWS sessions active"`
  - [x] 9.2 更新 `app/commands.go`：`commandForRequest` 返回的参数格式适配 WSS BatchCmdReq（cmdList 格式），移除 programName/ancillaryParameters 等 hOn 专用字段

- [x] 10. 清理 hOn 遗留辅助函数
  - [x] 10.1 更新 `client/helpers.go`：移除 hOn 专用函数（generateNonce/mustJSONString/mustJSONRawMessage/urlSafeUnescape）和正则表达式（loginContextRe/urlRefRe/tokenRe）；保留 StringFromAny/timezoneOffset/formatOffset/trimForError/randomHex

- [x] 11. 编译与静态分析验证
  - [x] 11.1 运行 `go build ./plugins/haier/...` 确认无编译错误
  - [x] 11.2 运行 `go vet ./plugins/haier/...` 确认无静态分析警告
  - [x] 11.3 运行 `go test ./plugins/haier/...` 确认所有单元测试和属性测试通过

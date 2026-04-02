# 需求文档

## 简介

将 Haier 插件的认证和 API 平台从国际 hOn 平台（account2.hon-smarthome.com + api-iot.he.services，基于 Salesforce OAuth + AWS Cognito）迁移到国内 UWS 平台（uws.haier.net），参考 banto6/haier Home Assistant 插件的实现方式。

迁移后，用户通过 `clientId` 和 `refreshToken` 配置账户，插件启动时自动通过 `refreshToken` 换取 `accessToken`（仅在内存中维护，不持久化），使用 HMAC-SHA256 请求签名与国内 UWS API 交互，通过 WebSocket（wss://uws.haier.net）接收实时设备状态推送并发送控制命令。

## 词汇表

- **UWS_Client**：替换现有 `HaierClient` 的国内平台 HTTP/WebSocket 客户端
- **UWS_Auth**：负责 accessToken 刷新和会话维护的认证模块
- **UWS_Signer**：负责生成每次请求签名的签名模块（SHA256）
- **UWS_Devices**：负责设备列表和数字模型状态获取的设备模块
- **UWS_WSS**：负责 WebSocket 连接、实时推送接收和命令发送的 WebSocket 模块
- **AccountConfig**：账户配置结构，存储 clientId 和 refreshToken（accessToken 仅在内存中维护，不持久化）
- **DigitalModel**：设备数字模型，包含设备属性键值对（来自 shadow/v1/devdigitalmodels）
- **accessToken**：UWS 平台的访问令牌，用于请求头 `accessToken` 字段
- **refreshToken**：用于刷新 accessToken 的令牌，通过 zj.haier.net 刷新接口获取新令牌
- **clientId**：用户在 UWS 平台的客户端标识，用于请求头 `clientId` 字段
- **appId**：固定应用标识 `MB-SHEZJAPPWXXCX-0000`，用于签名和请求头
- **appKey**：固定应用密钥 `79ce99cc7f9804663939676031b8a427`，用于签名计算
- **sign**：请求签名，格式为 `SHA256(urlPath + bodyStr + appId + appKey + timestamp)`
- **sequenceId**：每次请求的唯一序列号（UUID）
- **timestamp**：请求时间戳（毫秒级 Unix 时间戳字符串）

---

## 需求

### 需求 1：账户配置结构迁移

**用户故事：** 作为系统管理员，我希望通过 clientId 和 refreshToken 配置 Haier 账户，以便连接到国内 UWS 平台。

#### 验收标准

1. THE **AccountConfig** SHALL 包含 `clientId` 和 `refreshToken` 两个必填字段，以及可选的 `name` 字段。
2. THE **AccountConfig** SHALL 移除 `email`、`password`、`mobile_id`、`timezone`、`token` 字段（这些字段属于 hOn 平台或已不再持久化）。
3. WHEN 插件配置缺少 `clientId` 或 `refreshToken` 时，THE **UWS_Client** SHALL 返回明确的配置错误，拒绝启动。
4. THE **Plugin** SHALL 在 `ValidateConfig` 中验证每个账户条目均包含非空的 `clientId` 和 `refreshToken`。

---

### 需求 2：UWS 请求签名

**用户故事：** 作为开发者，我希望每次 API 请求都携带正确的 UWS 签名，以便通过平台的安全验证。

#### 验收标准

1. THE **UWS_Signer** SHALL 按照公式 `SHA256(urlPath + bodyStr + appId + appKey + timestamp)` 计算签名，其中 `urlPath` 为请求路径（不含域名和查询参数），`bodyStr` 为请求体的 JSON 字符串（GET 请求为空字符串），`timestamp` 为毫秒级 Unix 时间戳字符串。
2. THE **UWS_Client** SHALL 在每次 HTTP 请求中设置以下请求头：`accessToken`、`appId`（固定值 `MB-SHEZJAPPWXXCX-0000`）、`appKey`（固定值 `79ce99cc7f9804663939676031b8a427`）、`clientId`、`sequenceId`（UUID）、`sign`、`timestamp`、`timezone`（固定值 `Asia/Shanghai`）、`language`（固定值 `zh-cn`）。
3. WHEN 签名计算的输入参数相同时，THE **UWS_Signer** SHALL 产生相同的签名输出（确定性）。
4. THE **UWS_Signer** SHALL 使用十六进制小写字符串表示 SHA256 摘要结果。
5. FOR ALL 有效的签名输入，THE **UWS_Signer** SHALL 产生长度为 64 个字符的十六进制字符串。

---

### 需求 3：accessToken 刷新认证

**用户故事：** 作为系统，我希望插件启动时自动通过 refreshToken 换取 accessToken，并在 accessToken 过期时自动刷新，以便保持与 UWS 平台的持续连接。

#### 验收标准

1. WHEN 插件启动时，THE **UWS_Auth** SHALL 使用配置中的 `refreshToken` 向 `https://zj.haier.net/api-gw/oauthserver/account/v1/refreshToken` 发起请求，换取初始 `accessToken`，并仅将其保存在内存中。
2. WHEN UWS API 返回 HTTP 401 或业务错误码表示 token 失效时，THE **UWS_Auth** SHALL 使用内存中的 `refreshToken` 向刷新接口重新换取 `accessToken`。
3. WHEN 刷新请求成功时，THE **UWS_Auth** SHALL 更新内存中的 `accessToken` 和 `refreshToken`，并通过 Core 的 `coreapi.PersistPluginConfig` 仅将新的 `refreshToken` 持久化到插件配置（`accessToken` 不持久化）。
4. WHEN 刷新请求失败（网络错误或 refreshToken 已失效）时，THE **UWS_Auth** SHALL 返回明确的认证失败错误，不进行重试。
5. THE **UWS_Auth** SHALL 移除所有 hOn 平台的认证逻辑（Salesforce OAuth 流程、Cognito token 获取、`apiLogin` 调用）。
6. WHEN token 刷新成功后，THE **UWS_Auth** SHALL 重试原始失败的请求一次。

---

### 需求 4：设备列表获取

**用户故事：** 作为用户，我希望插件能发现我账户下的所有 Haier 设备，以便在网关中管理它们。

#### 验收标准

1. THE **UWS_Devices** SHALL 通过 HTTP GET 请求 `https://uws.haier.net/uds/v1/protected/deviceinfos` 获取设备列表。
2. WHEN 设备列表请求成功时，THE **UWS_Devices** SHALL 解析响应中的设备数组，提取每台设备的 `deviceId`、`deviceName`、`deviceType`、`online` 等字段。
3. THE **UWS_Devices** SHALL 移除对 hOn 平台设备 API（`/commands/v1/appliance`）的调用。
4. WHEN 设备列表响应中 `retCode` 不为 `"00000"` 时，THE **UWS_Devices** SHALL 返回包含 `retCode` 和 `retInfo` 的错误信息。

---

### 需求 5：设备数字模型状态获取

**用户故事：** 作为用户，我希望插件能获取设备的实时状态，以便在网关中显示准确的设备状态。

#### 验收标准

1. THE **UWS_Devices** SHALL 通过 HTTP POST 请求 `https://uws.haier.net/shadow/v1/devdigitalmodels` 获取一台或多台设备的数字模型状态，请求体包含 `deviceInfoList` 数组（每项含 `deviceId`）。
2. WHEN 数字模型请求成功时，THE **UWS_Devices** SHALL 解析响应中每台设备的属性列表（`attributes` 数组，每项含 `name` 和 `value`），构建键值对形式的设备状态。
3. THE **UWS_Devices** SHALL 移除对 hOn 平台状态 API（`/commands/v1/context`、`/commands/v1/statistics`、`/commands/v1/maintenance-cycle`）的调用。
4. WHEN 数字模型响应中某台设备的数据缺失时，THE **UWS_Devices** SHALL 跳过该设备并继续处理其他设备，不中断整体同步流程。

---

### 需求 6：WebSocket 实时推送与命令发送

**用户故事：** 作为用户，我希望设备状态变化能实时推送到网关，并能通过 WebSocket 发送控制命令，以便获得低延迟的设备控制体验。

#### 验收标准

1. THE **UWS_WSS** SHALL 通过 HTTP POST 请求 `https://uws.haier.net/gmsWS/wsag/assign` 获取 WebSocket 网关地址（`agAddr` 字段），请求体包含 `clientId` 和 `accessToken`。
2. WHEN 获取到网关地址后，THE **UWS_WSS** SHALL 建立 WSS 连接，连接 URL 格式为 `{agAddr}/userag?token={accessToken}&agClientId={clientId}`。
3. WHEN WSS 连接建立后，THE **UWS_WSS** SHALL 发送订阅消息（topic 为 `BoundDevs`，content 包含 `devs` 设备 ID 列表）以订阅设备状态推送。
4. WHEN 收到 topic 为 `GenMsgDown`、`businType` 为 `DigitalModel` 的消息时，THE **UWS_WSS** SHALL 解码消息（base64 → JSON → base64 → gzip → JSON），提取 `deviceId` 和属性列表，并触发状态更新回调。
5. THE **UWS_WSS** SHALL 每 60 秒发送一次心跳消息（topic 为 `HeartBeat`）以维持连接。
6. WHEN WSS 连接断开时，THE **UWS_WSS** SHALL 在 30 秒后自动重连，重连前重新执行 token 刷新和网关地址获取。
7. WHEN 需要发送设备控制命令时，THE **UWS_WSS** SHALL 通过 WSS 连接发送命令消息（topic 为 `WriteDeviceProperty` 或平台规定的命令 topic），而非通过 HTTP 接口发送。
8. WHEN WSS 连接不可用时，THE **UWS_WSS** SHALL 返回命令发送失败错误，不降级为 HTTP 命令接口。

---

### 需求 7：设备能力与状态映射

**用户故事：** 作为用户，我希望设备的 UWS 数字模型属性能正确映射到网关统一设备模型，以便控制面板显示正确的状态和控制项。

#### 验收标准

1. THE **Plugin** SHALL 从设备数字模型的属性列表中提取 `machMode`、`prCode`、`prPhase`、`remainingTimeMM`、`tempLevel`、`spinSpeed`、`delayTime`、`prewash`、`extraRinse`、`goodNight`、`totalElectricityUsed`、`totalWaterUsed`、`totalWashCycle` 等字段，映射到统一状态模型。
2. THE **Plugin** SHALL 根据设备数字模型中是否存在可写属性（如 `machMode`、`prCode`）来推断设备能力集，替代原有基于 `/commands/v1/retrieve` 的能力发现机制。
3. WHEN 设备数字模型中 `machMode` 值为 `"3"` 时，THE **Plugin** SHALL 将 `machine_status` 映射为 `"paused"`；值为 `"0"` 时映射为 `"idle"`；其他值映射为 `"running"`。
4. THE **Plugin** SHALL 保留现有的 `buildDevice`、`buildStateSnapshot` 函数签名和返回类型，仅替换数据来源（从 hOn API 响应改为 UWS 数字模型属性）。

---

### 需求 8：插件配置持久化

**用户故事：** 作为系统，我希望 token 刷新后新的凭证能自动持久化，以便重启后无需重新认证。

#### 验收标准

1. WHEN `refreshToken` 刷新成功后，THE **Plugin** SHALL 通过 `coreapi.PersistPluginConfig` 将更新后的账户配置（仅含新 `refreshToken`，不含 `accessToken`）写回 Core 持久化存储。
2. THE **Plugin** SHALL 在 `syncAccountConfig` 函数中检测 `refreshToken` 是否发生变化，仅在变化时触发持久化，避免不必要的写操作。
3. THE **Plugin** SHALL 移除对 hOn 平台 `refresh_token` 字段的持久化逻辑，改为仅持久化 UWS 平台的 `refreshToken` 字段，不持久化 `accessToken`。

---

### 需求 9：移除 hOn 平台遗留代码

**用户故事：** 作为开发者，我希望代码库中不再包含 hOn 平台的遗留实现，以便降低维护复杂度。

#### 验收标准

1. THE **Plugin** SHALL 移除 `client/auth.go` 中所有 hOn 平台认证函数：`loginWithPassword`、`introduce`、`handleRedirects`、`followOnce`、`loadLoginPage`、`submitLogin`、`loadToken`、`extractTokens`、`extractTokensFromText`、`apiLogin`。
2. THE **Plugin** SHALL 移除 `client/client.go` 中所有 hOn 平台常量：`haierAuthAPI`、`haierClientID`、`haierScope`、`haierAPIKey`、`haierOSVersion`、`haierOS`、`haierDeviceModel`、`haierUserAgent`，以及 `haierAuthState` 结构中的 `IDToken` 和 `CognitoToken` 字段。
3. THE **Plugin** SHALL 移除 `client/devices.go` 中对 hOn 平台 API 的调用：`LoadCommands`、`LoadAttributes`、`LoadStatistics`、`LoadMaintenance`，以及 `SendCommand` 中通过 HTTP POST 到 `/commands/v1/send` 的实现。
4. THE **Plugin** SHALL 移除 `app/helpers.go` 中 `parseAccountConfig` 对 `email`、`password`、`mobile_id`、`timezone`、`token` 字段的解析逻辑。
5. THE **Plugin** SHALL 移除 `app/plugin.go` 中 `HealthCheck` 返回消息中对 `"hOn sessions active"` 的引用，改为 `"UWS sessions active"`。
6. THE **Plugin** SHALL 移除 `client/helpers.go` 中仅服务于 hOn 认证流程的辅助函数：`generateNonce`、`mustJSONString`、`mustJSONRawMessage`、`urlSafeUnescape`，以及 `loginContextRe`、`urlRefRe`、`tokenRe` 正则表达式。

---

### 需求 10：端到端集成正确性

**用户故事：** 作为开发者，我希望迁移后的插件能通过编译检查和基本集成验证，以便确认实现的正确性。

#### 验收标准

1. THE **Plugin** SHALL 在 `go build ./plugins/haier/...` 时无编译错误。
2. THE **Plugin** SHALL 在 `go vet ./plugins/haier/...` 时无静态分析警告。
3. WHEN 提供有效的 `clientId`、`token`、`refreshToken` 配置时，THE **Plugin** SHALL 能成功完成设备发现并返回至少一台设备（集成测试，需真实凭证）。
4. THE **UWS_Signer** SHALL 提供单元测试，验证已知输入产生已知签名输出（round-trip 属性：相同输入 → 相同输出）。
5. THE **UWS_Auth** SHALL 提供单元测试，验证 token 刷新响应的解析逻辑（round-trip 属性：解析刷新响应 → 正确提取新 token）。

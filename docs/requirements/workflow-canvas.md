# Workflow Canvas

## Goal

提供一套通用 workflow 画布能力，让 Topic Summary、Market Analysis、设备状态触发、Recognition 等链路都能基于同一套节点、分组、连线和执行模型进行拼装，而不是为每个业务单独实现一套页面和运行链路。

## This Delivery

- Admin 侧提供基于 React Flow 的 workflow 画布。
- 用户可以创建、保存、删除、运行 workflow。
- workflow 支持节点、连线、基础分组（group 节点 + `parent_id` 归属）。
- 首批节点：
  - `Timer`
  - `Device State Changed`
  - `Device State Is`
  - `Time Window`
  - `RSS Sources`
  - `Text`
  - `LLM`
  - `Search Provider`
  - `WeCom Output`
  - `Device Command`
  - `Agent Function`
- `LLM` 节点暴露 `prompt`、`context`、`search`、`tool`、`skill` 输入端口，以及 `text` 输出端口。
- `Text` 节点支持在画布中直接编辑内容，并支持多个 `Text` 节点自顶部接入后按连线顺序拼接，再追加当前节点自身文本。
- 本次实际执行链路支持：
  - `text -> text`
  - `text -> llm.prompt`
  - `context`
  - `search`
  - `text -> WeCom Output`
- `tool` / `skill` 端口本次只做画布级保留，不做伪执行；如果连入运行链路，后端显式报错。
- 不在代码中预置任何默认 workflow，用户自行拼装。

## Runtime Rules

- RSS 节点抓取真实 RSS/Atom 源并按 sent log 去重。
- Search 节点走现有 Core search provider。
- LLM 节点走现有 Agent LLM provider。
- WeCom 输出仍通过 Touchpoint 边界发送，不能在 Agent runtime 内部重建 transport。
- Device Command 节点走 Core policy/audit/plugin command executor，不能绕过现有设备控制链路。
- Agent Function 节点走 project input envelope，仍然先经过 slash command dispatch 再进入 Agent ReAct。
- Timer、Device State Changed、Device State Is 是可以自主触发 workflow 的 trigger 节点。
- Time Window 是 trigger 附属 gate 节点，可与 trigger 并联或直接串联到触发路径上约束触发时间。
- 成功发送到 WeCom 的 RSS 项才写入 sent log。

## Out Of Scope

- 通用 Tool/Skill 执行节点
- 默认 workflow 模板市场或预制图库

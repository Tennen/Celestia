# Workflow Canvas

## Goal

提供一套通用 workflow 画布能力，让 Topic Summary、Market Analysis、Automation、Recognition 等链路都能基于同一套节点、分组、连线和执行模型进行拼装，而不是为每个业务单独实现一套页面和运行链路。

## This Delivery

- Admin 侧提供基于 React Flow 的 workflow 画布。
- 用户可以创建、保存、删除、运行 workflow。
- workflow 支持节点、连线、基础分组（group 节点 + `parent_id` 归属）。
- 首批节点：
  - `RSS Sources`
  - `Prompt Unit`
  - `LLM`
  - `Search Provider`
  - `WeCom Output`
- `LLM` 节点暴露 `prompt`、`context`、`search`、`tool`、`skill` 输入端口，以及 `text` 输出端口。
- 本次实际执行链路支持：
  - `prompt`
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
- 成功发送到 WeCom 的 RSS 项才写入 sent log。

## Out Of Scope

- 通用 Tool/Skill 执行节点
- Automation / Recognition 的迁移
- 默认 workflow 模板市场或预制图库

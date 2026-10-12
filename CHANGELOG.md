# Changelog

所有 notable 变更记录在此文件。格式基于 [Keep a Changelog](https://keepachangelog.com/)。

## [0.1.0] - 2026-10-12

### Added
- 首版发布
- CLI 入口：`tb.mjs`（serve / routes / health / skill 等命令）
- Web 面板：基于 Vite + React 19，与 CLI 同源 action 声明
- 随包分发 skill：`nx-tb`（含 SKILL.md + install hook）
- 工具：`logo-gen`（生成品牌 logo）

### Notes
- 本版本对应 `package.json#version: 0.1.0`
- 数据湖位置：`~/.tabboard/`（可通过环境变量覆盖）
- 状态存储：`<project root>/store.json`（运行时自动生成）

[0.1.0]: https://github.com/On-DevPlan/nx-tb/releases/tag/v0.1.0

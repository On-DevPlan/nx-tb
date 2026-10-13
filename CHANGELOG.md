# Changelog

所有 notable 变更记录在此文件。格式基于 [Keep a Changelog](https://keepachangelog.com/)。

## [0.2.0] - 2026-10-13

### Added
- **TabBoard native host 随包分发**（Go 编写，2.4 MB 自包含 exe，无需 Node 运行时）
  - `native-host/`：Go 源码（纯 stdlib，零外部依赖）
  - `binaries/win32-x64/tabboard-host.exe`：编译产物，CI 自动更新
  - 双 transport：Chrome stdio（native messaging 协议）+ CLI（argv）
- **`nx-tb tb` 命令组**（数据湖 + host 管理）：
  - `tb init` — 注册 native host 到 Chrome/Edge（首次跑一次）
  - `tb list [--kind <k>]` — 列数据湖内容
  - `tb show <kind> <id>` — 查单条
  - `tb backup --file <json>` — 写数据湖
  - `tb doctor` — 健康检查（exe / 数据湖 / manifest / 注册表 / 上次同步）
  - `tb skill [<name>]` — 跑数据湖 skill（recent-browse / favorites-analyze）
- 新 CI：`build-host.yml`（Go 编译 → 三层冒烟 → 二进制 commit 回仓库）

### Changed
- **`npm-publish.yml` 改为 `workflow_run` 触发**（挂在 build-host 之后）——
  保证 npm 上的包永远含最新二进制，不会发到旧产物
- `package.json` 加 `os: [win32]` / `cpu: [x64]`（平台不匹配直接拒绝安装）
- `files` 加 `binaries/`

### Notes
- 数据湖位置：`~/.tabboard/lake/`（JSONL，append-only；`TABBOARD_LAKE_DIR` 可覆盖）
- 二进制安装位置：npm 包内 `binaries/<platform>/`；`tb init` 会复制到 `~/.tabboard/bin/` 稳定目录再注册

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

[0.2.0]: https://github.com/On-DevPlan/nx-tb/releases/tag/v0.2.0
[0.1.0]: https://github.com/On-DevPlan/nx-tb/releases/tag/v0.1.0

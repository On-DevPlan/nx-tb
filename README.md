# nx-tb

本机工具：**CLI 与 Web 面板同源**。一条 `action` 声明同时派生出 CLI 子命令与 HTTP 路由，Web 上能做的，CLI 都能做。

## 安装

```bash
# 直接用 npx（不需要装）
npx nx-tb <command>

# 或者全局装
npm i -g nx-tb
```

## 常用命令

```bash
npx nx-tb serve                       # 启动 Web 面板（默认端口 7893）
npx nx-tb routes                      # 查看全部命令与 HTTP 路由对照
npx nx-tb health                      # 健康检查
npx nx-tb skill list                  # 列出可装 skill
npx nx-tb skill get nx-tb             # 导出 nx-tb 主 skill 文档
npx nx-tb skill install nx-tb         # 装给 agent
```

加 `--json` 可获取机器可读输出（错误时同样输出 JSON）。

端口被占不会报错退出：占用者就是**本项目自己的面板** → 不重复启动，直接把浏览器指向它；
占用者是别的服务 → 顺延到下一个空闲端口起自己的面板，并把浏览器指向新面板。
不想开浏览器：`--no-open`（只影响这一次）或 `NX_SERVE_NO_OPEN=1`（所有 nx-xx 项目通用的开关，对整个 shell 生效）。

## 项目结构

```
nx-tb/
├── bin/                    # CLI 唯一入口（tb.mjs）
├── src/
│   ├── core/               # paths / errors / open（基础设施）
│   ├── modules/            # 功能域（home / skill）
│   ├── runtime/            # action registry / CLI runner / HTTP server
│   └── web/                # React 面板
├── assets/                 # 随包分发的 skill
│   ├── nx-tb/              # 主 skill（SKILL.md + install hook）
│   └── groups.json         # skill group 配置
├── tests/                  # 冒烟 + 单元测试
└── tools/                  # 内部工具（logo-gen 等）
```

## 文档

完整文档在 `assets/nx-tb/SKILL.md`（用 `npx nx-tb skill get nx-tb` 取出）。

## License

MIT

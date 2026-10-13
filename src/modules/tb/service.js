// tb 域 service：native host 二进制的定位 + 调用。
//
// host 是 Go 编译的自包含 exe（~2.3MB），安装时随 npm 包分发在 binaries/<platform>-<arch>/。
// 本模块只做两件事：找到它、把命令透传过去。数据湖读写逻辑全在 Go 侧（单一实现）。
import { spawn } from 'node:child_process';
import { existsSync } from 'node:fs';
import { homedir } from 'node:os';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { badInput } from '../../core/errors/index.js';

const PKG_ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..', '..');

// 平台 key：win32-x64 / darwin-arm64 / linux-x64 …
export function platformKey() {
  return `${process.platform}-${process.arch}`;
}

/**
 * 二进制路径解析（fallback 链，顺序即优先级）：
 *   1. 环境变量 NX_TB_HOST_BINARY —— 调试 / 自定义构建覆盖
 *   2. 随包的 binaries/<platform>-<arch>/ —— npm install 用户的主路径
 *   3. native-host/ 下的本地构建 —— 仓库开发者的 go build 产物
 *   4. ~/.tabboard/bin/ —— host 自己 install 到稳定目录后的位置
 */
export function hostBinaryPath() {
  const exe = process.platform === 'win32' ? 'tabboard-host.exe' : 'tabboard-host';
  const candidates = [
    process.env.NX_TB_HOST_BINARY,
    join(PKG_ROOT, 'binaries', platformKey(), exe),
    join(PKG_ROOT, 'native-host', exe),
    join(homedir(), '.tabboard', 'bin', exe),
  ].filter(Boolean);

  for (const p of candidates) {
    if (existsSync(p)) return p;
  }
  throw badInput(
    `未找到 native host 二进制。三条出路：\n` +
      `  1. npm i -g nx-tb（重新安装带二进制的包）\n` +
      `  2. cd native-host && go build -ldflags="-s -w" -trimpath -o ${exe} .\n` +
      `  3. 设 NX_TB_HOST_BINARY=<绝对路径>`
  );
}

/**
 * 调 host 一条命令（CLI 模式，--json 输出）。
 * @param {string[]} args 形如 ['list', '--kind', 'groups']
 * @returns {Promise<object>} host 的 {status, data} 或 {status:'error', message}
 */
export function callHost(args) {
  const bin = hostBinaryPath();
  return new Promise((resolve) => {
    const child = spawn(bin, ['--json', ...args], { stdio: ['ignore', 'pipe', 'pipe'] });
    let out = '';
    let err = '';
    child.stdout.on('data', (d) => { out += d.toString(); });
    child.stderr.on('data', (d) => { err += d.toString(); });
    child.on('error', (e) => resolve({ status: 'error', message: `启动 host 失败: ${e.message}` }));
    child.on('close', (code) => {
      if (code !== 0 && !out.trim()) {
        return resolve({ status: 'error', message: err.trim() || `host 退出码 ${code}` });
      }
      try {
        resolve(JSON.parse(out));
      } catch {
        resolve({ status: 'error', message: `host 输出无法解析: ${out.slice(0, 200)}` });
      }
    });
  });
}

// 便捷包装：调用并把 host 的 data 拍平进返回值（供 action 的 run 直接用）
export async function callHostData(args) {
  const resp = await callHost(args);
  if (resp.status === 'error') {
    throw new Error(resp.message || 'host 返回错误');
  }
  return resp.data ?? {};
}

// ---- 数据湖 skill（Node 侧处理，Go host 只负责协议 + 读写）----

const SKILLS = [
  { name: 'recent-browse', summary: '列出最近浏览的收藏（--days N --limit N）' },
  { name: 'favorites-analyze', summary: '统计分组/域名分布' },
];

/**
 * 跑一个 skill。
 * @param {string} name 'list' 列出全部；其余为 skill 名
 */
export async function runSkill(name, opts = {}) {
  if (name === 'list') {
    return { status: 'ok', skill: 'list', skills: SKILLS };
  }

  if (name === 'recent-browse') {
    const { records } = await callHostData(['list', '--kind', 'tabs', '--limit', '5000']);
    const since = Date.now() - (opts.days ?? 7) * 86400_000;
    const recent = records
      .filter((t) => (t._ts ?? 0) >= since)
      .sort((a, b) => (b.visitCount ?? 0) - (a.visitCount ?? 0))
      .slice(0, opts.limit ?? 20);
    return {
      status: 'ok',
      skill: 'recent-browse',
      scope: `最近 ${opts.days ?? 7} 天（按访问次数排序）`,
      total: records.length,
      returned: recent.length,
      tabs: recent.map((t) => ({
        title: t.title,
        url: t.url,
        groupId: t.groupId,
        visitCount: t.visitCount ?? 0,
      })),
    };
  }

  if (name === 'favorites-analyze') {
    const [groups, tabs] = await Promise.all([
      callHostData(['list', '--kind', 'groups', '--limit', '1000']),
      callHostData(['list', '--kind', 'tabs', '--limit', '10000']),
    ]);
    const byGroup = new Map();
    for (const t of tabs.records ?? []) {
      const gid = t.groupId ?? '(unknown)';
      byGroup.set(gid, (byGroup.get(gid) ?? 0) + 1);
    }
    const domains = new Map();
    for (const t of tabs.records ?? []) {
      try {
        const host = new URL(t.url).hostname;
        domains.set(host, (domains.get(host) ?? 0) + 1);
      } catch { /* 跳过非法 URL */ }
    }
    return {
      status: 'ok',
      skill: 'favorites-analyze',
      groups: (groups.records ?? []).map((g) => ({
        name: g.name,
        tabCount: byGroup.get(g.id) ?? 0,
      })),
      topDomains: [...domains.entries()]
        .sort((a, b) => b[1] - a[1])
        .slice(0, 10)
        .map(([domain, count]) => ({ domain, count })),
    };
  }

  return { status: 'error', skill: name, message: `未知 skill: ${name}（跑 tb skill list 看可用列表）` };
}

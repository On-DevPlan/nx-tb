// tb 域：TabBoard 数据湖 + native host 管理。
//
// 一条 action 同时派生 CLI（nx-tb tb <cmd>）与 HTTP 路由；实现全在 service.js
// （委托给随包分发的 Go native host 二进制）。
import * as service from './service.js';

export default {
  id: 'tb',
  title: 'TabBoard 数据湖',
  order: 30,
  actions: [
    {
      id: 'tb.init',
      // 别名：nx-tb init ≡ nx-tb tb init（注册就是 tb 功能的总入口，值得少打两个字）
      cli: [['tb', 'init'], ['init']],
      http: ['POST', '/api/tb/init'],
      summary: '注册 native host 到 Chrome/Edge（首次使用跑一次）',
      flags: { 'skip-registry': { type: 'boolean' } },
      run: async (ctx) => {
        const args = ['init'];
        if (ctx['skip-registry']) args.push('--skip-registry');
        return service.callHostData(args);
      },
      render: (r) => {
        const lines = [
          `host 已安装到稳定目录: ${r.exePath}`,
          `浏览器注册: ${r.registryOk ? '成功（' + (r.registry || []).join(' / ') + '）' : '失败'}`,
          '',
          r.next || '',
        ];
        return lines.join('\n');
      },
    },
    {
      id: 'tb.backup',
      cli: ['tb', 'backup'],
      http: ['POST', '/api/tb/backup'],
      summary: '把 records JSON 文件写进数据湖',
      flags: { file: { type: 'string', required: true } },
      run: (ctx) => service.callHostData(['backup', '--file', ctx.file]),
      render: (r) => `已写入 ${r.wrote} 条（${(r.kinds || []).join(', ')}）→ ${r.lakeDir}`,
    },
    {
      id: 'tb.list',
      cli: ['tb', 'list'],
      http: ['GET', '/api/tb/list'],
      summary: '列数据湖内容（不传 --kind 时列所有 kind 概览）',
      flags: {
        kind: { type: 'string' },
        limit: { type: 'number', default: 50 },
      },
      run: (ctx) => {
        const args = ['list'];
        if (ctx.kind) args.push('--kind', ctx.kind, '--limit', String(ctx.limit));
        return service.callHostData(args);
      },
      render: (r) => {
        if (!r.kind) {
          const lines = [`数据湖: ${r.lakeDir}`, ''];
          for (const k of r.kinds || []) {
            lines.push(`  ${k.kind.padEnd(30)} ${String(k.bytes).padStart(9)} bytes`);
          }
          return lines.join('\n');
        }
        const lines = [`${r.kind}: ${r.count} 条`, ''];
        for (const rec of (r.records || []).slice(0, 20)) {
          lines.push(`  ${rec.id || '(no id)'}  ${rec.title || rec.name || rec.url || ''}`);
        }
        if (r.count > 20) lines.push(`  … 其余 ${r.count - 20} 条（--json 拿全量）`);
        return lines.join('\n');
      },
    },
    {
      id: 'tb.show',
      cli: ['tb', 'show'],
      http: ['GET', '/api/tb/show'],
      summary: '按 id 查单条记录',
      args: ['kind', 'id'],
      run: (ctx) => service.callHostData(['show', ctx.kind, ctx.id]),
      render: (r) => JSON.stringify(r.record, null, 2),
    },
    {
      id: 'tb.doctor',
      cli: ['tb', 'doctor'],
      http: ['GET', '/api/tb/doctor'],
      summary: '健康检查：exe / 数据湖 / manifest / 注册表 / 上次同步',
      run: () => service.callHostData(['doctor']),
      render: (r) => {
        const lines = [`host v${r.version} · ${r.summary}`, ''];
        for (const c of r.checks || []) {
          lines.push(`  [${c.ok ? 'OK ' : 'FAIL'}] ${c.name}`);
        }
        return lines.join('\n');
      },
    },
    {
      id: 'tb.skill',
      cli: ['tb', 'skill'],
      http: ['GET', '/api/tb/skill'],
      summary: '跑一个数据湖 skill（recent-browse / favorites-analyze）',
      args: [{ name: 'name', required: false }],
      flags: { days: { type: 'number' }, limit: { type: 'number' } },
      run: async (ctx) =>
        service.runSkill(ctx.name || 'list', {
          days: ctx.days ?? 7,
          limit: ctx.limit ?? 20,
        }),
      render: (r) => {
        if (r.skill === 'list') {
          return ['可用 skill：', ...r.skills.map((s) => '  ' + s.name + '    ' + s.summary)].join('\n');
        }
        return JSON.stringify(r, null, 2);
      },
    },
  ],
};

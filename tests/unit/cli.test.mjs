// 单元测试：验证 CLI 真实行为。
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const TB = join(ROOT, 'bin', 'tb.mjs');

function run(args) {
  return spawnSync(process.execPath, [TB, ...args], { encoding: 'utf8' });
}

test('CLI: `version` 默认输出裸版本号字符串', () => {
  const r = run(['version']);
  assert.equal(r.status, 0, `stderr: ${r.stderr}`);
  assert.match(r.stdout.trim(), /^\d+\.\d+\.\d+/, `got: ${r.stdout}`);
});

test('CLI: `--json version` 输出 JSON 编码的版本号字符串', () => {
  const r = run(['--json', 'version']);
  assert.equal(r.status, 0);
  // 拿掉尾换行,JSON.parse 它应该是字符串
  const parsed = JSON.parse(r.stdout);
  assert.equal(typeof parsed, 'string');
  assert.match(parsed, /^\d+\.\d+\.\d+/);
});

test('CLI: 无参数打印 help', () => {
  const r = run([]);
  assert.equal(r.status, 0);
  assert.match(r.stdout, /平台命令|可用命令/);
  assert.match(r.stdout, /serve/);
});

test('CLI: `--json routes` 返回 status:ok + routes 数组', () => {
  const r = run(['--json', 'routes']);
  assert.equal(r.status, 0);
  const obj = JSON.parse(r.stdout);
  assert.equal(obj.status, 'ok');
  assert.ok(Array.isArray(obj.routes), 'obj.routes 应是数组');
  assert.ok(obj.routes.length > 0, '应至少有一条命令');
  for (const e of obj.routes) {
    assert.ok(e.command || e.cli, 'entry.command/cli 必填');
    assert.ok(typeof e.summary === 'string', 'entry.summary 必填');
  }
});

test('CLI: 未知命令 exit 1', () => {
  const r = run(['nonsense-command']);
  assert.notEqual(r.status, 0);
  assert.match(r.stderr, /未知命令|用法/);
});

test('CLI: `routes` 人类模式输出至少一个真实命令', () => {
  const r = run(['routes']);
  assert.equal(r.status, 0);
  assert.match(r.stdout, /serve|skill/);
});

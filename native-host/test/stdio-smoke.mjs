// Go host stdio 协议测试:一次 spawn,连发 3 条消息,验证常驻语义。
// 二进制位置走与生产同一套 fallback 链(hostBinaryPath),
// 避免本地 go build / CI 构建到不同目录时测试找不到文件。
import { spawn } from 'node:child_process';
import { hostBinaryPath } from '../../src/modules/tb/service.js';

const EXE = hostBinaryPath();
const child = spawn(EXE, [], {
  stdio: ['pipe', 'pipe', 'inherit'],
  env: { ...process.env, TABBOARD_DIR: 'D:/tmp/gohost-test', TABBOARD_LAKE_DIR: 'D:/tmp/gohost-test/lake' },
});

function send(obj) {
  const body = Buffer.from(JSON.stringify(obj), 'utf8');
  const len = Buffer.alloc(4);
  len.writeUInt32LE(body.length, 0);
  child.stdin.write(Buffer.concat([len, body]));
}

const responses = [];
let buf = Buffer.alloc(0);
child.stdout.on('data', (d) => {
  buf = Buffer.concat([buf, d]);
  while (buf.length >= 4) {
    const len = buf.readUInt32LE(0);
    if (buf.length < 4 + len) break;
    responses.push(JSON.parse(buf.subarray(4, 4 + len).toString('utf8')));
    buf = buf.subarray(4 + len);
  }
});

send({ command: 'version' });
setTimeout(() => send({ command: 'list', kind: 'groups' }), 300);
setTimeout(() => send({ command: 'doctor' }), 600);

// doctor 要跑 PowerShell 查注册表(CI runner 上冷启动 ~1.5s),窗口给足
setTimeout(() => {
  console.log('响应数:', responses.length, '(期望 3)');
  for (const r of responses) console.log(' -', r.status, JSON.stringify(r.data ?? {}).slice(0, 70));
  console.log('常驻语义(进程存活):', child.exitCode === null ? 'YES' : 'NO');
  child.kill();
  process.exit(responses.length === 3 && child.exitCode === null ? 0 : 1);
}, 6000);

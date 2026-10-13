// Go host stdio 协议测试:一次 spawn,连发 3 条消息,验证常驻语义
import { spawn } from 'node:child_process';

const EXE = 'D:/code/a_js/nx-tb/native-host/tabboard-host.exe';
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
setTimeout(() => send({ command: 'list', kind: 'groups' }), 200);
setTimeout(() => send({ command: 'doctor' }), 400);

setTimeout(() => {
  console.log('响应数:', responses.length, '(期望 3)');
  for (const r of responses) console.log(' -', r.status, JSON.stringify(r.data ?? {}).slice(0, 70));
  console.log('常驻语义(进程存活):', child.exitCode === null ? 'YES' : 'NO');
  child.kill();
  process.exit(responses.length === 3 && child.exitCode === null ? 0 : 1);
}, 1000);

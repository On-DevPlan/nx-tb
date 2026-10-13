// JS resolver 冒烟:验证 nx-tb Node 侧能定位到二进制并调通。
// 二进制查找链的第 1 条（NX_TB_HOST_BINARY）由 CI 指向刚编译的产物;
// 本地开发时第 3 条（native-host/）命中。
import { hostBinaryPath, callHost } from '../../src/modules/tb/service.js';

const bin = hostBinaryPath();
console.log('resolved binary:', bin);

const resp = await callHost(['version']);
if (resp.status !== 'ok' || !resp.data?.version) {
  console.error('FAIL: 二进制无响应', JSON.stringify(resp));
  process.exit(1);
}
console.log('version:', resp.data.version, '· runtime:', resp.data.runtime);
console.log('JS resolver smoke OK');

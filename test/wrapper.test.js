// 包装器退出码与归档校验的单元测试。
//
// 守的两个背景 bug：
//   1. index.js 用 `process.exit(code)` 透传子进程状态。被信号杀死时 Node
//      给的 code 是 null，于是 `sourcepack` 崩了（SIGSEGV/SIGINT）包装器
//      退出 0 —— CI 把崩溃当成功，脚本里的 set -e 也拦不住。
//   2. download-binary.js 有 checksums.txt 却不校验，中间人缓存/镜像换掉
//      二进制也照装照跑。

const test = require('node:test');
const assert = require('node:assert');
const os = require('node:os');

const { exitCodeFor } = require('../index.js');
const { parseChecksums } = require('../download-binary.js');

test('exitCodeFor 正常退出直接透传', () => {
  assert.strictEqual(exitCodeFor(0, null), 0);
  assert.strictEqual(exitCodeFor(1, null), 1);
  assert.strictEqual(exitCodeFor(42, null), 42);
});

test('exitCodeFor 被信号杀死时返回 128+信号编号', () => {
  const sig = os.constants.signals;
  // 注意：code 为 null 才是“信号致死”的形状，不能退化成 0。
  assert.strictEqual(exitCodeFor(null, 'SIGSEGV'), 128 + sig.SIGSEGV); // 139
  assert.strictEqual(exitCodeFor(null, 'SIGINT'), 128 + sig.SIGINT); // 130
  assert.strictEqual(exitCodeFor(null, 'SIGKILL'), 128 + sig.SIGKILL); // 137
});

test('exitCodeFor 对未知信号 / 双 null 退到 1（不能是 0）', () => {
  assert.strictEqual(exitCodeFor(null, 'SIG_NOPE'), 1);
  assert.strictEqual(exitCodeFor(null, null), 1);
});

test('exitCodeFor 不会把 0 当成异常', () => {
  assert.strictEqual(exitCodeFor(0, null), 0);
  // 有些平台会同时给 code=0 与 signal（很少见），此时以 code 为准。
  assert.strictEqual(exitCodeFor(0, 'SIGTERM'), 0);
});

test('parseChecksums 解析 goreleaser 的 checksums.txt', () => {
  const sha = (c) => c.repeat(64);
  const text = [
    `# 注释行应被忽略`,
    ``,
    `${sha('a')}  sourcepack_2.1.0_darwin_amd64.tar.gz`,
    `${sha('b')}  sourcepack_2.1.0_linux_arm64.tar.gz`,
    // *name 的二进制模式写法
    `${sha('c')} *sourcepack_2.1.0_windows_arm64.zip`,
    `garbage line`,
  ].join('\n');

  const m = parseChecksums(text);
  assert.strictEqual(m.size, 3);
  assert.strictEqual(m.get('sourcepack_2.1.0_darwin_amd64.tar.gz'), sha('a'));
  assert.strictEqual(m.get('sourcepack_2.1.0_linux_arm64.tar.gz'), sha('b'));
  assert.strictEqual(m.get('sourcepack_2.1.0_windows_arm64.zip'), sha('c'));
});

test('parseChecksums 对大写哈希做小写归一', () => {
  const upper = 'ABCDEF'.repeat(10) + 'ABCD';
  const m = parseChecksums(`${upper}  x.tar.gz`);
  assert.strictEqual(m.get('x.tar.gz'), upper.toLowerCase());
});

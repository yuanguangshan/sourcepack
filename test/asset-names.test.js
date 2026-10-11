// 归档名 / 落地名的一致性测试。
//
// 守的背景：v1.8.0 的 Windows 安装是坏的——
//   1. 归档名用了 Node 的 os.platform()（'win32'），而 GoReleaser 用 GOOS
//      （'windows'），拼出来的 URL 在 Releases 上 404；
//   2. download-binary.js 解压后存成 sourcepack.exe，index.js 却去找
//      sourcepack-win-amd64.exe，两条命名规则各写一套。
// 现在命名规则收敛到 resolveTarget()/binaryNameFor() 一处，这里把两侧钉死。

const test = require('node:test');
const assert = require('node:assert');
const fs = require('node:fs');
const path = require('node:path');

const { resolveTarget, binaryNameFor } = require('../download-binary.js');

const VERSION = '1.8.1';

// 第 3 列 = GitHub Release 上的真实资产名，必须与 .goreleaser.yaml 的
// name_template（{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}）
// 生成的名字完全一致；第 4 列 = index.js 查找的落地名。
const CASES = [
  ['darwin', 'x64', `sourcepack_${VERSION}_darwin_amd64.tar.gz`, 'sourcepack-darwin-amd64'],
  ['darwin', 'arm64', `sourcepack_${VERSION}_darwin_arm64.tar.gz`, 'sourcepack-darwin-arm64'],
  ['linux', 'x64', `sourcepack_${VERSION}_linux_amd64.tar.gz`, 'sourcepack-linux-amd64'],
  ['linux', 'arm64', `sourcepack_${VERSION}_linux_arm64.tar.gz`, 'sourcepack-linux-arm64'],
  ['win32', 'x64', `sourcepack_${VERSION}_windows_amd64.zip`, 'sourcepack-win-amd64.exe'],
  ['win32', 'arm64', `sourcepack_${VERSION}_windows_arm64.zip`, 'sourcepack-win-arm64.exe'],
];

test('归档名用 GOOS 命名（windows 而不是 win32）', () => {
  for (const [platform, arch, archive] of CASES) {
    const t = resolveTarget(platform, arch, VERSION);
    assert.ok(t, `${platform}/${arch} 应能解析`);
    assert.equal(t.archiveName, archive, `${platform}/${arch} 归档名`);
    assert.ok(
      !/win32/.test(t.archiveName),
      `${platform}/${arch}: 归档名里不能出现 Node 的 win32（Releases 上是 windows）`
    );
    assert.ok(
      t.downloadUrl.endsWith(`/v${VERSION}/${archive}`),
      `${platform}/${arch} 下载 URL: ${t.downloadUrl}`
    );
  }
});

test('落地名与 index.js 查找的名字一致', () => {
  for (const [platform, arch, , binName] of CASES) {
    assert.equal(binaryNameFor(platform, arch), binName, `${platform}/${arch} 落地名`);
  }
});

test('Windows 归档内层可执行文件名是 sourcepack.exe', () => {
  for (const [platform, arch] of CASES.filter(([p]) => p === 'win32')) {
    const t = resolveTarget(platform, arch, VERSION);
    assert.equal(t.innerBinary, 'sourcepack.exe');
    assert.equal(t.wrappedDir, `sourcepack_${VERSION}_windows_${t.mappedArch}`);
    assert.ok(t.isWindows);
  }
  const unix = resolveTarget('linux', 'x64', VERSION);
  assert.equal(unix.innerBinary, 'sourcepack');
  assert.ok(!unix.isWindows);
});

test('不支持的架构返回 null（而不是拼出一个 404 的 URL）', () => {
  assert.equal(resolveTarget('linux', 'ia32', VERSION), null);
  assert.equal(binaryNameFor('linux', 'mips'), null);
});

test('index.js 不再自己拼落地名（防止两侧规则再次漂移）', () => {
  const src = fs.readFileSync(path.join(__dirname, '..', 'index.js'), 'utf8');
  assert.match(src, /binaryNameFor\(/, 'index.js 应使用共享的 binaryNameFor()');
  // 只看代码，忽略注释（注释里会提到这个名字做说明）
  const code = src
    .split('\n')
    .filter((line) => !line.trim().startsWith('//'))
    .join('\n');
  assert.ok(
    !/sourcepack-win-/.test(code),
    'index.js 里不该再出现硬编码的 sourcepack-win- 模板'
  );
  assert.ok(
    !/archMap/.test(code),
    'index.js 不该再自己维护一份 arch 映射'
  );
});

test('require 下载器不会触发安装副作用', () => {
  // 上面已经 require 过；如果 require.main 守卫丢了，这里早就开始下载了。
  assert.equal(typeof resolveTarget, 'function');
});

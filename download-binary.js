#!/usr/bin/env node

const https = require('https');
const http = require('http');
const fs = require('fs');
const path = require('path');
const os = require('os');
const crypto = require('crypto');
const { execFileSync } = require('child_process');

// 网络超时与重定向上限：以前两者都没有，镜像/代理卡在半路时
// `npm install` 会一直挂着（postinstall 没有超时，用户只能 Ctrl+C）。
const REQUEST_TIMEOUT_MS = 60 * 1000;
const MAX_REDIRECTS = 5;

const REPO = 'yuanguangshan/sourcepack';
const VERSION = require('./package.json').version;

const archMap = { x64: 'amd64', arm64: 'arm64' };

/**
 * 解析某个平台/架构对应的下载目标。
 *
 * 这里是**唯一**的命名真相：index.js 也用 binaryNameFor() 取落地名，
 * 避免"下载器解压出来的名字"和"入口脚本要找的名字"各写一套而漂移。
 *
 * 两个容易踩的坑：
 *  1. Node 的 os.platform() 返回 'win32'，而 GoReleaser 的归档名用的是
 *     Go 的 GOOS（'windows'）。不映射就会去下 ..._win32_amd64.zip → 404。
 *  2. 归档被 wrap_in_directory 包了一层，里面叫 sourcepack(.exe)；
 *     解压后必须重命名成 binName，且与 index.js 查找的名字完全一致。
 */
function resolveTarget(platform, arch, version) {
  const mappedArch = archMap[arch];
  if (!mappedArch) return null;

  const isWindows = platform === 'win32';
  const goos = isWindows ? 'windows' : platform; // win32 → windows
  const ext = isWindows ? '.zip' : '.tar.gz';
  const archiveName = `sourcepack_${version}_${goos}_${mappedArch}${ext}`;

  return {
    mappedArch,
    goos,
    isWindows,
    ext,
    archiveName,
    wrappedDir: `sourcepack_${version}_${goos}_${mappedArch}`,
    downloadUrl: `https://github.com/${REPO}/releases/download/v${version}/${archiveName}`,
    // 落地名：非 Windows 沿用 sourcepack-<platform>-<arch>；
    // Windows 用 sourcepack-win-<arch>.exe（平台段取 "win"，与非 Windows
    // 的简洁风格一致，index.js 按同一个函数取，不会不一致）
    binName: isWindows
      ? `sourcepack-win-${mappedArch}.exe`
      : `sourcepack-${platform}-${mappedArch}`,
    // 归档内部的可执行文件名（goreleaser 打包时的名字）
    innerBinary: isWindows ? 'sourcepack.exe' : 'sourcepack',
  };
}

/**
 * index.js 与 download-binary.js 共用的落地名；不支持的架构返回 null。
 * arch 用 Node 的值（os.arch()：'x64' / 'arm64'），不是 Go 的 GOARCH。
 */
function binaryNameFor(platform, arch) {
  const target = resolveTarget(platform, arch, '0');
  return target ? target.binName : null;
}

function download(url, redirectsLeft = MAX_REDIRECTS) {
  return new Promise((resolve, reject) => {
    const mod = url.startsWith('https') ? https : http;
    const req = mod.get(url, { headers: { 'User-Agent': 'node' }, timeout: REQUEST_TIMEOUT_MS }, (res) => {
      if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
        res.resume(); // 丢弃重定向的响应体，否则 socket 不复用
        if (redirectsLeft <= 0) {
          return reject(new Error(`Too many redirects for ${url}`));
        }
        // Location 可能是相对路径
        const next = new URL(res.headers.location, url).toString();
        return resolve(download(next, redirectsLeft - 1));
      }
      if (res.statusCode !== 200) {
        res.resume();
        return reject(new Error(`Download failed: HTTP ${res.statusCode} for ${url}`));
      }
      resolve(res);
    });
    req.on('timeout', () => {
      req.destroy(new Error(`Download timed out after ${REQUEST_TIMEOUT_MS / 1000}s: ${url}`));
    });
    req.on('error', reject);
  });
}

// downloadToFile 边下边写盘，不再把整个归档 concat 进内存（之前峰值 = 归档大小 ×2）。
async function downloadToFile(url, destPath) {
  const res = await download(url);
  await new Promise((resolve, reject) => {
    const out = fs.createWriteStream(destPath);
    res.on('error', reject);
    out.on('error', reject);
    out.on('finish', resolve);
    res.pipe(out);
  });
}

// parseChecksums 解析 goreleaser 的 checksums.txt：每行 "<sha256>  <文件名>"。
// 只保留 64 位十六进制的行，注释/空行忽略。
function parseChecksums(text) {
  const out = new Map();
  for (const line of text.split(/\r?\n/)) {
    const m = line.trim().match(/^([0-9a-fA-F]{64})\s+\*?(.+)$/);
    if (m) out.set(path.basename(m[2].trim()), m[1].toLowerCase());
  }
  return out;
}

function sha256File(filePath) {
  return new Promise((resolve, reject) => {
    const hash = crypto.createHash('sha256');
    fs.createReadStream(filePath)
      .on('error', reject)
      .on('data', (chunk) => hash.update(chunk))
      .on('end', () => resolve(hash.digest('hex')));
  });
}

// verifyChecksum 用同 release 的 checksums.txt 校验归档。
//
// 设计取舍：
//   - 校验不通过 → **硬失败**（说明内容与发布方声明不一致，绝不能解压执行）；
//   - 拿不到 checksums.txt（v2.1.0 以前的 release 没有，或网络受限）→ 警告后继续，
//     否则会把老版本的安装路径弄挂；
//   - checksums.txt 里没有这个归档 → 同上，警告后继续。
// 已有能用的 SHA-256 就不要再靠“从 GitHub 下的一定是好的”：镜像、
// 企业代理、中间人缓存都可能换掉二进制。
async function verifyChecksum(archivePath, archiveName, version) {
  const url = `https://github.com/${REPO}/releases/download/v${version}/checksums.txt`;
  let text;
  try {
    const res = await download(url);
    const chunks = [];
    for await (const chunk of res) chunks.push(chunk);
    text = Buffer.concat(chunks).toString('utf8');
  } catch (err) {
    console.warn(`⚠ checksums.txt not available (${err.message}); skipping integrity check`);
    return;
  }

  const expected = parseChecksums(text).get(archiveName);
  if (!expected) {
    console.warn(`⚠ checksums.txt has no entry for ${archiveName}; skipping integrity check`);
    return;
  }

  const actual = await sha256File(archivePath);
  if (actual !== expected) {
    throw new Error(
      `checksum mismatch for ${archiveName}\n  expected: ${expected}\n  actual:   ${actual}`
    );
  }
  console.log(`✓ checksum verified (sha256 ${actual.slice(0, 12)}...)`);
}

// psQuote 把值包成 PowerShell 单引号字符串（单引号内单引号写两遍）。
// 以前是直接把路径插进 -Command 字符串，路径里带 ' 就断了，
// 更糟的是可以被拼成任意命令。
function psQuote(s) {
  return `'${String(s).replace(/'/g, "''")}'`;
}

function extractWindows(target, binDir, archivePath) {
  // Node 没有内置 zip 解压，用 Windows 自带的 PowerShell
  execFileSync(
    'powershell',
    [
      '-NoProfile',
      '-Command',
      `Expand-Archive -LiteralPath ${psQuote(archivePath)} -DestinationPath ${psQuote(binDir)} -Force`
    ],
    { stdio: 'inherit' }
  );
  const wrapped = path.join(binDir, target.wrappedDir, target.innerBinary);
  const flat = path.join(binDir, target.innerBinary);
  const src = fs.existsSync(wrapped) ? wrapped : flat;
  if (fs.existsSync(src)) fs.renameSync(src, path.join(binDir, target.binName));
  fs.unlinkSync(archivePath);
  try {
    fs.rmSync(path.join(binDir, target.wrappedDir), { recursive: true, force: true });
  } catch {}
}

function extractUnix(target, binDir, archivePath) {
  // execFile 而不是 execSync：参数不过 shell，路径里的空格/引号/$
  // 都不会被当成语法，也不再需要手写转义。
  execFileSync('tar', ['xzf', archivePath, '-C', binDir, '--strip-components=1'], {
    stdio: 'inherit'
  });
  const extracted = path.join(binDir, target.innerBinary);
  if (fs.existsSync(extracted)) fs.renameSync(extracted, path.join(binDir, target.binName));
  fs.unlinkSync(archivePath);
}

async function main() {
  const platform = os.platform();
  const target = resolveTarget(platform, os.arch(), VERSION);
  if (!target) {
    console.error(`Unsupported architecture: ${os.arch()}`);
    process.exit(1);
  }

  console.log(`Downloading sourcepack v${VERSION} for ${platform}-${target.mappedArch}...`);

  const binDir = __dirname;
  const archivePath = path.join(binDir, target.archiveName);
  try {
    await downloadToFile(target.downloadUrl, archivePath);
    await verifyChecksum(archivePath, target.archiveName, VERSION);
    if (target.isWindows) {
      extractWindows(target, binDir, archivePath);
    } else {
      extractUnix(target, binDir, archivePath);
    }
  } catch (err) {
    // 失败时别把半个归档留在包里
    try {
      fs.unlinkSync(archivePath);
    } catch {}
    throw err;
  }

  const finalPath = path.join(binDir, target.binName);
  if (fs.existsSync(finalPath)) {
    fs.chmodSync(finalPath, 0o755);
    console.log(`✓ sourcepack v${VERSION} installed successfully.`);
  } else {
    console.error(`Error: expected binary not found at ${finalPath}`);
    process.exit(1);
  }
}

// 被 require（index.js / 单测）时不要执行安装
if (require.main === module) {
  main().catch((err) => {
    console.error(`Failed to download sourcepack: ${err.message}`);
    console.error(
      `You can manually download from: https://github.com/${REPO}/releases/tag/v${VERSION}`
    );
    process.exit(1);
  });
}

module.exports = { resolveTarget, binaryNameFor, parseChecksums, archMap, REPO };

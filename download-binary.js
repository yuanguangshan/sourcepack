#!/usr/bin/env node

const https = require('https');
const http = require('http');
const fs = require('fs');
const path = require('path');
const os = require('os');

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

function download(url) {
  return new Promise((resolve, reject) => {
    const mod = url.startsWith('https') ? https : http;
    mod
      .get(url, { headers: { 'User-Agent': 'node' } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return resolve(download(res.headers.location));
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`Download failed: HTTP ${res.statusCode} for ${url}`));
        }
        resolve(res);
      })
      .on('error', reject);
  });
}

function extractWindows(buf, target, binDir) {
  const { execSync } = require('child_process');
  const tmpFile = path.join(binDir, target.archiveName);
  fs.writeFileSync(tmpFile, buf);
  // Node 没有内置 zip 解压，用 Windows 自带的 PowerShell
  execSync(
    `powershell -Command "Expand-Archive -Path '${tmpFile}' -DestinationPath '${binDir}' -Force"`,
    { stdio: 'inherit' }
  );
  const wrapped = path.join(binDir, target.wrappedDir, target.innerBinary);
  const flat = path.join(binDir, target.innerBinary);
  const src = fs.existsSync(wrapped) ? wrapped : flat;
  if (fs.existsSync(src)) fs.renameSync(src, path.join(binDir, target.binName));
  fs.unlinkSync(tmpFile);
  try {
    fs.rmSync(path.join(binDir, target.wrappedDir), { recursive: true, force: true });
  } catch {}
}

function extractUnix(buf, target, binDir) {
  const { execSync } = require('child_process');
  const tmpFile = path.join(binDir, target.archiveName);
  fs.writeFileSync(tmpFile, buf);
  execSync(`tar xzf "${tmpFile}" -C "${binDir}" --strip-components=1`, { stdio: 'inherit' });
  const extracted = path.join(binDir, target.innerBinary);
  if (fs.existsSync(extracted)) fs.renameSync(extracted, path.join(binDir, target.binName));
  fs.unlinkSync(tmpFile);
}

async function main() {
  const platform = os.platform();
  const target = resolveTarget(platform, os.arch(), VERSION);
  if (!target) {
    console.error(`Unsupported architecture: ${os.arch()}`);
    process.exit(1);
  }

  console.log(`Downloading sourcepack v${VERSION} for ${platform}-${target.mappedArch}...`);

  const res = await download(target.downloadUrl);
  const chunks = [];
  for await (const chunk of res) chunks.push(chunk);
  const buf = Buffer.concat(chunks);

  const binDir = __dirname;
  if (target.isWindows) {
    extractWindows(buf, target, binDir);
  } else {
    extractUnix(buf, target, binDir);
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

module.exports = { resolveTarget, binaryNameFor, archMap, REPO };

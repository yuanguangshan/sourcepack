#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');
const { binaryNameFor } = require('./download-binary.js');

// exitCodeFor 把子进程的 (code, signal) 换算成本包装器自己的退出码。
//
// 被信号杀死时 Node 给的是 code=null + signal='SIGSEGV' 之类，直接
// `process.exit(code)` 会退化成 0 —— CI 里二进制崩溃被当成成功。
// 按 shell 惯例用 128+信号编号（SIGSEGV=11 → 139、SIGINT=2 → 130）。
function exitCodeFor(code, signal) {
  if (typeof code === 'number') return code;
  const signals = (os.constants && os.constants.signals) || {};
  const num = signals[signal];
  return typeof num === 'number' ? 128 + num : 1;
}

function run() {
  const platform = os.platform();
  const arch = os.arch();

  // 落地名与下载器共用同一个函数：以前两边各写一套模板，
  // Windows 下 download-binary.js 存成 sourcepack.exe、这里却找
  // sourcepack-win-amd64.exe，装完必然 "Binary not found"。
  const binaryName = binaryNameFor(platform, arch);

  if (!binaryName) {
    console.error(`Unsupported architecture: ${arch}`);
    process.exit(1);
  }

  const binaryPath = path.join(__dirname, binaryName);

  if (!fs.existsSync(binaryPath)) {
    console.error(`Error: Binary not found at ${binaryPath}`);
    console.error(`Platform: ${platform}, Arch: ${arch}`);
    console.error(`Try running: npm rebuild sourcepack`);
    process.exit(1);
  }

  const child = spawn(binaryPath, process.argv.slice(2), {
    stdio: 'inherit'
  });

  // spawn 的失败是异步事件（EACCES / ENOENT / ETXTBSY）。没有监听器时
  // Node 会把整个栈打印出来并抛 ERR_UNHANDLED_ERROR；用户看到的是
  // "node:events:497 throw er" 而不是一句能照着做的提示。
  child.on('error', (err) => {
    console.error(`Error: failed to run ${binaryPath}: ${err.message}`);
    if (err.code === 'EACCES') {
      console.error(`Try running: chmod +x "${binaryPath}"（或 npm rebuild sourcepack 重新安装）`);
    } else if (err.code === 'ENOENT') {
      console.error('Try running: npm rebuild sourcepack');
    }
    process.exit(1);
  });

  child.on('exit', (code, signal) => {
    process.exit(exitCodeFor(code, signal));
  });
}

// 被 require（单元测试）时只导出，不真的跑二进制。
if (require.main === module) {
  run();
}

module.exports = { exitCodeFor };

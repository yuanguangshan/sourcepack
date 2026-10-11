#!/usr/bin/env node

const { spawn } = require('child_process');
const path = require('path');
const os = require('os');
const fs = require('fs');
const { binaryNameFor } = require('./download-binary.js');

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

child.on('exit', (code) => {
  process.exit(code);
});

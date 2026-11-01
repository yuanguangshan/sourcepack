#!/usr/bin/env node

// gdx = sourcepack -s（统计模式）。
//
// npm 的 bin 只能映射到一个可执行文件，不能带预设参数；所以 gdx 单独
// 一个入口，把 -s 插到参数最前面。这样 `gdx`、`gdx -p`、`gdx --version`
// 的行为都与 README/PARAMETERS 里写的一致。
//
// 注意：不要把它改成“在 index.js 里根据 argv[1] 判断”。Windows 的 npm
// bin 是 .cmd shim，argv[1] 会变成 index.js，判断不可靠；独立入口跨平台稳定。

const { run } = require('./index.js');

function withStatsArgs(args) {
  return ['-s', ...args];
}

if (require.main === module) {
  process.argv.splice(2, 0, '-s');
  run();
}

module.exports = { withStatsArgs };

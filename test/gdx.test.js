// gdx = sourcepack -s 的入口测试。
//
// 守的背景：package.json 曾把 gdx 直接映射到 index.js，于是 `gdx` 实际
// 做的是全量快照，而 README/PARAMETERS 写的是 `sourcepack -s`（统计）。
// 现在 gdx.js 独立入口，固定插入 -s；这里把参数顺序和 package 映射钉死。

const test = require('node:test');
const assert = require('node:assert');

const { withStatsArgs } = require('../gdx.js');
const pkg = require('../package.json');

test('gdx 固定插入 -s，且保留用户参数', () => {
  assert.deepEqual(withStatsArgs([]), ['-s']);
  assert.deepEqual(withStatsArgs(['-p']), ['-s', '-p']);
  assert.deepEqual(withStatsArgs(['--max-size', '100']), ['-s', '--max-size', '100']);
});

test('package.json 把 gdx 映射到独立入口而不是 index.js', () => {
  assert.equal(pkg.bin.gdx, 'gdx.js');
  assert.ok(pkg.files.includes('gdx.js'), 'npm 包必须包含 gdx.js');
});

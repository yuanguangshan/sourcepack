# TODO / 未来演进

按"触发条件"排序，不是按重要性。每条都记了**为什么现在不做**，避免以后重复论证。

---

## 1. npm 生命周期脚本收紧 → 改用 `optionalDependencies` 拆包

**触发条件**：npm / pnpm / bun 正式默认禁止 `postinstall`（目前只是警告），
或出现"装了包但没有二进制"的用户反馈。

**现状**：`download-binary.js` 通过 `postinstall` 从 GitHub Releases 下载对应平台
二进制。npm 11（Node 26）已开始警告：

```
npm warn install-scripts  sourcepack@1.8.1 (postinstall: node download-binary.js)
npm warn install-scripts  Run `npm install -g --allow-scripts=sourcepack` to allow...
```

**方案：跨平台可选依赖拆包**（esbuild / turbo / @biomejs/biome / swc 的做法）

```
sourcepack                      # 主包：只有 index.js，无 postinstall
 └── optionalDependencies:
      ├── @sourcepack/darwin-arm64
      ├── @sourcepack/darwin-amd64
      ├── @sourcepack/linux-amd64
      ├── @sourcepack/linux-arm64
      ├── @sourcepack/windows-amd64
      └── @sourcepack/windows-arm64
```

- npm 依据子包的 `os` / `cpu` 字段**只下载本机那一个**（体积不会 ×6）；
- 零脚本、零额外网络请求、免疫 `--ignore-scripts`，内网/离线镜像也能用。

**迁移时的注意点（已踩过/已确认的）**

1. **绝不要改成"在 `index.js` 里缺文件就即时下载"**。理由：
   - `postinstall` 可能以 root 跑（`sudo npm i -g` → Linux/Docker 下
     `/usr/local/lib/node_modules` 属主是 root），而用户敲 `gx` 时是普通用户，
     往 `__dirname` 写 zip/解压/mv 必然 `EACCES: permission denied`——
     报错比现在的 `Binary not found` 更难懂；
   - 真要运行时下载，落盘必须改到用户缓存目录（`~/.cache/sourcepack/bin`、
     `%LOCALAPPDATA%`），并带超时熔断、进度展示、Ctrl+C 清理；
   - `index.js` 现在只有 37 行、纯同步、原样透传子进程退出码，
     塞进网络逻辑会让这条入口路径彻底失去可测性。
2. 发布流程要改：每个平台子包单独 `npm publish`（可做成矩阵 job），
   主包的 `optionalDependencies` 用精确版本号锁死。
3. `index.js` 的解析顺序：优先 `require.resolve('@sourcepack/<平台>/package.json')`
   定位二进制；都找不到时给出与现在同等清晰的指引。
4. 兼容性：`--omit=optional` / `--no-optional` 下会缺包，必须有明确报错；
   npm < 7 会尝试安装全部可选包（这些版本已 EOL，可忽略）。
5. 迁移后 `download-binary.js` 可保留为 `npm rebuild` 的手动兜底，也可直接删掉。

---

## 2. 扫描并发（worker pool）

**触发条件**：出现"1 万个以上文件"的仓库，且用户反馈慢。

**实测数据**（min-of-3、热缓存、Go 1.24 / Apple Silicon）：

| 语料 | 扫描+统计 | 全量（含渲染写盘） |
|---|---:|---:|
| 500 文件 / 10 万行（典型） | 20 ms | 40 ms |
| 5000 文件 / 10 万行 | 100 ms | 210 ms |
| **20000 文件 / 10 万行** | **2020 ms** | **3850 ms** |
| 5000 文件 / 100 万行 | 130 ms | 190 ms |

**耗时由文件数决定，不由行数决定**（100 万行的语料比 10 万行但有 2 万文件的语料快 20 倍）。
每文件约 0.1ms 固定开销；20000 文件的场景 `real 2.91s / CPU 1.48s`，
一半时间在等 I/O，所以并行的收益是真实的（估计 3–5×）。

**为什么现在不做**：10 万行级别、正常文件粒度下扫描只有 20–100ms，
收益低于确定性成本（输出必须保持路径有序、`Stats` 聚合要并发安全，
且项目有一条"改完产物与旧版逐字节一致"的硬约束）。

**如果要做**：遍历保持单线程（`SkipDir`/权限错误决策在遍历侧），
只把 `scanFileStats` 下发 worker，结果按路径排序后再聚合，可保持产物逐字节一致。

**顺带**：`-g` 模式下更值得优化的是**合并/并行 git 调用**——`-g` 一次要
spawn 约 8 个 git 进程（每个约 11ms），光进程开销就 ~90ms，比目录扫描还贵。

---

## 3. 多层 `.gitignore` / `.gdignore`

**触发条件**：monorepo 用户反馈"子目录里的 `.gitignore` 没生效"。

**现状**：只在根目录读一次 `.gitignore` / `.gdignore`
（`loadGitignore(config.RootDir)`）。

**为什么不是简单修补**：子目录 `.gitignore` 的语义要求 pattern **只在其所在
目录下生效**，而当前 matcher 是"扁平 + 最后匹配优先"，无法表达目录作用域。
要做对得改成"沿路径继承的 matcher 链"（遍历时按目录叠加），约 80 行 + 测试。

**注意**：git 模式（`-g`/`-G`）**故意不重复套用 `.gitignore`**——git 自己已经
过滤过未跟踪文件，而"已跟踪但恰好命中 `.gitignore`"的文件是真实变更，
不该因为一条 ignore 规则就藏起来。这个决定见 README 的 Git 变更模式章节。

---

## 4. `runLimited` 的峰值内存

**触发条件**：病态大 diff（单次 diff 输出接近 64MB 上限）导致的 OOM 反馈。

**现状**：`maxPatchBytes = 64MB` 的 patch 会整体读进内存，
`splitPatch`/`capDiff` 再复制几份，峰值约为上限的 3–4 倍（实测 ~190MB）。
截断发生在应用层而非获取层。

**如果要做**：在 `runLimited` 层按字节丢弃（而不是读完再截），
或在读取时就把"最后一个完整文件块"之后的字节丢掉。
代价是"整文件从变更清单里消失"的风险上升，需要谨慎设计。

# Sourcepack (gdoc)

**一键将项目代码库转换为 AI 可读的 Markdown 快照。**

Sourcepack 是一个极简、高性能的工具，用于扫描项目目录并将代码合并为单个 Markdown 文件，方便你快速喂给 LLM (GPT/Claude) 或进行代码审计。

## 安装

```bash
npm install -g sourcepack
# 或
bash install.sh
```

## 使用

三个命令名**完全等效**，指向同一个二进制：

```bash
gx            # 本机常用（另有别名 gxi/gxs/gxp/gdx，见下）
sourcepack    # 主命令（与包名一致）
gdoc          # 快捷命令
```

> ⚠️ `gx` / `gdoc` 是 Homebrew 全局安装时创建的软链，都指向
> `/opt/homebrew/lib/node_modules/sourcepack/index.js`。
> **在非交互 shell（脚本、CI、agent 的 bash 工具）里 `gx` 可能不在 PATH 上**，
> 因为 `/opt/homebrew/bin` 通常只由 `~/.zshrc` 注入。脚本里请写全路径或先导出：
> ```bash
> export PATH="/opt/homebrew/bin:$PATH"
> ```

### 全部参数（含义均经实测核对）

**筛选：包含**

| 参数 | 含义 | 实测 |
|---|---|---|
| `-i, --include` | 只保留这些扩展名（逗号分隔） | `-i go` → 仅 `.go` |
| `-m, --match` | 只保留路径含该关键字的文件 | `-m src` → 仅 `src/` 下 |
| `-X, --exclude-match` | 排除路径含该关键字的文件 | `-X src` → 排除 `src/` |
| `-x, --exclude` | 排除这些扩展名 | `-x exe` → 排除 `.exe` |

> `-i` 与 `-x` 可组合，`-x` 后生效：`-i go,md -x md` → 仅 `.go`。
> 注意 `-m`/`-X` 匹配的是**路径子串**，不是文件名全等。

**筛选：忽略规则**

| 参数 | 含义 | 实测 |
|---|---|---|
| `-I, --ignore` | 追加 gitignore 风格忽略模式（逗号分隔） | `-I '*.md'` → 排除 md |
| `--no-gitignore` | 不读 `.gitignore` | 原本被忽略的文件会进来 |
| `--no-default-ignore` | 关闭内置忽略（`node_modules`、`.git` 等） | `node_modules/` 会进来 |
| `-a, --all` | 关闭 `.gitignore` 与内置忽略（**不含 `.gdignore`**） | 见下方说明 |

> `-I` 对**文件和目录都生效**（v1.7.4 前只对目录生效，是 bug）。
> `-I` 的模式若不含 `/`，按 basename 匹配；含 `/` 则匹配整条相对路径。

**范围与体积**

| 参数 | 含义 | 实测 |
|---|---|---|
| `-d, --dir` | 要扫描的根目录（默认 `.`） | 可从任意位置指定目标目录 |
| `-n, --no-subdirs` | 只扫根目录，不进子目录 | 子目录文件被排除 |
| `--max-size` | 单文件大小上限，**单位 KB**（默认 500） | `--max-size 1` → 3KB 文件被跳过 |

**输出**

| 参数 | 含义 | 实测 |
|---|---|---|
| `-o, --out` | 输出文件名（默认 `project_snapshot.md`） | |
| `--icloud` | 存到 iCloud Drive Documents | **优先于 `-o`** |
| `-c, --copy` | 复制到剪贴板，**不写文件** | 目录内无产物 |
| `-s, --stats` | 只打印统计，**不生成文件** | 目录内无产物 |
| `--dry-run` | 预览（实际会打印统计摘要，非纯文件清单） | |
| `-v, --verbose` | 打印配置与扫描细节 | 输出 Root/Output 等 |

**远端推送**

| 参数 | 含义 |
|---|---|
| `-p, --push` | 推送到远端中继；需 `--push-url` 或环境变量 `SOURCEPACK_PUSH_URL` |
| `-u, --push-url` | 覆盖环境变量里的推送地址 |
| `--auth-key` | `X-Auth-Key` 认证（或用 `SOURCEPACK_AUTH_KEY`） |

> `-p` 未配 URL 时会明确报错 `SOURCEPACK_PUSH_URL env is required for push`，
> 不会静默失败。

**其它**

| 参数 | 含义 |
|---|---|
| `--version` | 打印版本号 |
| `-h, --help` | 帮助 |

**Git 变更打包（v1.8.0 新增）**

把"本次改了什么"打包给 AI 审查，而不是整个项目。

| 参数 | 含义 | 实测 |
|---|---|---|
| `-g, --git-diff` | 打包未提交的改动：工作区 + 索引 vs `--rev` | 等价 `git diff HEAD` 的变更集 |
| `--rev` | `-g` 的比较基准，任意 git revspec（默认 `HEAD`） | `-g --rev HEAD~3` |
| `--staged` | 只看暂存区（`git diff --cached`） | 与 `git diff --cached HEAD` 一致 |
| `-G, --commits` | 打包最近 N 个提交（`git log -N`） | `-G 5` |
| `--no-merges` | `-G` 时排除合并提交 | `-G 5 --no-merges` |
| `--diff-only` | 只输出 diff，不输出变更文件全文 | 输出体积显著变小 |
| `--max-diff-lines` | 单文件 diff 行数上限（默认 2000，**0 = 不限**） | 超限处标注省略行数 |

> 短参数为什么不是 `-n`：`-n` 已经是 `--no-subdirs`（v1.7.3 刚修好），
> 所以提交数用大写 `-G`，与 `-x`/`-X` 的大小写区分习惯一致。

Git 模式下 `-i -x -m -X -I --max-size -o -c -p -s --dry-run --icloud -a`
**全部照常生效**：git 只负责产出变更清单，后面的过滤、忽略、敏感文件
gate、体积控制、渲染、剪贴板/推送都走同一条流水线。

### 参数优先级（实测确认）

| 同时给出 | 结果 |
|---|---|
| `-c` + `-p` | **`-c` 优先**，只复制到剪贴板，不推送 |
| `--icloud` + `-o` | **`--icloud` 优先**，`-o` 指定的文件不会生成 |
| `-i` + `-x` | 先按 `-i` 收窄，再按 `-x` 排除 |
| `-g` + `-G` | **明确报错**（两种模式互斥） |
| `--staged` / `--rev` 不给 `-g` | **明确报错**（不静默忽略） |
| `-n` + `-g`/`-G` | **明确报错**（git 模式不走目录遍历） |

### 常用示例

```bash
gx -i go,md         # 只包含 Go 和 Markdown 文件
gx -x exe,bin       # 排除特定后缀
gx -X vendor        # 排除路径中含该关键字的文件
gx -n               # 只扫根目录，不进子目录
gx -I '*.log'       # 追加忽略模式（文件/目录都生效）
gx -s               # 显示详细统计，不生成文件
gx -c               # 复制到剪贴板，不写文件
gx -p               # 推送到远端中继（需 SOURCEPACK_PUSH_URL / SOURCEPACK_AUTH_KEY）
gx -s -p            # 只推送统计数据到远端
gx -v               # 详细模式
gx --max-size 200   # 单文件大小上限（KB，默认 500）
gx --dry-run        # 预览（实际会打印统计摘要，不是纯文件清单）
```

### Git 变更模式常用示例

```bash
gx -g                     # 打包所有未提交改动（默认产物 git_changes.md）
gx -g --staged            # 只看暂存区
gx -g --rev HEAD~3        # 与 3 个提交之前比较
gx -g --diff-only         # 只要 diff，不要变更文件全文
gx -g -i go               # 只看 .go 的改动
gx -g -m src/main         # 只看路径含 src/main 的改动
gx -g -d services/api     # monorepo 里只看某个子目录
gx -g -c                  # 直接把变更快照丢进剪贴板
gx -g --max-diff-lines 500  # 单文件 diff 最多 500 行
gx -G 5                   # 最近 5 个提交
gx -G 10 --no-merges      # 最近 10 个非合并提交
gx -G 1 --diff-only -c    # 只把最后一个提交的 diff 复制到剪贴板
```

### 本机别名（`~/.zshrc`）

| 别名 | 等价于 | 用途 |
|---|---|---|
| `gxi` | `gx --icloud` | **存到 iCloud Drive**（最常用） |
| `gxs` | `gx -s` | 看统计 |
| `gxp` | `gx -p` | 推到远端 |
| `gdx` | `sourcepack -s` | 同 `gxs` |

### 输出位置

默认写当前目录的 `project_snapshot.md`（`-o` 可改名）。

**`--icloud`（即 `gxi`）** 会把快照存到 iCloud Drive 的 Documents 下，文件名自动带日期：

```
~/Library/Mobile Documents/com~apple~CloudDocs/Documents/<目录名>+<YYYY-MM-DD>.md
```

例：在 `~/ygs/pushd` 里跑 `gxi` →

```
…/Documents/pushd+2026-09-15.md
```

同一天重复跑会**覆盖**同名文件（不追加、不加序号）。想留多份需自己改名。

`--icloud` 与 `-o` 同时给时，`--icloud` 优先。

## 生成的快照包含

- **项目结构树** — 目录优先、字母排序的树形图，快速把握项目全貌
- **文件目录 (TOC)** — 带锚点跳转的文件索引
- **完整源码** — 自动语法高亮、智能处理嵌套代码块
- **项目统计** — `-s` 输出多维度统计（文件、目录、语言、Token 分布）

## Git 变更快照包含（`-g` / `-G`）

默认产物名是 `git_changes.md`（全量快照是 `project_snapshot.md`，两者互不覆盖）：

1. **仓库信息** — 仓库根、分支、HEAD（含提交标题）、工作区是否 dirty、变更统计
2. **变更摘要表** — 每个文件的状态（added/modified/deleted/renamed/untracked）、
   `+N −M` 行数、标记（`staged` / `unstaged` / `untracked` / `binary` / `diff truncated`）
3. **提交列表** — 仅 `-G`：SHA、日期、作者、标题，合并提交有标注
4. **被规则排除的清单** — 每个被过滤的路径 + 原因（`Sensitive file`、
   `Default ignore dir: node_modules`、`Not in -i/--include` …）
5. **删除 / 重命名 / 二进制文件清单**
6. **逐文件明细** — `diff` 代码块 + **变更文件的完整内容**（`--diff-only` 可关掉）
   - `-G` 模式按提交分组，每个提交一节
   - 全文只在同一路径**最后出现的那次**输出，并标注为
     `Full content (current working tree)`（历史模式下它是当前工作区的内容）

### 与 git 行为对齐的几个关键点（实测）

| 情况 | 处理 |
|---|---|
| 一个文件既暂存又未暂存（`git add -p`） | 输出 `git diff HEAD` 作为真值，标记 `staged, unstaged` |
| 未跟踪文件 | 走 `git ls-files --others --exclude-standard`，不受 `git status` 文本格式影响 |
| 已跟踪但命中 `.gitignore` 的文件 | **仍会输出**（git 模式不重复套用 `.gitignore`） |
| 未跟踪且被 `.gitignore` 忽略的文件 | git 自己就排除了，不进变更集 |
| 重命名 | `-M` 检测，显示 `old → new`；`--name-status -z` 解析，不怕空格/换行路径 |
| 合并提交（`-G` 默认包含） | 相对**第一父**求 diff，并标注 merge |
| `-G 5 --no-merges` | 取 5 个非合并提交 |
| 二进制文件 | **一律按内容判定**（不看扩展名）：含 NUL 或不是合法 UTF-8 → 只列清单，不出正文；git 原样的 `Binary files ... differ` 保留 |
| 删除的文件 | 保留 diff；正文标注 `Full content omitted: Not in working tree` |
| 中文 / 空格文件名 | `-c core.quotepath=false`，原样 UTF-8；diff 头里的尾 TAB 会被正确剥离 |
| 大 diff（行多） | 单文件默认截断到 `--max-diff-lines 2000` 行并标注 |
| 大 diff（单行超长） | 再套一层**字节上限**（复用 `--max-size`，默认 500KB）——按行截断对"一行 20MB"完全无效 |
| 变更总行数 | 变更总行数（diff + 新文件正文）超 2 万行时终端与文档都会警告 |
| 未跟踪文件的行数 | git diff 里没有它，所以 `+N` 与总量告警按文件正文行数计入 |
| 超过 64 MB 的 diff | 在最后一个完整文件边界处截断并标注 |
| `--diff-only` | 与默认模式走**同一道**二进制/体积 gate：二进制与超 `--max-size` 的文件不会出现在合成 diff 里 |

> **产物永远是合法 UTF-8**：这是硬约束。除了上面两道 gate，git 给出的
> diff 正文还会再过一次内容检查——"前 8000 字节干净、后面才有 NUL"、
> 或"无 NUL 但非法 UTF-8"的文件，git 会当文本输出逐行 diff，正文里就是
> 裸字节，这种情况会只保留状态与行数、丢弃正文并标记为 binary。
> 扩展名不作为"内容是文本"的依据：含 NUL 的 `.md` 同样会被判为二进制。

> **安全**：`.env`、`id_rsa`、`.npmrc`、`.git-credentials` 等敏感文件沿用全量
> 快照的 gate，**默认排除**并列入"被规则排除的清单"。注意 `git add -f` 过的
> `.env` 会出现在 diff 里，所以这个 gate 是必需的，而不是可选项（`-a` 才会放行）。

> **`-d` 子目录**：在 monorepo 子目录里执行 `gx -g -d services/api` 时，
> 变更范围限制在该子目录（用 `git rev-parse --show-prefix` 判定，软链路径也不会错），
> 输出中的路径仍以仓库根为基准。

## 特性

- **极速**: Go 编写，秒级处理万行代码。
- **智能**: 自动处理 `.gitignore` 和 `.gdignore`，支持 `!` 否定、`**` 通配等完整 gitignore 语法；按**内容**自动跳过二进制/非 UTF-8 文件，保证产物永远是合法 UTF-8。
- **变更模式**: `-g` 打包未提交改动、`-G N` 打包最近 N 个提交，直接产出一份适合 AI 审查的变更快照（diff + 变更文件全文）。
- **统计**: `-s` 多维度代码统计（Token 预估、目录占比、语言分布）。
- **便捷**: `-c` 一键复制到剪贴板，`-p` 一键推送到远端。
- **清晰**: 自动生成带跳转链接的项目目录树。

## 远端推送

通过 `-p` 将快照推送到远端中继服务（如 knasync）：

```bash
export SOURCEPACK_PUSH_URL="https://your-endpoint/api/submit"
export SOURCEPACK_AUTH_KEY="your-secret"
sourcepack -p           # 推送完整文档
sourcepack -s -p        # 只推送统计数据
```

也可通过命令行参数 `--auth-key` 直接传入密钥。

---
*Simple, Fast, and AI-Friendly.*
## `.gdignore` — 专属忽略文件

在项目根目录放一个 `.gdignore` 文件，语法和 `.gitignore` 完全一致，但**只影响 sourcepack 的扫描**，不影响 Git。适合排除那些不想喂给 LLM 但又不想动 `.gitignore` 的文件（大 JSON、编译产物、测试夹具等）。

示例 `.gdignore`：

```gitignore
*.json
fixtures/
!important_schema.json
```

**`.gdignore` 与 `-I` 的区别**：

| | `.gdignore` | `-I '...'` |
|---|---|---|
| 持久性 | 存进仓库，团队共享 | 仅本次命令行 |
| 生效范围 | 该目录及其子目录 | 本次扫描 |
| 适合 | 项目级长期排除 | 一次性、临时过滤 |

两者语法一致，且都与 `.gitignore` 叠加生效。

> ⚠️ **`.gdignore` 无法被关闭**：即使加 `-a`（以及显式
> `--no-gitignore --no-default-ignore`），`.gdignore` 仍然生效。
> `-a` 只关闭 `.gitignore` 和内置忽略规则，不含 `.gdignore`。
> 注意 `--help` 里 `-a` 的说明写的是 "disable all ignore rules"，
> 与实现不符（实测确认）。


---

## 修复记录（2026-09-15）

本日修复了两个**长期静默失效**的过滤参数。它们的共同点是：
`--help` 和 README 都写着能用，实际却不起作用，且没有任何测试覆盖。

### `-n / --no-subdirs`：完全失效（v1.7.3 修复）

**症状**：加 `-n` 后子目录文件仍被扫入。

**根因**（`godoc.go`）：

```go
// 错误实现
if config.NoSubdirs && strings.Contains(relPath, "/") {
    return filepath.SkipDir
}
```

`filepath.WalkDir` 走到**一级子目录** `src` 时，`relPath` 就是 `"src"`
（不含 `/`），条件永不成立；等走到二级 `src/deep` 才含 `/`，
但那时父目录已遍历完，`SkipDir` 已无意义。

**修复**：走到目录分支说明 `relPath != "."`（根目录已在前面 return），
所以"任何目录"即"任何子目录"：

```go
if config.NoSubdirs {
    return filepath.SkipDir
}
```

### `-I / --ignore`：对文件不生效（v1.7.4 修复）

**症状**：`-I 'top.md'`、`-I '*.md'` 这类**针对文件**的模式无效；
只有 `-I 'src/'` 这种目录模式管用，而文档看不出这个区别。

**根因**：`AdditionalIgnores` 只在 `shouldIgnoreDir` 里检查，
`shouldIgnoreFile` 从未检查它。

**修复**：`shouldIgnoreFile` 也遍历 `AdditionalIgnores` 做 `matchPattern`。

### 为什么长期没被发现

两个参数都**没有任何测试覆盖**：

- `NoSubdirs` 唯一出现的地方是 `streaming_test.go` 里设为 `false`
- `AdditionalIgnores` 同样从未在测试里设过非空值

也就是说，它们从未被真正"使用"过，坏了也不会有人知道。

现已补 `nosubdirs_test.go`，四个用例，且每个都做过**反向验证**
（临时移除修复后测试确实失败，确认不是空断言）：

| 用例 | 断言 |
|---|---|
| `-n` | 只扫到根目录文件，且**不能是 0 个**（防"全都没扫到"也算通过） |
| 不带 `-n` | 扫到全部文件（对照，证明过滤确实在起作用） |
| `-I 'top.md'` | 该文件被排除，其余保留 |
| `-I '*.md'` | 只剩非 md 文件 |

**验证**：`go test ./...` 全绿；并用 npm 安装的实际二进制端到端复验。

---

## 新增记录（2026-10-11 · v1.8.0）：Git 变更打包

新增 `-g / --git-diff`、`-G / --commits` 两种变更模式，以及
`--rev`、`--staged`、`--no-merges`、`--diff-only`、`--max-diff-lines` 五个参数。

### 设计：git 只是"来源"，不是另一套流水线

```
                 ┌─ 来源 A：WalkDir 全量扫描 ──┐
changeset（git）─┤                              ├─→ 同一套过滤/忽略/敏感 gate/体积/渲染/剪贴板/推送
                 └─ 来源 B：git changeset ──────┘
```

`collectChangeSet()` 只产出一份 changeset（路径 + 状态 + diff），
`buildChangeFiles()` 把它换成与 `scanDirectory()` 完全相同的
`([]FileMetadata, Stats, []SkippedFile)`，所以 `-s / -c / -p / --icloud /
--dry-run / -o` 一行都不用改就能用。

实现落在新文件里，没往 43KB 的 `godoc.go` 里继续堆：

| 文件 | 职责 |
|---|---|
| `changeset.go` | git 调用、`-z` 解析、diff 分块、行数统计、截断 |
| `changeoutput.go` | 变更过滤（复用既有规则）+ 变更快照排版 |

### 实现期修掉的 5 个坑（都有回归测试或手工复验）

| 坑 | 症状 | 根因 / 修法 |
|---|---|---|
| `rename to` 解析 | 摘要里多出一条 `to src/deep/new.go` 的假条目 | 用 `ln[IndexByte(' ')+1:]` 取路径，把 `to ` 也留下了（"rename to " 是两段词）；改成 `TrimPrefix(ln, "rename to ")` |
| 暂存集合 | 所有变更都被标成 `staged` | 取暂存集合的 `git diff` 漏了 `--cached`，实际拿到的是"工作区 vs HEAD" |
| 未跟踪文件重复标记 | 表格里显示 `untracked, unstaged` | 未跟踪文件不在 `git diff` 里，`Unstaged` 是我们自己硬塞的 |
| `-d` 子目录范围失效 | `gx -g -d docs` 仍打包整个仓库 | `filepath.Rel(repoRoot, userDir)`：`--show-toplevel` 给的是真实路径 `/private/tmp/...`，而 `-d` 可能是软链 `/tmp/...`，算出 `../..` 直接放弃；改用 `git rev-parse --show-prefix` |
| `in N commits` 重复 | 标记变成 `in 2 commits, in 3 commits` | 在聚合循环里边累加边写标记；改为聚合结束再算 |

### 构建方式的变更（重要）

源文件从 1 个变成 3 个，所以所有构建入口都从"指定文件"改成"按包构建"：

```bash
# 以前
go build -o sourcepack godoc.go
# 现在
go build -o sourcepack .
```

涉及 `package.json` 的 `build` 脚本、`install.sh`、`.goreleaser.yaml`。
`go test ./...` 不受影响。

### 测试

`changeset_test.go` 覆盖解析层（`-z` 输出、diff 分块计数、路径反转义、
行数截断）与 git 集成层（暂存/未暂存/部分暂存、重命名、未跟踪、
`.gdignore`、敏感文件、`-a`、`-G` 与 `--no-merges`、删除文件、
子目录范围、空仓库、`--diff-only`、围栏加长）。

其中四个都做过**反向验证**（把修复临时改回错误实现，对应用例确实失败）：

| 反向改动 | 失败的用例 |
|---|---|
| `rename to` 改回 `ln[IndexByte(' ')+1:]` | `TestSplitPatchCountsAndPaths` |
| 暂存集合去掉 `--cached` | `TestGitWorkingTreeClassification`（staged 8 ≠ 3） |
| `--show-prefix` 改回 `filepath.Rel` | `TestGitSubdirScope`（Pathspec 变成 `""`） |
| hunk 计数改回"只看 `+`/`-` 前缀" | `TestSplitPatchCountsAndPaths`（+1 −1 ≠ +2 −2） |

---

## 新增记录（2026-10-12）：产物纯净性 —— 二进制不再渗进快照

起因是一轮外部评审：`--diff-only` 下未跟踪的二进制/超限文件拿到了合成 diff。
顺着查下去发现是**三类同源问题**，共同后果都是"裸字节进 Markdown、
产物不再是合法 UTF-8"。三个都不影响文本仓库的常规使用，但都是真的。

### 1（重构回归）`--diff-only` 的合成补丁绕过了二进制/体积 gate

`24f54cb` 里合成补丁生成在主循环的成功路径内，天然位于所有 `noContent`
gate 之后；`9b20203` 把它挪成独立循环（`changeoutput.go`）时只检查了
`filteredReason`，漏了 `noContent`。后果实测：

| 场景 | 修复前 | 修复后 |
|---|---|---|
| 未跟踪的小二进制（含 NUL） | 合成 diff 里塞裸字节、`+1 −0` | `+0 −0`、标记 binary、无 diff |
| 未跟踪的 60MB blob（超 `--max-size`） | **产物 62,952,472 字节**，RSS 148MB | 3,112 字节（`--max-size 1`），RSS 有界 |
| 已跟踪的单行 20MB 文本 | **产物 40MB**（按行截断对单行无效） | 514KB（字节上限生效） |

修法：合成补丁前补上 `noContent[p] != ""` 检查；给 diff 正文加字节上限
（复用 `--max-size`）；单行超限时在 UTF-8 字符边界内硬切。

### 2 二进制判定太弱：合法 UTF-8 + NUL 的文件被当文本

旧 `isBinaryBuffer` 只在"非法 UTF-8"时才去找 NUL，于是 `abc\x00def`
这类文件被当成文本，裸 NUL 直接写进产物（快照模式与变更模式都中招）。
判定收紧为：**含 NUL（与 git 的 `buffer_is_binary` 同标准）或不是合法
UTF-8 → 二进制**；探测窗口 1024 → 8000 字节（与 git 对齐）。

### 3 扩展名不是"内容是文本"的保证

旧实现里已知文本扩展名（`.md`/`.go`/`.json`…）**完全跳过探测**。实测一个
含 NUL 的 `.md` 会让整个产物变成非法 UTF-8。现在一律按内容探测——探测
复用同一个 `bufio` 缓冲，成本约等于 0。

> 顺带修掉一个自己引出的坑：探测窗口从 1024 改到 8000 后，`bufio.Peek`
> 因内部缓冲只有 4096 而返回 `ErrBufferFull`，所有非白名单文本文件都变成
> "Read error"（快照直接空掉）。缓冲区改为 32KB 并容忍 `ErrBufferFull`。

### 行为变化（是修复，但要知道）

1. **含 NUL 或非法 UTF-8 的文件不再进入快照**，即使它叫 `.md`。这类文件
   对 LLM 没有价值，且会污染整个产物。实测：含 NUL 的 `logo.bin` 从
   "7 个文件"变成"6 个文件 + 1 条 `[Binary file]` 跳过记录"。
2. `--max-size` 在变更模式下**同时限制单文件 diff 的字节数**（默认 500KB）。
3. `-a` 只关忽略规则，不打开二进制 gate（二进制文件永远不出正文）。

### 第二批（同日晚）：两个**已发布**版本里的用户可见 bug

这两条都在 `v1.8.0`（npm 上的当前版本）里，不是未发布改动引入的：

**1. `--staged` 的计数恒为 0。** 终端与文档写着
`staged 0 · unstaged 0 · untracked 0`，下面却列着一堆变更——因为
`collectWorkingTree` 只在非 `--staged` 时填 `stagedSet`，而 `--staged` 的
清单本身来自 `git diff --cached`，于是每条变更的 `Staged` 都是 false，
`summarizeChangeSet` 的 switch 全部落空。实测对比（同一仓库）：

```
修前:  Files changed: 3 (+2 −1)   staged 0 · unstaged 0 · untracked 0
修后:  Files changed: 3 (+2 −1)   staged 3 · unstaged 0 · untracked 0
git 真值: R100 src/deep/old.go→new.go / M src/main.go / A src/staged_new.go
```

**2. `-h` 的自定义说明整段丢失，且退出码为 2。** pflag 的
`(*FlagSet).usage()` 在 `f == CommandLine` 时调的是**包级** `pflag.Usage`，
而 v1.8 的重构只设了 `fs.Usage`——正好踩中这个分叉，`Convenience shortcuts:`
`Combinations:` `Environment variables:` 三段全没了；又因为 pflag 把
`ErrHelp` 交给 `ExitOnError` 处理，`gx -h` 还会打印 `pflag: help requested`
并以 2 退出。现在两份 Usage 都挂，并自己接管 `-h`：输出到 stdout、退出 0、
不再有 pflag 噪声（未知参数仍然是退出 2 并打印同一份说明）。

### 其余健壮性修复（内部，不改变用户可见行为）

| 位置 | 问题 |
|---|---|
| `runLimited` | 截断时吞掉 `waitErr`（`if waitErr != nil && !clipped`）——管道已排干，此时的失败一定是 git 真出错。用 `git diff --exit-code`（有差异时退出 1 且输出超限）可以稳定复现 |
| `collectHistory` | 64MB 截断只设了 `PatchClipped`，没往 `Warnings` 里记（`collectWorkingTree` 有） |
| `parseFlagsInto` | 丢掉 `fs.Parse` 的返回错误——生产路径靠 `ExitOnError` 无所谓，但它是为单测抽出来的，参数拼错会静默拿到零值 Config；现在记进 `Config.parseErr` 并由 `validateConfig` 抛出 |
| `pushStatsToRemote` | 没走 `artifactBaseName`，git 模式的 stats 推送会覆盖快照模式的 stats |
| 测试 `gitIn` | 只设了 `GIT_CONFIG_NOSYSTEM`，没隔离用户级 `~/.gitconfig`（`diff.external`/`core.autocrlf` 会污染断言），补 `GIT_CONFIG_GLOBAL=os.DevNull` |
| `capDiffBytes` | 单行硬切时返回的 `omitted` 兼作"是否截断"的标记，看起来像行数；标注文案改成用真实行数，语义写进注释 |
| `applyChunk` | `DiffFull` 在二进制检查之前就设了——正文都没输出，不该计入"总量过大"告警 |
| `mdCode` | 含换行/控制字符的路径原样输出会截断 Markdown（git 允许路径含换行）；现在先转义为可见写法 |

七处代码修复都做了反向验证（改回错误实现，对应用例确实失败），另有
`TestHelpFlagAndUsageText`、`TestUsageWiredForCommandLine`、
`TestGitStagedModeMarksEveryChangeStaged`、`TestRunLimitedSurfacesFailureEvenWhenClipped`、
`TestParseFlagsIntoKeepsParseError`、`TestMdCodeEscapesControlChars`、
`TestArtifactBaseName` 等新用例。

### 不可变性测试

`TestChangeOutputIsValidUTF8AndNULFree` 直接对两种模式的产物断言
"合法 UTF-8 且不含 NUL"，覆盖三类会漏字节的文件；另有
`TestGitDiffOnlyRespectsBinaryAndSizeGates`、`TestGitHistoryContentUsesNewestOccurrence`、
`TestScanDirectorySkipsBinaryByContentNotExtension`。四处修复都做过反向验证
（改回错误实现，对应用例确实失败）。

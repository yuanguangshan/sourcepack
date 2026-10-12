# Sourcepack 参数手册

> **本文所有示例都来自真机实测输出**，不是从代码推断的。
> 实测环境：macOS（Apple Silicon）、sourcepack **v1.9.0**；
> 夹具见 [附录 A](#附录-a实测夹具)，可复现。
> 生成产物的路径全部写在 `/tmp`，避免产物反过来污染夹具。

三个命令名完全等效：`sourcepack`、`gx`、`gdoc`（npm 全局安装后还可自定义别名，如 `gdx` = `sourcepack -s`）。

---

## 1. 参数速查表

### 范围与筛选

| 参数 | 默认 | 作用 |
| :--- | :--- | :--- |
| `-d, --dir` | `.` | 扫描根目录 |
| `-i, --include` | — | 只保留这些扩展名（`go,md`，可带点，大小写不敏感） |
| `-x, --exclude` | — | 排除这些扩展名 |
| `-m, --match` | — | 只保留**路径含**这些关键词的文件 |
| `-X, --exclude-match` | — | 排除**路径含**这些关键词的文件 |
| `-I, --ignore` | — | 追加 gitignore 语法忽略模式（支持 `**`、`!`、目录规则） |
| `-n, --no-subdirs` | false | 只扫根目录，不进子目录 |
| `--max-size` | 500 | 单文件大小上限（**单位 KB**） |
| `--no-default-ignore` | false | 关闭内置忽略（`node_modules`、`.git`、`build`…）与敏感文件拦截 |
| `--no-gitignore` | false | 不读 `.gitignore`（`.gdignore` 仍生效） |
| `-a, --all` | false | `--no-default-ignore` + `--no-gitignore`（**不含** `.gdignore`） |

### 输出方式

| 参数 | 默认 | 作用 |
| :--- | :--- | :--- |
| `-o, --out` | `project_snapshot.md` | 产物路径（git 模式下默认改为 `git_changes.md`） |
| `-s, --stats` | false | 只打印多维统计，**不写文件** |
| `--dry-run` | false | 只列出将被包含的文件，**不写文件** |
| `-v, --verbose` | false | 打印配置与跳过明细 |
| `-c, --copy` | false | 复制到剪贴板，**不写文件** |
| `--icloud` | false | 存到 iCloud Drive 的 Documents |

### Git 变更打包

| 参数 | 默认 | 作用 |
| :--- | :--- | :--- |
| `-g, --git-diff` | false | 打包未提交改动（工作树+索引 vs `--rev`） |
| `--rev` | `HEAD` | `-g` 的比较基准（任意 revspec） |
| `--staged` | false | 只打包已暂存改动（`git diff --cached`） |
| `-G, --commits` | 0 | 打包最近 N 个提交 |
| `--no-merges` | false | `-G` 时排除合并提交 |
| `--diff-only` | false | 只给 diff，不附变更文件全文 |
| `--max-diff-lines` | 2000 | 单文件 diff 行数上限（**0 = 不限**） |

### 远端推送

| 参数 | 默认 | 作用 |
| :--- | :--- | :--- |
| `-p, --push` | false | 把产物 POST 到远端 |
| `-u, --push-url` | 环境变量 | 推送地址（覆盖 `SOURCEPACK_PUSH_URL`） |
| `--auth-key` | 环境变量 | `X-Auth-Key` 头（或 `SOURCEPACK_AUTH_KEY`） |

### 元信息

| 参数 | 作用 |
| :--- | :--- |
| `-h, --help` | 打印完整用法（三段自定义说明），退出码 **0** |
| `--version` | 打印版本，退出码 0 |

---

## 2. 范围与筛选

### `-d, --dir` —— 扫描根目录

```bash
gx -d src -o /tmp/src.md
```

**实测**：产物里的路径是**相对于 `-d`** 的，不是相对于仓库根的。

```bash
$ gx --dry-run -d src
🔍 Files to be included (5):
  - app.go
  - deep/nested.go
  - feat.go
  - helper.go
  - staged_new.go
```

`-g`/`-G` 模式下 `-d` 指向子目录时，工具会自动定位仓库根（`git rev-parse --show-toplevel`）
并把变更限定在该子目录：

```bash
$ gx -g -d src --dry-run
  Repository:          /private/tmp/pt     # 仓库根
  Files changed:       1 (+50 −1)
🔍 Files to be included (1):
```

> **坑（已修，见 [第 10 节](#10-边界坑与已知问题)）**：v1.9.0 里 `-d` 给一个不存在的路径
> 会**成功**产出一份空快照（exit 0）。当前 main 已改为明确报错。

### `-i, --include` / `-x, --exclude` —— 按扩展名

```bash
gx -d . -i go,md -o /tmp/go-md.md
```

**实测**（逗号分隔；自动补点；大小写不敏感）：

| 命令 | 保留文件数 |
| :--- | ---: |
| 基线 | 14 |
| `-i go` | 7 |
| `-i GO` | 7 |
| `-i .go` | 7 |
| `-i go,md` | 10 |
| `-x go` | 7 |

> `-i` 与 `-x` 同时给时：先按 `-i` 收窄，再按 `-x` 排除。

### `-m, --match` / `-X, --exclude-match` —— 按路径**子串**

```bash
gx -d . -m deep          # 只保留路径含 "deep" 的
gx -d . -X src,assets    # 排除路径含 "src" 或 "assets" 的
```

**实测**：

| 命令 | 保留文件数 |
| :--- | ---: |
| 基线 | 14 |
| `-m src` | 5（`src/` 下 5 个） |
| `-m src,deep` | 5 |
| `-X src` | 9 |
| `-X src,assets` | 8 |
| `-X '*.go'` | **14（无变化）** |

> **重要**：`-m`/`-X` 是**子串匹配**，不是 glob。`-X '*.go'` 不会排除任何东西——
> 想按扩展名排除要用 `-x go`。多个值是"或"关系。

### `-I, --ignore` —— 追加 gitignore 语法规则

```bash
gx -d . -I '**/*.go'                    # 递归排除所有 .go
gx -d . -I '*.go,!src/feat.go'          # 排除 .go，但放行 feat.go
gx -d . -I 'src/'                       # 排除 src 目录
```

**实测**：

| 模式 | 保留文件数 | 说明 |
| :--- | ---: | :--- |
| 基线 | 14 | |
| `'**/*.go'` | 7 | `**` 递归生效 |
| `'*.go,!src/feat.go'` | 8 | `!` 负向重新纳入 |
| `'src/'` | 9 | 末尾 `/` 匹配目录 |
| `'docs 中文/'` | 13 | 含空格路径同样支持 |
| `'!main.go'` | 14 | 纯负向规则本身不排除任何东西 |

### `-n, --no-subdirs` —— 只扫根目录

```bash
$ gx --dry-run -d . -n
  .env.example
  .gdignore
  .gitignore
  README.md
  main.go
  pkg2.go
  未跟踪 文档.md
```

**实测**：14 → 7 个文件（根目录下的文件）。目录本身不进产物，也不会有空的目录树。

### `--max-size` —— 单文件上限（**KB**）

```bash
gx -d . --max-size 100      # 超过 100KB 的文件跳过
```

**实测**：

| 命令 | 保留 | 说明 |
| :--- | ---: | :--- |
| 基线（默认 500KB） | 14 | 300KB 的 `assets/medium.txt` 进来；600KB 的 `assets/big.txt` 被跳过 |
| `--max-size 100` | 13 | `medium.txt` 也被跳过 |
| `--max-size 1000` | 15 | `big.txt` 也进来 |

被跳过的文件在 `-v` 下列出原因：

```bash
$ gx -s -d . -v
⏭  Skipped 2 files:
  - assets/big.txt                           [Size limit]
  - assets/logo.bin                          [Binary file]
```

> **坑**：`--max-size 0` **不是"不限"**，而是"只剩 0 字节的空文件"（实测 0 个文件）。
> 与 `--max-diff-lines 0`（不限）语义相反，注意区分。

### `--no-gitignore` / `--no-default-ignore` / `-a`

```bash
gx -d . --no-gitignore        # 不读 .gitignore
gx -d . --no-default-ignore   # 关掉内置忽略表
gx -d . -a                    # 两者都关（.gdignore 仍生效）
```

**实测**：

| 命令 | 保留 | 说明 |
| :--- | ---: | :--- |
| 基线 | 14 | |
| `--no-gitignore` | 15 | `debug.log` 进来（`**/*.log` 不再生效） |
| `--no-default-ignore` | 43 | `node_modules/`、`vendor/`、`build/`、`.env`、**甚至 `.git/` 内部文件**都进来了（实测产物里 `.git/` 下的文件有 24 个） |
| `-a` | 46 | 再加上 `logs/app.log`（被 `.gitignore` 排除的） |

**结论与提醒**：

- `-a` 会**连敏感文件一起打包**（`.env` 实测确实进入产物），这是设计意图，但用之前要清楚；
- `--no-default-ignore` 会把 `.git/` 里的对象与配置一起打包（实测 24 个文件；`-a` 同理），
  几乎不是想要的，一般只在排查"为什么某个文件没进来"时临时用；
- `.gdignore` 在 `-a` 下**仍然生效**（实测 `work.tmp` 依旧被排除）。

---

## 3. 输出方式

### `-o, --out` —— 产物路径

```bash
gx -d . -o /tmp/snap.md
gx -d . -o out/snap.md        # 子目录路径也可以
```

**实测**：产物首行为 `# Project Snapshot: <目录名>`；路径目录不存在时会报错：

```bash
$ gx -d . -o /proc/x/y.md
▶ Sourcepack Started
❌ creating /proc/x/y.md: open /proc/x/y.md: no such file or directory
[退出码 1]
```

> 工具会**排除自己配置的产物文件**（实测 `project_snapshot.md` 不在待打包清单里），
> 但不会排除其它 .md 产物——`gx -o custom.md` 之后再跑一次全量扫描，`custom.md`
> 会作为普通文件进快照。建议产物一律写到项目外（如 `/tmp`）或用 `.gitignore` 排除。

### `-s, --stats` —— 只统计不写文件

```bash
$ gx -s -d .
📊 Project Statistics Summary
  Files Processed:     14
  Total Lines:         25037
  Total Size:          293.47 KB
  Directories:         6
  Est. Tokens:         ~75134 tokens
  Excluded by rules:   3
  Ignored dirs:        7 (.git, build, logs, node_modules, pkg/cache, scripts/release, vendor)
```

**实测**：不生成任何文件（已核实）。`Excluded by rules` / `Ignored dirs` 两行只在
有内容时出现，用来暴露"被静默跳过了什么"。

`-s` 与 `-p` 可以组合：把这份统计报告推送到远端（见 [第 5 节](#5-远端推送)）。

### `--dry-run` —— 只列清单不写文件

```bash
$ gx --dry-run -d .
🔍 Files to be included (14):
  - .env.example                             (1 lines)
  ...
```

**实测**：不生成任何文件；打印清单 + 统计 + 跳过明细。
这是排查"某个文件为什么没进快照"的首选命令。

### `-v, --verbose` —— 配置与跳过明细

```bash
$ gx -s -d . -v
⚙  Configuration:
  Root:                /tmp/pt
  Output:              project_snapshot.md
  Max file size:       500 KB
  Include matches:     []
  No subdirs:          false
  No default ignore:   false
  No .gitignore:       false
  ...
  Ignore files:        .gitignore, .gdignore (子目录中的同名文件同样生效)
```

**实测**：`-v` 会额外打印 `⏭ Skipped N files:` 段落与每个文件的跳过原因
（`Size limit` / `Binary file` / `Sensitive file` / `Ignored by xxx` 等）。

### `-c, --copy` —— 复制到剪贴板

```bash
$ gx -d . -c
✨ Done! Copied to clipboard in 16.3ms
```

macOS 用 `pbcopy`，Linux 用 `xclip` 或 `xsel`（都没有则报错），Windows 用 `clip`。

> **实测注意**：在本沙箱环境里 `pbpaste` 读回为空。已用"拦截式假 pbcopy"证明
> **工具确实把正确字节送给了 pbcopy**（263 字节、内容正确）；同一沙箱下
> `cat <含非 ASCII 的文件> | pbcopy` 同样读回 0 字节，而纯 ASCII 负载正常——
> 即**环境本身的限制**，与本工具无关。另外代码里 `cmd.Wait()` 的错误被丢弃
> （见 [第 10 节](#10-边界坑与已知问题)），pbcopy 真失败时会误报成功。

### `--icloud` —— 存到 iCloud Drive

```bash
$ gx -d . --icloud
✨ Done! Saved to iCloud: /Users/<你>/Library/Mobile Documents/com~apple~CloudDocs/Documents/pt+2026-10-12.md
```

**实测**：文件名 = `<目录名>+<日期>.md`，落在 iCloud Drive 的 `Documents` 下（手机可读）。
`--icloud` 与 `-o` 同时给时 `--icloud` 优先（`-o` 指定的文件不会生成）。

---

## 4. Git 变更打包

### `-g, --git-diff` —— 打包未提交改动

```bash
$ gx -g -d . -o /tmp/changes.md
📦 Git changes: git diff HEAD  (working tree + index vs HEAD)
  Repository:          /private/tmp/pt
  Branch:              main
  HEAD:                d522d85 test: 加中等大小文件与被忽略的 debug.log
  Files changed:       2 (+25165 −0)
                       staged 0 · unstaged 0 · untracked 2
  Filtered out:        1 (listed in the output)
```

产物的结构（实测）：

```markdown
# Git Change Snapshot: pt

> Source: `git diff HEAD  (working tree + index vs HEAD)`

> ⚠ 25165 changed lines total (diffs + new-file content) — this snapshot is large;
  consider `--diff-only` or a lower `--max-diff-lines`.

## Repository
- Root: `/private/tmp/pt`
- Branch: `main`
- HEAD: `d522d85` test: ...
- Mode: working tree
- Working tree: dirty
- Files changed: 2
- Excluded by rules: 1

## Change Summary
| # | File | Status | Lines | Flags |
...

## Changes
### 1. `src/app.go` — modified (+50 −1)

_staged, unstaged_
```diff
...
```
#### Full content
...
```

**关键点**：

- 每条变更同时给 **diff + 变更后的文件全文**（对 LLM 审查最有用）；
- 被排除的文件（敏感文件 / 忽略规则 / 超限）会在 `Excluded by rules` 段落里**列出原因**，
  不会静默消失；
- 已跟踪但命中 `.gitignore` 的文件**算真实变更**（不会被 gitignore 藏起来）；
- git 模式下默认产物名是 `git_changes.md`。

### `--rev` —— 指定比较基准

```bash
$ gx -g --rev HEAD~1 --dry-run -d .
📦 Git changes: git diff HEAD~1  (working tree + index vs HEAD~1)
  Files changed:       7 (+50173 −0)
```

**实测**：接受任意 revspec（`HEAD~1`、分支名、commit sha）。不给 `-g` 而单独用
`--rev` 会明确报错。

### `--staged` —— 只看已暂存

```bash
$ git add src/app.go
$ gx -g --staged --dry-run -d .
    Files changed:       1 (+1 −1)
                         staged 1 · unstaged 0 · untracked 0
```

**实测**：`--staged` 下 `staged` 计数与逐条标记都正确（v1.8.0 曾恒为 0，已修）。

### `-G, --commits` —— 打包最近 N 个提交

```bash
$ gx -G 5 -d . -o /tmp/last5.md
📦 Git changes: git log -5
  Commits:             5
```

产物按提交分段（实测）：

```markdown
### `d522d85` test: 加中等大小文件与被忽略的 debug.log
### `d1ddadf` merge: 合并 feature (merge — diff vs first parent)
### `4af625a` chore: main 侧改动
```

- 合并提交会标注 `(merge — diff vs first parent)`，diff 取第一父；
- 提交信息**只取标题**（`%s`）。正文（`%b`，往往写着"为什么这么改"）目前**不包含**——
  实测第 2 个提交的正文没有出现在产物里（见 [第 10 节](#10-边界坑与已知问题)）。

### `--no-merges` —— 排除合并提交

```bash
$ gx -G 5 --no-merges -d . -o /tmp/last5n.md
> Source: `git log -5 --no-merges`
### `d522d85` test: ...
### `4af625a` chore: main 侧改动
### `debccb9` feat: 新增 feat.go
### `cb17f63` refactor: 改名 helper、删除 keep.log、改 A/D
### `62b05c5` init: 初始提交
```

**实测**：`--no-merges` 后合并提交 `d1ddadf` 不再出现（git log 取 5 个非合并提交）。

### `--diff-only` —— 只给 diff

```bash
$ gx -g --diff-only -d . -o /tmp/diffonly.md
> Mode: **diff only** — full file contents are not included (`--diff-only`).
```

**实测体积对比**（同一变更集）：

| 模式 | 产物大小 |
| :--- | ---: |
| 默认（diff + 全文） | 303,345 字节 |
| `--diff-only` | 27,895 字节 |

适合"只看这次改了什么"的快速审查；缺点是模型看不到上下文代码。

### `--max-diff-lines` —— 单文件 diff 行数上限

```bash
$ gx -g --max-diff-lines 10 -d .
```

**实测**：超限处在产物中明确标注，且变更摘要的 Flags 会带 `diff truncated`：

```markdown
| 1 | `src/app.go` | modified | +50 −1 | staged, unstaged, diff truncated |
```

````markdown
```diff
...
... [sourcepack] 49 of 59 diff lines omitted (--max-diff-lines 10)
```
````

`--max-diff-lines 0` = 不限（实测产物 3,837 字节 vs 限 10 行的 2,496 字节）。

---

## 5. 远端推送

```bash
# 方式一：命令行
gx -d . -p -u https://example.com/submit --auth-key SECRET

# 方式二：环境变量（推荐，避免 token 出现在 ps / shell 历史里）
export SOURCEPACK_PUSH_URL=https://example.com/submit
export SOURCEPACK_AUTH_KEY=SECRET
gx -d . -p
```

**实测**（用本地 HTTP 服务接收）：

| 场景 | 服务端收到 |
| :--- | :--- |
| `-p -u .../submit --auth-key SECRET123` | `POST /submit`，`X-Auth-Key: SECRET123`，multipart 303,817 字节 |
| `SOURCEPACK_PUSH_URL=.../env SOURCEPACK_AUTH_KEY=ENVKEY gx -p` | `POST /env`，`X-Auth-Key: ENVKEY` |
| `gx -p`（无 URL） | 客户端报错，**退出码 1**：`❌ SOURCEPACK_PUSH_URL env is required for push` |
| `gx -s -p -u .../stats` | `POST /stats`，1554 字节的**统计报告**（`-s -p` 推的是 stats 而非快照） |

**注意**：

- 请求体是 `multipart/form-data`，内容不全在内存里（大快照会先落临时文件）；
- `-p` 与 `-c`、`--icloud` 互斥（见 [第 8 节](#8-组合互斥与优先级)）；
- `-s -p` 的完成提示是 `✨ Done! in ...`，不带推送目标（与 `-p` 的提示不同）。

---

## 6. 元信息

```bash
$ gx --version
sourcepack v1.9.0          # 从源码构建时显示 v1.9.1-0.<时间>-<sha>

$ gx -h
Usage of sourcepack:
  -a, --all                    Include all files (disable .gitignore + built-in ignores; .gdignore still applies)
  ...
Convenience shortcuts:
  ...
Combinations:
  -s -p           Show stats and push the stats-only markdown (the only allowed pair)
  other pairs     Error: -s / -c / -p / --icloud are mutually exclusive destinations
Environment variables:
  SOURCEPACK_PUSH_URL    Remote URL for push (e.g. https://host/submit)
  SOURCEPACK_AUTH_KEY    X-Auth-Key for push authentication
```

**实测**：`-h` 与 `--version` 退出码都是 **0**；`-h` 输出到 **stdout**
（可以 `gx -h | less`），且包含 `Convenience shortcuts` / `Combinations` /
`Environment variables` 三段（v1.8.0 曾整段丢失，v1.8.1 修复）。

---

## 7. 忽略规则（gitignore 语法）

`.gitignore`、`.gdignore`、`-I/--ignore` 共用同一套语义（v1.9.0 起与 git 对齐，
并有 `git check-ignore` 对照测试）。

| 写法 | 含义 | 例 |
| :--- | :--- | :--- |
| `*.log` | 不含 `/`，匹配**任意层级**的名字 | `a/b/x.log` 命中 |
| `src/*.go` | 含 `/`，相对规则文件所在目录；`*` **不跨过** `/` | `src/sub/x.go` **不**命中 |
| `**/foo` | 零层或多层目录 | `foo`、`a/b/foo` 都命中 |
| `src/**/*.go` | 中间 `/**/` 允许零层 | `src/x.go`、`src/a/b/x.go` 都命中 |
| `abc/**` | abc **内部**的内容（不含 `abc` 自身） | `abc` 保留，`abc/x` 命中 |
| `build/` | 末尾 `/` = 只匹配目录，其下内容一并忽略 | `src/build/x.go` 命中 |
| `!keep.log` | 负向模式，按"最后一条命中的规则生效" | 可与上面的规则配合重新纳入 |
| `\#name`、`\!name` | 字面量的 `#` / `!` | — |
| 行尾空格 | 未转义的行尾空格会被剥掉 | — |

**优先级**（低 → 高）：浅层 `.gitignore` → 深层 `.gitignore` → 浅层 `.gdignore`
→ 深层 `.gdignore` → 命令行 `-I`。即 **`.gdignore` 永远压过 `.gitignore`**。

**嵌套生效**：每一层目录的 `.gitignore` / `.gdignore` 都会叠加（与 git 一致）。

**`.gdignore`**：Sourcepack 自己的忽略文件，语法同 `.gitignore`，用于"不想改项目
`.gitignore` 但想让快照跳过某些文件"的场景。实测 `work.tmp` 被排除，
且在 `-a`（关闭 `.gitignore` 与内置忽略）下**依然生效**。

**被忽略目录下的内容无法用 `!` 救回**（git 规则）：遍历模式在该目录上 `SkipDir`；
git 变更模式由匹配器回溯各级父目录兜住。

---

## 8. 组合、互斥与优先级

| 同时给出 | 结果（实测） |
| :--- | :--- |
| `-c` + `-p` | **报错**，退出码 1：`output destinations are mutually exclusive, but got -c/--copy + -p/--push` |
| `-s` + `-c` | **报错**（同理） |
| `-s` + `-p` | ✅ 唯一允许的组合：推送 stats-only 报告 |
| `--icloud` + `-o` | `--icloud` 优先，`-o` 文件不生成 |
| `-g` + `-G` | **报错**：`-g/--git-diff and -G/--commits are mutually exclusive (pick one)` |
| `--staged` / `--rev` / `--no-merges` 不给对应模式 | **报错**（不静默忽略） |
| `-n` + `-g` | **报错**：`-n/--no-subdirs has no effect in git mode; use -m/--match or -d <subdir> instead` |
| `-i` + `-x` | 先按 `-i` 收窄，再按 `-x` 排除 |
| `--dry-run` + 任意 | dry-run 优先（只列清单，不写文件、不推送） |

**设计原则**：宁可明确报错，也不要"警告一句然后按某个隐式顺序取一个"。

---

## 9. 退出码

| 退出码 | 场景（实测） |
| :---: | :--- |
| `0` | 成功；`-h` / `--version` 也是 0 |
| `1` | 业务错误：`-d` 不存在、推送缺 URL、输出目的地互斥、参数适用性错误（`--staged` 没给 `-g`）、`-o` 不可写、`-g` 用在非 git 目录 |
| `2` | 参数解析错误（未知参数、类型错误），同时打印完整用法 |

实测样例：

```bash
$ gx --nope                 # → 退出码 2，末尾打印 "unknown flag: --nope"
$ gx -d . --staged          # → 退出码 1，❌ --staged only applies to -g/--git-diff
$ gx -d . -g -G 2           # → 退出码 1，❌ ... mutually exclusive (pick one)
$ gx -d . -G 0              # → 退出码 1，❌ -G/--commits must be >= 1
$ gx -p -d .                # → 退出码 1，❌ SOURCEPACK_PUSH_URL env is required for push
```

---

## 10. 边界、坑与已知问题

### 已修复（在 main，尚未发版）

**`-d` 指向不存在路径或文件时静默成功**。v1.9.0 实测：

```bash
$ gx -d /nope/nope -o /tmp/nodir.md ; echo $?
0                                  # ← 退出码 0
$ head -6 /tmp/nodir.md
# Project Snapshot: nope
## Project Structure
```
nope/
```

路径打错一个字母，用户会以为打包成功，喂给模型才发现是空的。已在 `validateConfig`
加上校验：不存在的路径报 `-d/--dir is not accessible`，指向文件报
`-d/--dir must be a directory`（均退出码 1）。

### 已知问题（未修）

| 问题 | 影响 | 说明 |
| :--- | :--- | :--- |
| `-c` 失败时误报成功 | 剪贴板没拿到内容，终端却说 `✨ Done! Copied to clipboard` | 代码里 `cmd.Wait()` 的错误被丢弃。实测本沙箱的失败是 pbcopy 环境限制（pbcopy 退出码 0），即使检查错误也拦不住；但真失败（如 Linux 无 xclip 已单独报错）值得一并处理 |
| 非 UTF-8 文本（GBK 等）按二进制丢弃 | 老项目源码会整文件消失，只显示 `[Binary file]` | 为守住"产物永远是合法 UTF-8"的不变量；可改进为区分 `Non-UTF-8 text` 与 `Binary`，或做转码 |
| 默认忽略目录较激进 | `pkg/cache/`、`scripts/release/`、`bin/`、`logs/` 等按目录名任意深度跳过 | 这些目录里若放着真代码会被跳过；`-s` 的 `Ignored dirs` 会列出来，可用 `--no-default-ignore` 关闭 |
| 敏感文件只按文件名判断 | 写在 `config.go`、`docker-compose.yml` 里的 token 不会被拦 | `-p` 推送源码前请自行确认 |
| 提交信息只取标题 | `-G` 产物里看不到提交正文（`%b`），而"为什么这么改"通常写在正文里 | 实测确认；取 `%s` 不取 `%b` |
| `.env.example` 等模板已放行 | 模板文件按约定不含真值 | v1.8.2 起 |

### 使用坑（实测确认，非 bug）

- `--max-size 0` ≠ 不限（只剩空文件）；`--max-diff-lines 0` = 不限。两者语义相反。
- `-X` 是子串不是 glob：`-X '*.go'` 什么都不排除。
- `--no-default-ignore` / `-a` 会把 `.git/` 内部文件一起打包。
- 产物文件自我排除只针对 `-o` 指定的那一个；其它 .md 产物会进快照。
- `-d <子目录>` 时产物内路径是相对该子目录的，不是相对仓库根。

---

## 附录 A：实测夹具

```bash
# 结构（含中文与空格路径、二进制、超限大文件、敏感文件、嵌套忽略文件）
/tmp/pt
├── .gitignore          # **/*.log / build/ / !keep.log
├── .gdignore           # **/*.tmp
├── .env                # 敏感 → 默认排除
├── .env.example        # 模板 → 保留
├── README.md  main.go  pkg2.go  debug.log(被 **/*.log 排除)  work.tmp(被 .gdignore 排除)
├── "docs 中文/指南.md"  # 含空格与中文的路径
├── src/{app.go,helper.go,feat.go,staged_new.go,deep/nested.go}
├── assets/{logo.bin(含 NUL),big.txt(600KB),medium.txt(300KB)}
├── pkg/cache/store.go   scripts/release/publish.sh   logs/app.log
├── build/out.o  vendor/lib.go  node_modules/x.js     # 被内置忽略
└── 未跟踪 文档.md        # 未跟踪
# git 历史：init → refactor(改名+删除+改两文件) → feat → chore → merge
```

**基线**：14 个文件 / 25,037 行 / 293.47 KB / ~75,134 token；
`Skipped`: `big.txt [Size limit]`、`logo.bin [Binary file]`；
`Excluded by rules`: 3；`Ignored dirs`: 7。

## 附录 B：本次实测覆盖

| 类别 | 覆盖 | 方式 |
| :--- | :--- | :--- |
| 筛选 11 个 | 全部 | 每个参数单独跑 `--dry-run`，比对文件清单与数量 |
| 输出 6 个 | 全部 | 校验文件是否生成/不生成、剪贴板与产物比对、iCloud 落盘路径 |
| Git 7 个 | 全部 | 真 git 仓库（改名/删除/合并/暂存/未跟踪）+ 产物逐段检查 |
| 推送 3 个 | 全部 | 本地 HTTP 服务接收，校验路径/`X-Auth-Key`/体积/环境变量 |
| 元信息 2 个 | 全部 | 退出码与三段说明 |
| 互斥/错误 | 12 种组合 | 逐个验证报错文案与退出码（0/1/2） |
| 边界 | 空目录、不存在目录、指向文件的目录、不可写输出、非 git 目录用 `-g` | 逐个验证 |

发现并修复 1 个缺陷（`-d` 静默成功）、记录 6 个已知问题与 5 个使用坑，
并纠正了本文档第一版里一处写反的断言（`-G` 的提交正文）。

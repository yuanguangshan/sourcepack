# Sourcepack (gdoc)

**一键将项目代码库转换为 AI 可读的 Markdown 快照。**

> 📖 全部参数的详细说明与实测示例见 **[PARAMETERS.md](PARAMETERS.md)**（参数手册）。
>
> 📚 想拿它当**书**读？`gx -book` 出书排版：封面 + 目录 + 按目录分「部」的章节 + 附录，
> Markdown 文章按正文排版（不是代码块）。见 [书排版](#书排版-book给人读的版本)。
>
> 🎓 想深入 Git？本仓库附带一套 12 章的 **[Git 深入课](git-course/README.md)**——以本项目为标本，从对象模型、索引、提交图讲到性能与发布链路，每章都附本机实测输出与动手实验（[博客版](git-course/published-urls.md)）。**写给日常在用 Git、想建立底层模型的工程师，不是入门课。**
>
> 📘 这套课也已装订成**一本书**（5 部 12 章 + 6 份附录：术语表 / 命令速查 / 知识地图 / 延伸阅读 / [进阶主题地图](book/src/back-50-appendix-e-advanced-map.md) / [把 checklist 跑起来](book/src/back-60-appendix-f-minigx.md)）。
> 书稿源在 [`book/`](book/src/)（`book/src/` 是元内容与附录，`git-course/` 是正文）；
> 要 HTML / EPUB / Markdown 三种形态，先跑一次 `python3 book/tools/build.py`（零依赖，只用标准库），
> 产物落在 `book/dist/`。HTML 版默认跟随系统配色，右下角有**三态主题开关**（跟随系统 / 浅色 / 深色）。

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
| `-I, --ignore` | 追加 gitignore 语法忽略模式（逗号分隔） | `-I '**/*.log'` → 递归排除日志 |
| `--no-gitignore` | 不读 `.gitignore` | 原本被忽略的文件会进来 |
| `--no-default-ignore` | 关闭内置忽略（`node_modules`、`.git` 等） | `node_modules/` 会进来 |
| `-a, --all` | 关闭 `.gitignore` 与内置忽略（**不含 `.gdignore`**） | 见下方说明 |

> `-I` 对**文件和目录都生效**（v1.7.4 前只对目录生效，是 bug）。

#### 忽略语法（v1.9.0 起与 git 对齐）

`.gitignore`、`.gdignore`、`-I/--ignore` 现在共用**同一套 gitignore 语义**，
并且有对照测试：`ignore_test.go` 会把一批模式交给 `git check-ignore` 判定，
再断言我们得到同样的答案（当前 14 个模式 × 40 条路径完全一致）。

| 写法 | 含义 | 例 |
|---|---|---|
| `*.log` | 不含 `/`，匹配**任意层级**的名字 | `a/b/x.log` 命中 |
| `src/*.go` | 含 `/`，相对 ignore 文件所在目录；`*` **不跨过** `/` | `src/sub/x.go` **不**命中 |
| `**/foo` | 零层或多层目录 | `foo`、`a/b/foo` 都命中 |
| `src/**/*.go` | 同上，中间 `/**/` 允许零层 | `src/x.go`、`src/a/b/x.go` 都命中 |
| `abc/**` | abc **内部**的内容（不含 `abc` 自身） | `abc` 保留，`abc/x` 命中 |
| `build/` | 末尾 `/` = 只匹配目录，其下内容一并忽略 | `src/build/x.go` 命中 |
| `!keep.log` | 负向模式，按"最后一条命中的规则生效" | 可与上面的规则配合重新纳入 |
| `\#name`、`\!name` | 字面量的 `#` / `!` | — |
| `logs/` 后跟空格 | 行尾未转义的空格会被剥掉 | — |

**嵌套生效**：每一层目录的 `.gitignore` / `.gdignore` 都会叠加，
与 git 一致（v1.9.0 前只读根目录那一份）。

**优先级**（低 → 高）：浅层 `.gitignore` → 深层 `.gitignore` → 浅层 `.gdignore`
→ 深层 `.gdignore` → 命令行 `-I`。也就是 **`.gdignore` 永远压过 `.gitignore`**。

**被忽略目录下的内容无法用 `!` 救回**（git 的规则：不能重新纳入"父目录已被排除"
的文件）。这条有两种表现：遍历模式在该目录上 `SkipDir`；git 变更模式没有遍历，
由匹配器在判定文件时回溯各级父目录兜住。

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
| `-book, --book` | **书排版**：封面 + 目录 + 按目录分「部」的章节 + 附录 | 给人读的 md，默认产物 `project_book.md` |
| `--book-title` | `--book` 的书名（默认用目录名） | `--book-title '《XXX》'` |
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
| `-c` + `-p` | **明确报错**（输出目的地互斥；唯一允许的组合是 `-s` + `-p`） |
| `--icloud` + `-o` | **`--icloud` 优先**，`-o` 指定的文件不会生成 |
| `-i` + `-x` | 先按 `-i` 收窄，再按 `-x` 排除 |
| `-g` + `-G` | **明确报错**（两种模式互斥） |
| `--staged` / `--rev` 不给 `-g` | **明确报错**（不静默忽略） |
| `-n` + `-g`/`-G` | **明确报错**（git 模式不走目录遍历） |
| `-book` + `-g`/`-G` | **明确报错**（书排版只对全量快照有意义；变更集是给审查用的） |
| `-book` + `-o` | `-o` 优先（默认名 `project_book.md` 只在没给 `-o` 时生效） |

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

### 书排版常用示例

```bash
gx -book                          # 给人读的版本 → project_book.md
gx -book -i md                    # 只把 md 文章装订成书（正文排版，不是代码块）
gx -book --book-title '《XXX》'    # 指定书名（默认用目录名）
gx -book -x html,png              # 把大块 HTML/图片产物挡在外面
gx -book --icloud                 # 存进 iCloud Drive，手机上当书读
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

## 书排版：`-book`（给人读的版本）

默认产物是给模型读的：结构树 + 文件清单 + 围栏代码，密度优先。`-book` 换一套版式，
目标是**在任意 Markdown 阅读器里从头读到尾**——Typora / Obsidian / VS Code 预览 /
GitHub 网页版都能直接看，目录锚点可跳转：

- **封面** — 书名（默认取目录名，可用 `--book-title` 换）+ 一句话说明 +
  「关于本书」表：部/章数、行数、体积、估算 token、主要语言、生成器版本
- **目录** — 部/章两级（部不缩进、章缩进一级，纸质书目录的样子），全部带锚点
- **正文** — 按顶层目录分「部」，每个文件一章：
  - **Markdown 文件按正文排版**：用它自己的 H1 当章标题，正文标题整体下沉
    （章 = H2，正文最小标题级别落到 H3），所以散文章节读起来就是一本书，
    而不是"一堆被代码块包住的文章"；代码块里的 `# 注释` 不会被误当成标题
  - 其余文件（代码等）保持完整的围栏代码
  - 每章开头一行标注：语言 · 行数 · 体积 · 文件路径 · 最后修改时间
- **附录 A 目录结构** — 结构树（含每个文件的行数）
- **附录 B 统计** — 总览 / 按目录 / 按语言（与 `-s` 共用同一套排序口径）
- **附录 C 打包参数** — 这次用了哪些过滤、忽略规则是否生效、挡掉了多少文件，
  以及**一条能复现这份书的命令**（`cd <目录> && sourcepack --book -i md …`）

```bash
gx -book                          # → project_book.md
gx -book -i md                    # 文档型项目：只收 md 文章，成一本纯散文书
gx -book --book-title '《XXX》'    # 指定书名（默认用目录名）
gx -book -o repo-book.md          # 换个产物名
gx -book --icloud                 # 存进 iCloud Drive，手机上当书读
```

与默认快照的差别（同一个项目、同一批文件）：

| | 默认快照 | `-book` |
|---|---|---|
| 面向 | 模型 / 脚本 | 人 |
| 首行 | `# Project Snapshot: <目录>` | HTML 溯源注释 + 书名页 |
| 文件定位 | `## <相对路径>` | `第 N 部`（H1）→ 章（H2，标题来自内容或路径） |
| Markdown 文件 | 围栏代码块 | **按正文排版**，标题下沉一级 |
| 每文件元信息 | 无 | 语言 · 行数 · 体积 · 修改时间 |
| 结尾 | 可选 `-s` 统计 | 目录结构 / 统计 / 打包参数 三个附录 |

**它只换版式，不改内容**：走的是同一批 `files/stats`，所以 `-s`、`-c`、`-p`、`--icloud`
全部照旧可用；默认产物名 `project_book.md` 与 `project_snapshot.md` 不同，两份互不覆盖。
`-g`/`-G`（变更集）不支持 `-book`，会明确报错而不是给出一份四不像。

> **单横线写法一律接受**：`-book` 等价 `--book`，`-stats`、`-dry-run`、`-no-subdirs`
> 这类长参数名同样可以直接用单横线写。pflag 默认只认 `--`，单横线会被当成短参数簇
> （`-dir` 曾经等于 `-d ir`、`-no-subdirs` 等于 `-n -o subdirs`，静默解析成别的参数）。

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

## 未来演进

已知但**有意推迟**的改进（含触发条件与实测数据）记在 [TODO.md](TODO.md)：
npm 生命周期脚本收紧后的 `optionalDependencies` 拆包方案（以及为什么
**不能**改成运行时即时下载）、扫描并发的触发条件、多层 `.gitignore`、
`runLimited` 峰值内存。

---

## 修复记录（2026-09-15）

本日修复了两个**长期静默失效**的过滤参数。它们的共同点是：
`--help` 和 README 都写着能用，实际却不起作用，且没有任何测试覆盖。

### `-n / --no-subdirs`：完全失效（v1.7.3 修复）

**症状**：加 `-n` 后子目录文件仍被扫入。

**根因**（`main.go`，当时叫 `godoc.go`）：

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

实现落在新文件里，没往 43KB 的 `main.go` 里继续堆：

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
# 以前（源文件还叫 godoc.go，单文件时代）
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
UTF-8 → 二进制**，且判定范围是**整个文件**而不是前若干字节（按字节截断
的窗口会把"切在多字节字符中间"的合法 UTF-8 误判成非法，见 v1.8.1）。

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

### `-h` 的行为变化（用户可见）

| | 修前（含已发布的 v1.8.0） | 修后 |
|---|---|---|
| 三段自定义说明 | 全丢（`Convenience shortcuts:` / `Combinations:` / `Environment variables:`） | 全在 |
| 退出码 | 2 | **0** |
| 输出里多一句 | `pflag: help requested` | 无 |
| 帮助内容输出到 | stderr | **stdout**（`gx -h \| less` 可用） |
| 未知参数 | 打印同一份说明 + exit 2 | 不变 |

顺带把 pflag 从 v1.0.5 升到 v1.0.7（v1.0.6+ 才导出 `Output()`，
`printUsage` 才能精确恢复调用者的输出目标而不是一律恢复成 stderr）。

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

---

## 修复记录（2026-10-12 · v1.8.1）

v1.8.1 是质量修复版，四件事都对应线上真实痛点。

### 1. Windows 安装彻底修好（v1.8.0 上完全不工作）

两个独立缺陷叠在一起，任何一个都会让 Windows 用户装完即挂：

| 缺陷 | 症状 |
|---|---|
| 归档名用了 Node 的 `os.platform()`（`win32`），而 GoReleaser 用 GOOS（`windows`） | 请求 `sourcepack_<v>_win32_amd64.zip` → **404** |
| `download-binary.js` 解压后存成 `sourcepack.exe`，`index.js` 却找 `sourcepack-win-amd64.exe` | 即使下载成功也 **Binary not found** |

修法不是改字符串，而是**把命名规则收敛到一处**：`download-binary.js` 导出
`resolveTarget()` / `binaryNameFor()`，`index.js` 也调用同一个函数取落地名，
两侧不再各写一套模板。新增 `test/asset-names.test.js`（6 组平台×架构断言
归档名与落地名，外加"index.js 不得再硬编码模板"的结构性断言），CI 增加
`node --test test/`。

### 2. >8KB 且含中文的文件被误判成二进制（fbd6401 引入，未发布）

v1.8.1 开发期间实测发现：11KB 的 `.go` 文件（含中文注释）被静默跳过。
根因是二进制探测对**按字节截断的 8000 字节窗口**做 UTF-8 校验，而窗口
很可能正好切在一个汉字中间——多字节字符被腰斩，`utf8.Valid` 为假，
于是判成二进制。**任何大于探测窗口且含中文/emoji 的文件都会中招。**

修法：判定范围改为**整个文件**（含 NUL 或非法 UTF-8），流式读取时用
`incompleteUTF8Tail()` 把末尾最多 3 字节的半截字符留到下一块，不参与校验。
这比 git 的"前 8000 字节"窗口更严，也正好满足"正文必须能安全写进
Markdown"这个真实目的。

回归测试 `TestScanFileStatsLargeUTF8FileIsNotBinary` 的夹具特意让 8000 这个
截断点**必定**落在汉字中间（`8000 % 3 == 2`）——第一版夹具用整行文本，
截断点正好落在行首 ASCII 上，测不出这个 bug。

### 3. 中文 Token 估算不再偏低

旧实现一律 `size / 4`：UTF-8 汉字 3 字节只算 0.75 token，中文项目明显偏低。
现在按内容估算：

- CJK 汉字/全角标点约 **1 token/字**（UTF-8 三字节序列首字节 0xE3–0xE9）；
- 其余（英文、代码、空白）约 **4 字节/token**。

实测同一份 13.6KB 中文文档：旧值 3400 → 新值 4350；纯英文文件数值不变
（不回归）。估算在统计行数的同一次流式读取里完成，零额外 I/O、零依赖。

> 这是跨模型都偏保守的粗估：DeepSeek/Qwen 这类中文优化过的分词器实际更省，
> gpt 系列更费。`-s` 里的数字当量级看即可。

### 4. `godoc.go` → `main.go`

单文件时代的历史包袱。构建入口早已是 `go build .`，所以这次只是
`git mv`，同步更新了 `install.sh`、代码注释与 README 里的引用。

---

## 修复记录（2026-10-12 · v1.8.2）

### 1. 跨 32KB 块边界的中文文件被误判成二进制（P0，v1.8.1 线上）

`scanFileStats` 里这一行取的是上一块**开头**的字节，`carry` 里的半截字符
从未被拼回缓冲区：

```go
scan = append(scan[:carryLen], chunk...)   // ✗ carry 没用上
scan = append(scan[:0], carry[:carryLen]...) // ✓ 先把 carry 拼回去
scan = append(scan, chunk...)
```

于是块边界上的汉字被当成非法 UTF-8，整个文件被判 Binary file 并静默丢弃：

| 文件 | 大小 | v1.8.1 | v1.8.2 |
|---|---:|---|---|
| 中文注释 + ASCII 代码 | 183 KB | ❌ Binary file | ✅ |
| 中文正文 + ASCII | 156 KB | ❌ Binary file | ✅ |
| 纯汉字 | 120 KB | ✅ | ✅ |
| 纯 ASCII | 95 KB | ✅ | ✅ |

纯汉字之所以幸存，是因为文件开头恰好是 `E4 B8`（一个汉字的 lead+continuation），
拼上第二块首字节正好凑成一个"合法"字符——纯属巧合，混入 ASCII 就必现。

**比修 bug 更重要的是补上系统性防线**：`scanFileStatsChunk(path, probe, chunkSize)`
把块大小参数化，测试用 1/2/3/5/7/13/64/1024/32K 逐档断言"二进制判定与块大小无关，
且恒等于整份内容含 NUL 或非法 UTF-8"。`chunk=1` 时每个字节都是一条边界，
跨块情形被穷举，不再依赖"夹具够不够大"。反向验证：改回错误写法后，
中文+ASCII 用例在 `chunk=2..13` 立即失败。

> 这块逻辑两天内改写过四代（1024 窗口 → 整文件 → 8000 窗口 → 流式 carry），
> 前几次都靠"夹具是否恰好落在边界"碰运气；性质测试是让它系统性消失的办法。

### 2. diff 路径解析不再受用户 git 配置影响

两个独立缺陷：

1. **前缀不固定**：用户若设了 `diff.mnemonicPrefix = true`，diff 头部变成
   `--- c/b/gone.go`；设了 `diff.noprefix` 则完全没有前缀。路径还原不出来，
   diff 贴不上对应变更，还会冒出幽灵条目。现在所有产生 patch 的调用都显式带
   `--src-prefix=a/ --dst-prefix=b/`。
2. **`a/`、`b/` 连剥两层**：仓库里真实存在 `b/` 目录时，`--- a/b/x.go`
   会被剥成 `x.go`（应为 `b/x.go`）。现在只剥与本行匹配的那一个前缀。

实测（仓库里有 `b/` 目录 + 删除 `b/gone.go`，且用户开了 `mnemonicPrefix`）：

| | v1.8.1 | v1.8.2 |
|---|---|---|
| 变更数 | **2**（幽灵条目） | **1** |
| 条目 | `b/gone.go` deleted `+0 −0`（diff 没贴上）<br>`c/b/gone.go` modified `+0 −1` | `b/gone.go` deleted `+0 −1` |

### 3. 输出目的地互斥改为报错

原实现只打印一句警告，然后按真实代码顺序悄悄取一个——而警告里的优先级
（`clipboard > push > icloud > stats > file`）**与实际顺序相反**（`stats` 分支在
`copy` 之前 return）。现在互斥组合直接报错，唯一例外是文档化的 `-s -p`
（`-s` 生成的那份报告就是被推送的内容）：

```
$ gx -s -c
Error: output destinations are mutually exclusive, but got -s/--stats + -c/--copy
  (the only allowed pair is -s -p: the stats report is what gets pushed)
```

### 4. `.env` 模板不再被当敏感文件

`.env.example` / `.env.sample` / `.env.template` / `.env.dist` 按约定不含真值，
而且是 LLM 理解"这个项目需要哪些配置项"最有用的文件，现在放行；
`.env`、`.env.local`、`.env.production` 等仍然拦截。

### 5. 被内置规则跳过的目录现在会报告

`ignoreDirs`（`cache`、`release`、`logs`、`bin`…）是按目录名、任意深度匹配的，
`pkg/cache/`、`scripts/release/` 这类放着真代码的目录会被整目录跳过，而以前
**完全静默**——`-v`、`--dry-run` 都看不到。现在统计里会列出来：

```
  Excluded by rules:   1
  Ignored dirs:        3 (.git, pkg/cache, scripts/release)
```

默认列表本身没有收窄（那是行为变更，见 [TODO.md](TODO.md)），先让跳过的东西可见。

### 6. 发布闸门与 CI

v1.8.0 是"打 tag 直接构建发布"放出去的，当天暴露三个线上可见问题。现在：

- `release.yml` 增加 `gate` job（`go vet` + `gofmt -l` + `go test -race` + `node --test`），
  `goreleaser` 与 `npm-publish` 都要 `needs` 它；
- 发布构建固定 Go `1.24.x`（原来 `stable`，同一个 tag 在不同时间会用不同工具链）；
- 权限按 job 最小化（`npm-publish` 不再继承 `contents: write`）；
- npm 认证改用 `setup-node` 的 `registry-url` + `NODE_AUTH_TOKEN`，
  不再 echo token 到 `~/.npmrc`，并移除会打印配置的 Debug 步骤；
- `test.yml` 矩阵加入 **windows-latest**（v1.8.0 的安装 bug 就是因为矩阵里没有
  Windows 才留到发布后才发现），并加上 `go vet` / `gofmt -l` / `-race`。

### 7. CI 加上 windows-latest 之后，当天就抓到两个平台问题

这两个以前根本看不见——v1.8.0 的 Windows 安装 bug 也是同一个原因留到发布后才暴露。

| 问题 | 现象 | 修法 |
|---|---|---|
| CRLF 让格式闸门假失败 | Windows runner 的系统 gitconfig 里 `core.autocrlf=true`，检出的文本文件是 CRLF，而 `gofmt -l` 把 CRLF 判成"未格式化"，3 个 Windows job 全挂在 `gofmt` | 新增 `.gitattributes`：`* text=auto eol=lf`（同时让 `gofmt` 步骤打印违规文件，不再靠 `test -z` 吞掉输出） |
| `*` 在 Windows 上跨过 `/` | `matchPattern("src/sub/main.go", "src/*.go")` 返回 `true`：`filepath.Match` 在 Windows 上把 `\` 当分隔符、把 `/` 当普通字符。影响 `-I/-x/-X` 在 Windows 上过度排除 | 改用斜杠语义的 `path.Match`（调用方传入的路径早已 `ToSlash` 归一） |

> `gitignorePattern` 里还有三处 `filepath.Match`，同一个问题；它要连同 `**` 一起重写，
> 见 [TODO.md](TODO.md)。

### 8. 零碎

| 项 | 说明 |
|---|---|
| `install.sh` | 卸载分支写的是 `$SHORTCUT_NAME`（变量实为数组 `SHORTCUT_NAMES`），`~/.local/bin` 清理从未执行 |
| `LICENSE` | 补上 ISC 全文（`package.json` 早就声明了 ISC 却没有许可证文件） |
| `package.json` | 补 `engines.node >= 16`（`fs.rmSync` 需要 14.14+）；版本 1.8.2 |
| `.gitignore` | 补 `git_changes.md`、`project_book.md`（旧产物会在下次全量扫描里自我污染）、`.sessions/`、`.claude/settings.local.json` |
| `go.mod` | `go 1.21` → `go 1.23`：CI 从 1.21/1.22 起就不测（macOS 15 dyld 问题），声明与实测对齐 |

---

## 新增记录（2026-10-12 · v1.9.0）：忽略匹配器按 git 语义重写

旧实现只支持简化 glob，**README 承诺的 `**` 系列几乎全废**——`**/*.log`、
`src/**/*.go` 这类中文项目里最常见的忽略写法一个都不生效，文件照样进快照，
而且没有任何测试（测试只覆盖了"写的人想到的写法"）。

| 文件 | `.gitignore`: `**/*.log` `src/**/*.go` `docs/**/*.md` | git 真值 |
|---|---|---|
| `app.log` | v1.8.2：进快照 → v1.9.0：排除 | 忽略 |
| `src/app.log` | v1.8.2：进快照 → v1.9.0：排除 | 忽略 |
| `src/h.go` | v1.8.2：进快照 → v1.9.0：排除 | 忽略 |
| `docs/g.md` | v1.8.2：进快照 → v1.9.0：排除 | 忽略 |
| `docs/keep.md`（`!` 负向） | 保留 → 保留 | 保留 |

同时修掉四个同源问题：

1. **嵌套 `.gitignore` / `.gdignore`**：以前只读根目录那一份，每层目录的文件现在
   都叠加生效（与 git 一致）；
2. **`!` 负向、`\#`/`\!` 转义、行尾空格、`[!...]` 取反**：以前没有这些语义；
3. **匹配器内部三处 `filepath.Match`**：Windows 上 `*` 会跨过 `/`
   （`matchPattern` 在 v1.8.2 修过，这三处这次一起清掉）；
4. **`-I/--ignore`**：原来走另一套简化 glob（`-I '**/*.log'` 完全无效），
   现在与 `.gitignore` 共用同一套语义。

**一条容易写错的 git 语义**：`a/**/b` 之下，`a/b/c` 也被忽略——因为 `a/b`
这一级目录命中了规则，而 git 规定"父目录被排除 → 其内容一律排除，`!` 也救不回"。
遍历模式靠 `SkipDir` 天然满足，但 **git 变更模式没有遍历**，必须在判定文件时
回溯各级父目录兜住。这条是 `git check-ignore` 对照测试第一次跑就报出来的，
不是靠想用例能覆盖到的。

**验证方式**（`ignore_test.go`）：把一批模式交给 `git check-ignore` 判定，
再断言我们得到同样的答案——当前 **14 个模式 × 40 条路径完全一致**。
另外确认了零回归：无 `**` 的普通仓库与 sourcepack 自身，文件清单与旧版逐条一致；
5000 文件语料扫描耗时不变（目录判定加了记忆化，深目录不会退化成 O(深度²)）。

结构上顺带把"什么会被排除"整块从 `main.go` 搬到 `ignore.go`：
`main.go` 1798 → 1487 行。

---

## 新增记录（2026-10-12 · v2.0.0）

主版本号 +1 的原因只有一条：**`-d` 的行为变了**——原先"静默成功"的输入现在会明确报错。

### 1. `-d` 指向非法路径改为明确报错（**行为变更**）

v1.9.0 及之前，`gx -d /nope/nope`（路径打错）或 `gx -d main.go`（指向文件）
会**成功**产出一份空快照：

```bash
$ gx -d /nope/nope -o /tmp/nodir.md ; echo $?
0                                   # ← 退出码 0，产物 185 字节，树里只有一个 "nope/"
```

用户会以为打包成功，直到把产物喂给模型才发现里面什么都没有。v2.0.0 起：

| 输入 | 结果 |
| :--- | :--- |
| `-d /nope/nope` | `❌ -d/--dir is not accessible: /nope/nope`，退出码 1 |
| `-d main.go` | `❌ -d/--dir must be a directory, but main.go is a file`，退出码 1 |
| 正常目录 | 不受影响 |

**受影响的用法**：脚本里若依赖"目录不存在也照样产出一份空文件"，升级后会失败——
这正是本次跳主版本的原因。

### 2. `-h` 里新增 Examples 示例段

常用的组合直接可抄，按五组划分（打包整个项目 / 只打包一部分 / 先看会打包什么 /
交给 AI 审查改动 / 分享投喂），并在末尾指向参数手册：

```bash
$ gx -h
...
Examples:
  # 打包整个项目（生成 project_snapshot.md）
  sourcepack

  # 只打包一部分
  sourcepack -d src -i go             只扫 src 下的 .go（-i 可逗号分隔、大小写不敏感）
  sourcepack -m handler -x test       路径含 handler，且扩展名不是 test
  sourcepack -I '**/*.log'            按 gitignore 语法排除日志（支持 ** 与 !）
  sourcepack -n                      只扫根目录，不进子目录

  # 先看会打包什么、有多大（都不写文件）
  sourcepack --dry-run                列出将被包含的文件清单
  sourcepack -s                       只打印统计与 token 估算

  # 交给 AI 审查「这次改了什么」
  sourcepack -g                       未提交改动（diff + 变更后全文）
  sourcepack -g --staged              只看已暂存的改动
  sourcepack -g --diff-only           只给 diff，不附全文（产物小十几倍）
  sourcepack -g --rev main            与 main 分支比较
  sourcepack -G 5 --no-merges         最近 5 个非合并提交

  # 分享 / 投喂
  sourcepack -o /tmp/snap.md          写到指定路径（建议写在项目外，避免自我污染）
  sourcepack -c                       复制到剪贴板
  sourcepack --icloud                 存到 iCloud Drive（手机可读）
  sourcepack -p -u https://host/submit  推送到远端（或用 SOURCEPACK_PUSH_URL）

完整参数说明与实测示例：见仓库里的 PARAMETERS.md
  https://github.com/yuanguangshan/sourcepack/blob/main/PARAMETERS.md
```

示例里的命令名跟随实际调用的名字，npm 全局安装后用 `gx -h` 也不会出现
`sourcepack-darwin-arm64` 这种内部二进制名。

> 上面这段是 v2.0.0 当时的真实输出；help 末尾的链接在 v2.0.1 换成了
> 参数手册的博客版（见下一节）。

### 3. 新增参数手册 [PARAMETERS.md](PARAMETERS.md)

29 个参数逐一实测（含真机输出）、忽略规则语法表与优先级、组合互斥、退出码表、
边界与已知问题、实测夹具附录。README 顶部加入口，同时打进 npm 包。

### 4. 其它

- 新增 `-h` 示例段的回归测试（`TestHelpContainsExamples`）；
- `-d` 校验的回归测试（`TestValidateConfigRejectsBadRootDir`）。

---

## 新增记录（2026-10-12 · v2.0.1）

只有一处用户可见变化：**`-h` 末尾的参考链接换成了参数手册的博客版**。

```diff
-完整参数说明与实测示例：见仓库里的 PARAMETERS.md
-  https://github.com/yuanguangshan/sourcepack/blob/main/PARAMETERS.md
+更多用法请参考：https://s.want.biz/sourcepack
+（完整参数手册也随包发布：PARAMETERS.md）
```

- 博客版渲染更好读（<https://s.want.biz/sourcepack>）；
- 保留 `PARAMETERS.md` 那行是为了**离线/内网**场景——它随 npm 包一起发布；
- 小提示：该短链**不带浏览器 UA 请求会被挡成 404**（`curl` 直接访问可能看不到内容），
  浏览器打开正常；
- 新增断言：`TestHelpContainsExamples` 现在要求这行链接存在。

---

## 修复记录（2026-10-13）：评审驱动的 9 项修复与 2 个新参数

来自一次外部代码评审，逐条**先在真机上复现/核实**再动手。

### 新参数

| 参数 | 作用 | 为什么 |
| :--- | :--- | :--- |
| **`--base <branch>`** | `-g` 时与**共同祖先**比较（`merge-base(base, HEAD)`） | `--rev main` 是双点语义，main 前进过会把**对方改动反向算进来**。实测同一分支：`--rev` = 3 文件 (+2 −2)，`--base` = 2 文件 (+2 −1)，与 `git diff main...HEAD` 逐条一致 |
| **`--first-parent`** | `-G` 时只看主线（`git log --first-parent`） | "这条主干推进了哪些点"的视角；产物 `Source:` 行会标注 |

`--base` 与 `--rev` **互斥**（同时给明确报错），因为两者口径不同，静默取一个会让产物语义不明。

### 修复（每项都有回归用例，关键项做了反向验证）

| # | 问题 | 影响 | 修法 |
| :--- | :--- | :--- | :--- |
| 1 | 剪贴板：`cmd.Wait()` 的错误被丢弃 | pbcopy/xclip/clip 真失败时仍打印 `✨ Done! Copied to clipboard`，用户拿到空剪贴板 | 检查退出码并返回错误（用"必定失败的假 pbcopy"做回归测试） |
| 2 | `--icloud` 在任何平台都 `MkdirAll` iCloud 路径 | Linux/Windows 上"成功"写进一棵假的 `~/Library/...`，用户以为同步到手机了 | 非 macOS 明确报错；macOS 上容器不存在也报错（不自建） |
| 3 | `uploadViaMultipart` 把整份产物读进 `bytes.Buffer` | 上游特意把大快照落临时文件以免占内存，这里又整份读回，收益被抹掉 | 用 `io.MultiReader` 拼接 multipart（头部 + 文件 + 结尾），**流式且保留 Content-Length**。实测 30MB 产物峰值 RSS **92.5MiB → 13.0MiB（约 1/7）**，服务端收到的字节数完全一致 |
| 4 | 推送把 `201/202/204` 当失败 | 服务端明明收下了，却报 `server returned 201` | 接受全部 2xx |
| 5 | `-X '*.go'` 静默无效 | `-m/-X` 是子串匹配，写通配符不排除任何东西 | 检测到通配符时提示一行（"用 -x go"） |
| 6 | `--max-size 0` 的语义 | 与 `--max-diff-lines 0`（不限）相反：它是"只剩空文件" | help 文本写明 `0 = only empty files, NOT "unlimited"` |
| 7 | `mdCode` 对含反引号的路径退化为裸文本 | 路径里的 `**`/`_`/`|` 会被渲染成加粗/斜体/表格分隔，破坏产物结构 | 改用 HTML 实体转义（`&#96;`/`&#42;`…），渲染仍是原样字符 |

### 评审里核实为"已正确"或"不成立"的项（未改动）

- 评审称 `-a` 的 help 写着"disable all ignore rules"、与 `.gdignore` 仍生效矛盾——
  实测 help 文本**已经写明**".gdignore still applies"，无需改；
- 评审称 `mdCell` 没处理 `|`——实测已有 `\|` 转义；
- 评审建议的 `--split-by-dir`、`--quiet`、全局 exclude、内容级密钥扫描、
  `optionalDependencies` 拆包等，均**记为待办**（`TODO.md`），本轮不做：
  它们是功能/架构级改动，与本轮"修真实缺陷"目标不同。

---

## 新增记录（2026-10-13 · v2.1.0）：书排版 `-book`

主版本号不动、次版本号 +1：**新增两个参数，默认行为一字未改**。

> 本版同时包含 `v2.0.1` 之后**尚未发布**的内容：`--base` / `--first-parent`
> 两个 git 参数与评审驱动的 9 项修复（见上一节）。

### 为什么需要它

默认产物是"给模型读"的：结构树 + 文件清单 + 围栏代码，密度优先。
但同一份快照拿给人读时有两个硬伤：

1. **没有"书"的结构**——只有一串 `## <路径>`，没有封面、目录、章节层次，长文件里
   想回看某个文件只能靠搜索；
2. **Markdown 文章被塞进代码块**——打包一个文档型项目（散文章节 + 代码）时，
   产物是"一堆被 ``` 包住的文章"，不是一本书。

### 用法

```bash
gx -book                              # → project_book.md（与 project_snapshot.md 互不覆盖）
gx -book -i md                        # 文档型项目：只收 md，成一本纯散文书
gx -book --book-title '《XXX》'         # 书名（默认用目录名）
gx -book --icloud                     # 存 iCloud（文件名带 +book，不覆盖默认快照）
```

单横线写法直接可用：`-book` 等价 `--book`。

### 产物结构

层级刻意与本仓库 `book/` 那本书的产物（`book/dist/BOOK.md`）对齐：
书名/部/附录 = H1，章 = H2，章内小节 = H3 起。

| 部分 | 内容 |
| :--- | :--- |
| 顶部 | HTML 注释一行（书名 · 文件数 · 版本 · 时间）——不渲染，便于溯源/grep |
| 封面 | 书名 + 一句话说明 + 「关于本书」表（部/章数、行数、体积、token、主要语言） |
| 目录 | 部不缩进、章缩进一级；每条都是显式 `<a id>` 锚点，点击可跳 |
| 正文 | 按顶层目录分「部」（根目录永远第一部，其余按目录名升序） |
| 附录 A | 目录结构树（含每文件行数） |
| 附录 B | 统计：总览 / 按目录 / 按语言（与 `-s` 共用同一套排序口径） |
| 附录 C | 打包参数 + **一条能复现这份书的命令**（`cd <目录> && sourcepack --book -i md …`） |

**Markdown 按正文排版**（与默认快照最实质的差别）：

- 章标题取该文件自己的第一个 H1（跳过 BOM、前置 HTML 注释、YAML front matter），
  没有 H1 就用文件路径；
- 正文标题整体下沉，使"最小的标题级别"落到 H3（章 = H2），阅读器大纲里
  "章 → 节"的从属关系才成立；只下沉不上浮，最深封顶 H6；
- **围栏代码块里的 `# 注释` 不会被当成标题**（书稿代码大量以 `#` 开头——
  实测某书稿 129 行以 `#` 开头，绝大部分在代码块里）；
- 其余文件保持围栏代码，围栏长度仍按正文里最长反引号串自动加长。

### 三个刻意的取舍

1. **不重新编号章节**。书稿自己写的 `第 1 章`、小节号 `1.1` 才是权威；
   工具另起一套编号会与正文里的交叉引用对不上（`book/tools/build.py` 里
   记着同样的教训：早期重编号破坏了全书 109 处引用）。因此章标题是
   "内容里的 H1 → 文件路径"二选一，不额外加序号。
2. **默认产物一字未动**。书排版是纯展示层：走同一批 `files/stats`，
   `-s`/`-c`/`-p`/`--icloud` 全部照旧可用。用 HEAD 单独建 worktree 编译对比过，
   `project_snapshot.md` 与 `-s` 报告逐字节一致（只差生成时间行）。
3. **`-g`/`-G` 明确报错**，不静默套书版式：变更集是给审查用的 diff，
   套上书版式只会得到一份四不像。同理 `--book-title` 不给 `--book` 也报错。

### 顺带修掉的一个老坑：单横线长参数

pflag 只把 `--book` 当长参数，`-book` 会被当成**短参数簇**：要么报
`unknown shorthand flag: 'b'`，要么更糟——静默解析成别的参数。实测旧版本：

| 你敲的 | 旧行为 | 现在 |
| :--- | :--- | :--- |
| `-book` | `unknown shorthand flag: 'b'` | = `--book` |
| `-dir` | `-d ir`（扫一个叫 `ir` 的目录） | = `--dir` |
| `-no-subdirs` | `-n -o subdirs`（顺手把输出名改成 subdirs） | = `--no-subdirs` |
| `-stats` | `unknown shorthand flag: 't'` | = `--stats` |
| `-dry-run` | `-d ry-run` | = `--dry-run` |

规则刻意保守：**只有"单横线 + 名字精确等于某个已注册长参数名"才改写**，
所以真正的短参数簇（`-sp`、`-ac`）与 `--` 之后的位置参数都不受影响。

### 验证

- 19 个新单测（`bookoutput_test.go`）：渲染结构、锚点唯一且目录链接全部可达、
  标题提取、标题下沉（含代码块内 `#` 不误伤）、分组编号、排序确定性、参数解析与互斥；
- 全量闸门：`gofmt -l` / `go vet` / `go test -race` / `node --test test/` 全绿；
- 真机端到端：在一份 8 篇 md、3,152 行的书稿上出 3,354 行产物，在
  **md.want.biz**（marked + highlight.js + DOMPurify）里验证：
  标题层级 H1×6 / H2×11 / H3×69 / H4×10、21 张表、60 个代码块、
  13 条目录链接**全部可达**、点目录能跳到目标、阅读器自带大纲层级正确；
- 长围栏压测（打包 sourcepack 自身，20,964 行）：代码块外 ` ``` ` 与
  `<a id=` 泄漏均为 0，最大代码块 59,078 字符（整个 `main.go`）仍是一整块——
  marked 正确支持 4 反引号围栏。

### 已知边界

- `-g`/`-G` 暂不支持书排版（见上）；
- 内联 md 里的**站内锚点链接**（`[第 5 节](#5-远端推送)`）不重写，
  能否跳转取决于阅读器的 slug 规则；实测源文档里本就有几条是坏的
  （如 `#附录-a实测夹具`），与书排版无关；
- setext 式标题（`===` 下划线）不参与下沉。

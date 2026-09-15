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
| `-a, --all` | 关闭**所有**忽略规则（=`--no-gitignore` + `--no-default-ignore`） | 全部文件都进来 |

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

### 参数优先级（实测确认）

| 同时给出 | 结果 |
|---|---|
| `-c` + `-p` | **`-c` 优先**，只复制到剪贴板，不推送 |
| `--icloud` + `-o` | **`--icloud` 优先**，`-o` 指定的文件不会生成 |
| `-i` + `-x` | 先按 `-i` 收窄，再按 `-x` 排除 |

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

## 特性

- **极速**: Go 编写，秒级处理万行代码。
- **智能**: 自动处理 `.gitignore` 和 `.gdignore`，支持 `!` 否定、`**` 通配等完整 gitignore 语法；自动跳过二进制文件。
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


---

## `-n` 修复记录（2026-09-15）

`-n / --no-subdirs` 曾经**完全失效**，已修复。

**症状**：加 `-n` 后子目录文件仍被扫入，与文档描述不符。

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

**修复**：走到目录分支时 `relPath != "."` 必然成立（根目录已在前面 return），
所以"任何目录"即"任何子目录"，直接跳过：

```go
if config.NoSubdirs {
    return filepath.SkipDir
}
```

**为什么长期没被发现**：`NoSubdirs` 在此之前**没有任何测试**覆盖
（唯一出现是 `streaming_test.go` 里设为 `false`）。现已补
`nosubdirs_test.go` 两个用例：`-n` 时只扫根目录、不设时扫全部，
两者互为对照，避免"一个都没扫到"也能通过。

**验证**：`go test ./...` 全绿；并用实际二进制端到端复验
（`-n` 只产出根目录文件，不带 `-n` 含全部嵌套文件）。

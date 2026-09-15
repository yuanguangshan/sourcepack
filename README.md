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

### 常用示例

```bash
gx -i go,md         # 只包含 Go 和 Markdown 文件
gx -x exe,bin       # 排除特定后缀
gx -X vendor        # 排除路径中含该关键字的文件
gx -I 'test/*'      # 追加 gitignore 风格忽略模式
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

## ⚠️ 已知问题：`-n` 未生效（2026-09-15 实测）

`-n / --no-subdirs` 的实际行为与文档不符：**它不会排除子目录文件**。

三次独立复验（每次用全新临时目录，避免上次的产物被扫进来）：

```
目录结构: README.md  src/a.go  top.md
$ gx -n          → 产出仍含 src/a.go   ❌
```

对照实验（其它过滤参数都正常）：

| 参数 | 是否生效 |
|---|---|
| `-i go`（只含 go） | ✅ 生效 |
| `-X src`（按关键字排除） | ✅ 生效 |
| `-I 'top.md'`（追加忽略） | ✅ 生效 |
| `--max-size 1`（大小上限） | ✅ 生效 |
| **`-n`（不扫子目录）** | ❌ **不生效** |

**原因**（`godoc.go:635`）：

```go
if config.NoSubdirs && strings.Contains(relPath, "/") {
    return filepath.SkipDir
}
```

判断用的是 `strings.Contains(relPath, "/")`，但 `filepath.Walk` 走到一级子目录
`src` 时，传入的 `relPath` 就是 `src`（本身不含 `/`），条件不成立，因此不会
`SkipDir`。`-d` 指定相对路径与默认 `-d .` 两种调用方式都复现。

**绕过办法**：用 `-X 子目录名` 或在 `.gdignore` 里写 `子目录/`。

这属于上游实现问题，尚未修复。

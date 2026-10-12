# 附录 D　延伸阅读与工具

> 这一份是"接下来去哪"的清单。
>
> **关于链接的一条硬规矩**：我只写我有把握的资源。凡是**没有把握的 URL，我一律不写**，
> 而是给出"名称 + 搜索关键词"——让你自己搜到的一定是真的，比我编一个看起来很像的
> 链接要有用。同理，**版本号与选项名在不同 Git 版本上会变**，用之前请以本机
> `git <cmd> --help` 的输出为准。

---

## D.1　官方文档与书籍

### 必读的三份官方材料

| 资源 | 是什么 | 怎么拿到 |
| :--- | :--- | :--- |
| **Pro Git**（Scott Chacon / Ben Straub） | Git 的"正经书"，从入门到内部机制都有；第 10 章讲内部原理，与本课程第 1–2 章重合度最高 | 书是免费开源的，搜 **`Pro Git book`**；中文版搜 **`Pro Git 中文版`** |
| **Git Reference（官方命令手册）** | 每条命令的权威定义，含全部选项与退出码 | 本机 `git help <cmd>` 或 `man git-<cmd>`；离线可用，永远与你的版本匹配 |
| **Git 源码里的 `Documentation/`** | 手册页的**源文件**（AsciiDoc），比网页版更早、更全 | 克隆 Git 官方仓库后看 `Documentation/` 目录；搜 **`git git source repository`** 找官方镜像地址 |

> **为什么把 `git help` 排第二**：本课程正文里反复出现"以 Git 自己为真值"
> （第 7.9.2 节的 `check-ignore` 对照测试、第 12.7 节的差分测试）。
> `git help` 就是那个"真值"的书面形式——它和你本机二进制是同一个版本，
> 不会像网上文章那样写着三年前的行为。

### 值得追的几类文章

| 来源 | 搜什么 | 为什么值得 |
| :--- | :--- | :--- |
| **GitHub Blog** | `github blog git` + 你的主题（如 `git force push`、`git shallow clone`） | 托管平台视角的实践文，尤其"大仓库怎么办"这一类 |
| **Git 官方 Release Notes** | 搜 **`git release notes`**，或本机看 Git 源码的 `Documentation/RelNotes/` | 新选项（如 `--force-with-lease` 的语义细化、`maintenance` 子命令）都从这里第一次出现 |
| **Git 邮件列表**（git@vger.kernel.org） | 搜 **`git mailing list archive`** | 想搞清"某个行为到底是设计还是 bug"时，这里是终审法院 |

---

## D.2　深入方向的论文与设计文档

这一节是给"想知道算法为什么这么写"的读者的。**这些都不是 Git 专属文档，而是它借鉴的理论。**

### 差分算法

| 主题 | 出处 | 与本课程的连接 |
| :--- | :--- | :--- |
| **Myers 差分算法**：*An O(ND) Difference Algorithm and Its Variations*（Eugene W. Myers, 1986） | 论文标题可直接搜到；也可搜 **`Myers diff algorithm paper`** | 第 6.2 节讲了它在做什么（编辑图最短路、O(ND)）；论文给你完整证明与"分而治之"的改进版 |
| **diff 算法的四种取舍** | Git 源码 `xdiff/` 目录，以及 `git diff --help` 里 `--diff-algorithm` 一节 | 第 6.10 节实测了 myers/minimal/patience/histogram 在重复块上的分歧 |
| **patience diff 的原始说明** | 搜 **`patience diff bram cohen`**（Bram Cohen 提出的思路） | 解释"为什么它拒绝在重复块内部配对"——第 6.10 节的实测现象 |

### 内容寻址与存储

| 主题 | 出处 | 与本课程的连接 |
| :--- | :--- | :--- |
| **内容寻址存储（content-addressable storage）** | 概念性主题，搜 **`content-addressable storage`**；对照 Git 的 `hash-object` 实现 | 第 1.4、2.2 节：键 = `类型 长度 NUL 内容` 的哈希 |
| **Rabin 指纹（Rabin fingerprint）与滚动哈希** | 搜 **`Rabin fingerprint rolling hash`**（Michael Rabin, 1981） | 理解"为什么能以低成本在内容里找相似块"——这正是 delta 压缩与 rsync 这类工具的基础思路 |
| **packfile 格式与 delta 设计** | Git 源码里的 `Documentation/gitformat-pack.txt`（搜 **`gitformat-pack`**） | 第 2.9、10.2 节的 `chain length`、`--depth`/`--window` 旋钮的官方定义 |
| **SHA-1 碰撞（SHAttered）** | 搜 **`SHAttered SHA-1 collision`**（CWI 与 Google 的联合项目） | 第 2.10 节讲的迁移背景；注意 Git 的哈希输入带类型头，不是裸 SHA-1 |

> **一句话提醒**：这些论文读起来比本文档累得多，收益是"**你不再需要相信任何人的解释**"。
> 如果你只是想用对 Git，第 6、10 章给的实测结论已经够了。

---

## D.3　工具清单

### Git 自带的诊断（不用装任何东西）

| 工具 | 干什么 | 什么时候用 |
| :--- | :--- | :--- |
| `git bugreport` | 生成一份含版本、配置、系统信息的缺陷报告模板 | 要给别人报 Git 的问题时先跑它，省掉一堆来回追问。**两个坑**：① 默认文件名只到**分钟**，同一分钟内重复跑会因文件已存在而失败（实测 exit 128），用 `-s <后缀>`（如 `-s a`、`-s b`）才能拿到不同文件名——注意 `-o <目录>` **只改位置不改名**，规避不了这个撞名；② 产出的 zip 会包含仓库结构信息，公开前先看一眼内容 |
| `git diagnose --mode=all` | 收集完整诊断包（含仓库结构统计）到一个 zip | 怀疑仓库本身有问题（对象损坏、性能异常）时；产物含仓库结构信息，**公开前先看一眼内容** |
| `GIT_TRACE=1` | 打印 Git 实际执行了哪些内部命令 | "它到底替我跑了什么"——最常见的第一次排查 |
| `GIT_TRACE_PACKET=1` | 打印传输协议往返（`want`/`have`） | 怀疑 fetch/push 传得太多或卡住时（第 8.7 节用它看协商） |
| `GIT_TRACE_PERFORMANCE=1` | 打印内部各相位耗时 | "到底慢在哪一段"（第 10.8 节的诊断三件套之一） |
| `git count-objects -v` | 松散/打包对象数、体积 | 判断"该不该 `gc`"的第一步 |

### 可视化与交互式客户端

| 工具 | 是什么 | 适合 |
| :--- | :--- | :--- |
| `gitk` | Git 自带的图形化历史浏览器（多数发行版随 Git 一起装） | 想看提交图的**真实连线**（比 `git log --graph` 的 ASCII 图直观） |
| **tig** | 终端里的 Git 浏览器（TUI） | 长期在终端工作、又想要可交互的 log/blame/diff |
| **lazygit** | 终端全功能 Git 客户端 | 想用键盘完成暂存、改写、分支操作 |
| 各编辑器的 Git 集成（VS Code / JetBrains 等） | 行级暂存、图形 diff | 日常提交；但**改写历史类操作**建议回命令行做，图形界面容易隐藏后果 |

> 安装方式请用你所在平台的包管理器（`brew install`、`apt install`、`scoop install` 等），
> 本附录不写具体安装命令——各平台的包名与源差异太大，写死了反而误导。

### 仓库体检与历史清理

| 工具 | 是什么 | 注意 |
| :--- | :--- | :--- |
| **git-sizer** | 扫描仓库并给出"哪一项指标异常"的报告（最大 blob、最深路径、最多 tree 条目……） | 判断"仓库为什么这么大/这么慢"的**第一件该做的事**，比盲目 `gc`/`repack` 有效 |
| **git-filter-repo** | 官方推荐的历史重写工具（取代 `git filter-branch`） | 第 9.9 节的"两把刀"之一；用前**必须**读它的说明，且务必先备份（`git clone --mirror`） |
| `git filter-branch` | 老工具，Git 已不推荐（运行时会自己打印警告） | 只在前者不可用时考虑；它会留下 `refs/original/*` 备份引用 |
| `git fsck` | 对象完整性与可达性校验 | 恢复的**第三层**（第 3.9、9.8 节）；`--lost-found` 会把吊对象写进 `.git/lost-found/` |

### 提交前检查与团队规范

| 工具 | 是什么 | 该放在哪一层 |
| :--- | :--- | :--- |
| **pre-commit** 框架 | 管理客户端 `pre-commit` 钩子的工具链（配置文件声明检查项，自动装到 `.git/hooks/`） | **便利层**——注意客户端钩子**可被 `--no-verify` 绕过**（第 11.1 节实测），团队规范不能只靠它 |
| **git-secrets** / **gitleaks** 类密钥扫描 | 阻止把密钥提交进仓库 | 必须在 **CI 或服务端钩子**（不可绕过）与本地钩子**双层**做 |
| `pre-commit` 框架 + CI 里的同一套检查 | 本地快反馈、CI 做真正闸门 | 这正是 sourcepack 的做法：本地不装钩子，闸门全放 CI（第 11.7 节） |

> **一条来自第 11 章的经验**：钩子的定位是"让你少犯错"，不是"保证没人犯错"。
> 真正绕不过去的检查只有三种位置——**CI**、**服务端钩子**、**托管平台的分支保护规则**。

### 传输与托管

| 工具/服务 | 说明 |
| :--- | :--- |
| 自建 Git 服务（如 Gitea、GitLab CE 等） | 想在内网跑完整托管时；它们都支持服务端钩子与分支保护 |
| 静态 Git 托管 + CI | 最常见的组合；注意"推 tag 触发发布"这类约定（第 11.4 节） |
| `git bundle` | **不需要任何服务**的传输方式：把整个仓库打成一个文件搬走（附录 B.6） |
| 本地路径 / `file://` 远端 | 学习传输机制的最佳方式——第 8 章全部实验都靠它，不需要网络 |

---

## D.4　怎么继续用 sourcepack 练手

本课程的所有结论都能在 **sourcepack** 这个真实项目里复现。它是本仓库，
不是一个"教学示例"——所以它有真实的历史、真实的坑、真实的发布链路。

### 先建一个隔离的实验室

**这是全书第一件要做的事，也是最重要的一条纪律**：破坏性实验绝不在你正在用的仓库上做。

```bash
git clone /path/to/sourcepack /tmp/git-lab
cd /tmp/git-lab
git config user.email lab@example.com
git config user.name Lab
```

后面的 `reset --hard`、`rebase`、`filter-branch`、造"丢失的提交"之类都在这里折腾，
`sourcepack` 本体毫发无损。

### Windows 读者的实验环境（书里命令是按 Unix 写的）

书里的实验与输出都在 **macOS + git 2.39.5** 上跑出来，所以管道命令是 Unix 风格
（`/tmp`、`sed`、`cat`、`printf`、`python3`）。Windows 上照着抄会在几处卡住，
但**不影响理解**——它们都是"怎么造实验现场"，不是 Git 本身的知识。建议：

| 书里写的 | Windows 上怎么办 |
| :--- | :--- |
| `/tmp/git-lab` | 用 `$env:TEMP\git-lab`（PowerShell）或 `/tmp/git-lab`（**Git Bash** 里可直接用） |
| `sed -i '' -e ...` | **Git Bash 里可用**（注意 macOS 的 BSD sed 与 GNU sed 语法略有差异：GNU 不要那个空 `''`）。或改用 `printf ... > 文件` |
| `printf 'x\n' \| git ...` | Git Bash 可用；PowerShell 里 `printf` 不存在，用 `'x' + "`n"` 或 `echo x` |
| `cat .git/HEAD` | PowerShell 用 `Get-Content .git/HEAD`；Git Bash 用 `cat` |
| `chmod +x hook` | 见下面第 11 章的钩子实验说明 |

**三条实用建议**：

1. **装 Git for Windows 时选带 Git Bash 的默认安装**，然后**在 Git Bash 里做实验**——
   书里 95% 的命令可以原样跑。
2. **行尾（CRLF）**：Windows 检出默认会把 LF 转成 CRLF，这会让第 11 章的钩子脚本
   报 `bad interpreter: /bin/sh^M`。仓库里若有 `.gitattributes`（本仓库就有一条
   `* text=auto eol=lf`），clone 后即是 LF；没有的话先跑
   `git config --global core.autocrlf input` 再重新 clone。
3. **钩子实验（第 11 章）需要可执行位**：Windows 的文件系统没有 POSIX 可执行位，
   `chmod +x .git/hooks/pre-commit` 可能无效。两个办法：用 Git Bash（它对钩子做了
   兼容处理），或直接写 `#!/bin/sh` 开头并让 Git 用 `sh` 调用——也可以用第 11 章
   的替代做法：**只读钩子源码、观察 CI 里的等效门禁**（`pre-commit` 拦截在
   CI 上是 `vet`/`gofmt`/`go test`，这部分的观察在 Windows 上完全一样）。

> 换个角度看：第 7 章讲的 `filepath.Match` 在 Windows 上让 `*` 跨过 `/`
> （`sourcepack` 真实踩过、被 CI 抓到），以及本附录提到的 CRLF 问题——
> **这两个坑本身就是"平台差异会咬人"的活教材**。你在 Windows 上遇到的麻烦，
> 恰好印证了书里反复强调的那件事：工具必须显式声明自己的假设。

### 课程正文在哪

| 位置 | 内容 |
| :--- | :--- |
| `git-course/README.md` | 课程索引：12 章的目录、各章一句话要点、进度表 |
| `git-course/00-preface.md` | 前言：动机、为什么拿 sourcepack 当标本、三条方法论 |
| `git-course/01-…` 到 `12-…` | 12 章正文，每章末尾都有「动手实验」与「自测题」 |
| `git-course/published-urls.md` | 各章的博客版链接 |

### 把课程和代码对照着读

这是本书区别于"纯教程"的地方——**每个 Git 概念都能在本仓库的代码里找到落点**：

| 想验证的 Git 概念 | 去看哪 |
| :--- | :--- |
| 空树对象 `4b825dc...` 的用途 | `changeset.go` 里的 `emptyTreeSHA` 常量与它的调用点 |
| 分支名 / detached / unborn 三段回退 | `changeset.go` 里 `symbolic-ref` → `rev-parse` 的那段 |
| 忽略规则的完整语义实现 | `ignore.go`，以及 `ignore_test.go` 里那张"模式 × 路径"对照表 |
| 只读命令清单与 hardening | `changeset.go` 里的 `gitRunner`（逻辑集中在 `command()`） |
| 发布链路与闸门 | `.github/workflows/release.yml` 与 `.goreleaser.yaml` |
| 12 条"读 Git"的 checklist | 第 12.9 节，逐条都能在代码里指认现场 |

### 三个可以立刻做的小实验

1. **把第 7 章的对照测试跑起来**：读 `ignore_test.go` 里的 `oraclePatterns`，
   自己加一个模式，看它是否与 `git check-ignore` 一致——这是"以 Git 为真值"的最短路径。
2. **用 `GIT_TRACE` 看 sourcepack 到底调了什么**：
   `GIT_TRACE=1 ./sourcepack -g -d .`，把输出与第 12.3 节的命令清单对照。
3. **在自己的仓库里跑一次体检**：`git count-objects -v` + `git fsck --lost-found`，
   看看有没有你从不知道的吊对象——它们可能正是你"以为丢了"的提交。

> **最后一点建议**：别只读。这套课里所有"恍然大悟"的时刻，
> 都发生在你敲下回车、拿到**和书上不一样**的输出、然后去查为什么的时候。

---

## D.5　继续深挖的四个方向

如果你读完这本书还想往下走，下面四条是自然的延伸路线（按投入产出比排序）：

| 方向 | 起始点 | 大致路径 |
| :--- | :--- | :--- |
| **1. 把 Git 当数据库用** | 第 2、12 章 | 只用 plumbing 命令（`hash-object`/`mktree`/`commit-tree`/`update-ref`）实现一个玩具版本管理；你会彻底理解"对象 + 引用"的最小内核 |
| **2. 写一个"读 Git"的工具** | 第 12.9 节的 12 条 checklist | 本课程本身就是从"一个真实工具怎么正确读 Git"倒推出来的；照着 checklist 写，能避开我们踩过的所有坑 |
| **3. 大仓库与大传输** | 第 10 章 | partial clone / sparse checkout / commit-graph / bitmap / multi-pack-index；建议先在一个自己造的大仓库上量一遍（第 10.9 节给了 `fast-import` 造数据的方法） |
| **4. 协作流程与发布工程** | 第 8、11 章 | refspec 与保护规则、发布闸门与冒烟、签名 tag 与供应链安全；这是"团队级 Git"的主战场 |

> **一个判断标准**：什么时候说明你真的懂了？
> 当你看到一条没见过的 `git` 命令，能先猜出"它动了三棵树里的哪几棵、会不会新增对象、
> 会不会移动引用"，然后**用 `git reflog`/`git fsck` 去验证你的猜测**——
> 那时候你就不再需要这本速查了。

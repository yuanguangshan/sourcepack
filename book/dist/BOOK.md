<!-- Git 深入课：以 sourcepack 为标本 v1.0 （由 book/tools/build.py 生成，正文源：git-course/） -->

<a id="book-title"></a>

# STUB front-00-title

占位内容，等待真实文件。


## 目录

- [STUB front-10-preface](#stub-front-10-preface)
- [STUB front-20-copyright](#stub-front-20-copyright)
- [第一部　地基](#第一部地基)
- [第 1 章　起源与设计哲学](#第-1-章起源与设计哲学)
- [第 2 章　对象模型](#第-2-章对象模型)
- [第二部　日常操作的真相](#第二部日常操作的真相)
- [第 3 章　引用、HEAD 与 reflog](#第-3-章引用head-与-reflog)
- [第 4 章　索引（暂存区）](#第-4-章索引暂存区)
- [第 5 章　提交图与分支合并](#第-5-章提交图与分支合并)
- [第三部　格式与传输](#第三部格式与传输)
- [第 6 章　diff 算法与 patch 格式](#第-6-章diff-算法与-patch-格式)
- [第 7 章　忽略规则](#第-7-章忽略规则)
- [第 8 章　远端与传输](#第-8-章远端与传输)
- [第四部　改写、性能与工程](#第四部改写性能与工程)
- [第 9 章　历史改写与恢复](#第-9-章历史改写与恢复)
- [第 10 章　性能与底层](#第-10-章性能与底层)
- [第五部　实战](#第五部实战)
- [第 11 章　钩子、签名与发布](#第-11-章钩子签名与发布)
- [第 12 章　sourcepack 的 Git 用法全解（结课）](#第-12-章sourcepack-的-git-用法全解结课)
- [STUB back-10-appendix-a-glossary](#stub-back-10-appendix-a-glossary)
- [STUB back-20-appendix-b-commands](#stub-back-20-appendix-b-commands)
- [STUB back-30-appendix-c-map](#stub-back-30-appendix-c-map)
- [STUB back-40-appendix-d-reading](#stub-back-40-appendix-d-reading)

---

<a id="stub-front-10-preface"></a>

# STUB front-10-preface

占位内容，等待真实文件。

---

<a id="stub-front-20-copyright"></a>

# STUB front-20-copyright

占位内容，等待真实文件。

---

<a id="第一部地基"></a>

# 第一部　地基

> 先把 Git 的世界观看清楚：它存什么、怎么定位内容、为什么不可变。

---

<a id="第-1-章起源与设计哲学"></a>

# 第 1 章　起源与设计哲学

---

> **本章目标**：理解 Git 的"世界观"。学完你应该能回答：
> 为什么 Git 存快照而不是差异？为什么说它是个"内容寻址的文件系统"？
> 为什么"改历史"要造新对象？以及——为什么 sourcepack 选择调用 git CLI 而不是引库。
>
> 本章不需要背命令，只需要跟着跑几条，看清 Git 的骨架。

---

### 1.1 起源：一个被逼出来的工具（2005）

要理解 Git 的设计，先看它诞生时被什么逼出来的。

| 时间 | 事件 |
| :--- | :--- |
| 1991–2002 | Linux 内核用 tarball + patch 管理代码，靠 Linus 手工合并 |
| 2002 | 内核改用 **BitKeeper**（专有软件，作者 Larry McVoy 给内核免费授权） |
| 2005-04 | Andrew Tridgell 逆向 BitKeeper 的协议，McVoy 撤销免费授权 |
| 2005-04-03 | Linus 决定自己写，目标是"两周内能用" |
| 2005-04-07 | **Git 自举**：Git 的第一个提交由 Git 自己管理（`git` 项目用 `git` 开发） |
| 2005-07 | 维护权交给 Junio Hamano（至今） |

Linus 当时提的设计要求（原话大意）：

1. **速度**——内核是几十万文件、上千贡献者，操作必须快；
2. **简单设计**——不要 SVN 那种复杂模型；
3. **强力支持非线性开发**——内核同时有几百个分支，分支/合并必须廉价；
4. **完全分布式**——没有"中心服务器"这个概念上的特殊性。

这四条决定了后面所有设计。注意第 3、4 条：**BitKeeper 事件的核心教训不是
"专有软件不可靠"，而是"代码托管的基础设施不能依赖别人"**。

---

### 1.2 集中式 vs 分布式：本质区别

| | CVS / SVN（集中式） | Git（分布式） |
| :--- | :--- | :--- |
| 历史存在哪 | **只有服务器有** | **每个 clone 都有一份完整历史** |
| `commit` | 需要联网，写服务器 | 纯本地操作 |
| `log` / `diff` / `blame` | 通常要联网 | 本地，离线可用 |
| 分支 | 目录拷贝级别的重操作 | 一个 41 字节的文件 |
| 单点故障 | 服务器挂了就停摆 | 任何 clone 都能成为新的中心 |

**在标本仓库上验证"每个克隆都有完整历史"** `[lab]`：

```bash
$ git clone -q /Users/ygs/ygs/sourcepack /tmp/git-lab && cd /tmp/git-lab
$ git log --oneline | wc -l
135
$ git log --oneline -1
aa686d8 docs: v2.0.1 记录（-h 末尾换成参数手册博客版）；还原 v2.0.0 的 help 引用为当时真实输出
```

**拔网线也一样能跑**——`git log`、`git diff`、`git blame` 全部是读本地
`.git/objects`，不需要任何网络。这就是"分布式"的实际含义。

> **回到 sourcepack**：正因为历史在本地，我们的 `-g`/`-G` 才能**纯离线**工作——
> 它们只是读本地对象库并渲染成 Markdown，从不联网（除了你显式用 `-p` 推送）。

---

### 1.3 快照 vs 差异：Git 的世界观

这是 Git 与 SVN 最根本的分歧：

- **SVN/CVS 存差异**：每个版本记录"相对上一版改了哪些行"；
- **Git 存快照**：每次提交记录**整个项目在那一刻的完整状态**（没变的文件复用旧对象，
  所以并不浪费空间）。

**证据**：`HEAD` 不是"一串改动"，而是一棵**完整的目录树** `[lab]`：

```bash
$ git ls-tree HEAD
100644 blob c9f80462afe787371af06c0ae9a94e152dc469b8	.gitattributes
040000 tree ce6bcbe6b33fc754f22c8998b92183e3f3e3ee11	.github
100644 blob 4f82a9f9c613930094fdaebe86b62cfb48d5080b	.gitignore
100644 blob d84a0514f3c4c8620005b064206a73137dd9d93f	.goreleaser.yaml
100644 blob 3ff5447b8587158fc6a9f40b45bd262be7b17c3f	LICENSE
100644 blob 0f41aa62a8877263ff864c64f666bf0d5598a018	PARAMETERS.md
100644 blob 32c7dff26af3fa0745103d0104d0c8f58b3971f2	README.md
100644 blob 6c494d32d34a6977bda1dc5bd55a27cfc6010218	TODO.md
100644 blob fcf78f202b4468b2ed6fbac832d72cb179e4e5f5	changeoutput.go
100644 blob 94d69f96decb1bc87805b5cb72e6e36e4e63f755	changeset.go
...（共 26 个条目）
```

注意每一行是 `模式 类型 哈希 名字`——**没有"上一版是什么"的信息**。

**那 diff 是怎么来的？用时算的。** Git 保存了两棵完整的树，要看差异就现算：

```bash
$ git diff --stat HEAD~1 HEAD | tail -3
 PARAMETERS.md | 12 ++++++++++--
 README.md     | 25 +++++++++++++++++++++++++
 2 files changed, 35 insertions(+), 2 deletions(-)
```

**为什么这个选择是对的**：

1. **读快**：拿到某次提交的完整状态 = 一次树遍历，不需要从第一版开始重放所有差异
   （SVN 的 `checkout` 慢就是因为要重放）；
2. **合并简单**：合并是"对两棵完整树求三方合并"，而不是对两串差异做补丁套补丁；
3. **内容寻址天然去重**：没改的文件内容一样 → 哈希一样 → 天然共享同一对象。

> **回到 sourcepack**：我们的 `-G` 产物**同时给 diff 和变更文件的全文**，
> 正是因为这两种视图各有用途——diff 说明"改了什么"，快照说明"现在是什么"。
> 缺了全文，模型看不到上下文；缺了 diff，模型不知道改动范围。
> 这不是偷懒，是把 Git 的两种视图都利用起来。

---

### 1.4 内容寻址与不可变性

Git 的键不是"文件名"或"版本号"，而是**内容本身的 SHA-1 哈希**。

**实测：同一内容永远得到同一个哈希** `[lab]`：

```bash
$ printf 'hello\n' | git hash-object --stdin
ce013625030ba8dba906f756967f9e9ca394464a
$ printf 'hello' | git hash-object --stdin          # 少一个换行
b6fc4c620b67d95f953a5c1c1230aaab5db5a1b0
$ printf 'hello\n' | git hash-object --stdin        # 再算一次
ce013625030ba8dba906f756967f9e9ca394464a
```

**这一个字节的差别就是整个 Git 安全模型的基础**：

- **去重免费**：项目里两份完全相同的文件只存一份；
- **完整性可校验**：`git fsck` 能发现对象被篡改（哈希对不上）；
- **不可变**：对象一旦写入就永不修改。所谓"改历史"（rebase、commit --amend、
  filter-branch）**本质是造一批新对象，然后把引用指过去**——旧对象仍然存在，
  这正是第 9 章能"找回丢失提交"的原因。

**链条传递性** `[lab]`：

```bash
$ git rev-parse HEAD
aa686d8e14f8fc729a32a30d42dbf32c0b94b29f
$ git rev-parse HEAD^{tree}
ca33473e4e00110896dcdc9b4a56fd1c2dd76d02
```

commit 对象的哈希包含它的 tree 哈希，tree 的哈希包含所有 blob 哈希……
所以**改动任意一个字节，从该文件到 HEAD 的所有哈希都会变**。历史不可伪造。

---

### 1.5 三个核心抽象

Git 内部只有三块东西（记住这三块，后面所有命令都能想通）：

| 抽象 | 位置 | 特性 | 谁在改它 |
| :--- | :--- | :--- | :--- |
| **对象库** | `.git/objects/` | 内容寻址、**不可变**、只追加 | `add` / `commit` / `fetch` 写入新对象 |
| **引用** | `.git/refs/`、`.git/packed-refs` | **可变**的"指针"（41 字节） | `commit` / `reset` / `branch` 移动指针 |
| **索引** | `.git/index` | 下一次提交的候选快照（二进制格式） | `add` / `rm` / `mv` 更新 |

**实测：这就是整个"数据库"** `[lab]`：

```bash
$ ls -1 .git
HEAD
config
description
hooks
index
info
logs
objects
packed-refs
refs
```

```bash
$ git count-objects -v | grep -E "^count|in-pack|packs|size-pack"
count: 492          # 松散对象（loose）个数
in-pack: 170        # 打包对象个数
packs: 1            # packfile 个数
size-pack: 25874    # packfile 大小（KB）
```

- `objects/` 里既有**松散对象**（每次 `commit` 新写的，一对象一文件、zlib 压缩），
  也有**打包文件**（`git gc` 把大量对象压进一个 `.pack`，还做 delta 压缩）；
- `refs/heads/` 里就是分支——第 3 章会看到它真的只是一个装着哈希的文本文件；
- `index` 是二进制文件，第 4 章专门拆。

> **回到 sourcepack**：我们**从不写**这三块里的任何东西——`-g`/`-G` 只发
> `git diff` / `git log` / `git ls-files` 这类**只读**命令（见 `changeset.go`
> 里的 `gitRunner`）。所以这个工具可以在任何仓库里安全运行，不会留下副作用，
> 也不需要备份提示。

---

### 1.6 为什么 sourcepack 调用 git CLI，而不是引入 go-git

这是本课程第一个"设计哲学 → 工程决策"的映射。我们的理由（README 里也写了）：

| 考虑 | 结论 |
| :--- | :--- |
| 二进制体积 | go-git 会让产物膨胀数 MB，而我们追求"装完即用的小工具" |
| **行为一致性** | `core.autocrlf`、`filter`、`.gitattributes`、用户级 `diff.external`……这些只有**真 git** 才会正确处理；自己实现一套必然在某些仓库上与 git 不一致 |
| 报错可解释 | 报错就是 git 自己的报错，用户能直接搜到答案 |
| 代价 | 每次调用要 fork 一个进程（约 11ms，实测），所以我们的 `-g` 一次要 spawn 约 8 个 git 进程 |

**代价的量化**（这是第 8 章会展开的优化方向）：

```
单次 git 进程 spawn ≈ 11 ms × 8 次 ≈ 90 ms
git diff 产出 patch 本身 ≈ 60 ms
sp -g 全流程 ≈ 210 ms（500 个文件变更时）
```

也就是说 `-g` 里有近一半时间是"启动 git 进程"本身。这就是为什么我们坚持
"文件清单一律用 `-z` 一次拿全"、"能合并的调用不拆开"。

---

### 1.7 动手实验 `[lab]`

```bash
# 0) 建实验克隆（如果你还没建）
git clone -q /Users/ygs/ygs/sourcepack /tmp/git-lab && cd /tmp/git-lab
git config user.email lab@example.com && git config user.name Lab

# 1) 看清 .git 的骨架
ls -1 .git
git count-objects -v

# 2) 快照而非差异
git ls-tree HEAD | head          # 完整目录树
git ls-tree -r HEAD | wc -l      # 仓库里的文件条目数

# 3) 内容寻址：同内容 → 同哈希
printf 'hello\n' | git hash-object --stdin
printf 'hello'   | git hash-object --stdin

# 4) 不可变性：HEAD 的哈希由哪些东西决定
git rev-parse HEAD
git rev-parse HEAD^{tree}
git cat-file -p HEAD | head -3

# 5) 分布式：断网也能查历史
git log --oneline | wc -l
git log -1 --format='%H%n%an%n%ad%n%s'
```

#### 自测题（答案在第 2、3 章）

1. `git hash-object` 算哈希时，哈希的输入里除了文件内容还有什么？（提示：`blob 6\0`）
2. 如果 Git 存的是快照，为什么改一个文件不会让仓库体积翻倍？
3. `git commit --amend` 之后，旧提交还在磁盘上吗？为什么？

---

### 1.8 小结

- Git 是为**内核级规模**设计的：速度、廉价分支、完全分布式；
- 它存**快照**（完整树）而不是差异；diff 是用时算的；
- 键是**内容的 SHA-1**，对象**不可变**，改历史 = 造新对象 + 移动引用；
- 内部只有三块：**对象库**（不可变）、**引用**（可变指针）、**索引**（候选快照）；
- sourcepack 选择"调 git CLI + 只读操作"，换来的是行为一致性与可解释的报错，
  代价是进程启动开销（第 8 章量化优化）。

**下一章**：把对象库拆开——看 blob/tree/commit 的真实字节、哈希的计算过程，
以及 `changeset.go` 里那个空树常量 `4b825dc6...` 的来历。

---

<a id="第-2-章对象模型"></a>

# 第 2 章　对象模型

---

> **本章目标**：把 `.git/objects` 拆开看清它的字节。学完你应该能回答：
> blob/tree/commit 的哈希到底是怎么算的？磁盘上那个 21 字节的文件里是什么？
> `git diff` 面对一个"没有父提交的根提交"时拿什么当基准？（答案直接对应
> `changeset.go` 里的一个常量。）
>
> 本章是"设计哲学 → 具体实现"的转折：所有概念都开始有字节级的证据。

---

### 2.1 四种对象

Git 的对象库只有四种对象：

| 类型 | 内容 | 类比 |
| :--- | :--- | :--- |
| **blob** | 文件内容，**不含文件名** | 一块字节 |
| **tree** | 目录列表（`模式 类型 哈希\t名字`） | 一个目录项 |
| **commit** | 指向一棵 tree + 父提交 + 作者/提交者/时间/信息 | 一次快照的元数据 |
| **tag** | 指向一个对象（通常是 commit）+ 标签信息 | 一个有名字的固定指针 |

**实测四种对象** `[lab]`：

```bash
$ git cat-file -t HEAD
commit
$ git cat-file -t HEAD^{tree}
tree
$ git cat-file -t HEAD:README.md
blob
```

> **关键设计**：**blob 里没有文件名**。名字只存在于 tree 的条目里。
> 这带来两个后果：① 两份内容相同的文件只存一份 blob（去重免费）；
> ② **重命名不改变 blob**——第 2.8 节用我们自己的改名历史证明。

---

### 2.2 哈希是怎么算的：动手复现

Git 的哈希输入不是"文件内容"，而是：

```
<类型> <空格> <内容的字节数> <NUL 字节> <内容>
```

**实测：用 Python 按这个格式复现，结果与 git 完全一致** `[lab]`：

```bash
$ printf 'hello\n' | git hash-object --stdin
ce013625030ba8dba906f756967f9e9ca394464a
```

```python
import hashlib
data = b'hello\n'
hdr  = b'blob %d\x00' % len(data)      # ← 关键：类型头 + NUL
print('待哈希字节: %r' % (hdr + data))  # b'blob 6\x00hello\n'
print('SHA-1 =', hashlib.sha1(hdr + data).hexdigest())
```

输出：

```
待哈希字节: b'blob 6\x00hello\n'
SHA-1 = ce013625030ba8dba906f756967f9e9ca394464a
git 给的 = ce013625030ba8dba906f756967f9e9ca394464a
```

**逐字相同。** 所以"内容寻址"的准确表述是：**对（类型+长度+内容）这件事寻址**。
这意味着不同内容不可能碰撞出同一 ID（SHA-1 意义上），而同一内容必然同一 ID。

**一个字节的威力**：

```bash
$ printf 'hello\n' | git hash-object --stdin   # ce013625...
$ printf 'hello'   | git hash-object --stdin   # b6fc4c62...
```

少了末尾一个换行 = 完全不同的对象。这正是 Git 检测损坏、保证历史不可篡改的基础。

---

### 2.3 磁盘上的对象：zlib 压缩 + 目录分片

对象文件名是"前 2 位十六进制 / 后 38 位"：

```bash
$ printf 'hello\n' | git hash-object -w --stdin
$ ls -l .git/objects/ce/013625030ba8dba906f756967f9e9ca394464a
.git/objects/ce/013625030ba8dba906f756967f9e9ca394464a (21 字节)
21 字节怎么装下 6 字节内容？—— zlib 压缩，且头也是压缩的一部分。
```

**用 Python 解压看看**：

```python
import zlib
raw = open('.git/objects/ce/013625030ba8dba906f756967f9e9ca394464a','rb').read()
print('前 2 字节(魔数):', raw[:2].hex())
print('解压后:', zlib.decompress(raw))
```

输出：

```
前 2 字节(魔数): 7801        # zlib 默认压缩级别的魔数
解压后: b'blob 6\x00hello\n'  # ← 与 2.2 节里"待哈希字节"逐字节相同
```

**到这里闭环了**：`git hash-object` 算的就是那串字节的 SHA-1；磁盘上存的是
同一串字节的 zlib 压缩。所以对象库既是内容寻址的，也是自校验的。

**为什么目录名只取前 2 位**：Git 用前 2 位把对象分到 256 个目录里，避免单目录
放几十万个文件（文件系统性能）——和"用文件夹而不是一个巨型文件夹管理下载"同理。

---

### 2.4 commit 对象：字段与"信息是怎么变成摘要的"

```bash
$ git cat-file -p HEAD
tree ca33473e4e00110896dcdc9b4a56fd1c2dd76d02
parent 196c9a76126e549ee8afd7d0bc9f2d6e731e3e4d
author yuanguangshan <yuanguangshan@gmail.com> 1791813823 +0800
committer yuanguangshan <yuanguangshan@gmail.com> 1791813823 +0800

docs: v2.0.1 记录（-h 末尾换成参数手册博客版）；还原 v2.0.0 的 help 引用为当时真实输出
```

字段含义：

| 字段 | 含义 |
| :--- | :--- |
| `tree` | 这次提交对应的**完整目录树**（第 1.3 节的快照） |
| `parent` | 父提交哈希；可以有**多个**（合并提交就有两条父边） |
| `author` | 谁写的这个名字/邮箱 + **时间戳 + 时区** |
| `committer` | 谁提交的（rebase 时代码内容不变，author 保留，committer 变） |
| 空行后 | 提交信息（标题 + 正文） |

> **回到 sourcepack**：我们 `-G` 产物里那三行
> `| <短哈希> | <日期> | 作者 | 标题|` 正是直接读这些字段。
> 注意我们**只取标题，不取正文**（`%s`）——"为什么这样改"通常写在正文里，
> 这是 `PARAMETERS.md` 第 10 节记的一个已知缺口。要补的话只需把
> `git log --format=%s` 换成 `%s%n%b`。

---

### 2.5 tree 对象与文件模式

```bash
$ git cat-file -p HEAD^{tree} | head -3
100644 blob c9f80462afe787371af06c0ae9a94e152dc469b8	.gitattributes
040000 tree ce6bcbe6b33fc754f22c8998b92183e3f3e3ee11	.github
100755 blob 121a1f65d0818c8a3d4402b7423912d529c0b9a8	index.js
```

| 模式 | 含义 |
| :--- | :--- |
| `100644` | 普通文件（rw-r--r--） |
| `100755` | **可执行文件**（注意 `index.js` 与 `install.sh` 是 755，因为 npm/CI 要直接执行） |
| `120000` | 符号链接（内容 = 目标路径字符串） |
| `040000` | 子目录（下一棵 tree） |
| `160000` | **submodule**（指向另一个仓库的提交） |

**一个常见坑**：Windows 上 `core.filemode` 若设置不当，模式位会"变"，
Enterprise 里表现为"改了个文件，diff 里却整文件变化"。sourcepack 不依赖文件模式
（它读内容、不判可执行位），所以不受影响。

---

### 2.6 空树对象：`4b825dc...` 从哪来

现在回答本章埋的问题——**`changeset.go:44` 那个常量是什么**：

```go
	// emptyTreeSHA 是 Git 里那个众所周知的空树对象，用于给"没有父提交"
	// 的根提交做 diff 基准
	emptyTreeSHA = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
```

**它可以在任何 Git 仓库里复现** `[lab]`：

```bash
$ git hash-object -t tree /dev/null
4b825dc642cb6eb9a060e54bf8d69288fbee4904
```

**逐字节匹配。** 原理：一棵"没有条目的树"字节串是空的，于是
`tree 0\0` 的 SHA-1 就是这个值——**所有 Git 仓库都一样**，所以可以硬编码。

**它解决什么实际问题**：仓库的**第一个提交没有父提交**，
`git diff HEAD~1 HEAD` 这种"相对父提交"的思路对它失效。sourcepack 的做法是
把基准换成空树——"相对空树"的 diff 就是"全部都是新增"：

```bash
$ ROOT=$(git rev-list --max-parents=0 HEAD); echo $ROOT
ee1b393                              # A tool to conveniently generate documents for your project!
$ git diff --stat 4b825dc642cb6eb9a060e54bf8d69288fbee4904 $ROOT
 install-gen-docs.sh |  77 ++++++
 3 files changed, 799 insertions(+)
```

这正是 `changeset.go:319` 那行 `base = emptyTreeSHA` 在做的事：
**根提交 vs 空树 = 整个项目的初始内容**。

> **顺带**：这也是 Git 的"基线树"思想的一个缩影——任何两个 tree 都能 diff，
> 无论它们有没有 commit 关系。这是第 6 章会展开的重点。

---

### 2.7 手工造一个提交（只用 plumbing）

Git 命令分两类：

- **瓷器命令（porcelain）**：`add` / `commit` / `branch`，给人用的；
- **管道命令（plumbing）**：`hash-object` / `mktree` / `commit-tree`，
  低层原语，给脚本用的。

**用 plumbing 造一个提交，全程不碰工作区** `[lab]`：

```bash
$ BLOB=$(printf 'made by hand\n' | git hash-object -w --stdin)
$ TREE=$(printf '100644 blob %s\thandmade.txt\n' "$BLOB" | git mktree)
$ COMMIT=$(git commit-tree "$TREE" -m "手工提交：只用 plumbing 命令")
$ git cat-file -p "$COMMIT"
tree 8e36a72351c9b9eead9d1bed05962ea3ebf95829
author Lab <lab@example.com> 1791819499 +0800
committer Lab <lab@example.com> 1791819499 +0800

手工提交：只用 plumbing 命令
$ git show --stat --oneline "$COMMIT"
7f41af6 手工提交：只用 plumbing 命令
 handmade.txt | 1 +
 1 file changed, 1 insertion(+)
```

**观察**：
1. commit 里**没有 `parent` 行**——它是个"根提交"；
2. 我们造出了完整的 `tree → commit` 链条，但**没有任何分支指向它**
   （第 3 章会说：这就是一次"丢失的提交"，只能靠 reflog 或哈希找到）；
3. `git show` 能读它，因为对象库是内容寻址的——只要有哈希，就能取到东西。

---

### 2.8 blob 不含文件名的两个后果

#### 后果一：重命名不改变 blob

**用 sourcepack 自己的改名历史当证据**（这个仓库三次改名）`[lab]`：

```bash
$ git log --oneline --name-status -M --follow -- main.go | grep '^R'
R093	godoc.go	main.go
R098	gdox.go	godoc.go
R099	gen-docs.go	gdox.go
```

`R093` 表示"相似度 93%，`godoc.go` → `main.go`"。
**这三次改名都没有产生新的 blob**——文件内容几乎没变，变的只是 tree 条目里的名字。
`99%` / `98%` 的相似度说明那两次改名夹带了极小的调整。

> 这解释了为什么 `git diff` 默认**不显示重命名**：它必须对比两个 blob，
> 发现内容相似度超过阈值（默认 50%）才判定为 rename。sourcepack 的
> `git mv`/rename 支持靠的正是 `-M` 参数（见 `changeset.go:337`）：
> `diff --no-ext-diff --no-textconv --no-color -M --src-prefix=a/ --dst-prefix=b/`。

#### 后果二：内容相同的两个文件只存一份

```bash
$ git hash-object README.md
32c7dff26af3fa0745103d0104d0c8f58b3971f2
$ git ls-tree HEAD README.md | awk '{print $3}'
32c7dff26af3fa0745103d0104d0c8f58b3971f2
```

工作区文件的哈希 == 对象库里那个 blob 的哈希。**这就是 `git status` 的原理**
（第 4 章详细拆）：比较工作区文件、索引、HEAD 三方的哈希。

---

### 2.9 松散对象 vs packfile：`count-objects -v` 告诉我们什么

```bash
$ git count-objects -v
count: 492          # 松散对象（loose，一对象一文件）
in-pack: 170        # 被打包进 .pack 的对象
packs: 1            # packfile 个数
size-pack: 25874    # packfile 总大小（KB）
$ git verify-pack -v .git/objects/pack/*.idx | tail -2
chain length = 4: 1 object          # ← 最长 delta 链 = 4
.git/objects/pack/pack-2e4208e2...pack: ok
```

- 新写的对象先是**松散对象**（zlib 压缩）；
- `git gc` / `git gc --auto`（默认约 6700 个松散对象触发）会把它们压进 packfile，
  并做 **delta 压缩**——`chain length = 4` 意思是"最长 4 个对象串成一条 delta 链"；
- **这是 Git 体积小的真正原因**：快照模型 + delta 压缩 = 比"纯存快照"省得多，
  比"纯存差异"读得快。第 10 章会展开 delta 与 bitmap。

> **回到 sourcepack**：我们**不需要**关心对象是松散还是打包——
> `cat-file` / `diff` 透明处理。这也是"调 CLI"的一个隐性好处。

---

### 2.10 SHA-1 与 SHA-256：一个正在发生的迁移

| 事实 | 说明 |
| :--- | :--- |
| SHA-1 碰撞已被公开演示 | SHAttered（2017）用 6500 CPU 年造出一次碰撞 |
| Git 的 SHA-1 **不是**直接哈希内容 | 先拼 `类型+长度+NUL` 头，且常用工具会用空缩进/空白规范——比裸哈希难攻击 |
| 迁移已就绪 | `git init --object-format=sha256`，Git ≥ 2.29 支持 |
| 现状 | SHA-1 仍是绝对主流；新项目可选 sha256 |

**实测新建一个 sha256 实验仓库** `[lab]`（只验证机制，不影响主仓库）：

```bash
$ git init --object-format=sha256 -q /tmp/sha256-lab && cd /tmp/sha256-lab
$ printf 'hello\n' | git hash-object --stdin
# 同样的 'hello\n'，SHA-1 与 SHA-256 给出完全不同的 ID，
# 但算法结构完全一致（仍是"类型 长度 \0 内容"再哈希）
```

**对我们的影响**：sourcepack 不存哈希、不算哈希，全靠 git 命令，
所以 SHA-1 → SHA-256 的仓库迁移对工具**零影响**——这是我们"不自己实现 git 层"
的又一个好处。

---

### 2.11 动手实验 `[lab]`

```bash
# 1) 亲眼确认哈希输入的构造
printf 'hello\n' | git hash-object --stdin
python3 -c "import hashlib; print(hashlib.sha1(b'blob 6\x00hello\n').hexdigest())"

# 2) 拆开磁盘上的对象
printf 'hello\n' | git hash-object -w --stdin
ls .git/objects/ce/*
python3 -c "import zlib; print(zlib.decompress(open('.git/objects/ce/013625030ba8dba906f756967f9e9ca394464a','rb').read()))"

# 3) 空树：复现 sourcepack 的常量
git hash-object -t tree /dev/null

# 4) 手工造提交（plumbing）
BLOB=$(printf 'made by hand\n' | git hash-object -w --stdin)
TREE=$(printf '100644 blob %s\thandmade.txt\n' "$BLOB" | git mktree)
COMMIT=$(git commit-tree "$TREE" -m "手工提交")
git cat-file -p "$COMMIT"
# 然后试试：这个提交为什么不在 git log 里？（第 3 章揭晓）

# 5) 改名检测
git log --oneline --name-status -M --follow -- main.go | grep '^R'
```

#### 自测题

1. 如果两个文件内容完全相同、文件名不同，`git add` 之后 `.git/objects` 里有几个对象？
2. `git commit` 之后索引变了吗？HEAD 变了吗？如果此时 `git status`，三个区域谁和谁比对？
3. 手工造的 commit 有什么办法让它出现在 `git log` 里？（提示：第 3 章，给它一个引用）

---

### 2.12 小结

- Git 只有四种对象：**blob（内容）/ tree（目录）/ commit（快照元数据）/ tag（固定指针）**；
- 哈希输入 = **类型头 + 长度 + NUL + 内容**，可在仓库外用 Python 逐字节复现；
- 对象在磁盘上是 **zlib 压缩**的松散文件（`objects/xx/yyy...`），`gc` 后打包进
  packfile 并做 **delta 链**（`chain length = 4`）；
- commit 的字段（tree/parent/author/committer/message）正是 `-G` 产物摘要里
  日期与作者的来源；我们**只取标题不取正文**是个已知缺口；
- **空树 `4b825dc6...` 是普适常量**，sourcepack 用它当"根提交"的 diff 基准
  （`changeset.go:319`），让首提交显示为"全部新增"；
- **blob 不含文件名** → 重命名免费、内容去重，rename 检测靠 `-M` 与相似度
  （我们仓库的 `R093 godoc.go → main.go` 就是证据）。

**下一章**：引用系统——为什么"分支只是一个 41 字节的文件"、HEAD 与 detached、
reflog 为什么能救回你以为删掉的提交。

---

<a id="第二部日常操作的真相"></a>

# 第二部　日常操作的真相

> 你每天都在敲的 add / commit / diff / branch，底下到底发生了什么。

---

<a id="第-3-章引用head-与-reflog"></a>

# 第 3 章　引用、HEAD 与 reflog

---

> **本章目标**：搞清"分支"到底是什么、"提交丢了"能不能救，以及 Git 为什么敢承诺
> "几乎所有误操作都可恢复"。学完你会理解：
> 对象库不可变（第 2 章） + 引用只是可移动的指针 = 历史永不被真正删除。
>
> 本章还会当场解掉第 2 章留的思考题：**手工造的提交怎么才能出现在 `git log` 里**。

---

### 3.1 引用就是文件

第 1 章说过 Git 内部只有三块东西，其中"引用"是**唯一可变**的部分。
它可变的方式非常朴素——**就是一个装着 40 位哈希的文本文件**：

```bash
# 新建一个分支（= 写一个 41 字节的文件）
$ git update-ref refs/heads/demo <sha>
$ cat .git/refs/heads/demo
94af1cd57e1573048348bc429bc053de2492dd1a
$ cat .git/refs/heads/demo | wc -c
41            # 40 位哈希 + 一个换行
```

**"创建分支"就是这么便宜**——第 1 章说的"分支廉价"，字面意思就是写个 41 字节的文件。

#### 松散引用 vs 打包引用

引用可以以两种形态存在，**松散优先**：

| 形态 | 位置 | 何时出现 |
| :--- | :--- | :--- |
| 松散（loose） | `.git/refs/heads/<name>` | 每次提交/移动分支时新写 |
| 打包（packed） | `.git/packed-refs`（一个文件装很多引用） | `git gc` / `git pack-refs` 之后 |

**实测**：仓库里 44 个引用被打包，而新提交会立刻产生一个松散文件覆盖它 `[lab]`：

```bash
$ grep -c "refs/" .git/packed-refs
44
$ git commit -q --allow-empty -m "空提交"
$ cat .git/refs/heads/demo
94af1cd57e1573048348bc429bc053de2492dd1a     # ← 新写的松散引用，值已变
```

> **对工具的意义**：**永远不要自己读 `.git/refs/`**。你以为它在文件里，它可能在
> `packed-refs` 里，也可能在两者之中（松散优先）。sourcepack 一律用
> `git rev-parse` / `git show-ref` / `git for-each-ref` 去问 Git 本身——
> 这也是我们"不自己实现 git 层"的第一条具体收益。

---

### 3.2 HEAD 是"符号引用"

`.git/HEAD` 里装的**不是哈希，而是另一个引用的名字**：

```bash
$ cat .git/HEAD
ref: refs/heads/git-course
```

这叫**符号引用**（symbolic ref）。所以：

- `HEAD` 的意思是"我现在在哪个分支上"；
- `git rev-parse HEAD` 会**递归解析**：HEAD → refs/heads/git-course → 实际哈希；
- 直接问"HEAD 这个符号指向谁"要用另一个命令：

```bash
$ git symbolic-ref HEAD
refs/heads/git-course
$ git rev-parse --short HEAD
aa686d8
```

**这两个命令的区别是整个第 3 章的关键**：`symbolic-ref` 问"名字"，
`rev-parse` 问"值"。

---

### 3.3 引用的命名空间

所有引用都在 `refs/` 下，按用途分命名空间：

```bash
$ git for-each-ref --format='%(refname)  →  %(objectname:short)'
refs/heads/git-course          →  aa686d8      # 本地分支
refs/remotes/origin/HEAD       →  aa686d8      # 远端分支（fetch 得到）
refs/remotes/origin/main       →  aa686d8
refs/tags/v1.0.0               →  962753c      # 标签
refs/tags/v1.0.1               →  b1feaa8
refs/tags/v2.0.1               →  c7a057a      # 注意：这不是 commit 的哈希（见 3.4）
```

除了 `for-each-ref` 能看到的这些，还有几个**特殊引用**（存在 `.git/` 根下）：

| 引用 | 含义 |
| :--- | :--- |
| `HEAD` | 当前所在位置 |
| `ORIG_HEAD` | 上次 `reset` / `merge` / `rebase` **之前**的位置（"后悔药"） |
| `FETCH_HEAD` | 上次 `fetch` 抓到了什么 |
| `MERGE_HEAD` | 正在合并的那个"对方"提交（合并冲突时存在） |
| `CHERRY_PICK_HEAD` | 正在 cherry-pick 的提交 |

**这些文件的存在与否本身就是状态**：比如 `.git/MERGE_HEAD` 存在 = 仓库正处于
"合并未完成"状态。`git status` 会据此提示你。

---

### 3.4 轻量标签 vs 附注标签（一个真实的 tag 对象）

| | 轻量标签（lightweight） | 附注标签（annotated） |
| :--- | :--- | :--- |
| 本质 | 一个**直接指向 commit** 的 ref | 一个**独立的对象**（第 4 种对象类型） |
| 创建 | `git tag name` | `git tag -a name -m msg` |
| `cat-file -t` | `commit` | `tag` |

**实测我们仓库的 v2.0.1**（`git tag -a` 创建的）`[lab]`：

```bash
$ git cat-file -t v2.0.1
tag
$ git cat-file -p v2.0.1
object aa686d8e14f8fc729a32a30d42dbf32c0b94b29f
type commit
tag v2.0.1
tagger yuanguangshan <yuanguangshan@gmail.com> 1791813987 +0800

v2.0.1: -h 末尾换成参数手册博客版（https://s.want.biz/sourcepack）
$ git cat-file -t lightweight-demo        # 对照：轻量标签
commit
```

**为什么发布要用附注标签**：它带**签名、日期、发布说明**，而且是一个不可变对象
（第 2 章：对象不可变）。我们的 goreleaser 工作流就是靠 `v*` 标签触发的——
`refs/tags/v2.0.1` 这个引用一旦推送，CI 就能拿到它。

---

### 3.5 revspec：引用系统的表达式语言

Git 允许用**文字表达式**指代任意提交。sourcepack 的 `--rev` 直接把你的输入交给
Git 解析，所以下面这些**全部可用**（实测）`[lab]`：

| revspec | 含义 | 实测结果 |
| :--- | :--- | :--- |
| `HEAD` | 当前提交 | ✅ |
| `HEAD~1` | 第一个父提交（往回 1 代） | ✅ |
| `HEAD@{1}` | **reflog 里的上一步**（3.8 节的主角） | ✅ |
| `handmade` | 任意引用名（分支/tag/远端） | ✅ |
| `v2.0.1` | 标签 | ✅ |

```
$ gx -g --rev HEAD@{1} --dry-run -d .
📦 Git changes: git diff HEAD@{1}  (working tree + index vs HEAD@{1})
```

常用语法速查：

| 写法 | 含义 |
| :--- | :--- |
| `<rev>~<n>` | 沿**第一父**往回 n 代 |
| `<rev>^<n>` | 第 n 个父（合并提交：`^1` 主线、`^2` 被并进来的分支） |
| `<rev>@{<n>}` | 该引用的第 n 步 reflog（`@{0}` 是当前） |
| `<rev>@{<time>}` | 按时间：`HEAD@{yesterday}`、`main@{2.hours.ago}` |
| `<rev>:<path>` | 该版本下的某个文件（`HEAD:README.md`） |
| `<rev>^{commit}` | **剥壳**：一路剥到 commit（tag → commit，见下） |
| `<rev>^{tree}` | 剥到 tree |

#### 剥壳 `^{commit}`：为什么 sourcepack 需要它

`v2.0.1` 是附注标签，它的哈希指向一个 **tag 对象**，不是 commit：

```bash
$ git rev-parse v2.0.1
c7a057a324abc15567c01951c1003be614f94608      # tag 对象
$ git rev-parse "v2.0.1^{commit}"
aa686d8e14f8fc729a32a30d42dbf32c0b94b29f       # 它指向的提交
```

所以 `changeset.go:321` 用 `^{commit}` 校验用户给的 `--rev`：

```go
	} else if _, err := g.run("rev-parse", "--verify", base+"^{commit}"); err != nil {
```

**实测这个校验的价值**：给它一个**树对象**（合法 revspec，但不是提交）也会被拦住，
并给出可读报错：

```bash
$ gx -g --rev "not-a-rev" --dry-run -d .
❌ invalid --rev "not-a-rev": git rev-parse --verify not-a-rev^{commit} failed:
   fatal: Needed a single revision
```

**这里体现两种设计的选择**：我们**不解析 revspec**（那是 Git 的领域，规则极多），
只做两件事——① 让 Git 去解析；② 把 Git 的报错包装成用户能看懂的一句话。

---

### 3.6 第 2 章思考题解答：怎么让"手工提交"进入历史

第 2 章我们用 plumbing 造了一个提交，但它不出现 `git log` 里。原因很简单：
**提交之间不靠"列表"连接，靠的是"引用能到达它"**。`git log` 的起点是引用
（默认 HEAD），顺着 `parent` 边回溯——没有引用指向的提交就是"不可达"的。

**实测**（同一个提交，加引用前后）`[lab]`：

```bash
$ C=$(git commit-tree "$TREE" -m "手工提交：现在给它一个引用")
$ git log --oneline --all | grep -c b42db3a
0                                   # ← 加引用之前：找不到
$ git update-ref refs/heads/handmade "$C"
$ git log --oneline --all | grep -c b42db3a
1                                   # ← 加引用之后：出现了
$ git log --oneline -1 handmade
b42db3a 手工提交：现在给它一个引用
```

**所以"创建分支"= 给某个提交一个名字**。这也解释了 `git branch <name> <sha>`
为什么可以给**任意**提交起名字——包括你以为已经丢掉的提交（见 3.8）。

---

### 3.7 detached HEAD：符号引用会"断"

`git checkout <sha>` / `git checkout --detach` 会让 HEAD **不再指向分支**，
而是直接指向一个提交：

```bash
$ git checkout -q --detach HEAD~1
$ git status
HEAD detached at aa686d8
$ git symbolic-ref --short HEAD
fatal: ref HEAD is not a symbolic ref        # ← 这个命令失败了
$ git rev-parse --short HEAD
aa686d8                                       # ← 但值还是能取到
```

**这就是为什么"取值"和"取名字"必须分开**：detached 状态下有值、没有名字。

#### 回到 sourcepack：我们的三段式回退

`changeset.go:282-288` 正是为这三种状态写的（**代码原样**）：

```go
	if br, berr := g.run("symbolic-ref", "--short", "HEAD"); berr == nil && strings.TrimSpace(br) != "" {
		cs.Branch = strings.TrimSpace(br)
	} else if sha, serr := g.run("rev-parse", "--short", "HEAD"); serr == nil {
		cs.Branch = "detached at " + strings.TrimSpace(sha)
	} else {
		cs.Branch = "(no commits yet)"
	}
```

三种状态一一对应（**下面这张表是实测出来的，不是推想的**）：

| 状态 | `symbolic-ref --short HEAD` | `rev-parse HEAD` | sourcepack 显示 |
| :--- | :--- | :--- | :--- |
| 在分支上 | 成功 → `refs/heads/x` | 成功 | `Branch: x` |
| **detached HEAD** | **失败**（`fatal: ref HEAD is not a symbolic ref`） | 成功 | `Branch: detached at aa686d8` |
| **还没有任何提交**（unborn HEAD） | **仍然成功** → `main` | **失败**（退出码 128） | `Branch: main` + 警告（见下） |

**unborn HEAD 这一格值得停一下**：`git init` 之后、第一次提交之前，
`.git/HEAD` 里已经写着 `ref: refs/heads/main`，但 `.git/refs/heads/main`
**这个文件并不存在**——HEAD 是"指向一个还不存在的分支"的符号引用。
所以 `symbolic-ref` 成功、`rev-parse` 失败。

**实测（空仓库 + 一个未跟踪文件）** `[lab]`：

```bash
$ git init -q -b main . && printf 'package main\n' > a.go
$ git symbolic-ref --short HEAD
main                                  # 成功：符号引用有效
$ git rev-parse HEAD ; echo $?
128                                   # 失败：还没有提交对象
$ test -f .git/refs/heads/main || echo "分支文件不存在"
分支文件不存在

$ gx -g --dry-run -d .
📦 Git changes: git diff 4b825dc642cb6eb9a060e54bf8d69288fbee4904  (working tree + index vs 4b825dc...)
  Repository:          /private/tmp/unborn
  Branch:              main
  Files changed:       1 (+1 −0)
                       staged 0 · unstaged 0 · untracked 1
  ⚠ repository has no commits yet — diffing against the empty tree
```

**注意这一行的基准是 `4b825dc...`——正是第 2 章那个空树对象。**
源码在 `changeset.go:320` 附近：

```go
		cs.Warnings = append(cs.Warnings, "repository has no commits yet — diffing against the empty tree")
```

而 `-G`（按提交数打包）在无提交时直接报错（`changeset.go:444`），
因为"最近 N 个提交"没有可数的对象：

```bash
$ gx -G 3 --dry-run -d .
❌ repository has no commits yet: -G/--commits needs at least one commit
```

> **这里体现了第 2 章那个常量的第二个用途**：不仅是"根提交的基准"，
> 也是"整个仓库还没有提交时"的基准。`-g` 与 `-G` 面对同一状态给出不同反应
> （一个降级并警告、一个明确报错）也是有意为之：前者"相对空树看工作区"是
> **有意义的**（就是整个项目），后者"最近 N 个提交"**没有意义**。

**实测 detached 状态下的产物** `[lab]`：

```bash
$ gx -g --dry-run -d . | grep -E "Branch|HEAD"
  Branch:              detached at aa686d8
  HEAD:                aa686d8 docs: v2.0.1 记录（-h 末尾换成参数手册博客版）；还原 v2.0.0 的 help 引用为当时真实输出
```

> 注意第三行 `HEAD:` 是**另一个调用**的结果（`changeset.go:290`：
> `rev-parse HEAD` + `log -1 --format=%s`），与 `Branch` 分开取——
> 因为 detached 时"名字"没有，但"值"和"提交标题"都有。

---

### 3.8 reflog：Git 的"后悔药"（本章重头戏）

**reflog 是 Git 记录"引用是怎么移动的"的日志**，存在：

- `.git/logs/HEAD`（HEAD 的移动史）
- `.git/logs/refs/heads/<branch>`（每个分支各自的移动史）

**它不是历史，是"历史的历史"**——每次 checkout/commit/reset/merge/rebase 都追加一行。

#### 完整实验：把提交 reset 掉，再救回来 `[lab]`

```bash
# 1) 造一个"重要提交"
$ printf 'important\n' > important.txt
$ git add important.txt && git commit -q -m "重要提交：马上会被我 reset 掉"
$ GOOD=$(git rev-parse HEAD)
$ git log --oneline -1
4f2ada7 重要提交：马上会被我 reset 掉

# 2) 事故：硬重置回上一个提交
$ git reset --hard HEAD~1
HEAD is now at aa686d8 docs: v2.0.1 记录（...）
$ test -f important.txt || echo "文件不在了（工作区被重置）"
文件不在了（工作区被重置）
$ git log --oneline | grep -c 4f2ada7
0                                   # ← 从"当前历史"里消失了

# 3) 救援第一步：reflog 记得一切
$ git reflog -5
aa686d8 HEAD@{0}: reset: moving to HEAD~1
4f2ada7 HEAD@{1}: commit: 重要提交：马上会被我 reset 掉     # ← 就在这儿
aa686d8 HEAD@{2}: checkout: moving from aa686d8... to recover

# 4) 救援第二步：把引用指回去
$ git reset --hard "$GOOD"
HEAD is now at 4f2ada7 重要提交：马上会被我 reset 掉
$ cat important.txt
important                            # ← 文件、历史、全部回来
$ git reflog -1
4f2ada7 HEAD@{0}: reset: moving to 4f2ada71141360476dd16561680c432a695e256b
```

**四个值得记住的观察**：

1. `reset --hard` 后提交"看不见了"，但**对象还在磁盘上**（第 2 章的不可变性）；
2. reflog 是**本地**的：clone 不会带上别人的 reflog（所以它是"个人操作史"）；
3. 救援动作本身也被记进 reflog——reflog 是追加日志，不会被"覆盖"；
4. 同理，`git reset` 回退分支**不会丢提交**，只是把指针往后挪。

#### reflog 也会过期

| 配置 | 默认 | 含义 |
| :--- | :--- | :--- |
| `gc.reflogExpire` | 90 天 | HEAD 的 reflog 保留期 |
| `gc.reflogExpireUnreachable` | 30 天 | 不可达提交的 reflog 保留期 |

过期 + `git gc` 之后，没有任何引用的对象才会被真正清除。
**所以"事故后别慌、别急着 gc"是有道理的**——reflog 还在，就能救。

---

### 3.9 reflog 也没了怎么办：dangling 对象与 `fsck`

如果提交从来没被引用过（比如第 2 章手工造的），reflog 里也不会有它。
但**对象仍在对象库里**，`git fsck` 能看到"没人引用"的对象：

```bash
$ git fsck --lost-found | grep -c "dangling commit"
6                                    # ← 6 个吊着的提交（含我实验里造的那些）
$ git log --oneline -1 8c8ccb5        # 直接按哈希读
8c8ccb5 孤儿提交
```

| 概念 | 含义 |
| :--- | :--- |
| **可达对象** | 从某个引用出发、顺着 parent/tree 能找到的 |
| **dangling（吊着的）对象** | 对象库里存在、但没有任何引用能到达 |
| 会被 gc 删掉吗 | 只有"过期 + 不可达"才会。reflog 里的引用也算一种到达 |

**实用技巧**：`git fsck --lost-found` 会把吊着的对象写进 `.git/lost-found/`；
`git fsck --unreachable` 只列不写。找到哈希后用 `git show <sha>` 看内容，
`git branch rescue <sha>` 把它救回历史。

---

### 3.10 回到 sourcepack：一行行对照

| 代码位置 | 做什么 | 对应本章概念 |
| :--- | :--- | :--- |
| `changeset.go:256` `rev-parse --show-toplevel` | 找仓库根 | 不自己找 `.git`，问 Git |
| `changeset.go:271` `rev-parse --show-prefix` | 算子目录作用域 | 用 Git 的路径语义（绕开 `/tmp` 软链坑） |
| `changeset.go:282-285` `symbolic-ref` → `rev-parse` | 分支名 / detached | **符号引用 vs 值** |
| `changeset.go:287` `(no commits yet)` | unborn HEAD 的兜底 | 第三种状态 |
| `changeset.go:290-297` `rev-parse HEAD` → `log -1 --format=%s` | 取当前提交与标题 | 递归解析；无提交时退到 `(no commits yet)` |
| `changeset.go:291` `log -1 --format=%s` | 取提交标题 | commit 对象的 message |
| `changeset.go:321` `rev-parse --verify <base>^{commit}` | 校验 `--rev` | **剥壳** + 让 Git 报错（unborn 时走 :320 的空树分支） |
| `changeset.go:542` `shortSHA(s)` | 取前 7 位 | Git 的最小无歧义缩写习惯 |

`shortSHA` 的实现（我们**没有**调 `git rev-parse --short`，因为哈希已在手）：

```go
func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}
```

> **一个诚实的注脚**：硬截 7 位在**超大仓库**里理论上有歧义风险
> （Git 默认会自动加长到无歧义）。sourcepack 的场景是"给人看的摘要"，
> 且同一产物里同时有完整路径与提交标题，所以这个简化是可接受的——
> 但如果哪天要拿它当**唯一标识**去查询，就该换成
> `git rev-parse --short=8` 或 `git rev-parse --verify <short>^{commit}`。

---

### 3.11 动手实验 `[lab]`

```bash
cd /tmp/git-lab

# 1) 引用就是文件
git update-ref refs/heads/demo HEAD
cat .git/refs/heads/demo          # 看它只有 41 字节
cat .git/packed-refs | head -3    # 打包形态
git for-each-ref --format='%(refname) → %(objectname:short)'

# 2) 名字 vs 值
cat .git/HEAD
git symbolic-ref HEAD
git rev-parse HEAD

# 3) 让手工提交进入历史（第 2 章思考题）
BLOB=$(printf 'x\n' | git hash-object -w --stdin)
TREE=$(printf '100644 blob %s\tx.txt\n' "$BLOB" | git mktree)
C=$(git commit-tree "$TREE" -m "手动提交")
git log --all --oneline | grep "$(echo $C | cut -c1-7)" || echo "不可达：log 里没有"
git update-ref refs/heads/handmade "$C"
git log --oneline handmade

# 4) detached HEAD 与三段式回退
git checkout --detach HEAD~1
git symbolic-ref HEAD ; echo "上一条命令的退出码: $?"
gx -g --dry-run -d . | grep Branch
git checkout -  # 回到原分支

# 5) 救援演练（本章必须动手做一遍）
printf 'x\n' > rescue.txt && git add rescue.txt && git commit -q -m "待救提交"
GOOD=$(git rev-parse HEAD)
git reset --hard HEAD~1
git reflog -3
git reset --hard "$GOOD"
cat rescue.txt
```

**自测题**：

1. `git branch new` 和 `git update-ref refs/heads/new HEAD` 等价吗？
2. `HEAD@{1}` 与 `HEAD~1` 有什么本质不同？（提示：一个是"历史的形状"，一个是"你的操作史"）
3. 为什么 `git clone` 之后，别人看不到你本地的 reflog？
4. 一个提交既没有分支指向、也没有 tag、reflog 也已过期——它一定被删了吗？
5. 为什么 sourcepack 判断"分支名"要先用 `symbolic-ref` 而不是直接 `rev-parse HEAD`？

---

### 3.12 小结

- **引用 = 装着哈希的文件**（松散 `.git/refs/...` 或打包 `.git/packed-refs`，松散优先），
  创建分支的代价就是写 41 字节；
- **HEAD 是符号引用**（指向另一个引用）；`symbolic-ref` 问名字、`rev-parse` 问值，
  detached HEAD 时前者失败、后者成功；
- 引用按命名空间组织：`heads` / `remotes` / `tags`，另有 `ORIG_HEAD`、`FETCH_HEAD`、
  `MERGE_HEAD` 等特殊引用（**它们的存在与否本身就是仓库状态**）;
- **附注标签是独立对象**（`cat-file -t` = `tag`），轻量标签直接指向 commit；
- revspec 是引用系统的表达式语言（`~`、`^`、`@{n}`、`^{commit}`）；
  sourcepack 不自己解析，只做校验 + 包装报错（`changeset.go:321`）；
- **提交靠"引用可达"进入历史**——这是第 2 章思考题的答案（`update-ref` 0 次 → 1 次）；
- **reflog 是后悔药**：`reset --hard` 丢的提交可以靠 `HEAD@{n}` 找回来（完整实验见 3.8），
  reflog 过期后还能靠 `fsck` 找 dangling 对象，**最终救回靠的还是"对象不可变"**；
- sourcepack 的分支名三段回退（`symbolic-ref` → detached → no commits yet）是本章
  概念在真实代码里的落地，实测输出 `Branch: detached at aa686d8`。

**下一章**：索引（暂存区）——`git add` 到底写了什么、为什么会有 staged/unstaged 两种
状态、以及 sourcepack 的 `--staged` 为什么曾经把计数写成了恒为 0。

---

<a id="第-4-章索引暂存区"></a>

# 第 4 章　索引（暂存区）

---

> **本章目标**：搞清 `.git/index` 到底是什么、`git add` 究竟写了哪些字节，
> 以及为什么"同一个文件可以既是 staged 又是 unstaged"。
> 学完你会理解 Git 的三棵树模型，并能解释 sourcepack 里
> `staged · unstaged · untracked` 这三个计数是怎么来的——
> 以及它在 v1.8.0 里为什么**恒为 0**。

---

### 4.1 索引是"三棵树"里的中间那棵

Git 的日常工作围绕三棵树：

| 树 | 存在哪 | 含义 |
| :--- | :--- | :--- |
| **HEAD** | 对象库（上一章的引用指向的 commit 里的 tree） | 上次提交的快照 |
| **索引（index / staging area）** | `.git/index`（**二进制文件**） | **下一次提交的候选快照** |
| **工作区（working tree）** | 你的文件系统 | 你此刻正在编辑的东西 |

`git status` 干的事就是**三方比对**：

```
        git diff --cached          git diff
HEAD  ──────────────────► 索引  ──────────────────► 工作区
（上次提交）              （下次提交）              （此刻）
```

- `git diff --cached` = HEAD ↔ 索引 → 这就是 **staged**（已暂存）
- `git diff` = 索引 ↔ 工作区 → 这就是 **unstaged**（未暂存）
- `??` = 工作区里有、但索引里没有 → **untracked**（未跟踪）

> 记住这张图，"staged 是什么意思"就再也不会混了：
> **staged = 索引里的内容与 HEAD 不同**，而不是"我运行过 add"。

---

### 4.2 索引是二进制文件：逐字节看

```bash
$ ls -l .git/index
2225 字节

$ xxd .git/index | head -3
00000000: 4449 5243 0000 0002 0000 001a 6acc febc  DIRC........j...
00000010: 328d f414 6acc febc 328d f414 0100 0010  2...j...2.......
00000020: 1357 452a 0000 81a4 0000 01f5 0000 0000  .WE*............
```

- 前 4 字节 **`DIRC`** = "directory cache"（索引的旧称）；
- 接着 4 字节版本号（`00000002` = v2）、4 字节条目数（`0000001a` = **26**，
  与 `git ls-files --stage | wc -l` 一致）；
- 之后是**每个条目一整块**。（本节实测环境：`git version 2.39.5 (Apple Git-154)`）

**用 Python 把条目解出来**（索引格式的最佳学习方式）：

```python
import struct
data = open('.git/index','rb').read()
ver, n = struct.unpack('>II', data[4:12])
print(f'魔数={data[:4].decode()} 版本={ver} 条目数={n}')
off = 12
for i in range(3):
    start = off
    ctime_s, ctime_n, mtime_s, mtime_n, dev, ino, mode, uid, gid, size = struct.unpack('>10I', data[off:off+40])
    sha = data[off+40:off+60].hex()
    flags, = struct.unpack('>H', data[off+60:off+62])
    namelen = flags & 0x0FFF
    name = data[off+62:off+62+namelen].decode()
    print(f'条目{i}: mode={mode:o} size={size} sha={sha[:10]}… name={name}')
    off = start + 62 + namelen
    off = start + ((off - start + 7) // 8) * 8     # v2：条目按 8 字节对齐
```

实测输出：

```
魔数=DIRC 版本=2 条目数=26 文件大小=2225
条目0: mode=100644 size=322    uid=501 gid=0 sha=c9f80462af… name=.gitattributes
条目1: mode=100644 size=4579   uid=501 gid=0 sha=eb018ce0ca… name=.github/workflows/release.yml
条目2: mode=100644 size=1436   uid=501 gid=0 sha=582b8e75c5… name=.github/workflows/test.yml
```

（`size` 与 `stat -f%z` 的结果逐字节吻合，说明这就是工作区文件的 stat 缓存。）

**每条包含三类信息**（这三类缺一不可）：

1. **stat 信息**：`ctime / mtime / dev / ino / uid / gid / size` —— 用于**快速判断文件是否变过**；
2. **对象引用**：`mode` + `blob 哈希` —— 暂存的内容在对象库里的位置；
3. **路径名** + 标志位（名字长度、是否冲突等）。

> **这是索引的第一个身份：缓存**。`git status` 不必读遍所有文件内容——
> 先比 stat（mtime/size/ino），只有对不上的才去读内容算哈希。
> 上万个文件的项目里 `git status` 依然飞快，靠的就是这块 stat 缓存
> （第 10 章会展开 racy timestamp 的坑）。

---

### 4.3 `git add` 到底做了什么

两步，都可验证：

```bash
# ① 把内容写进对象库（第 2 章的 blob）
$ git hash-object -w newfile.txt
# ② 更新索引条目（路径 → 该 blob 的哈希 + 记录当前 stat）

# 手工模拟 git add 的第 ② 步：
$ git update-index --add --cacheinfo 100644,<blob-sha>,newfile.txt
```

**验证"索引里存的哈希 == 工作区内容的哈希 == HEAD 里的哈希"**（未修改的文件）：

```bash
$ git ls-files -s .gitattributes
100644 c9f80462afe787371af06c0ae9a94e152dc469b8 0	.gitattributes
$ git rev-parse HEAD:.gitattributes
c9f80462afe787371af06c0ae9a94e152dc469b8
$ git hash-object .gitattributes
c9f80462afe787371af06c0ae9a94e152dc469b8
```

三者一致 = 这个文件"干净"（clean）。改任意一处，三者就不再同时相等——
**`git status` 的全部逻辑就是比较这三个哈希**。

> `git ls-files -s` 输出的中间那个数字是**阶段号（stage）**，正常是 0，
> 冲突时会出现 1/2/3（见 4.7）。

---

### 4.4 三种状态与 porcelain 输出的两种列

`git status --porcelain` 的每行前两个字符是**两列**：
**第一列 = 索引 vs HEAD（staged）**，**第二列 = 工作区 vs 索引（unstaged）**。

**实测四种组合** `[lab]`：

```bash
$ printf 'package main\n// 未暂存改动\n' >> main.go          # 只改工作区
$ printf 'package main\n' > staged_only.go && git add staged_only.go   # 只进索引
$ printf 'untracked\n' > brand_new.txt                        # 什么都不做

$ git status --porcelain
 M main.go              # ← 第二列 M：工作区与索引不同（unstaged）
A  staged_only.go       # ← 第一列 A：索引与 HEAD 不同（staged，新增）
?? brand_new.txt        # ← 未跟踪
```

| porcelain | 含义 |
| :--- | :--- |
| ` M` | 工作区改了，**没** `add` |
| `M ` | `add` 过了，工作区没再改 |
| `MM` | **先 add、又改了工作区**（两份内容同时存在） |
| `A ` / `D ` | 索引里新增 / 删除 |
| `??` | 未跟踪 |
| `UU` | 冲突（both modified） |

---

### 4.5 三种 `git diff` 各看什么

这是最容易搞混的地方，**一次测清楚** `[lab]`：

```bash
$ git diff --name-only          | wc -l
1        # 工作区 vs 索引      → unstaged
$ git diff --cached --name-only | wc -l
1        # 索引 vs HEAD        → staged
$ git diff HEAD --name-only     | wc -l
2        # 工作区 vs HEAD      → 两者之和
```

**一句话记忆**：`git diff` 默认**不看索引之外**（它比的是"索引 ↔ 工作区"），
要看已暂存的必须加 `--cached`（等价写法 `--staged`）。

> **对 sourcepack 的意义**：我们的 `--staged` 模式正是把命令换成
> `git diff --cached`（`changeset.go:342-351`），而不是"先拿全部再过滤"。

---

### 4.6 `MM`：同一个文件的两份内容并存

```bash
$ git add main.go                 # 此刻的内容进索引
$ printf '// 又改了第三行\n' >> main.go   # 工作区再改
$ git status --porcelain main.go
MM main.go
$ git diff --cached main.go | wc -l
10        # 索引 vs HEAD：第一次 add 时的内容
$ git diff main.go | wc -l
9         # 工作区 vs 索引：后来又改的那部分
```

**索引里存的是"你 add 那一刻的内容"**（一个完整 blob），
它和 HEAD、和工作区都可以不同。这就是"暂存区"能"部分提交"的原理：

- `git add -p`（交互式按块暂存）本质是**拼一个只包含部分改动的 blob** 放进索引；
- `git commit` 只把**索引**写成一个 tree（工作区里剩下的改动完全不进这次提交）。

---

### 4.7 索引的第二个身份：合并工作台（stage 1/2/3）

平时一个路径在索引里只有 **1 条**记录（stage 0）。**冲突时会有 3 条**：

```bash
$ git merge ctheirs
CONFLICT (content): Merge conflict in conflict.txt
$ git status --porcelain conflict.txt
UU conflict.txt

$ git ls-files -u conflict.txt          # -u = 只列未合并（冲突）条目
100644 df967b96a579e45a18b8251732d16804b2e56a55 1	conflict.txt
100644 b19a1e93bec1317dc6097229e12afaffbfa74dc2 2	conflict.txt
100644 950b81b7eee953d050aa05a641f8e056c85dd1bd 3	conflict.txt
```

| 阶段号 | 含义 |
| :--- | :--- |
| **1** | **共同祖先**（merge base）里的版本 |
| **2** | **ours**：当前分支的版本 |
| **3** | **theirs**：被合并进来的版本 |

**同一个路径，三个版本同时躺在索引里**——这就是"三方合并"在磁盘上的样子。
冲突标记（`<<<<<<<` / `=======` / `>>>>>>>`）只是 Git 为了让人看懂而写进工作区的**文本提示**；
真正的三方数据在索引里。

**解决冲突 = 把工作区内容 add 进去，让三条归一**：

```bash
$ printf 'resolved\n' > conflict.txt && git add conflict.txt
$ git ls-files --stage conflict.txt
100644 2ab19ae607aabda796309682e0448237aab03047 0	conflict.txt    # ← 回到 stage 0
```

> 所以"解决冲突"在 Git 内部就是**一次 `add`**：用你的内容覆盖掉那三条。

---

### 4.8 其它常用索引操作

| 命令 | 作用 |
| :--- | :--- |
| `git ls-files --stage` | 列出索引条目（模式 / 哈希 / 阶段 / 路径） |
| `git ls-files --others --exclude-standard` | 未跟踪文件（**尊重 ignore 规则**） |
| `git ls-files -u` | 只列冲突条目 |
| `git rm --cached <f>` | 从索引移除、保留工作区文件（"取消跟踪"） |
| `git update-index --assume-unchanged <f>` | 告诉 Git"别管这个文件的变化"（临时性能手段，**不是**忽略） |
| `git update-index --skip-worktree <f>` | 本地配置类文件常用（同样的坑：别当 ignore 用） |
| `git update-index --refresh` | 刷新 stat 缓存 |
| `git reset <f>` | 用 HEAD 版本覆盖索引条目（撤销 `add`） |

> `--assume-unchanged` / `--skip-worktree` 都**不是** `.gitignore` 的替代品：
> 它们只影响本地索引行为，克隆到别处就失效（第 7 章讲真正的忽略规则）。

---

### 4.9 回到 sourcepack：三个计数是怎么来的

#### 4.9.1 取三个集合（代码原样，`changeset.go:366-388`）

```go
	unstagedSet := map[string]bool{}
	if !config.GitStaged && !cs.Unborn {
		stagedArgs := append([]string{"diff", "--no-ext-diff", "--name-status", "-z", "-M", "--cached", base}, g.scoped()...)
		if out, cerr := g.run(stagedArgs...); cerr == nil {
			markPaths(out, stagedSet, true)
		}
		unstagedArgs := append([]string{"diff", "--no-ext-diff", "--name-status", "-z", "-M"}, g.scoped()...)
		if out, uerr := g.run(unstagedArgs...); uerr == nil {
			markPaths(out, unstagedSet, false)
		}
	}

	// 3. 未跟踪文件（--staged 时不适用）
	var untracked []string
	if !config.GitStaged {
		lsArgs := append([]string{"ls-files", "--others", "--exclude-standard", "--full-name", "-z"}, g.scoped()...)
		...
	}
```

**三个集合对应三棵树的两条边 + 索引之外**，与 4.1 那张图完全一致：

| 集合 | 命令 | 对应 |
| :--- | :--- | :--- |
| staged | `diff --name-status --cached` | HEAD ↔ 索引 |
| unstaged | `diff --name-status` | 索引 ↔ 工作区 |
| untracked | `ls-files --others --exclude-standard` | 索引里没有的 |

#### 4.9.2 用 git 的真值校验我们的输出

**实测（三个状态各造一个文件）** `[lab]`：

```bash
$ printf 'pkg\n' > only_staged.go && git add only_staged.go
$ printf '// unstaged\n' >> main.go
$ printf 'untracked\n' > untracked_中文.txt

# git 的真值
$ echo "staged=$(git diff --cached --name-only|wc -l) unstaged=$(git diff --name-only|wc -l) untracked=$(git ls-files --others --exclude-standard|wc -l)"
staged=1 unstaged=1 untracked=1

# sourcepack 的输出
$ gx -g --dry-run -d .
  Files changed:       3 (+3 −0)
                       staged 1 · unstaged 1 · untracked 1
```

**逐项吻合。** 产物里每条变更也带标记：

```markdown
| 1 | `main.go` | modified | +1 −0 | unstaged |
| 2 | `only_staged.go` | added | +1 −0 | staged |
| 3 | `untracked_中文.txt` | untracked | +1 −0 | untracked |
```

#### 4.9.3 为什么 v1.8.0 里这三个计数**恒为 0**

这是一次真实事故，根因就在 4.9.1 那段的 `if` 条件里：

```go
	if !config.GitStaged && !cs.Unborn {      // ← --staged 模式时整段被跳过
```

`--staged` 模式下 `stagedSet` / `unstagedSet` **都不填充**（因为清单本身来自
`git diff --cached`，当时的作者认为"不必再算一遍"）。而计数函数是这么写的
（`changeset.go:561-577`）：

```go
func summarizeChangeSet(cs *ChangeSet) {
	cs.StagedCount, cs.UnstagedCount, cs.UntrackedCount = 0, 0, 0
	for _, grp := range cs.Commits {
		for _, ch := range grp.Changes {
			switch {
			case ch.Untracked:
				cs.UntrackedCount++
			case ch.Staged && ch.Unstaged:
				cs.StagedCount++ ...
			case ch.Staged:
				cs.StagedCount++
			...
```

两个集合都是空的 → 每条变更的 `Staged`/`Untracked` 全为 `false` →
**`switch` 的四个分支全部落空** → 三个计数全是 0。于是终端上出现自相矛盾的输出：

```
Files changed: 3
              staged 0 · unstaged 0 · untracked 0     ← 下面却列着 3 个变更
```

**v1.8.1 的修法**（`changeset.go:416-423`）：

```go
	if config.GitStaged {
		// 上面的 stagedSet 只在非 --staged 时才填充，所以这里必须显式标，
		// 否则 summarizeChangeSet 的 switch 全落空、StagedCount 恒为 0。
		for _, ch := range changes {
			ch.Staged = true
		}
	}
```

**为什么这样是对的**：`--staged` 的清单**来源本身就是证据**——
它来自 `git diff --cached`，所以每一条都必然是 staged。不需要再跑一次 diff 去"确认"。

**修复后的实测**：

```bash
$ gx -g --staged --dry-run -d .
  Files changed:       1 (+1 −0)
                       staged 1 · unstaged 0 · untracked 0    # ← 与 git diff --cached 一致
```

> **这一节的教训值得单独记住**：三个计数"看起来只是显示问题"，但它让用户
> 无法判断"我到底暂存了什么"。一个 *展示层* 的恒零 bug，削弱的是整个
> `-g --staged` 工作流的可信度——这也解释了为什么它被排在 v1.8.1 的必修清单里。
> 顺带：`--staged` 时 `untracked` 计数**本就应该为 0**，因为未跟踪文件
> 不可能被暂存（`ls-files --others` 那段也整段跳过）。

---

### 4.10 动手实验 `[lab]`

```bash
cd /tmp/git-lab

# 1) 看清索引的字节
xxd .git/index | head -3
git ls-files --stage | head -5
git ls-files --stage | wc -l

# 2) 三棵树与三种状态
printf 'x\n' > a.txt && git add a.txt        # staged
printf 'y\n' >> README.md                    # unstaged
printf 'z\n' > b.txt                         # untracked
git status --porcelain
git diff --name-only ; git diff --cached --name-only ; git diff HEAD --name-only

# 3) MM：两份内容并存
git add README.md && printf '// 再改\n' >> README.md
git status --porcelain README.md
git diff --cached README.md | head -5
git diff README.md | head -5

# 4) 冲突时索引里的三份
git checkout -q -B base4 HEAD && printf 'base\n' > c.txt && git add c.txt && git commit -qm base
git checkout -q -B theirs4 && printf 'theirs\n' > c.txt && git commit -qam theirs
git checkout -q base4 && printf 'ours\n' > c.txt && git commit -qam ours
git merge theirs4 || true
git ls-files -u c.txt                 # ← stage 1/2/3
printf 'resolved\n' > c.txt && git add c.txt && git ls-files --stage c.txt   # ← 归一

# 5) 对照 sourcepack 的计数（先回到干净分支）
git merge --abort ; git checkout -q main 2>/dev/null || git checkout -q -B main HEAD
printf 'only\n' > only_staged.go && git add only_staged.go
printf '// u\n' >> main.go
printf 'z\n' > b2.txt
gx -g --dry-run -d . | grep -E "staged|Files changed"
```

**自测题**：

1. "staged" 的准确含义是什么？（对照 4.1 那张图回答）
2. `git add` 之后又改了工作区，`git commit` 提交的是哪一版？为什么？
3. 冲突时为什么索引里同一路径能有 3 条记录？它们分别是什么？
4. `git status` 为什么不必读完所有文件内容就能判断谁变了？
5. sourcepack 的 `--staged` 模式为什么可以"直接标记 Staged=true"，而不必再跑 diff？

---

### 4.11 小结

- 索引就是**三棵树的中间那棵**：`.git/index`（二进制，魔数 `DIRC`），
  每条含 **stat 缓存 + blob 哈希 + 路径**；
- **staged = 索引与 HEAD 不同**，**unstaged = 工作区与索引不同**，
  porcelain 的**两列**正好对应这两条边；
- 三种 diff 各看一条边：`git diff`（索引↔工作区）、`git diff --cached`（HEAD↔索引）、
  `git diff HEAD`（两端之和）；
- `MM` 是"同一个文件两份内容并存"——这是"部分提交"（`add -p`）的基础；
- **冲突时索引里同一路径有 stage 1/2/3**（base/ours/theirs），
  解决冲突 = 一次 `add` 把三条归一为 stage 0；
- 索引还是**性能结构**（stat 缓存让 `git status` 不必读全部内容）；
- sourcepack：三个集合 = `diff --cached` / `diff` / `ls-files --others`；
  三个计数由 `summarizeChangeSet` 的 `switch` 得出；
  **v1.8.0 的 `--staged` 恒为 0 是因为两个集合在 `--staged` 下被整段跳过，
  修法是"清单来源即证据"，直接标 `Staged = true`**。

**下一章**：提交图与合并——commit 的 `parent` 边构成有向无环图，
`merge` 的两条父边、fast-forward、`merge-base`，以及 sourcepack 为什么要取
"第一父"、以及 PR 审查场景下"应该和 merge-base 比而不是和分支尖端比"。

---

<a id="第-5-章提交图与分支合并"></a>

# 第 5 章　提交图与分支合并

---

> **本章目标**：把"历史"从"一串提交"升级为**有向无环图（DAG）**来看。
> 学完你能解释：合并提交为什么有两条父边、`^1`/`^2` 是什么、
> `A..B` 与 `A...B` 差在哪，以及为什么"和 main 比"在 PR 审查里是个坑。
>
> 本章最后一个实测会给出**量化证据**：同一个分支，用错基准会多出 1 个文件、
> 多算 1 行删除。

---

### 5.1 提交图：parent 边构成的 DAG

第 2 章看到 commit 里有 `parent` 字段。**每个提交可以指向 0、1 或多个父提交**：

| 父提交数 | 含义 |
| :--- | :--- |
| 0 | 根提交（仓库的第一个提交） |
| 1 | 普通提交 |
| 2+ | **合并提交**（第 2 个及以后的父 = 被并进来的分支尖端） |

所以历史不是"一条线"，而是一张**有向无环图**（DAG）：
边从提交指向它的父（时间上倒着走），且不可能成环（哈希包含父哈希，
第 1 章讲过的不可变性保证了这一点——**你无法构造一个自己是自己祖先的对象**）。

```bash
$ git log --graph --oneline -4
* aa686d8 docs: v2.0.1 记录（...）
* 196c9a7 docs: -h 末尾加"更多用法请参考：..."
* 224230f feat!: -d 非法路径明确报错...
* 8878d72 test+docs: 全量参数实测 + 新增参数手册 PARAMETERS.md...
$ git cat-file -p HEAD | grep -E "^tree|^parent"
tree ca33473e4e00110896dcdc9b4a56fd1c2dd76d02
parent 196c9a76126e549ee8afd7d0bc9f2d6e731e3e4d      # ← 只有一个父
```

**一个真实观察**：sourcepack 自己的 135 个提交里**没有任何合并提交**：

```bash
$ git log --merges --oneline | wc -l
0
$ git log --oneline | wc -l
135
```

也就是说我们的历史是**严格线性**的（开发时用 rebase/squash 保持单线）。
这带来一个工程后果：**合并相关的代码路径，本地仓库永远测不到**——
所以我们的测试夹具必须**自己合成合并提交**（第 12 章会看到夹具怎么造 merge）。

> 这也解释了为什么"合并提交取第一父"这种细节很容易写错却测不出来：
> 在真实仓库里根本没机会跑到那条分支。

---

### 5.2 分叉与合并：在实验室里造一个

```bash
$ git checkout -q -B main5 aa686d8
$ printf 'x\n' > base.txt && git add base.txt && git commit -q -m "共同祖先"

$ git checkout -q -B feat5            # 从共同祖先拉出特性分支
$ printf 'f1\n' > feat.txt && git add feat.txt && git commit -q -m "feature: 新增 feat.txt"
$ printf 'f2\n' >> feat.txt && git commit -q -am "feature: 改 feat.txt"

$ git checkout -q main5                # 主线也前进了
$ printf 'm1\n' >> base.txt && git commit -q -am "main: 改 base.txt"

$ git log --graph --oneline --all -6
* 7e94b86 feature: 改 feat.txt
* 7d60908 feature: 新增 feat.txt
| * 8f0c8ed main: 改 base.txt
|/
* 1b915db 共同祖先
```

**分叉点（共同祖先）就是后面所有比较的基准**：

```bash
$ git merge-base main5 feat5
1b915dba7f52c72a8e03f07977055e2bb0bd5de5
```

---

### 5.3 合并的两种形态

`git merge` 的结果取决于两边是否有分叉：

| 情况 | 结果 | 图形 |
| :--- | :--- | :--- |
| main 是 feat 的**祖先**（main 没前进） | **fast-forward**：只把 main 指针挪到 feat，**不产生新提交** | 一条直线 |
| 两边都前进了 | **merge commit**：产生一个新提交，**有两条父边** | 分叉再汇合 |

**实测：本例里 fast-forward 已经不可能**（因为 main 已前进）：

```bash
$ git checkout -q -B ff-demo main5
$ git merge --ff-only feat5
fatal: Not possible to fast-forward, aborting.
```

**用 `--no-ff` 强制产生合并提交**：

```bash
$ git checkout -q main5
$ git merge --no-ff -m "merge: 合并 feat5（no-ff）" feat5
$ git log --graph --oneline -5
*   8b69c5c merge: 合并 feat5（no-ff）
|\
| * 7e94b86 feature: 改 feat.txt
| * 7d60908 feature: 新增 feat.txt
* | 8f0c8ed main: 改 base.txt
|/
* 1b915db 共同祖先
```

> **`--no-ff` 的工程价值**：它让"这个特性是一个整体"在图里永久可见
> （否则 fast-forward 之后，特性分支的边界就消失了）。
> 代价是历史不再是直线——这也是各团队在"rebase 流"与"merge 流"之间取舍的根源。

---

### 5.4 两条父边：`^1`、`^2` 与 `--first-parent`

```bash
$ git cat-file -p HEAD | grep -E "^tree|^parent"
tree ae5d630c7f69355bbb8d1d8fa57bee54f5d34e43
parent 8f0c8ed2918c8d1076acb59da1a3d23155e403b7     # 父 1：接收方（主线的上一个提交）
parent 7e94b86e1be0f31590e1f710612d43eb99d95171     # 父 2：被并进来的分支尖端

$ git rev-parse --short HEAD^1   → 8f0c8ed
$ git rev-parse --short HEAD^2   → 7e94b86
$ test "$(git rev-parse HEAD^)" = "$(git rev-parse HEAD^1)" && echo 是
是                                  # ^ 默认等于 ^1
```

**约定**：`^1` = "我刚才站在的那条线"（第一父），`^2` = "被并进来的那条线"。
合并顺序决定了谁是第一父：`git merge feat5` 时当前分支是 main5，所以 main5 侧是 `^1`。

**`--first-parent`：只看主线**，把特性分支的提交折叠起来：

```bash
$ git log --oneline -5                 # 全部
8b69c5c merge: 合并 feat5（no-ff）
8f0c8ed main: 改 base.txt
7e94b86 feature: 改 feat.txt           # ← 来自特性分支
1b915db 共同祖先
7d60908 feature: 新增 feat.txt         # ← 来自特性分支

$ git log --first-parent --oneline -4  # 只看第一父链
8b69c5c merge: 合并 feat5（no-ff）
8f0c8ed main: 改 base.txt
1b915db 共同祖先
aa686d8 docs: v2.0.1 记录（...）
```

**用途**：在"主干历史"视图里（`git log --first-parent`、`git blame` 的默认策略、
某些 CI 的变更检测）人们只关心"主线推进了哪些点"，
特性分支内部的中间提交（可能很脏）被折叠。这也是 `-G` 处理合并提交时取第一父的理由（见 5.8）。

---

### 5.5 对合并提交做 diff：三种都合法，含义完全不同

```bash
$ MB=$(git merge-base HEAD^1 HEAD^2)

$ git diff --name-only HEAD^1 HEAD        # 相对第一父：这次合并"引入"了什么
feat.txt                                   （1 个文件）

$ git diff --name-only HEAD^2 HEAD        # 相对第二父：主线侧带来了什么
base.txt                                   （1 个文件）

$ git diff --name-only $MB HEAD           # 相对共同祖先：整条分支的全量改动
base.txt
feat.txt                                   （2 个文件）
```

| 你想知道的 | 该用哪个 |
| :--- | :--- |
| "这次合并把哪些文件带进了主线" | `HEAD^1..HEAD`（**GitHub 的 PR diff 用这个**） |
| "如果这条分支从没分叉，现在会是什么样" | `merge-base..HEAD` |
| "合并有没有冲突解决错误" | 需要看 `git show` 的 combined diff |

> **注意**：`git show <merge>` 默认什么都不显示（因为合并的"改动"取决于和哪个父比），
> 要么用 `git show -m`（分别对每个父显示），要么显式指定 `HEAD^1..HEAD`。

---

### 5.6 双点 `..` vs 三点 `...`（最容易踩的坑）

这两个符号在 `git diff` 和 `git log` 里含义不同，先看 **diff**：

| 写法 | 含义 |
| :--- | :--- |
| `git diff A..B` | **两个尖端直接比**（文件层面的 A 与 B） |
| `git diff A...B` | **从它们的 merge-base 比到 B**（等价 `git diff $(git merge-base A B) B`） |

**实测（真正分叉的一对分支）**：

```bash
# d1 与 d2 各自前进了一个提交，共同改了 shared.txt
$ git diff --name-only d1..d2
d1b.txt          # ← 注意：这是 d1 自己的改动，被"反向"算了进来
d2.txt
shared.txt
（3 个文件）

$ git diff --name-only d1...d2
d2.txt
shared.txt
（2 个文件）

$ git diff --name-only "$(git merge-base d1 d2)" d2
d2.txt
shared.txt
（2 个文件 —— 与三点完全一致 ✅）
```

**为什么 `A..B` 会把 A 的改动也算进来**：`diff A B` 比较的是**两个快照**。
B 里没有 A 的新增文件（`d1b.txt`），所以从 A 的角度看，B "删掉了" `d1b.txt`——
于是它出现在 diff 里。**这不是 bug，是双点的定义**。

**结论**：想表达"我这条分支相对分叉点做了什么"，**永远用三点**（或显式 merge-base）。

> 顺带说：`git log A..B` 与 diff 的双点含义一致（"B 有而 A 没有的提交"），
> 而 `git log A...B` 是"两边独有的提交"（对称差）。同一个符号在 log 与 diff 里
> 语义不完全一样，这是 Git 的历史包袱之一。

---

### 5.7 merge-base 是怎么算的

`git merge-base A B` = **两个提交在 DAG 上的最近公共祖先**（lowest common ancestor）。

- 朴素做法：把 A 的所有祖先和 B 的所有祖先求交集，取"深度最大"的；
- 实际实现用 **commit-graph / generation number** 剪枝（第 10 章）；
- **多个 merge-base 是可能的**（criss-cross merge：两边互相合并过），
  这时 `git merge-base` 默认只返回一个，`--all` 返回全部。

**实测**：

```bash
$ git merge-base main5 feat5
1b915dba7f52c72a8e03f07977055e2bb0bd5de5
$ git log -1 --oneline $(git merge-base main5 feat5)
1b915db 共同祖先
```

> **一个反直觉但重要的推论**：合并**完成后**，被合并的分支尖端就成了合并提交的祖先，
> 于是 `merge-base(main, feat) == feat`，`git diff main...feat` **变成空**。
> 我在做本章实验时就先踩了这个坑：在合并后的状态测三点，得到 0 个文件——
> **测语义必须用真正的分叉状态**（这也是为什么上面 5.6 我重建了一对分支）。

---

### 5.8 回到 sourcepack：三个具体问题

#### 5.8.1 合并提交的 diff 取第一父

源码（`changeset.go:496-499`，`-G` 的逐提交遍历里）：

```go
	for _, grp := range groups {
		base := emptyTreeSHA
		if len(grp.Parents) > 0 {
			base = grp.Parents[0]        // ← 合并提交取第一父
		}
```

（`grp.Parents` 来自 `git log --format=%P`；根提交没有父，于是回落到空树——
第 2 章那个常量在这里第三次出场。）

**实测产物里的标注**（在实验室仓库跑 `gx -G 3`）：

```markdown
### `8b69c5c` merge: 合并 feat5（no-ff） (merge — diff vs first parent)
```

摘表里也会标：

```markdown
| `8b69c5c` | 2026-10-12 | Lab | merge: 合并 feat5（no-ff） _(merge)_ |
```

**为什么取第一父**：语义上等价于 GitHub 的 PR diff——"这次合并给主线带来了什么"。
取第二父会得到"主线侧带来了什么"，取 merge-base 会得到整条分支的全量改动（5.5 已量化）。

#### 5.8.2 `-G` 的"最近 N 个提交"就是 `git log -N`

```bash
$ gx -G 3 --dry-run -d .
📦 Git changes: git log -3
  Commits:             3
  Files changed:       2 (+4 −0)
```

**注意它没有用 `--first-parent`**：所以 `-G 3` 里可能既有合并提交、
也有特性分支内部的提交（本例 3 个提交里含 1 个 merge + 2 个普通提交）。

#### 5.8.3 **PR 审查的经典坑：`--rev main` 是双点语义**

假设你从 main 拉出 `d2` 干了两件事，而 main 自己也前进了一个提交。
你想让 AI 审查"**我这条分支改了什么**"：

```bash
$ gx -g --rev d1 --dry-run -d .        # d1 相当于 main 的尖端
  Files changed:       3 (+2 −2)       # ← 多了 1 个文件、多了 1 行删除

$ gx -g --rev "$(git merge-base d1 d2)" --dry-run -d .
  Files changed:       2 (+2 −1)       # ← 只有 d2 自己的改动
```

**那多出来的一个文件是什么**：d1 自己新增的 `d1b.txt`——因为 `git diff d1`（双点）
从 d1 的角度看，工作区"删掉了"它。**审阅者会看到一处根本不存在的删除。**

**正确做法**（当前版本需要手工组合）：

```bash
gx -g --rev "$(git merge-base main HEAD)"
```

**这就是"应该提供 `--base main`"的实证依据**——工具内部做
`git merge-base main HEAD` 再 diff，用户就不必记住这个坑。
（该改进已记录在 TODO；本章的实测数据就是它的价值证明。）

---

### 5.9 动手实验 `[lab]`

```bash
cd /tmp/git-lab

# 1) 造一个真正的分叉 + 合并
git checkout -q -B base5 HEAD
printf 'x\n' > b.txt && git add b.txt && git commit -qm base
git checkout -q -B topic5
printf 't\n' > t.txt && git add t.txt && git commit -qm "topic: t.txt"
git checkout -q base5
printf 'm\n' >> b.txt && git commit -qam "base: 改 b.txt"
git log --graph --oneline --all -5
git merge-base base5 topic5

# 2) fast-forward 与 --no-ff
git checkout -q -B ffdemo base5 && git merge --ff-only topic5 || echo "不能 ff（正常，两边都前进了）"
git checkout -q base5 && git merge --no-ff -m "merge topic5" topic5
git cat-file -p HEAD | grep ^parent        # 两条父边

# 3) 三种 diff 与 --first-parent
MB=$(git merge-base HEAD^1 HEAD^2)
git diff --name-only HEAD^1 HEAD ; git diff --name-only "$MB" HEAD
git log --first-parent --oneline -4

# 4) 双点 vs 三点（必须用分叉状态，合并后就测不出来了）
git checkout -q -B x1 HEAD~2 && printf '1\n' > x1.txt && git add . && git commit -qm x1
git checkout -q -B x2 HEAD~1 && printf '2\n' > x2.txt && git add . && git commit -qm x2
git diff --name-only x1..x2      # 会带上 x1 的改动
git diff --name-only x1...x2     # 只带 x2 的改动

# 5) 用 sourcepack 看差异（对照 5.8.3）
git checkout -q x2
gx -g --rev x1 --dry-run -d . | grep "Files changed"
gx -g --rev "$(git merge-base x1 x2)" --dry-run -d . | grep "Files changed"
```

**自测题**：

1. 合并提交的 `^1` 和 `^2` 分别是什么？谁决定顺序？
2. 为什么 sourcepack 处理合并提交时要取第一父而不是第二父？
3. 合并**完成之后**，`git diff main...feature` 为什么不显示任何东西？
4. `git diff A..B` 与 `git diff A...B` 分别等于哪个显式命令？
5. `git log --first-parent` 在什么场景下是"正确视图"？给出两个例子。

---

### 5.10 小结

- 历史是 **DAG**；父边数决定提交类型（0=根、1=普通、2+=合并）；
- **fast-forward 不产生提交**，`--no-ff` 强制留一个合并提交（让特性边界可见）；
- 合并提交有**两条父边**：`^1` 接收方（主线）、`^2` 被并进来的分支尖端；
  `--first-parent` 只看主线；
- 对合并做 diff 有三种都合法的口径（`^1..HEAD` / `^2..HEAD` / `merge-base..HEAD`），
  实测分别 1 / 1 / 2 个文件；
- **双点 vs 三点是 PR 审查的关键**：`A..B` 会把 A 的改动反向算进来
  （实测 3 个文件），`A...B` 等价 `merge-base B`（实测 2 个文件）；
- **合并后三点会变空**（被合并的分支成了祖先）——所以测语义必须用分叉状态；
- sourcepack：`-G` 的合并提交取**第一父**并把标注写进产物；
  `-g --rev <分支>` 是**双点语义**，在 PR 审查场景会多算改动
  （实测 3 文件 vs 正确的 2 文件）——`--base` 的实证依据；
- 我们自己的 135 个提交里**一个合并都没有**，所以合并路径全靠夹具合成，
  这是"测试盲区"的一个真实来源。

**下一章**：diff 算法与 patch 格式——Myers 算法在做什么、unified diff 的每一行怎么读、
`a/` `b/` 前缀为什么会让我们的路径解析出错（第 2 章已经埋了 `--src-prefix` 的线），
以及 rename detection 的成本。

---

<a id="第三部格式与传输"></a>

# 第三部　格式与传输

> 差异是怎么算出来的、忽略规则的真实语义、以及数据怎么在仓库间流动。

---

<a id="第-6-章diff-算法与-patch-格式"></a>

# 第 6 章　diff 算法与 patch 格式

---

> **本章目标**：读懂 unified diff 的每一行，理解 diff 是"**用时算出来的**"而不是存下来的，
> 并看清三个真实陷阱：**用户配置会改写前缀**、**textconv 能让 git diff 直接失败**、
> **`-M` 让 diff 需要做内容相似度比较**。
> 本章最后回到 `changeset.go`，解释两个幽灵条目 bug 的来历。

---

### 6.1 diff 的本质：两个快照的比较

第 1 章说过 Git 存快照。所以 `git diff` 做的事是：

```
取两个 tree（或两个 blob） → 逐文件比较内容 → 用 diff 算法算出"最小编辑脚本" → 渲染成文本
```

**没有"变更记录"这种东西**。这也解释了几个常见困惑：

| 困惑 | 原因 |
| :--- | :--- |
| 重命名后 `git log -p` 显示"删除+新增" | 不指定 `-M` 时，diff 只看内容，而"改名"这件事**不在内容里**（第 2 章：blob 不含文件名） |
| 合并提交 `git show` 什么都不显示 | 合并的"改动"取决于和哪个父比（第 5 章） |
| diff 的输出取决于配置 | 算法、前缀、textconv、空白处理**全部可配**（本章 6.7/6.9/6.10） |

---

### 6.2 Myers 算法：最小编辑脚本

Git 默认用 **Myers（1986）** 算法：在"编辑图"上找从 (0,0) 到 (N,M) 的**最短路径**，
只允许三种动作——**保留**（对角线，匹配）、**删除**（向下）、**插入**（向右）。

- 复杂度 **O(ND)**：D 是编辑距离（差异行数）。差异小时极快，这正是日常场景；
- 差异大时退化为 O(N²)，所以 Git 有 `--diff-algorithm` 备选（见 6.10）；
- 输出的"最小编辑脚本"就是 diff 的 hunk。

**直觉版例子**：

```
旧: A B C D
新: A X C D
```

编辑图上有两条路径都能到终点：
① 保留 A，删 B，插 X，保留 C D（D=2，2 个 hunk 行）；
② 保留 A，删 B C D，插 X C D（D=6）。
Myers 取最短 → ①，于是 diff 显示 `-B` / `+X`。

**关键**：算法找的是"最短"，不是"最符合人类直觉"。当文件里有大量重复行时，
两者会冲突——这就是 6.10 节四种算法存在的理由。

---

### 6.3 unified diff 逐行解剖

**真实输出**（`$` 表示行尾）：

```
diff --git a/f.txt b/f.txt$
index 84275f9..07685c9 100644$
--- a/f.txt$
+++ b/f.txt$
@@ -1,4 +1,5 @@$
 line1$
-line2$
+LINE2-changed$
 line3$
 line4$
+line5$
```

| 行 | 含义 |
| :--- | :--- |
| `diff --git a/f.txt b/f.txt` | 一个文件块的开始（**注意：这是"协议行"，不是真正的 diff 行**） |
| `index 84275f9..07685c9 100644` | 旧 blob 哈希 `..` 新 blob 哈希 + 文件模式 |
| `--- a/f.txt` | 旧版本路径（`a/` 前缀） |
| `+++ b/f.txt` | 新版本路径（`b/` 前缀） |
| `@@ -1,4 +1,5 @@` | hunk 头：旧侧从第 1 行起 4 行、新侧从第 1 行起 5 行 |
| ` `（空格开头） | 上下文行（两侧都有） |
| `-` | 旧侧独有（删除） |
| `+` | 新侧独有（新增） |
| `\ No newline at end of file` | 上一行的文件没有以换行结尾（见 6.6） |

**为什么 `diff --git` 行不能当路径来源**：它同时包含两个路径，
且路径本身可能含空格（`a/my file.txt b/my file.txt` 无法用简单切分还原）。
**权威的路径来源是 `--name-status -z`**（这正是 sourcepack 的做法，见 6.11）。

---

### 6.4 hunk 头的数学

```
@@ -<旧起点>,<旧行数> +<新起点>,<新行数> @@
```

- 行数为 1 时可省略：`@@ -5 +5,2 @@`；
- 行数为 0 时起点是"插入位置前一行"：新增文件用 `-0,0`；
- 起点是 **1-based**，`-1,4` 表示旧文件第 1–4 行。

**实测**（原 4 行，新 5 行）：

```bash
$ git diff -U3 f.txt | grep "^@@"
@@ -1,4 +1,5 @@
```

**你要能一眼看懂**：`-1,4` = 旧文件的第 1 行起 4 行；`+1,5` = 新文件第 1 行起 5 行。
两侧行数差 = 净增删行数（这里 +1）。

---

### 6.5 上下文与空白选项

| 选项 | 作用 |
| :--- | :--- |
| `-U<n>` | 上下文行数（默认 3）；`-U0` 只给改动行 |
| `-w` / `--ignore-all-space` | 忽略空白差异 |
| `-b` | 忽略空白数量变化 |
| `--ignore-blank-lines` | 忽略空行增删 |
| `--word-diff` / `--color-words` | 按词（而非按行）给 diff |

**实测**：

```bash
$ for u in 0 1 3; do printf "U%s: %s 行\n" $u "$(git diff -U$u f.txt | wc -l)"; done
U0: 9 行
U1: 11 行
U3: 11 行          # 本文件太小，U1 与 U3 相同

$ printf 'line2 changed with spaces\n' > g.txt ; printf 'line2   changed   with spaces\n' > g2.txt
$ git diff --no-index g.txt g2.txt | wc -l
8
$ git diff --no-index -w g.txt g2.txt | wc -l
0                  # ← -w：忽略空白后完全一致
```

> **提示**：`git diff --no-index` 可以比较**两个任意文件**（不必在仓库里），
> 是做实验时最顺手的 diff 工具。

---

### 6.6 文件末尾没有换行符

Git 会用一行 `\ No newline at end of file` 标注**上一行**没有以换行结尾：

```bash
$ printf 'a\nb' > n1.txt ; printf 'a\nb\nc' > n2.txt
$ git diff --no-index n1.txt n2.txt | tail -6
 a
-b
\ No newline at end of file
+b
+c
\ No newline at end of file
```

**读法**：`\ No newline...` 紧跟在某个 `-` 或 `+` 行之后，说明**那一行**缺换行。

**对工具的坑（以及 sourcepack 是怎么绕开的）**：自己解析 patch 时，如果按
"每行以 `+`/`-` 开头就计数"，有两个陷阱：

1. `\ No newline at end of file` 以 `\` 开头 → **不会被误判**（天然安全）；
2. 真正危险的是协议行 `+++ b/x` 与 `--- a/x`——它们也以 `+`/`-` 开头！

`changeset.go:701-737` 的解法是引入 **`inHunk` 状态机**（代码原样）：

```go
	inHunk := false          // ← 只有 @@ 之后的行才算数
	...
		if inHunk {
			switch {
			case strings.HasPrefix(ln, "+"):
				cur.Added++
			case strings.HasPrefix(ln, "-"):
				cur.Deleted++
			}
			continue
		}
		switch {
		case strings.HasPrefix(ln, "@@"):
			inHunk = true
		case strings.HasPrefix(ln, "+++ "):
			if p := parseDiffPath(ln[4:]); p != "" {
				cur.Path = p
			}
		case strings.HasPrefix(ln, "--- "):
```

**`+++ `/`--- ` 只用来取路径、不计入增删**；增删只在 `@@` 打开 hunk 之后统计。
下一个 `diff --git` 会把状态归零——`splitPatch` 正是按这个边界切块的（见 6.11.3）。

---

### 6.7 前缀：`a/` `b/` 为什么会变

默认 diff 头部用 `a/`（旧）与 `b/`（新）当**人为前缀**，目的是让路径在
"运行于仓库根"这个前提下看起来像相对路径。

**但用户配置可以改掉它**：

| 配置 | 头部变成 |
| :--- | :--- |
| （默认） | `--- a/f.txt` / `+++ b/f.txt` |
| `diff.mnemonicPrefix=true` | `--- i/f.txt` / `+++ w/f.txt`（i=index, w=worktree） |
| `diff.noprefix=true` | `--- f.txt` / `+++ f.txt`（没有前缀） |

**实测**：

```bash
$ git -c diff.mnemonicPrefix=true diff f.txt | grep -E '^--- |^\+\+\+ '
--- i/f.txt
+++ w/f.txt
$ git -c diff.noprefix=true diff f.txt | grep -E '^--- |^\+\+\+ '
--- f.txt
+++ f.txt
# 显式固定前缀（sourcepack 的做法）
$ git -c diff.mnemonicPrefix=true diff --src-prefix=a/ --dst-prefix=b/ f.txt | grep -E '^--- |^\+\+\+ '
--- a/f.txt
+++ b/f.txt
```

> **这就是 v1.8.2 修复的那个 bug**：sourcepack 靠"剥掉 `a/`、`b/` 前缀"来还原路径。
> 用户若开了 `mnemonicPrefix`，头部变成 `i/`、`w/`，路径还原不出来 → diff 贴不到
> 对应变更上 → 产生"幽灵条目"（第 5 章实测：一条变更变成两条，其中
> `c/b/gone.go` 是假的）。修法是**每次调用都显式带 `--src-prefix=a/ --dst-prefix=b/`**：
> 不去猜用户配置，而是把协议固定下来。

---

### 6.8 rename / copy 检测：`-M` / `-C`

因为 blob 不含文件名（第 2 章），"改名"必须靠**内容相似度**推断：

```bash
$ git diff --cached --name-status -M
R094	old_name.py	new_name.py          # R + 相似度百分比
```

**实测四种设置的差别**（同一改动：改名 + 改 1 行 / 20 行）：

| 选项 | `--name-status` 输出 | 说明 |
| :--- | :--- | :--- |
| 默认（`-M`，阈值 50%） | `R094 old_name.py new_name.py` | 认作改名，相似度 94% |
| `--no-renames` | `A new_name.py` + `D old_name.py` | 两条独立记录 |
| `-M100%` | `A new_name.py` + `D old_name.py` | 要求 100% 相同才算改名 |
| 内容全改（相似度≈0） | `A new_name.py` + `D old_name.py` | 相似度为 0，不算改名 |

**完整 diff 头**（`-M` 生效时）：

```
diff --git a/old_name.py b/new_name.py
similarity index 94%
rename from old_name.py
rename to new_name.py
```

**`-C`（copy 检测）**比 `-M` 更贵：它要在**所有删除的文件**与新增文件之间找相似块。

> **成本提醒**：`-M` 需要对"删除的文件"和"新增的文件"两两比较内容。
> 一次删 500 个、加 500 个文件的重构里，这是 25 万次内容比较。
> sourcepack 的 `-M` 是必要开销（否则用户看到的重命名会变成删除+新增，
> 对审查毫无帮助），但也正因如此，`-g` 在大重构上会明显变慢。

---

### 6.9 二进制文件与 textconv：`--no-textconv` 的真实价值

Git 判定二进制的规则很简单：**前 8000 字节里有没有 NUL**
（`buffer_is_binary`）。命中就不给文本 diff，只给一行：

```bash
$ printf 'PNG\x00\x02changed' > logo.bin
$ git diff logo.bin
diff --git a/logo.bin b/logo.bin
index e65757e..b11395c 100644
Binary files a/logo.bin and b/logo.bin differ
```

> **与 sourcepack 的联系**：我们的 `binaryProbeSize = 8000` 就是照这个来的
> （原注释写着"与 git 的 buffer_is_binary 一致"）。不过我们后来把它**改成整文件校验**
> ——因为按字节截断的窗口会把"切在多字节字符中间"的合法 UTF-8 误判成二进制
> （v1.8.1 的真实 P0 bug，见 `PARAMETERS.md`）。

#### textconv：让用户配置"翻译"二进制

`.gitattributes` 可以给某类文件配一个转换命令，git 会先把内容转成文本再 diff：

```bash
$ printf 'blob.dat diff=mybin\n' > .gitattributes
$ git config diff.mybin.textconv "tr -d '\0'"
```

**实测三种情形**：

| 情形 | 结果 |
| :--- | :--- |
| 未配 textconv | `Binary files a/blob.dat and b/blob.dat differ` |
| 配了 textconv 但命令**执行失败** | `fatal: unable to read files to diff`，**`git diff` 退出码 128** |
| 配了 textconv 且加 `--no-textconv` | 回到 `Binary files ... differ` ✅ |

**这就是 sourcepack 每次都传 `--no-textconv` 的原因**。实测同一个仓库里：

```bash
$ git diff blob.dat ; echo $?          # 用户配的 textconv 会失败
128
$ gx -g --dry-run -d . ; echo $?       # sourcepack 仍然正常
0
  Files changed:       1 (+0 −0)
$ gx -g -d . -o /tmp/tc.md && grep blob.dat /tmp/tc.md
| 1 | `blob.dat` | modified | +0 −0 | unstaged, binary |
## Binary Files (no textual diff)
- `blob.dat`
```

**结论**：`--no-textconv` 不是"洁癖式加固"，而是**让工具在用户环境里不会因为
外部命令失败而整个挂掉**。同理 `--no-ext-diff` 挡掉用户配的外部 diff 程序。

---

### 6.10 四种 diff 算法：差异出现在重复行上

| 算法 | 特点 |
| :--- | :--- |
| `myers`（默认） | 最短编辑脚本，O(ND)，差异小时最快 |
| `minimal` | 花更多时间确保"最短"，避免 myers 的启发式剪枝 |
| `patience` | 不做"块内匹配"：只在**唯一**的行上找锚点，通常更符合人眼 |
| `histogram` | patience 的改进版（Git 2.12+ 起 `--histogram` 常用），对重复块更稳 |

**我原本以为"普通改动上它们都一样"——实测确实如此**：

```bash
$ git diff --diff-algorithm=myers f.txt | wc -l      → 11
$ ... minimal / patience / histogram                  → 11 / 11 / 11
```

**但构造一个"重复块被移动"的例子就分道扬镳了**：

```python
# 旧：a() 与 b() 内容完全相同（重复块），c() 在最后
# 新：把 c() 移到最前面
```

```bash
$ for algo in myers minimal patience histogram; do
    printf "%-10s: %s 行改动 / %s 个 hunk\n" $algo \
      "$(git diff --diff-algorithm=$algo r.py | grep -cE '^[+-][^+-]')" \
      "$(git diff --diff-algorithm=$algo r.py | grep -c '^@@')"
  done
myers     :  6 行改动 / 2 个 hunk
minimal   :  6 行改动 / 2 个 hunk
patience  : 16 行改动 / 1 个 hunk      # ← 明显不同
histogram :  6 行改动 / 2 个 hunk
```

`patience` 的 hunk 是"整段重写"：

```
@@ -1,15 +1,15 @@
-def a():
-    x = 1
-    y = 2
-    return x + y
-def b():
-    x = 1
-    y = 2
-    return x + y
+def a():
...
```

**怎么解释**：`patience` **拒绝**在重复块内部配对（因为 `a()` 与 `b()` 完全一样，
"哪个对应哪个"没有唯一答案），于是它选择不匹配、整段重写——
在这个例子里**反而给出了更长的 diff**。

> **所以"哪个更好"没有普适答案**：diff 算法的好坏取决于你想让读者看到什么。
> Git 默认 myers，是因为它在绝大多数场景下又快又够用；
> `histogram`/`patience` 常在"代码被搬来搬去"的提交上更可读。
> **sourcepack 不传 `--diff-algorithm`**：用 Git 默认值，避免在不同 Git 版本上
> 产生不同的产物（可复现性优先）。

---

### 6.11 回到 sourcepack：解析 patch 的四处硬骨头

#### 6.11.1 从 `diff --git` 行提取路径

```go
	if i := strings.Index(h, " b/"); i >= 0 {
		return parseDiffPath(h[i+1:])
	}
```

**为什么不用 `strings.Fields` 或 `Split("/")`**：路径可能含空格
（`diff --git a/my file.txt b/my file.txt`），任何"按空格切"的方案都会错。
用 **`" b/"` 作为锚点**（第二个路径的前缀），并配合我们显式固定的
`--dst-prefix=b/`，才能稳定拿到新路径。
**再叠一层保险**：真正的权威路径来自 `--name-status -z`（见 6.11.3）。

#### 6.11.2 `parseDiffPath`：**只剥一层**前缀（v1.8.0 的双剥 bug）

```go
	// 只剥与本行匹配的那一个前缀。原来连着剥 "a/" 再剥 "b/" 是错的：
	// 仓库里真实存在 b/ 目录时，--- a/b/x.go 会被剥成 x.go（应为 b/x.go），
	// 于是 diff 贴不到对应的变更上，还会多出一条"幽灵变更"。
	if strings.HasPrefix(p, "a/") {
		p = p[2:]
	} else if strings.HasPrefix(p, "b/") {
		p = p[2:]
	}
```

**实测 v1.8.2 修复前后的差别**（仓库里有 `b/` 目录 + 删除 `b/gone.go` +
用户开了 `mnemonicPrefix`）：

| | v1.8.1 | v1.8.2 |
| :--- | :--- | :--- |
| 变更数 | **2**（幽灵条目） | **1** |
| 条目 | `b/gone.go` deleted `+0 −0`（diff 没贴上）<br>`c/b/gone.go` modified `+0 −1` | `b/gone.go` deleted `+0 −1` |

**注意那个 `c/b/gone.go`**：`c/` 是 mnemonicPrefix 的"commit"前缀，
被当成路径的一部分留了下来——**这就是"不固定前缀 + 连剥两层"叠加的产物**。

#### 6.11.3 按 `diff --git` 切块，再"贴"回权威清单

流程（`changeset.go`）：

1. `git diff --name-status -z -M` → **权威文件清单**（路径、状态）
2. `git diff --patch ...` → patch 文本
3. `splitPatch(patch)` → 按 `diff --git` 切成多块，每块用 `parseDiffGitHeader`/
   `parseDiffPath` 取出路径，建索引 `byPath`
4. 遍历清单，用 `applyChunk(ch, byPath, ...)` 把对应的块"贴"到变更上
5. `appendUnmatchedChunks(...)` 处理**清单里没有、patch 里却有**的块

**第 5 步就是幽灵条目的来源**：如果第 3 步解析出的路径与第 1 步清单里的路径
对不上（比如双剥前缀），这段 diff 就找不到主人，于是被当成"一个新变更"追加进去。
**v1.8.2 之后**：前缀固定 + 只剥一层，两边路径必然一致，幽灵条目消失。

#### 6.11.4 体积上限与二进制防线

| 机制 | 位置 | 作用 |
| :--- | :--- | :--- |
| `capDiff(...)` | `changeset.go` | 单文件 diff 的行数上限（`--max-diff-lines`，默认 2000） |
| `capDiffBytes(...)` | 同上 | **字节**上限（复用 `--max-size`），防"单行 20MB"绕过行数限制 |
| `truncateUTF8(...)` | 同上 | 硬切时保证不切断多字节字符（第 2 章：产物必须是合法 UTF-8） |
| `diffBodyLooksBinary(...)` | 同上 | 最后一道防线：diff 正文若含裸字节则丢弃 |

**产物里的标注**（实测 `--max-diff-lines 10`）：

```
... [sourcepack] 49 of 59 diff lines omitted (--max-diff-lines 10)
```

---

### 6.12 动手实验 `[lab]`

```bash
cd /tmp/git-lab
git checkout -q -B lab6 HEAD && printf 'a\nb\nc\nd\n' > f.txt && git add f.txt && git commit -qm base

# 1) 逐行解剖
printf 'a\nB\nc\nd\ne\n' > f.txt
git diff f.txt | sed -n l            # $ 显示行尾
git diff f.txt | grep '^@@'          # hunk 头的数学

# 2) 上下文与空白
for u in 0 1 3; do printf "U%s: %s 行\n" $u "$(git diff -U$u f.txt | wc -l)"; done
printf 'a   b\n' > x1 ; printf 'a b\n' > x2
git diff --no-index x1 x2 | wc -l ; git diff --no-index -w x1 x2 | wc -l

# 3) 前缀：用户配置会改头部
git -c diff.mnemonicPrefix=true diff f.txt | grep -E '^--- |^\+\+\+ '
git -c diff.mnemonicPrefix=true diff --src-prefix=a/ --dst-prefix=b/ f.txt | grep -E '^--- |^\+\+\+ '

# 4) rename 检测
python3 -c "open('o.py','w').write('\n'.join('f%d()'%i for i in range(20)))"
git add o.py && git commit -qm o
python3 -c "
l=open('o.py').read().split('\n'); l[3]='changed()'
open('n.py','w').write('\n'.join(l))"
git add n.py && git rm -q o.py
git diff --cached --name-status -M        # R0xx
git diff --cached --name-status --no-renames

# 5) 无换行结尾
printf 'a\nb' > w1 ; printf 'a\nb\n' > w2 ; git diff --no-index w1 w2 | tail -3

# 6) 四种算法在重复块上分道扬镳（见 6.10 的构造）
```

**自测题**：

1. `@@ -1,4 +1,5 @@` 里的四个数字分别是什么？
2. 为什么 `diff --git a/x b/x` 这一行**不能**用来断定路径？
3. 用户开了 `diff.mnemonicPrefix=true` 时，`--- ` 行会变成什么？我们怎么防御？
4. `-M100%` 与默认 `-M` 的区别是什么？什么样的操作会让它变慢？
5. `--no-textconv` 保护的是什么？（用 6.9 的实测回答）
6. 什么情况下会出现"幽灵变更"？两个成因分别是什么？

---

### 6.13 小结

- diff 是**用时算的**：两个快照 → 最短编辑脚本 → unified diff；
- **Myers** 找最短路径（O(ND)）；`minimal`/`patience`/`histogram` 是不同取舍，
  在"重复块被搬动"时结果明显不同（实测 patience 16 行 vs 其余 6 行）；
- unified diff 的每一行都有明确含义，`diff --git`/`index`/`---`/`+++`/`@@` 各有职责；
  `\ No newline at end of file` 容易在统计行数时被误判；
- **前缀可被用户配置改写**（`mnemonicPrefix` → `i/`/`w/`，`noprefix` → 无前缀）——
  sourcepack 的防御是**显式固定 `--src-prefix/--dst-prefix`**，
  并且 `parseDiffPath` **只剥一层**前缀（v1.8.0 的双剥 + 不固定前缀 =
  幽灵条目 `c/b/gone.go`，v1.8.2 修复）；
- **rename 检测是内容相似度推断**（`R094`）、`-M100%` 与 `--no-renames` 是两端；
  代价是两两内容比较；
- **二进制判定是"前 8000 字节含 NUL"**（我们照抄后又改成整文件校验，见 v1.8.1 的 P0）；
  **`--no-textconv` 让工具不受用户外部命令影响**——实测用户 textconv 失败时
  `git diff` 退出码 128，而 `gx -g` 照常工作并正确标注 `binary`；
- 解析 patch 的四个硬骨头：协议行取路径、前缀只剥一层、把块贴回权威清单、
  体积/二进制双重防线。

**下一章**：忽略规则——`.gitignore` 的语义、嵌套与优先级、`core.excludesFile`、
`git check-ignore`，以及 v1.9.0 按 git 语义重写匹配器时踩到的三个坑
（`abc/**` 不含自身、父目录被排除后 `!` 救不回、`filepath.Match` 在 Windows 上跨过 `/`）。

---

<a id="第-7-章忽略规则"></a>

# 第 7 章　忽略规则

---

> **本章目标**：把 `.gitignore` 从"抄来的模板"变成"我能预测其行为"的规则系统。
> 学完你能回答：为什么 `abc/**` 不忽略 `abc` 自己？为什么 `!` 有时候"不起作用"？
> 为什么 `.gitignore` 挡不住已经被提交的文件？
> 以及——**为什么 sourcepack 明明调 git，却还要自己写一套匹配器**。

---

### 7.1 忽略规则有五个来源，优先级不同

| 来源 | 位置 | 是否进版本库 | 优先级 |
| :--- | :--- | :--- | :--- |
| **命令行** | `git add --force`、`git check-ignore` 的 `-c` 配置等 | — | **最高** |
| **`.gitignore`** | 仓库各层目录 | 是（可共享） | 中（**就近优先**） |
| **`.git/info/exclude`** | 仓库内部 | **否**（本地） | 中低 |
| **`core.excludesFile`** | 用户全局（如 `~/.config/git/ignore`） | 否 | 低 |
| `.gitattributes` 的 `export-ignore` | 仓库 | 是 | 只管 `git archive`，**与忽略无关** |

**实测三种来源同时生效**（`git check-ignore -v` 会告诉你"被谁命中的"）：

```bash
$ git check-ignore -v root.log                    # 仓库 .gitignore
.gitignore:1:*.log	root.log
$ git check-ignore -v local-only.txt              # .git/info/exclude
.git/info/exclude:1:local-only.txt	local-only.txt
$ git config core.excludesFile /tmp/global-excludes.txt
$ git check-ignore -v global-ignore.tmp           # 全局忽略
/tmp/global-excludes.txt:1:global-ignore.tmp	global-ignore.tmp
```

**为什么要有三套**：

- `.gitignore`：**团队共享**的规则（构建产物、依赖目录）；
- `.git/info/exclude`：**你自己的**本地噪音（编辑器临时文件），不想让同事看到；
- `core.excludesFile`：**你所有仓库**的通用规则（`.DS_Store`、`*.swp`）。

---

### 7.2 语法全表

| 写法 | 含义 |
| :--- | :--- |
| `*.log` | 不含 `/` → 匹配**任意层级**的名字 |
| `/build` | 前导 `/` → 锚定到**该文件所在目录**（这里是仓库根） |
| `build/` | 尾部 `/` → **只匹配目录**，且其下内容一并忽略 |
| `**/foo` | 零层或多层目录 |
| `foo/**` | foo **内部**的内容（**不含 foo 自身**，见 7.6） |
| `a/**/b` | `a/b`、`a/x/b`、`a/x/y/b` |
| `!keep.log` | 负向模式（重新纳入），按"**最后一条命中的规则**生效" |
| `\#name`、`\!name` | 字面量的 `#` / `!` |
| `temp?`、`[ab]c.txt` | `?` 单字符；`[...]` 字符组（`[!...]` 取反） |
| `# 注释`、空行 | 忽略 |
| 行尾空格 | 未转义的行尾空格会被剥掉 |

**关于 `\#` 与 `\!` 的实测**（很容易写错的一处）：行首的 `#` 会被当成注释，
所以想忽略一个真的叫 `#hash.txt` 的文件，必须转义：

```bash
$ printf '#hash.txt\n'  > .gitignore ; git check-ignore -q '#hash.txt' ; echo $?
1                       # ← 行首 # 被当注释，文件根本没被忽略
$ printf '\\#hash.txt\n' > .gitignore ; git check-ignore -q '#hash.txt' ; echo $?
0                       # ← 转义后才真正生效
```

> **注意**：`git check-ignore` 的**参数是路径**，不是模式——想验证某个模式，
> 要把它写进 `.gitignore` / `.git/info/exclude` / `core.excludesFile` 之后再做检查。
> 我第一版就在这里写错了（以为能直接传模式），实测纠正。

---

### 7.3 `git check-ignore -v`：排查忽略问题的第一工具

它不只回答"是否忽略"，还回答"**被哪个文件的第几行、哪条模式忽略**"：

```bash
$ git check-ignore -v app.log keep.log build/out.o sub/skip.txt deep/x.tmp
.gitignore:4:!keep.log	keep.log
.gitignore:3:build/	build/out.o
.gitignore:5:sub/skip.txt	sub/skip.txt
.gitignore:6:**/deep/*.tmp	deep/x.tmp
```

**格式**：`来源文件:行号:模式<TAB>路径`。

注意第 1 行：`keep.log` 被**负向规则** `!keep.log` 命中——它列出来是因为
"命中的规则是这条"，而 `!` 的含义是"**不要**忽略"。**判断是否真的被忽略要看退出码**：

```bash
$ git check-ignore -q keep.log ; echo $?
1                  # ← 退出码 1 = 没有任何规则让它被忽略（保留）
$ git check-ignore -q app.log ; echo $?
0                  # ← 0 = 被忽略
```

> **踩坑提示**：一次传多个路径时，**只要有任意一个不被忽略，整条命令就退出 1
> 且不打印任何内容**。排查时请一次只查一个，或用 `git check-ignore -v --stdin`。

**其它排查命令**：

| 命令 | 用途 |
| :--- | :--- |
| `git status --ignored` | 连被忽略的文件一起列（`!!` 前缀） |
| `git clean -ndX` | **演习**：列出"只删被忽略文件"会删掉什么（`-n` 演习，`-X` 只删忽略项） |
| `git ls-files --others --exclude-standard` | 未跟踪且未被忽略的文件（**sourcepack 取未跟踪清单用的就是它**） |
| `git check-ignore -v --no-index <p>` | 连**已跟踪**文件也按规则判定（见 7.4） |

实测：

```bash
$ git status --ignored --porcelain
 M .gitignore
?? a/
!! build/
!! deep/
!! sub/skip.txt
$ git clean -ndX
Would remove build/
Would remove deep/
Would remove sub/skip.txt
```

---

### 7.4 **`.gitignore` 只管未跟踪文件**（最常见的误解）

```bash
$ git add -f app.log && git commit -m "强制跟踪 app.log"
$ git ls-files --error-unmatch app.log >/dev/null && echo 已跟踪
已跟踪
$ git check-ignore -q app.log ; echo $?
1                                    # ← 已跟踪文件：check-ignore 直接说"不忽略"
$ git status --porcelain app.log
 M app.log                           # ← 照常显示修改

$ git rm --cached app.log            # 取消跟踪
$ git check-ignore -q app.log ; echo $?
0                                    # ← 现在又被忽略了
```

**规则**：`.gitignore` 的作用是"**决定哪些未跟踪文件不出现在 `git status` / `git add .` 的候选里**"。
一旦文件已进索引，忽略规则对它**完全无效**——它照常显示修改、照常被提交。

**实用推论**：

- "我把它写进 `.gitignore` 了，为什么还在被提交？" → 因为它已经被跟踪，
  需要 `git rm --cached`（保留工作区文件）才会"脱离版本控制"；
- "为什么同事那边这个文件没被忽略？" → `.gitignore` 是**工作区文件**，必须提交并推送才对别人生效；
  本地规则请用 `.git/info/exclude`。

---

### 7.5 嵌套 `.gitignore` 与"就近优先"

规则：**每一层目录的 `.gitignore` 都生效**；冲突时"**更深的目录里的规则**"优先；
同一个文件内"**最后一条命中的规则**"赢。

**实测**：根 `.gitignore` 是 `*.log` + `!keep.log`，而 `sub/.gitignore` 里顺序相反：

```bash
$ cat .gitignore            $ cat sub/.gitignore
*.log                       !keep.log
!keep.log                   *.log

$ for f in root.log keep.log sub/a.log sub/keep.log; do git check-ignore -v $f; done
.gitignore:1:*.log	root.log
.gitignore:2:!keep.log	keep.log          # ← 根目录的 ! 生效：保留
sub/.gitignore:2:*.log	sub/a.log
sub/.gitignore:2:*.log	sub/keep.log      # ← 子目录的 *.log 在 !keep.log 之后 → 忽略
```

**读法**：`sub/keep.log` 先命中 `!keep.log`（第 1 行），再命中 `*.log`（第 2 行），
后者更靠后 → 最终被忽略。**所以"顺序"是 `.gitignore` 里最容易被忽略的语义**。

---

### 7.6 两条反直觉规则（sourcepack 都踩过）

#### 7.6.1 `abc/**` 不匹配 `abc` 自身

```bash
$ printf 'abc/**\n' > .gitignore
$ for p in abc abc/f.txt abc/d/deep.txt abcd; do
    printf "%-16s → %s\n" $p "$(git check-ignore -q $p && echo 忽略 || echo 保留)"; done
abc              → 保留
abc/f.txt        → 忽略
abc/d/deep.txt   → 忽略
abcd             → 保留
```

**原因**：git 的定义是"`abc/**` 匹配 `abc` **内部**的内容"，
所以零层的 `**` 在这里**不被允许**——必须至少有一层。
而相反地，**前导** `**/`（`**/foo`）必须允许零层（否则根目录下的 `foo` 匹配不到）。

**sourcepack 的实现**（`ignore.go:275-279`）：

```go
	// abc/** 表示"abc 内部的内容"（不含 abc 自身）：补一个 * 段要求至少一层，
	// 否则 ** 会匹配零层、把 abc 自己也吞掉。**/ 开头则必须允许零层。
	if len(r.segs) > 1 && r.segs[len(r.segs)-1] == "**" {
		r.segs = append(r.segs, "*")
	}
```

**一行代码就体现了这条规则的方向性**——这也是"自己实现匹配器"最容易写错的地方之一。

#### 7.6.2 父目录被排除后，`!` 救不回来

```bash
$ printf 'a/**/b\n!a/b/c\n' > .gitignore
$ mkdir -p a/b && printf 'x\n' > a/b/c
$ git check-ignore -q a/b    ; echo $?    → 0（a/b 是目录，被 a/**/b 忽略）
$ git check-ignore -q a/b/c  ; echo $?    → 0（仍然被忽略！）
$ git check-ignore -v --no-index a/b/c
.gitignore:1:a/**/b	a/b/c                  # ← 归咎于 a/**/b，而不是!a/b/c
```

**git 的规则**：*"无法重新纳入一个其父目录已被排除的文件"*
（`It is not possible to re-include a file if a parent directory of that file is excluded`）。

**为什么**：Git 遍历到 `a/b` 时发现它被忽略，就**不再往里走**——
`a/b/c` 根本没被"看见"，自然无从被 `!` 救回。

**对照**：如果模式命中**文件**而不是目录，`!` 就有效：

```bash
$ printf '*.tmp\n!keep.tmp\n' > .gitignore && printf 'x\n' > keep.tmp
$ git check-ignore -q keep.tmp ; echo $?  → 1（保留 ✅）
```

**这条规则对工具的影响极大**（sourcepack 的两个模式处理方式不同）：

| 模式 | 怎么处理"目录下的内容" |
| :--- | :--- |
| **全量扫描**（遍历工作区） | 在目录上判定 → 命中就 `SkipDir`，天然满足 git 的规则 |
| **git 变更模式**（清单来自 git，没有遍历） | **必须显式回溯各级父目录**，否则 `a/b/c` 会被当成"未被忽略"漏进产物 |

源码（`ignore.go:101-111`）：

```go
	// 祖先目录被忽略 → 本路径也被忽略，而且**无法用 ! 重新纳入**
	// （git 的原话：不能重新纳入"父目录已被排除"的文件）。
	// 遍历模式靠 SkipDir 达到同样效果，但 git 变更模式没有遍历，必须在这里兜住。
	comps := strings.Split(relPath, "/")
	for i := 1; i < len(comps); i++ {
		if v := m.dirVerdict(strings.Join(comps[:i], "/")); v.ignored {
			return true, true, v.source
		}
	}
```

> **我在实现这块时的真实经历**：对照测试第一次跑就报了一处不一致——
> `a/**/b` 之下 `a/b/c` 我判成"保留"，git 判成"忽略"。
> 这条规则**不是靠想用例能覆盖到的**，是 `git check-ignore` 告诉我答案的。
> 这也说明"以 git 为真值"的测试策略有多值。

---

### 7.7 三种"看起来像忽略、其实不是"的东西

| 机制 | 真实作用 | 常见误用 |
| :--- | :--- | :--- |
| `git update-index --assume-unchanged <f>` | 告诉 Git"别检查这个文件的变化"（性能手段） | 当 `.gitignore` 用 → 克隆到别处就失效，还可能导致改动被静默忽略 |
| `git update-index --skip-worktree <f>` | 让 Git **忽略工作区版本**（用于本地配置覆盖） | 同上；且 `git pull` 撞上它时会报冲突 |
| `.gitattributes` 的 `export-ignore` | **只影响 `git archive`** 打包内容 | 以为它会阻止提交 → 不会 |
| 空文件 `.gitkeep` | 纯约定：让空目录能被提交（Git 不跟踪空目录） | 以为它有特殊功能 → 没有，任何文件名都行 |

---

### 7.8 跨平台的两个坑

#### 7.8.1 路径分隔符：`filepath.Match` 在 Windows 会让 `*` 跨过 `/`

这是 **sourcepack 真实踩到的 bug**（v1.8.2 修复）：

- 我们内部的 `relPath` 一律用 `filepath.ToSlash` 归一（`/` 分隔）；
- 但 `filepath.Match` 是**平台相关**的：Windows 上它把 `\` 当分隔符、把 `/` 当普通字符；
- 于是 `src/*.go` 在 Windows 上会匹配 `src/sub/main.go` —— **过度排除**。

```go
// ignore.go:357
// 用 path.Match 而不是 filepath.Match：后者在 Windows 上把 \ 当分隔符、把 /
// 当普通字符，* 于是会跨过 /（这正是 CI 在 windows-latest 上抓到的差异）。
```

**CI 的报错原文**（三个 Windows job 同时挂）：

```
--- FAIL: TestMatchPattern (0.00s)
    main_test.go:40: matchPattern("src/sub/main.go", "src/*.go") = true, want false
```

**教训**：这类"只在某个平台成立"的差异，**没有对应平台的 CI 就永远发现不了**。
这也是我们把 `windows-latest` 加进矩阵的直接原因——加完当天就抓到它和
另一个 CRLF 问题（`gofmt` 把 CRLF 判成未格式化）。

#### 7.8.2 大小写

| 平台 | 默认行为 |
| :--- | :--- |
| Linux | 大小写敏感（`core.ignoreCase=false`） |
| macOS / Windows | 文件系统大小写不敏感（git 会设 `core.ignoreCase=true`） |

后果：`.gitignore` 里写 `*.LOG` 在 macOS 上会匹配 `x.log`（因为文件系统不敏感），
在 Linux 上不会。**跨平台项目建议两边都写**，或统一小写命名。

---

### 7.9 回到 sourcepack：为什么要自己实现匹配器

一个自然的疑问：既然 sourcepack 到处调 git，**为什么忽略规则要自己写一套**？

答案有三层：

1. **`.gdignore` 是项目自己的机制**——git 完全不认识这个文件，
   而"不改项目 `.gitignore` 也能让快照跳过某些文件"是这个工具的特性；
2. **全量扫描必须在"不是 git 仓库"的目录里工作**（`gx -d /tmp/foo` 很常见）；
3. **git 模式虽然由 git 给清单，但 `.gdignore` 仍要叠加在 git 的结论之上**。

#### 7.9.1 语义实现要点（`ignore.go`）

| 环节 | 做法 |
| :--- | :--- |
| 解析 | `parseIgnoreLine`：`#` 注释、`\!`/`\#` 字面量、`!` 负向、尾部 `/` 目录、行尾空格 |
| 切段 | 按 `/` 切；末尾 `**` 补一个 `*` 段（7.6.1） |
| 匹配 | `matchSegments` 支持 `**` 匹配零到多段；`segmentMatch` 用 **`path.Match`** |
| 目录规则 | `dirOnly` 命中任意一层父目录即命中（7.6.2） |
| 祖先传递 | `dirVerdict` + 缓存（避免深目录 O(深度²)） |
| 优先级 | 浅→深 `.gitignore` → 浅→深 `.gdignore` → 命令行 `-I` |

**"`.gdignore` 永远压过 `.gitignore`"这条是我们自己的规定**（git 没有这个概念）：
实现方式是"先遍历 `.gitignore` 的全部规则，再遍历 `.gdignore` 的"，
于是后者天然是"最后命中的规则"。

#### 7.9.2 以 git 为真值的对照测试

`ignore_test.go` 里有一张**模式表 × 路径表**，逐个对比 `git check-ignore` 的结论：

```go
// 当前规模：14 个模式 × 40 条路径，完全一致
var oraclePatterns = []string{
	"*.log", "**/*.log", "**/*.tmp", "src/**/*.go", "**/node_modules",
	"a/**/b", "abc/**", "build/", "/rooted.txt", "docs/**/*.md",
	"*.min.js", "temp?", "[ab]racket.txt", "keep/**/deep.txt",
}
```

**为什么这比"多写几个用例"强**：模式的组合爆炸（锚定 × `**` 位置 × 目录规则 × 负向）
不是靠列举能覆盖的，而把判定权交给 git，就变成了"对齐一个已有实现"。

---

### 7.10 动手实验 `[lab]`

```bash
cd /tmp/git-lab
git checkout -q -B lab7 HEAD && git clean -qfd

# 1) 谁忽略的？—— check-ignore -v 是最强工具
cat > .gitignore <<'EOF'
*.log
!keep.log
build/
a/**/b
abc/**
EOF
mkdir -p build a/b abc/d && for f in app.log keep.log build/o.o a/b/c abc/f.txt; do mkdir -p $(dirname $f); echo x > $f; done
git check-ignore -v app.log keep.log build/o.o              # 来源文件:行号:模式
git check-ignore -q keep.log ; echo "keep.log 退出码 $?"    # 1 = 保留

# 2) 已跟踪文件不受忽略规则影响
git add -f app.log && git commit -qm "track app.log"
git check-ignore -q app.log ; echo $?        # 1：git 说"不忽略"（已跟踪）
git rm --cached -q app.log
git check-ignore -q app.log ; echo $?        # 0：回到未跟踪，又被忽略

# 3) 嵌套与顺序
printf '*.log\n!keep.log\n' > .gitignore ; mkdir -p sub
printf '!keep.log\n*.log\n' > sub/.gitignore ; touch sub/keep.log
git check-ignore -v sub/keep.log             # 子目录的第 2 行赢

# 4) 两个反直觉规则
printf 'abc/**\n' > .gitignore
git check-ignore -q abc ; echo "abc 退出码 $?（1=保留）"
printf 'a/**/b\n!a/b/c\n' > .gitignore
git check-ignore -v --no-index a/b/c         # 归咎于 a/**/b，! 无效

# 5) 三种来源
echo local-noise.txt > .git/info/exclude
printf 'global-noise.tmp\n' > /tmp/gx.txt && git config core.excludesFile /tmp/gx.txt
touch local-noise.txt global-noise.tmp
git check-ignore -v local-noise.txt global-noise.tmp
git config --unset core.excludesFile

# 6) 排查工具箱
git status --ignored --porcelain | head
git clean -ndX | head
git ls-files --others --exclude-standard
```

**自测题**：

1. `.gitignore` 为什么管不住已经提交的文件？怎么让一个文件"脱离版本控制但保留在工作区"？
2. `abc/**` 与 `**/abc` 的区别是什么？各自匹配到什么？
3. `!` 什么情况下"不起作用"？为什么？
4. `git check-ignore -v` 输出 `.gitignore:4:!keep.log	keep.log`，说明这个文件被忽略了还是被保留了？
5. 为什么 sourcepack 不干脆让 git 来决定一切忽略（而要实现 `.gdignore` 匹配器）？
6. `filepath.Match` 与 `path.Match` 在 Windows 上的差别会导致什么后果？

---

### 7.11 小结

- 忽略规则有**五个来源**、优先级不同：命令行 > `.gitignore`（就近/深者优先）
  > `.git/info/exclude` > `core.excludesFile`；`.gitattributes` 的 `export-ignore` 与忽略无关；
- 同一文件内**最后一条命中的规则**生效——顺序是语义的一部分（实测 `sub/keep.log` 的例子）；
- **`.gitignore` 只管未跟踪文件**：已跟踪文件照常修改与提交（`check-ignore` 对已跟踪文件退出 1，
  `git rm --cached` 后变 0）；
- `git check-ignore -v` 给出`来源文件:行号:模式`，是排查的第一工具；
  但它**混入未忽略路径时会整体退出 1 且不打印**；
- 两条反直觉规则：**`abc/**` 不含自身**（sourcepack 补一个 `*` 段）、
  **父目录被排除后 `!` 无效**（遍历靠 `SkipDir`、git 模式靠显式回溯祖先）；
- 跨平台坑：`filepath.Match` 在 Windows 让 `*` 跨过 `/`（我们改用 `path.Match`，
  并由 `windows-latest` CI 抓到）、大小写敏感性差异；
- sourcepack 自己实现匹配器的三层理由（`.gdignore` 是自有机制、非 git 目录要能扫、
  git 模式仍需叠加），并用 **14 模式 × 40 路径**的 `git check-ignore` 对照测试保证语义对齐。

**下一章**：远端与传输——refspec 语法、`fetch`/`push` 到底交换了什么、
packfile 协商与"增量传输"、`force-with-lease` 为什么比 `--force` 安全，
以及 sourcepack 的推送（`-p`）与 git 传输的区别（我们推的是产物，不是提交）。

---

<a id="第-8-章远端与传输"></a>

# 第 8 章　远端与传输

---

> **本章目标**：去掉"远端很神秘"的感觉。学完你能解释：
> `fetch` 到底改动了本地哪些文件？`+refs/heads/*:refs/remotes/origin/*` 每个符号什么意思？
> 为什么 `--force-with-lease` 能救命而 `--force` 会毁掉别人的提交？
> 以及**传输为什么是增量的**（有实测数字）。
>
> 本章所有实验都在本地用"裸仓库当远端"完成——不需要网络，也不需要任何服务。

---

### 8.1 远端就是一个普通的仓库（只是没有工作区）

```bash
$ git init -q --bare origin.git
$ ls origin.git
HEAD  config  description  hooks  info  objects  refs
```

**没有 `index`、没有工作区文件**——其余结构与第 1 章讲的完全一样：
对象库 + 引用。所谓"远端"只是**另一个可以被 fetch/push 的仓库**，
它并不特殊（第 1 章的分布式）：

```bash
$ git -C origin.git rev-parse --short main
6c71e63
$ git -C origin.git count-objects -v | grep ^count
count: 3              # 只有 3 个对象：1 commit + 1 tree + 1 blob
```

> **推论**：`git clone /path/to/repo` 与 `git clone https://...` 在**对象层面没有区别**，
> 只是"怎么把对象搬过来"的传输方式不同。这也是本章能用本地路径做全部实验的原因。

---

### 8.2 fetch 与 push：搬什么

| 操作 | 方向 | 搬动的东西 |
| :--- | :--- | :--- |
| `git fetch` | 远端 → 本地 | **对象**（补全本地缺的）+ **远端跟踪引用**（`refs/remotes/...`） |
| `git push` | 本地 → 远端 | 对象 + **远端的分支引用**（`refs/heads/...`） |

**关键差异**：`fetch` 只更新 `refs/remotes/origin/*`，**不碰你的本地分支、不动工作区**；
`push` 会移动远端仓库里的分支引用（这是"改变别人能看到的东西"，所以要保护——见 8.5）。

```bash
$ git config remote.origin.fetch
+refs/heads/*:refs/remotes/origin/*
```

---

### 8.3 refspec 语法拆解

`+refs/heads/*:refs/remotes/origin/*` 逐部分看：

| 部分 | 含义 |
| :--- | :--- |
| `+` | **允许非快进更新**（fetch 时很常见：远端被 force push 过，本地要跟着走） |
| `refs/heads/*` | 源（远端这一侧）的所有分支 |
| `:` | "映射到" |
| `refs/remotes/origin/*` | 目标（本地这一侧）的远端跟踪引用 |

**push 的显式形式**（实测）：

```bash
$ git push origin HEAD:refs/heads/bob-feature
$ git -C origin.git branch
  bob-feature
  main
```

**常用简写**：

| 写法 | 含义 |
| :--- | :--- |
| `git push origin main` | `refs/heads/main:refs/heads/main`（同名） |
| `git push origin HEAD` | 把当前提交推到远端**同名分支** |
| `git push origin :branch` | 源为空 → **删除**远端分支 |
| `git fetch origin main` | 只取 main（仍然写进 `refs/remotes/origin/main`） |
| `git fetch origin main:tmp` | 直接写进本地 `tmp` 分支（不经过 `refs/remotes`） |

---

### 8.4 `fetch` 具体改了什么

```bash
$ git branch -a           # fetch 前
（空 —— bob 刚克隆完空仓库时）
$ git fetch origin
$ git branch -a
* main
  remotes/origin/main
$ git rev-parse --short refs/remotes/origin/main
6c71e63
$ head -c 40 .git/FETCH_HEAD
6c71e63e84e134716878571e751ae6d738bd3a53
```

三件事：① 下载缺的对象；② 更新 `refs/remotes/origin/main`；③ 写 `FETCH_HEAD`
（记录"这次抓到了什么"，`git merge FETCH_HEAD` / `git pull` 会用到）。

**没有动的**：你的 `main`、你的工作区、你的索引。**所以 `fetch` 永远是安全的**。

---

### 8.5 非快进推送会被拒绝

**实测**：alice 改写了自己已推送的提交（`--amend`），再推：

```bash
$ git commit --amend -m "alice: 第二次提交（改写）"
$ git push origin main
 ! [rejected]        main -> main (non-fast-forward)
hint: Updates were rejected because the tip of your current branch is behind
hint: its remote counterpart. Integrate the remote changes (e.g. 'git pull ...')
$ echo $?
1
```

**为什么拒绝**：如果允许，远端历史里那个旧提交会"消失"——
**已经在别人机器上的提交可能被无声丢掉**。Git 默认把这个危险动作挡下来。

**三种正当的处理方式**：

| 场景 | 做法 |
| :--- | :--- |
| 别人推了、我没推 | `git pull --rebase` 后正常 push |
| 我确实在整理自己的分支（未共享） | `git push --force-with-lease` |
| 危险动作（明确知道后果） | `git push --force` |

---

### 8.6 `--force` vs `--force-with-lease`：谁保护了你

`--force-with-lease` 的含义：**"只有当远端仍是我上次看到的样子，才允许我覆盖"**。
如果在你 fetch 之后别人推了新提交，它会**拒绝**。

**实测（两个克隆 a1 / b1 打同一个裸仓库）**：

```bash
# b1 先记录远端状态
$ git rev-parse --short refs/remotes/origin/main
9c33fa9
# 此时 a1 又推了一次 → 远端前进到 6de79ab（b1 的记录过期了）

# b1 想强制推：
$ git push --force-with-lease origin main
 ! [rejected]        main -> main (stale info)          # ← 被保护了
$ echo $?
1

# 换成 --force：
$ git push --force origin main
$ echo $?
0
$ git -C ../shared.git log --oneline | grep -c 'A: 第二个提交'
0                     # ← A 的提交被彻底丢掉
```

**对照实验的价值**：同一份工作、同一个时刻，`--force` 会**静默删除同事的提交**；
`--force-with-lease` 用一句 `stale info` 拦住了它。

> **实践建议**：把 `--force-with-lease` 当成习惯；
> 只有"这个分支百分之百只有我在用"时才用 `--force`。

---

### 8.7 传输是增量的：want/have 协商与量化

`fetch`/`push` 不会重传整个仓库。客户端会告诉服务端：
**"我要这个（want）"+"我已经有这些（have）"**，服务端据此算出一个**最小 packfile**。

**用 `GIT_TRACE_PACKET` 看协议**（实测片段）：

```
fetch> want 6914313854f6387aa59a3d3e196706697312b500
fetch> have aed8c2cf8adfdfa59002a10df35c633a6ed2e05a
fetch> have 46a8fe2e6c0953133fe2f76b4bf0a2c2ff341bb3
fetch> have 5830959350cc59ee34f77bf8495b11db5bd2a9ab
...（一长串 have，直到服务端认为"够了"）
```

**增量性的量化实测**（用 sourcepack 的真实历史，143 个提交 / 697 个对象）：

| 步骤 | 裸仓库对象数 |
| :--- | ---: |
| 初始克隆 | **697** |
| 生产者推入 1 个提交（1 blob + 1 tree + 1 commit） | **700**（+3） |
| 消费者首次克隆 | 700 |
| 生产者再推 1 个提交 | — |
| 消费者 `git fetch` | **703**（**+3**，而不是重传 700） |

**这就是 Git 能在几千个提交的仓库上"秒级 fetch"的原因**：
传输量正比于"新增对象"，而不是仓库总大小。

> **与第 10 章的联系**：服务端那边还会用 **bitmap**、**commit-graph** 加速
> "算哪些对象要发"这一步；客户端有 **partial clone**（`--filter=blob:none`）
> 进一步把"要的对象"变少。

---

### 8.8 协议、认证与"卡住"的坑

| 协议 | 特点 |
| :--- | :--- |
| `ssh://` / `git@host:path` | 用 SSH 密钥认证；企业内网最常见 |
| `https://` | 用 token/密码；对防火墙友好；goreleaser 与 CI 常用 |
| `git://` | 匿名只读；**没有认证也没有加密**，公网基本已淘汰 |
| 本地路径 | 直接复制/硬链接对象，最快（本章实验全用它） |

**在脚本/自动化里必须防"卡住"**：git 可能提示输入密码而永久等待。
Sourcepack 的做法（`changeset.go` 的 `gitRunner`）：

| 措施 | 作用 |
| :--- | :--- |
| `GIT_TERMINAL_PROMPT=0` | 禁止弹出凭据提示，宁可失败也不要挂起 |
| `GIT_PAGER=cat` + `--no-pager` | 禁止分页器（否则在管道里会挂住） |
| `cmd.Stdin = nil` | 没有 stdin，也就不会有交互 |
| `GIT_OPTIONAL_LOCKS=0` | 只读操作不去抢 index 锁 |
| `-c core.quotepath=false` | 否则中文文件名在 patch 里会变成 `\344\275\240` 这类八进制转义（第 6 章的路径解析会全错） |

> 这几条**不是洁癖**：在 CI、agent、cron 里，一个"等待输入密码"的 git 进程
> 会让整条流水线挂到超时。第 12 章会把它们逐条讲透。

---

### 8.9 "推不上去"排查清单

| 现象 | 原因 | 处理 |
| :--- | :--- | :--- |
| `non-fast-forward` | 远端有你没有的提交 | `git pull --rebase` 后再推 |
| `stale info` | `--force-with-lease` 发现远端变了 | 先 `git fetch`，确认后再推 |
| `remote: Permission denied` | 认证/权限问题 | 检查 SSH key 或 token |
| `src refspec X does not match any` | 本地根本没有这个分支名（**我本轮就踩到了**：裸仓库的默认分支是 `git-course`，而我推的是 `main`） | 确认分支名，或用 `HEAD:refs/heads/X` |
| `Everything up-to-date` | 引用没变化 | 正常 |
| 卡住不动 | 在等密码或分页 | 设 `GIT_TERMINAL_PROMPT=0`、`GIT_PAGER=cat` |

---

### 8.10 回到 sourcepack：我们**不用** git 传输

一个必须说清的区别：**sourcepack 的 `-p`（推送）与 git 的远端传输毫无关系**。

| | git push | sourcepack `-p` |
| :--- | :--- | :--- |
| 推的是什么 | **对象**（commit/tree/blob） | **生成的 Markdown 产物** |
| 目标 | git 仓库（引用会移动） | 任意 HTTP 端点（`SOURCEPACK_PUSH_URL`） |
| 协议 | git 协议（pkt-line/packfile） | `multipart/form-data` POST |
| 认证 | SSH key / token | `X-Auth-Key` 头 |
| 副作用 | 改变远端分支 | 对方收到一份文本 |

实测（本地 HTTP 服务接收）：

```bash
$ gx -d . -p -u http://127.0.0.1:18099/submit --auth-key SECRET123
✨ Done! Pushed to http://127.0.0.1:18099/submit in 9.88ms
# 服务端收到：POST /submit，X-Auth-Key: SECRET123，multipart 303817 字节
```

**另外两点**：

1. **sourcepack 从不做网络 git 操作**：`-g`/`-G` 只读本地对象库（第 1 章），
   所以它**不需要任何 remote 权限**，甚至可以在没有任何 remote 的仓库里工作；
2. **tag 与发布**：本章讲的 push 在 sourcepack 自己的开发里是"推 tag 触发 CI"
   （第 11 章会讲 `release.yml`：`on: push: tags: ['v*']` → goreleaser → npm publish）。
   **tag 也是一种引用**（第 3 章），推它和推分支走同一套 refspec 机制：

```bash
$ git push origin v2.0.1          # 等价于 refs/tags/v2.0.1:refs/tags/v2.0.1

# 实测：来源是本地（这里直接问一个裸仓库）
$ git ls-remote --tags up.git | tail -3
224230fc78835daac526f7402404f3aba09426c6	refs/tags/v2.0.0^{}
c7a057a324abc15567c01951c1003be614f94608	refs/tags/v2.0.1
aa686d8e14f8fc729a32a30d42dbf32c0b94b29f	refs/tags/v2.0.1^{}
```

**注意附注标签会出现两行**：`refs/tags/v2.0.1`（tag 对象本身）+ `refs/tags/v2.0.1^{}`
（**剥壳后**指向的提交）——这正是第 3 章 `^{commit}` 与第 5 章剥壳在传输层面的表现。

---

### 8.11 动手实验 `[lab]`

```bash
# 全程不需要网络：用裸仓库当远端
rm -rf /tmp/rl && mkdir -p /tmp/rl && cd /tmp/rl
git init -q --bare origin.git
git clone -q origin.git a && git clone -q origin.git b
cd a && git config user.email a@x.c && git config user.name A
printf 'v1\n' > f.txt && git add f.txt && git commit -qm "A: init"
git push -q origin HEAD && echo "  默认分支: $(git branch --show-current)"

# 1) 远端结构
ls ../origin.git
git -C ../origin.git count-objects -v | grep -E "^(count|in-pack)"

# 2) refspec
git config remote.origin.fetch

# 3) fetch 改了什么
cd ../b && git config user.email b@x.c && git config user.name B
git branch -a ; git fetch origin ; git branch -a
cat .git/FETCH_HEAD | head -1

# 4) 非快进被拒
cd ../a && printf 'v2\n' >> f.txt && git commit -qam "A: 2" && git push -q origin HEAD
git commit -q --amend -m "A: 2（改写）"
git push origin HEAD ; echo "退出码 $?"

# 5) --force vs --force-with-lease
cd ../b && git fetch -q origin && printf 'b\n' > b.txt && git add b.txt && git commit -qm "B"
cd ../a && git push -q --force origin HEAD          # 先让 b 的记录过期
cd ../b && git push --force-with-lease origin HEAD ; echo "退出码 $?"   # 应被拒（stale info）
git push --force origin HEAD ; echo "退出码 $?"      # 会成功（并把 A 的提交丢掉）

# 6) 看协商
cd ../a && printf 'v3\n' >> f.txt && git commit -qam "A: 3"
GIT_TRACE_PACKET=1 git push origin HEAD 2>&1 | grep -E "want|have" | head -5
```

**自测题**：

1. `+refs/heads/*:refs/remotes/origin/*` 里那个 `+` 是干什么的？没有它会怎样？
2. `git fetch` 会改动你的本地分支或工作区吗？
3. 为什么 Git 默认拒绝非快进的 push？
4. `--force-with-lease` 保护的是谁？（用 8.6 的实验回答）
5. 一次 `fetch` 传输的对象数量取决于什么？
6. sourcepack 的 `-p` 与 `git push` 有什么本质区别？为什么我们的工具不需要 remote 权限？

---

### 8.12 小结

- **远端就是一个没有工作区的普通仓库**（对象库 + 引用），本地路径也能当远端，
  这使得传输机制完全可以在没有网络的情况下研究；
- `fetch` 搬"对象 + `refs/remotes/*`"，**永远安全**（不动本地分支与工作区）；
  `push` 会移动远端引用，所以有**非快进保护**（实测 `rejected (non-fast-forward)`，退出码 1）；
- **refspec** 是"源:目标"的映射语言，`+` 允许非快进、`:` 前留空表示删除；
- **`--force-with-lease` 是协作安全的强制推送**：本地记录过期时拒绝
  （实测 `rejected (stale info)`），而 `--force` 会静默丢掉别人的提交（实测确认）；
- 传输是**增量**的：靠 `want`/`have` 协商算出最小 packfile
  （实测 697 → 700 → 703，每次只多 3 个对象，而不是重传全部）；
- 自动化里必须防挂起：`GIT_TERMINAL_PROMPT=0`、`GIT_PAGER=cat`、`Stdin=nil`、
  `GIT_OPTIONAL_LOCKS=0`（sourcepack 逐条都做了，第 12 章细讲）；
- **sourcepack 的 `-p` 与 git 传输无关**：它 POST 的是产出的 Markdown；
  工具本身只读本地对象库，**不需要任何 remote 权限**。

**下一章**：历史改写与恢复——`rebase` 到底重写了什么、`reset --soft/--mixed/--hard`
对三棵树的影响（第 4 章的三棵树在这里第二次出场）、`cherry-pick`/`revert` 的差别，
以及"提交丢了"的三层找回手段（reflog → fsck → 反查对象）。

---

<a id="第四部改写性能与工程"></a>

# 第四部　改写、性能与工程

> 会改写历史才敢用 Git；知道它为什么快，才敢在大仓库里用它。

---

<a id="第-9-章历史改写与恢复"></a>

# 第 9 章　历史改写与恢复

---

> **本章目标**：把"改写历史"从玄学变成可推理的操作。
> 核心只有一句话：**对象不可变（第 2 章），所以改写 = 造一批新对象 + 把引用指过去**。
> 学完你能预测每个命令会动三棵树里的哪几棵、哪些哈希会变、
> 以及**事故之后按什么顺序救**。

---

### 9.1 改写的本质

| 你以为的 | 实际发生的 |
| :--- | :--- |
| "`amend` 修改了那个提交" | 造了一个**新**提交对象，把分支引用挪过去；旧的仍在对象库里 |
| "`rebase` 移动了提交" | 把每个提交**重新应用到新基底**，产生一串**全新**提交 |
| "`reset` 删掉了提交" | 只把引用往回挪；对象仍在（reflog 还引用着它） |

**这就是"几乎所有误操作都能恢复"的根基**：Git 从不修改对象，只移动引用（第 3 章）。

---

### 9.2 `reset` 的三种模式：动三棵树里的哪几棵

用第 4 章的三棵树（HEAD / 索引 / 工作区）来理解，一次讲清：

| 模式 | HEAD（当前分支） | 索引 | 工作区 | 典型用途 |
| :--- | :---: | :---: | :---: | :--- |
| `--soft` | ✅ 移动 | ❌ | ❌ | 撤销提交、保留改动在**暂存区**（重新组织提交） |
| `--mixed`（默认） | ✅ | ✅ 重置 | ❌ | 撤销提交与暂存，**改动留在工作区** |
| `--hard` | ✅ | ✅ | ✅ **覆盖** | 彻底丢弃（⚠️ 工作区改动会消失） |

**实测**（固定夹具：`c1=99592e1`、`c2=54fb056`，每次都从 `c2` 干净状态出发）：

| 探测 | 起点 | `--soft c1` | `--mixed c1` | `--hard c1` |
| :--- | :--- | :--- | :--- | :--- |
| `HEAD` | 54fb056 | 99592e1 | 99592e1 | 99592e1 |
| 索引里有 `g.txt` | 1 | **1** | **0** | 0 |
| 工作区有 `g.txt` | 1 | **1** | **1** | **0** |
| `git status` 残留 | 干净 | `M  f.txt` `A  g.txt`（已暂存） | ` M f.txt` `?? g.txt`（未暂存/未跟踪） | 干净（改动消失） |

**读法**：`--soft` 之后那些改动全在**暂存区**（`A ` 表示已 add），
`--mixed` 之后掉回**工作区**（`??` 未跟踪），`--hard` 之后**连文件都不在了**。

```bash
# 被 --hard 丢掉的提交还在吗？
$ git log --oneline -1 54fb056
54fb056 c2                                # ← 按哈希仍可读
$ git fsck --unreachable | grep -c 54fb056
0                                         # ← 甚至不算"吊对象"：reflog 还引用着它
```

> **实用模式**：`git reset --soft HEAD~1` 是"把上一个提交拆开重做"的起手式；
> `--hard` 是唯一会**丢工作区改动**的模式（且未提交的改动真的很多都救不回来，
> 它们从没进过对象库）。

---

### 9.3 `revert`：只增不改的反向提交

**实测**（三个线性提交 `c-a`/`c-b`/`c-c`）：

```bash
$ git revert --no-edit 889875e        # 撤销中间的 c-b
$ git log --oneline
d2caee3 Revert "c-b: 加 b.txt"        # ← 新提交
af88922 c-c: 加 c.txt
889875e c-b: 加 b.txt                 # ← 原提交仍在历史里
18755e3 c-a: 加 a.txt
$ test -f b.txt || echo "b.txt 被 revert 删掉了"
b.txt 被 revert 删掉了
$ git log --oneline | grep -c 'c-b: 加 b.txt'
2                                     # 一条是原提交，一条是它的 Revert
```

**关键性质**：

- **历史只增长**：不重写任何既有提交 → **已经推送的历史可以安全 revert**；
- 适合**公共分支**（main / release）：撤销某次改动而不影响别人的 clone；
- 代价：历史里会留下"加了又撤"的痕迹（对审计反而是好事）。

---

### 9.4 `cherry-pick`：复制"改动"，不是复制提交

```bash
$ git checkout -B side <root>
$ git cherry-pick 889875e            # 把 c-b 的改动打到 side 上
$ git log --oneline -2
d9f7adc c-b: 加 b.txt                # ← 新哈希：d9f7adc ≠ 889875e
e1ad902 side: 独立提交
$ test -f b.txt && echo "b.txt 回来了"
b.txt 回来了
```

**机制**：把该提交的 diff（第 6 章）重新应用一次，生成**新提交**。
因为父提交不同，哈希必然不同。

**用途**：把修复从 main 挑到 release 分支（backport）、把某个提交搬到另一条分支。

> **和 `rebase` 的关系**：`rebase` 本质就是"对一串提交自动做 cherry-pick"（见 9.6）。

---

### 9.5 `amend`：最常用的"改写"

```bash
$ git rev-parse --short HEAD            # 前
54fb056
$ git add h.txt && git commit --amend --no-edit
$ git rev-parse --short HEAD            # 后
7b194b5                                 # ← 新对象
$ git log --oneline -1 54fb056          # 旧对象仍可读
54fb056 c2
```

**但要注意**：`--amend` 之后，如果该提交**已经推送过**，你需要 force push，
并且**其他人的历史会与你不一致**（第 8 章的 `--force-with-lease` 出场）。

---

### 9.6 `rebase`：把提交"重放"到新基底

**实测**：`feat` 分支有两个提交，基线前进后执行 `git rebase mainline`：

```bash
# 重放前
$ git log --oneline feat -2
b5e1aee feat: 2
0fcdaaf feat: 1

$ git checkout feat && git rebase mainline

# 重放后
$ git log --oneline feat -2
8e6fb08 feat: 2                      # ← 提交信息一样
ba860b5 feat: 1                      # ← 哈希全变了
```

**为什么哈希全变**：`feat: 1` 的父提交从 `c1` 变成了 `mainline` 的尖端，
而 commit 的哈希包含 `parent` 字段（第 2 章）→ 它变了，`feat: 2` 的父提交跟着变 → 连锁反应。

#### 交互式 `rebase -i`：squash / reword / drop

**squash（合并提交）实测**：

```bash
$ git log --oneline                    # 4 个提交（base + wip 1/2/3）
71d6ba4 wip 3
620c53d wip 2
9a61c5c wip 1
5eba07e base

$ GIT_SEQUENCE_EDITOR="sed -i '' '3,4s/^pick/squash/'" GIT_EDITOR=true git rebase -i -q HEAD~3
$ git log --oneline                    # 变成 3 个
9d06044 wip 2
9a61c5c wip 1
5eba07e base
$ cat w.txt
1 2 3                                  # ← 内容一字未变，只是提交被压扁了
```

> `GIT_SEQUENCE_EDITOR` 改"要做哪些操作"（pick/squash/reword/drop），
> `GIT_EDITOR` 改"提交信息"。两者都设才能完全非交互，适合写脚本与教学演示。

**reword（改提交信息）实测**——注意下游哈希全部变化：

```bash
$ git log --oneline -3
02fe3e4 wip 3   49cfc8f wip 2   3b80547 wip 1

$ GIT_SEQUENCE_EDITOR="sed -i '' '2s/^pick/reword/'" \
  GIT_EDITOR="sed -i '' 's/^wip 1/feat: 初始化 w.txt/'" git rebase -i -q --root

$ git log --oneline -3
5e74cb3 wip 3   2f606d0 wip 2   b41185a feat: 初始化 w.txt
```

**只改了一条信息，三个哈希全变**——这是"改写历史影响面"的最直观演示。

**drop（丢弃提交）实测**：我试着 drop 掉 `wip 2`，结果**撞上冲突**：

```
Could not apply 5e74cb3... wip 3
```

**原因**：`wip 3` 是在 `wip 2` 的基础上继续写的，drop 掉中间那个提交后，
`wip 3` 的改动不再能干净地应用（工作区出现冲突标记）。
**结论**：`drop` 不是"删除文件"，而是"撤掉这次改动"；**后续提交若依赖它，必然冲突**。

---

### 9.7 改写之后，什么会"跟不上"

| 东西 | 会怎样 | 应对 |
| :--- | :--- | :--- |
| **tag** | **不跟随**（实测：`tag` 仍指向旧哈希，而分支已前移） | 发布锚点用 tag 正是这个原因；要移动就 `git tag -f` |
| 别人 clone 的历史 | 与你的不一致，推送需要 force | `--force-with-lease` + 提前通知（第 8 章） |
| CI 里缓存的哈希 | 失效 | 用分支名/tag，别缓存提交哈希 |
| **sourcepack 生成的产物** | 里面记录的短哈希**对不上**了 | 见 9.10 |
| `git blame` 的作者归属 | 改写会把 author 保留、committer 变成你（第 2 章的字段差异） | 正常现象 |

**tag 实测**：

```bash
$ git tag -a my-release -m "发布" HEAD
$ git rev-parse --short my-release^{commit}
d349aec
$ git reset --hard HEAD~1               # 历史被回退（模拟改写）
$ git rev-parse --short my-release^{commit}
d349aec                                 # ← tag 一动不动
$ git rev-parse --short HEAD
b0add57                                 # ← 分支走了
```

---

### 9.8 恢复的三层：ORIG_HEAD → reflog → fsck

**第一层：`ORIG_HEAD`（一枪救回）**——`reset`/`merge`/`rebase` 之前的位置会被记下来：

```bash
$ git log --oneline -1
d2caee3 Revert "c-b: 加 b.txt"
$ printf 'oops\n' > oops.txt && git add oops.txt && git commit -m "重要提交（会被搞丢）"
$ git log --oneline -1
b0add57 重要提交（会被搞丢）

$ git reset --hard HEAD~2                # 事故：回退过头
$ git log --oneline -1
af88922 c-c: 加 c.txt                    # 重要提交不见了，oops.txt 也没了

$ git rev-parse --short ORIG_HEAD
b0add57                                  # ← 正是那个提交
$ git reset --hard ORIG_HEAD
$ git log --oneline -1 ; test -f oops.txt && echo "oops.txt 回来了"
b0add57 重要提交（会被搞丢）
oops.txt 回来了
```

**第二层：`reflog`（完整的引用移动史）**——`ORIG_HEAD` 会被下一次操作覆盖，
但 reflog 记录了每一次移动：

```bash
$ git reflog -2
af88922 HEAD@{0}: reset: moving to HEAD~2
b0add57 HEAD@{1}: commit: 重要提交（会被搞丢）
```

于是 `git reset --hard HEAD@{1}` 等价于上面的救援。

**第三层：`fsck`（最后手段）**——reflog 过期、引用全无时，
仍可用 `git fsck --lost-found` 找 dangling 对象（第 3 章实测过 6 个 dangling commit），
再 `git branch rescue <sha>` 捞回。

**预防胜过抢救**：动手改写前先 `git branch backup` 或 `git tag save-point`
（成本 41 字节，第 3 章）。

---

### 9.9 清理历史的两把刀

| 工具 | 说明 |
| :--- | :--- |
| `git filter-branch` | 老工具（重写全部历史，很慢）。**Git 已不推荐**，会留下 `refs/original/*` 备份引用 |
| `git filter-repo` | 官方推荐的替代品（需单独安装），快且安全默认更多 |

**铁律**（改写历史的社会性约束）：

1. **只在自己独占的分支/仓库上做**；
2. 共享分支改写前必须通知所有人——他们**必须重新 clone**（或至少 `fetch` + `reset --hard`），
   因为哈希全变，旧 clone 会与你的历史"分叉"；
3. 改写前先做备份 tag（9.8 第二层）。

---

### 9.10 回到 sourcepack：我们不改写历史，但会被它影响

#### 9.10.1 只读工具：从不参与改写

sourcepack **生产代码**里发出的 git 子命令只有五种，全部只读：

```bash
# changeset.go 里实际用到的子命令
diff        # 拿清单与 patch
log         # -G 的逐提交遍历
ls-files    # 未跟踪文件清单
rev-parse   # 解析 revspec / HEAD / --show-toplevel / --show-prefix
symbolic-ref # 取当前分支名（detached 时会失败，见第 3 章）
```

```bash
$ grep -oE '"(diff|log|ls-files|rev-parse|symbolic-ref)"' changeset.go | sort -u | tr '\n' ' '
"diff" "log" "ls-files" "rev-parse" "symbolic-ref"
$ grep -c '"commit"\|"push"\|"reset"\|"rebase"' changeset.go
0
```

**没有任何** `commit`/`reset`/`rebase`/`checkout`/`add` 这类会改动仓库状态的调用。
（`main.go` 里有 `"push"` 字样，但那是我们自己的 `-p` 推送与 HTTP 请求，
**不是 `git push`**；`changeset_test.go` 等测试文件里确实有 `add`/`commit`/`init`/`merge`，
那是**测试夹具**在搭建场景。）

**工程含义**：这个工具可以在任何仓库里安全运行——没有副作用、不需要备份、
不需要权限（第 8 章），甚至可以在 CI 的只读 checkout 里跑。

#### 9.10.2 但产物里记着哈希——改写会让它"对不上"

`-g`/`-G` 的产物会写上 HEAD 的短哈希（`shortSHA`，取前 7 位）：

```bash
$ gx -G 1 --dry-run -d . | grep HEAD
    HEAD:                7ae297f docs(git-course): 第 8 章——远端与传输（含 force-with-lease 对照实验与增量传输量化）
```

**真实场景**：你把这份产物发给了同事（或喂给了模型），
然后你 rebase/amend 了那个提交——产物里的 `7ae297f` 在新历史里找不到了。
这不是 bug，而是"**哈希是内容的函数**"（第 2 章）的直接后果。

**实用推论**：

- 需要长期引用某次审查 → **打 tag**（tag 不跟随改写，9.7 已实测）；
- 想让产物里的哈希稳定 → 在**已冻结**的提交上生成（例如合并后的 main、或 tag 指向的提交）；
- 这正是我们 **release 流程用 tag 触发**（`on: push: tags: ['v*']`）的原因：
  分支会动，tag 是锚点。

#### 9.10.3 `--rev` 支持 reflog 语法，改写后特别好用

`HEAD@{1}` 是合法 revspec（第 3 章实测过），于是"刚 rebase 完，看看改了什么"变成：

```bash
$ gx -g --rev 'HEAD@{1}' --dry-run -d .
📦 Git changes: git diff HEAD@{1}  (working tree + index vs HEAD@{1})
```

**这比记住旧哈希方便得多**——因为 reflog 里就写着"上一步在哪"。

---

### 9.11 动手实验 `[lab]`

```bash
# A. reset 三模式 × 三棵树
rm -rf /tmp/l9 && mkdir /tmp/l9 && cd /tmp/l9 && git init -q -b main . \
  && git config user.email t@t.c && git config user.name T
printf 'v1\n' > f.txt && git add -A && git commit -qm c1 && C1=$(git rev-parse HEAD)
printf 'v2\n' >> f.txt && printf 'new\n' > g.txt && git add -A && git commit -qm c2 && C2=$(git rev-parse HEAD)
probe() { printf "%-9s HEAD=%s 索引g=%s 工作区g=%s status=[%s]\n" "$1" \
  "$(git rev-parse --short HEAD)" "$(git ls-files|grep -c '^g.txt$')" \
  "$(test -f g.txt && echo 1 || echo 0)" "$(git status --porcelain|tr '\n' ' ')"; }
base() { git reset -q --hard $C2; git clean -qfd; }
base; probe 起点 ; base; git reset -q --soft $C1 ; probe --soft
base; git reset -q --mixed $C1 ; probe --mixed
base; git reset -q --hard $C1  ; probe --hard

# B. revert vs cherry-pick
git reset -q --hard $C2; git clean -qfd
git revert --no-edit $C1 && git log --oneline -2
git reset -q --hard $C2; git clean -qfd
git cherry-pick $C1 ; git log --oneline -1      # 新哈希

# C. rebase 换哈希
git branch feat $C1 && git checkout -q feat && printf 'x\n' > x.txt && git add x.txt && git commit -qm "feat 1"
git checkout -q main && printf 'y\n' > y.txt && git add y.txt && git commit -qm "main 前进"
git checkout -q feat && git log --oneline -1 && git rebase main && git log --oneline -1

# D. squash（非交互）
git checkout -q -B sq main
for i in 1 2 3; do printf "$i\n" >> q.txt; git add q.txt; git commit -qm "wip $i"; done
GIT_SEQUENCE_EDITOR="sed -i '' '3,4s/^pick/squash/'" GIT_EDITOR=true git rebase -i -q HEAD~3
git log --oneline

# E. 一枪救回：ORIG_HEAD
printf 'oops\n' > o.txt && git add o.txt && git commit -qm "重要提交"
git reset --hard HEAD~2
git rev-parse --short ORIG_HEAD
git reset --hard ORIG_HEAD && test -f o.txt && echo "救回来了"
```

**自测题**：

1. `reset --soft/--mixed/--hard` 分别动三棵树里的哪几棵？各自"改动去哪了"？
2. `revert` 与 `cherry-pick` 都会产生新提交，它们的**新哈希**分别为什么与原来不同？
3. `rebase` 之后为什么**所有**被重放提交的哈希都变了？
4. `--amend` 之后 tag 会跟着动吗？为什么？（用 9.7 的实测回答）
5. 事故后第一、第二、第三层恢复手段分别是什么？哪一层会被下一次操作覆盖？
6. sourcepack 为什么不参与任何改写？它记录的哈希在改写后会怎样？

---

### 9.12 小结

- 改写历史的本质：**造新对象 + 移动引用**（对象库只追加，从不修改）；
- **`reset` 三模式**用三棵树解释：`--soft` 只动 HEAD、`--mixed` 动 HEAD+索引、
  `--hard` 三棵全动（唯一会丢工作区改动的）；
- **`revert` 只增不改**（适合公共分支）、**`cherry-pick` 复制改动产生新哈希**、
  **`amend`/`rebase` 造新提交**——reword 一个提交会让**下游哈希全变**（实测）；
- 交互式 rebase 的三种操作都有实测：squash（4→3 提交，内容不变）、reword（哈希全变）、
  drop（与后续提交冲突——drop 撤的是改动，不是文件）；
- 改写后 **tag 不跟随**、别人必须重新 clone、推送需要 `--force-with-lease`；
- 恢复三层：**`ORIG_HEAD`（一枪救回，实测）→ reflog（完整移动史）→
  `fsck`（最后的 dangling 对象）**；最好的办法是动手前先 `git branch backup`；
- 清理历史（filter-branch/filter-repo）有**社会性铁律**：只在自己独占的仓库上做，
  共享仓库必须先通知全员重新 clone；
- sourcepack **只读、不改写**，但**产物里的短哈希会因改写而失效**——
  发布锚点因此必须用 tag（tag 不跟随改写，已实测）。

**下一章**：性能与底层——delta 链为什么让快照模型省空间、commit-graph 与 bitmap 如何加速
`log`/`fetch`、`core.fsmonitor` 与索引的 stat 缓存（第 4 章那条线）、
partial clone 与 sparse checkout，以及"大仓库为什么变慢"的诊断方法。

---

<a id="第-10-章性能与底层"></a>

# 第 10 章　性能与底层

---

> **本章目标**：理解"Git 为什么不慢"，以及**什么时候它真的会慢**。
> 学完你能：判断某个操作是"启动开销主导"还是"数据量主导"、
> 知道 commit-graph / bitmap / delta / partial clone 各自加速哪一段，
> 并且能用 `GIT_TRACE_PERFORMANCE` 自己做诊断。
>
> 本章所有数据都来自本机实测（含一个用 `git fast-import` 造的 3 万提交仓库），
> 并且**包括我没能测出差异的部分**——那同样是有价值的信息。

---

### 10.1 先看结论：三件事决定了 Git 快不快

| 层 | 加速手段 | 加速的是什么 | 本章实测 |
| :--- | :--- | :--- | :--- |
| **对象存储** | **delta 压缩** | 磁盘体积（快照模型仍然省空间） | `--depth=1` → 体积 **5.6×** |
| **提交图遍历** | **commit-graph** | 历史 walk（`rev-list`、`log --graph`） | **3.4×** |
| **打包发送** | **bitmap** | 服务端算"要发哪些对象" | 机制 + 文件证据（本机规模下未测出差异） |
| **工作区状态** | stat 缓存 / untracked cache | `git status` | 启动开销主导，本机未测出差异 |
| **网络传输** | **partial clone** | 首次克隆体积 | 对象数 90k → **60k**（去掉全部 blob） |

**一句话**：Git 的默认实现已经把"常见操作"优化到**几十毫秒**；
优化手段真正生效的地方是**大历史**（几十万提交）与**大传输**（几 GB 仓库）。

---

### 10.2 delta 压缩：快照模型为什么仍然省空间

第 2 章说过 Git 存**快照**；第 2 章的 `verify-pack` 也露出了 `chain length = 4`。
这里把它讲透：packfile 会把**相似对象**串成 delta 链——只存"与基准对象的差异"。

**实测**（本机 3 万提交 / 9 万对象的合成仓库）：

| 打包方式 | pack 大小 | 相对默认 |
| :--- | ---: | :--- |
| 默认（`--depth` 未指定，实际 50） | **8,964,390** 字节 | 1.0× |
| `--depth=50 --window=50` | **7,400,287** 字节 | 0.83× |
| **`--depth=1`（禁止深链）** | **49,806,711** 字节 | **5.6×** ← 灾难 |

**`--depth=1` 的 5.6 倍**就是"如果没有 delta"的代价：
每次提交都完整存一份文件内容。**这就是 delta 存在的意义**，
也是"Git 存快照却不臃肿"的关键。

**delta 链长直方图**（`git verify-pack -v`，同一个仓库）：

```
chain length = 1: 402 objects
chain length = 2: 438 objects
...
chain length = 49: 3400 objects        # 长链是"改了很多次的小文件"
chain length = 50: 1186 objects        # 深度上限（默认 50）
非 delta: 60430 个 / delta: 29620 个
```

**读法**：深链省空间，但**读取时要把链上的 delta 依次应用**——
所以 Git 用 `--depth` 限制链长（空间换时间的旋钮）。
日常你不需要调它；只有"仓库特别大又很少读旧对象"（如归档镜像）才考虑加深。

---

### 10.3 commit-graph：加速历史遍历

`commit-graph` 是一个**预计算的提交元数据文件**（父提交、提交时间、根树位置），
让 Git 不必为每个提交去解压 commit 对象。

**实测**（3 万提交 / 9 万对象，三次取最快）：

| 操作 | 无 commit-graph | 有 commit-graph | 提升 |
| :--- | ---: | ---: | ---: |
| `git rev-list --count --all` | **110 ms** | **32 ms** | **3.4×** |
| `git log --graph --oneline -n 1000` | **112 ms** | **32 ms** | **3.5×** |
| `git log --oneline`（全量、要输出 3 万行文本） | 144 ms | 140 ms | ~0 |
| `git log -1 --format=%H` | 27 ms | 28 ms | ~0 |

```bash
$ git commit-graph write --reachable
$ ls -lh .git/objects/info/commit-graph
1.7M          # 3 万提交 → 1.7MB
```

**怎么解释"有的快 3 倍、有的没变"**：

- `rev-list`/`-graph` 是**纯遍历**（只读提交元数据）→ 直接受益；
- `git log --oneline` 要**逐个解压 commit 对象取出标题** → 瓶颈在对象读取；
- `git log -1` 只读一个提交 → 整个耗时几乎全是**进程启动**（约 27ms 固定开销）。

> **这条"固定启动开销"对 sourcepack 特别重要**——见 10.8。

---

### 10.4 bitmap：加速"要发哪些对象"

服务端在 `fetch`/`clone` 时要回答："客户端缺哪些对象？"
朴素做法是从提交一路走到每个 tree/blob（可能几百万次），
**bitmap** 预先把"某个提交可达的对象集合"编码成位图（每个对象一位），
于是这一步变成**位运算**。

```bash
$ git repack -adb          # -a 全部重打、-d 删旧包、-b 写 bitmap
$ ls -lh .git/objects/pack/*.bitmap
399K                        # 3 万提交 / 9 万对象 → 399KB
```

**诚实的实测结果**：在这台机器、这个规模下，我没能测出 bitmap 带来的差异：

| 操作 | 无 bitmap | 有 bitmap |
| :--- | ---: | ---: |
| `git rev-list --objects --all`（本地遍历） | 282 ms | 279 ms |
| file:// 克隆 | 962 ms | 1026 ms |

**原因**：bitmap 优化的是 **`pack-objects` 在服务端的可达性计算**（大仓库上能省掉几百万次对象访问），
而本机 9 万对象、本地克隆的瓶颈在别处（进程与磁盘），差异落在噪声里。
**所以：机制确定存在，量化收益需要几十万对象以上的仓库才显出来**——
这本身是个重要认知：**别在小仓库上做"性能优化"**。

---

### 10.5 `git status` 为什么快：stat 缓存（第 4 章那条线）

第 4 章拆过索引条目里的 `ctime/mtime/dev/ino/size`。它的用途就是这里：
`git status` **先比 stat**，只有对不上的才去读内容算哈希。

**实测**（2000 文件仓库）：`git status --porcelain` 首次 31ms、二次 32ms、
开启 `core.untrackedCache` 后 30ms —— **同样落在噪声里**。
原因：2000 个文件的 stat 检查本身就只要几毫秒，其余是进程启动。

**什么时候这块会真的痛**：

| 场景 | 症状 | 手段 |
| :--- | :--- | :--- |
| 十万级文件 | `git status` 变成秒级 | `core.untrackedCache=true`、`core.fsmonitor=true`（macOS 有 fsmonitor） |
| 网络文件系统（NFS/SMB） | stat 极慢 | `core.checkStat=minimal` |
| 巨型 monorepo | 检出本身很慢 | `git sparse-checkout`（见 10.7） |

---

### 10.6 partial clone：只要历史，不要 blob

`--filter=blob:none` 让 clone **不下载文件内容**，只在需要时按需拉取（promisor 机制）。

**实测**（3 万提交 / 9 万对象；**注意服务端必须开 `uploadpack.allowFilter`**）：

```bash
# 服务端（提供克隆的那个仓库）
$ git config uploadpack.allowFilter true

# 客户端
$ git clone --bare --filter=blob:none file:///tmp/big part.git
$ du -sh part.git ; git -C part.git count-objects -v | grep in-pack
8.9M
in-pack: 60000                    # ← 完整克隆是 90000（少了 3 万个 blob）
$ git -C part.git log --oneline -1
66f65d24 c29999                   # ← 历史在本地，log 正常
$ git -C part.git config --get remote.origin.promisor
true
```

**两个我踩到的坑（都很常见）**：

1. `git clone --filter=blob:none /local/path` →
   `warning: --filter is ignored in local clones; use file:// instead.`
   本地路径克隆会走硬链接优化，**过滤被忽略**；要用 `file://` 才会走真正的传输路径；
2. 即使 `file://`，服务端没开 `uploadpack.allowFilter` 时也只是
   `warning: filtering not recognized by server, ignoring` —— **静默退化成完整克隆**。

**用途**：CI 里只需要读历史/跑 `git log`、大仓库的代码搜索
（配合 `git grep` 时按需取 blob）。

---

### 10.7 sparse checkout：只检出需要的部分

`--filter` 省的是**对象**，`sparse-checkout` 省的是**工作区文件**（大 monorepo 常配一起用）。

```bash
$ git clone -q /tmp/big sp && cd sp && ls | wc -l
200
$ git sparse-checkout set --no-cone '/f1*.txt'
$ ls | wc -l
111                                  # ← 工作区只留下匹配的文件
```

| 模式 | 说明 |
| :--- | :--- |
| `--cone`（默认） | 按**目录**（`git sparse-checkout set dirA dirB`），最常用 |
| `--no-cone` | 按 gitignore 风格模式（更灵活，也更容易写错） |

---

### 10.8 回到 sourcepack：我们的性能账本

前几章的结论在这里汇总成一张"账"：

| 事实 | 数据 | 来源 |
| :--- | :--- | :--- |
| **每次 git 调用的固定开销** | **约 11 ms**（进程启动） | 第 8 章实测 |
| `-g` 一次要发起几次 git 调用 | **约 10 次**：`rev-parse ×4`（show-toplevel / show-prefix / HEAD / verify）、`symbolic-ref`、`log -1`、`diff --name-status ×2`（暂存/未暂存）、`ls-files --others`、`diff --patch`（大块输出） | `changeset.go:282-393` |
| 于是"纯 spawn 开销" | **≈ 110 ms**（10 × 11ms） | 第 1 章 + 第 8 章 |
| `-g` 全流程（500 文件变更） | **≈ 210 ms** | 第 1 章 |
| 目录扫描耗时（2 万文件 / 10 万行） | **3.85 s** | TODO.md 实测 |
| 同一规模的 100 万行（5000 文件） | 仅 190 ms | 同上 |

**三个工程结论**：

1. **不要为小项目做 git 层优化**：`-g` 里近一半时间就是"启动 8 个 git 进程"，
   而这是"调 CLI 换取行为一致性"的既定代价（第 1 章的设计取舍）；
2. **真正的瓶颈是"文件数"**：2 万文件的仓库扫描要 3.85 秒，
   而 100 万行但只有 5000 文件的仓库只要 190ms——
   所以扫描并发的收益只在"文件数极多"时才出现（已记入 `TODO.md`，暂不做）；
3. **我们用到的 git 优化都是"免费"的**：
   - 只读命令 + 一次性拿清单（`--name-status -z` 一次拿全）→ 减少 spawn 次数；
   - `--no-ext-diff --no-textconv` 避免外部程序（第 6 章）；
   - 体积上限（`--max-diff-lines` + 字节上限）防止"单行 20MB"这类病态输入；
   - **反向的**：我们**不**动 commit-graph / bitmap / repack——
     那是仓库所有者的选择，我们只读、不改（第 9 章）。

**诊断工具**（我们自己在排查时用的）：

| 命令 | 看什么 |
| :--- | :--- |
| `GIT_TRACE_PERFORMANCE=1 git <cmd>` | Git 内部各相位耗时（最后一行是总计） |
| `GIT_TRACE2_PERF=1 git <cmd>` | 更细的 trace2 事件（含是否读 commit-graph） |
| `git count-objects -v` | 松散/打包对象数、体积 |
| `git verify-pack -v <idx>` | delta 链分布 |
| `/usr/bin/time -p` + 多次取最快 | 命令级基准（避开单次抖动） |

> **诚实提醒**：本章有**三项没能测出差异**（bitmap、stat 缓存、`log --oneline`）。
> 原因已分析（规模不够 / 启动开销主导），而不是"这些技术无效"。
> 把"没测出差异"如实写出来，比编一个好看的数字有用——

---

### 10.9 动手实验 `[lab]`

```bash
# 0) 用 fast-import 快速造大仓库（这也是一个值得知道的工具）
mkdir -p /tmp/l10 && cd /tmp/l10 && git init -q -b main .
python3 - <<'PY' > stream.fi
files=200; commits=5000
print('blob\nmark :1\ndata <<EOM\nf0\nl0\nEOM')
print('commit refs/heads/main\nmark :1000000\nauthor P <p@x.c> 1700000000 +0800\ncommitter P <p@x.c> 1700000000 +0800\ndata <<EOM\ninit\nEOM')
for i in range(files): print(f'M 100644 :1 f{i}.txt')
prev=':1000000'
for c in range(1,commits):
    idx=(c*7)%files; m=2000000+c*2
    print(f'blob\nmark :{m}\ndata <<EOM\nf{idx} rev {c}\nl0\nEOM')
    print(f'commit refs/heads/main\nmark :{m+1}\nauthor P <p@x.c> {1700000000+c} +0800\ncommitter P <p@x.c> {1700000000+c} +0800\ndata <<EOM\nc{c}\nEOM\nfrom {prev}')
    print(f'M 100644 :{m} f{idx}.txt'); prev=f':{m+1}'
print('done')
PY
git fast-import --quiet < stream.fi && git reset -q --hard main
git rev-list --count HEAD

# 1) delta 深度对体积的影响
git repack -adf -q --depth=1   ; ls -l .git/objects/pack/*.pack | awk '{print "depth=1:  "$5}'
git repack -adf -q --depth=50  ; ls -l .git/objects/pack/*.pack | awk '{print "depth=50: "$5}'

# 2) commit-graph 的效果
rm -f .git/objects/info/commit-graph
time git rev-list --count --all
git commit-graph write --reachable
time git rev-list --count --all          # 应明显更快

# 3) delta 链分布
git verify-pack -v .git/objects/pack/*.idx | grep '^chain length' | head

# 4) partial clone（服务端要开 allowFilter）
git config uploadpack.allowFilter true
git clone -q --bare --filter=blob:none file://$PWD ../l10-part.git
git -C ../l10-part.git count-objects -v | grep in-pack   # 比完整克隆少掉全部 blob

# 5) 诊断
GIT_TRACE_PERFORMANCE=1 git rev-list --count --all 2>&1 | tail -2
```

**自测题**：

1. `--depth=1` 为什么会让仓库体积暴涨 5 倍以上？
2. 为什么 commit-graph 让 `rev-list` 快 3 倍，却几乎不影响 `git log -1`？
3. bitmap 加速的是哪一侧的哪一步？为什么小仓库测不出差异？
4. `--filter=blob:none` 在本地路径克隆时为什么被忽略？怎么让它生效？
5. `sparse-checkout` 与 `partial clone` 分别省的是什么？
6. sourcepack 的 `-g` 里，最大的固定开销是什么？为什么这是"刻意接受"的代价？

---

### 10.10 小结

- **delta 压缩**是"快照模型不臃肿"的原因：禁止深链（`--depth=1`）会让体积涨 **5.6×**；
  链长直方图可用 `verify-pack -v` 观察（本机 3 万提交仓库链长 1–50）；
- **commit-graph** 预计算提交元数据：纯遍历类操作（`rev-list`、`log --graph`）
  实测 **3.4×** 提升，而"读对象取内容"和"单次短命令"不受益（后者被进程启动主导）；
- **bitmap** 让服务端算可达集合变成位运算；**本机规模下未测出差异**——
  它是为大仓库/大传输准备的；
- **stat 缓存 / untracked cache** 是 `git status` 快的原因（第 4 章那条线），
  本机 2000 文件规模同样测不出差异，十万级文件才显出来；
- **partial clone**（`--filter=blob:none`）能把对象数从 90k 降到 60k（去掉全部 blob），
  但有**两个静默陷阱**：本地路径克隆会忽略过滤（要用 `file://`）、
  服务端要开 `uploadpack.allowFilter`；**sparse-checkout** 省的是工作区文件；
- 诊断三件套：`GIT_TRACE_PERFORMANCE` / `git count-objects -v` / 多次取最快的计时；
- **sourcepack 的性能账本**：`-g` 的约 90ms 花在 8 次 git 进程启动上（刻意的设计取舍），
  真正瓶颈是**文件数**（2 万文件 3.85s vs 100 万行 5000 文件 190ms），
  而我们**不碰** repo 级的优化（commit-graph/bitmap/repack）——只读、不改。

**下一章**：钩子、签名与发布——`pre-commit`/`pre-push` 能做什么与不能做什么、
为什么钩子不能替代 CI、GPG/SSH 签名的意义，以及 **sourcepack 自己的发布链路**
（`release.yml`：tag → gate → goreleaser → npm publish → 冒烟）。

---

<a id="第五部实战"></a>

# 第五部　实战

> 把前四部收束到一个真实项目上：发布链路，以及一个「读 Git 的工具」该怎么写。

---

<a id="第-11-章钩子签名与发布"></a>

# 第 11 章　钩子、签名与发布

---

> **本章目标**：搞清"自动化检查该放在哪一层"——本地钩子、服务端钩子、还是 CI？
> 学完你能解释：为什么钩子替代不了 CI、为什么发布要用**签名 tag**、
> 以及 sourcepack 的发布链路为什么长成现在这样（含一次真实事故的复盘）。

---

### 11.1 钩子的机制：就是可执行文件

钩子（hook）是 Git 在特定时机调用的**可执行文件**，默认放在 `.git/hooks/`：

```bash
$ ls .git/hooks | head -4
applypatch-msg.sample
commit-msg.sample
pre-commit.sample          # ← .sample 后缀不会被调用
pre-push.sample
```

**启用方式就是去掉 `.sample` 并给执行权限**（第 3 章：Git 的很多"机制"都是文件系统约定）。

#### 实测：一个 pre-commit 拦截

```bash
$ cat .git/hooks/pre-commit
#!/bin/sh
if grep -rn "TODO-FORBIDDEN" --include="*.txt" . >/dev/null 2>&1; then
  echo "❌ pre-commit: 发现禁止的内容 TODO-FORBIDDEN" >&2
  exit 1
fi
$ chmod +x .git/hooks/pre-commit

$ printf 'ok\n' > ok.txt && git add ok.txt && git commit -m "正常提交"
[main abc1234] 正常提交                       # ✅ 通过

$ printf 'TODO-FORBIDDEN\n' > bad.txt && git add bad.txt && git commit -m "坏提交"
❌ pre-commit: 发现禁止的内容 TODO-FORBIDDEN
$ echo $?
1                                             # ✅ 被拦下
```

#### 实测：`--no-verify` 可以绕过

```bash
$ git commit --no-verify -m "绕过钩子"
[main 2b2cba1] 绕过钩子                        # ⚠️ 钩子形同虚设
```

#### 实测：钩子**不进版本库**

```bash
$ git ls-files .git/hooks | wc -l
0                                             # 克隆不会带走钩子
```

**想让团队共享钩子**，常见做法是把脚本放进仓库并设置路径：

```bash
$ git config core.hooksPath .githooks         # 团队成员各自设置一次
```

---

### 11.2 钩子的位置：客户端 vs 服务端

| 层 | 代表钩子 | 何时跑 | 能否绕过 |
| :--- | :--- | :--- | :--- |
| **客户端**（本地） | `pre-commit`、`commit-msg`、`pre-push`、`pre-rebase` | 你执行对应命令时 | ✅ `--no-verify` / 直接不装 |
| **服务端**（仓库侧） | `pre-receive`、`update`、`post-receive` | 收到推送时 | ❌ 客户端无法绕过 |

#### 实测：服务端 `pre-receive` 是真正的闸

```bash
# 在裸仓库里放一个钩子
$ cat /tmp/hook-lab-remote.git/hooks/pre-receive
#!/bin/sh
while read oldrev newrev refname; do
  echo "服务端收到推送: $refname" >&2
  if [ "$refname" = "refs/heads/forbidden" ]; then
    echo "❌ pre-receive: 禁止推送 forbidden 分支" >&2
    exit 1
  fi
done
exit 0

$ git push origin allowed
remote: 服务端收到推送: refs/heads/allowed
$ echo $?
0

$ git push origin forbidden
remote: ❌ pre-receive: 禁止推送 forbidden 分支
 ! [remote rejected] forbidden -> forbidden (pre-receive hook declined)
$ echo $?
1

$ git push --no-verify origin forbidden       # 试试绕过
remote: ❌ pre-receive: 禁止推送 forbidden 分支
 ! [remote rejected] ...                       # ← 依然被拒：服务端钩子不理会 --no-verify
```

**这张对照表是本章的核心结论**：

| 需求 | 正确的位置 |
| :--- | :--- |
| 我自己的快速检查（格式化、拼写） | 客户端 `pre-commit`（**可绕过，只当便利**） |
| 团队必须遵守的规范（禁止大文件、密钥扫描） | **CI** 或**服务端钩子**（不可绕过） |
| 保护分支（必须 review / 必须绿） | 托管平台的分支保护规则 |
| 发布前把关 | **CI 的发布闸门**（见 11.4） |

---

### 11.3 签名：证明"这个提交/标签确实是你发的"

Git 支持两种签名（Git ≥ 2.34 起 SSH 签名很常用）：

| 方式 | 配置 | 说明 |
| :--- | :--- | :--- |
| GPG | `user.signingkey=<keyid>` | 传统方式，需要 gpg 与密钥环 |
| **SSH** | `gpg.format=ssh` + `user.signingkey=~/.ssh/id_ed25519.pub` | 复用你已有的 SSH key，**不需要 gpg** |

#### 实测：用 SSH key 签提交与 tag（本机没有 gpg，正好演示 SSH 方式）

```bash
$ git config gpg.format ssh
$ git config user.signingkey ~/.ssh/id_ed25519.pub

$ git commit -S -m "签名的提交"
$ git cat-file -p HEAD | sed -n '/^gpgsig/,+2p'
gpgsig -----BEGIN SSH SIGNATURE-----
 U1NIU0lHAAAAAQAAADMAAAALc3NoLWVkMjU1MTkAAAAgR6Nur4hH58/4CmcLlBzmLDQl8c
 GAuNZ4rHTTTirZNHMAAAADZ2aXQAAAAAAAAABzaGE1MTIAAABTAAAAC3NzaC1lZDI1NTE5
```

**验证需要一份"允许签名者"清单**（`gpg.ssh.allowedSignersFile`）：

```bash
$ echo "t@x.c $(cat ~/.ssh/id_ed25519.pub)" > /tmp/allowed_signers
$ git config gpg.ssh.allowedSignersFile /tmp/allowed_signers

$ git verify-commit HEAD
Good "git" signature for t@x.c with ED25519 key SHA256:sS7Y056TZOqIQfVXqPXfjPubt0Qs9+yvTscib7XbLR4
```

**签 tag（发布用）**：

```bash
$ git tag -a -s v9.9.9-test -m "测试签名标签"
$ git tag -v v9.9.9-test
Good "git" signature for t@x.c with ED25519 key SHA256:sS7Y...
$ git cat-file -t v9.9.9-test
tag                                # ← 附注标签是独立对象（第 3 章）
```

**为什么要签发布 tag**：

| 威胁 | 签名的作用 |
| :--- | :--- |
| 账号被盗后发布恶意版本 | 没有你的私钥就签不出有效 tag |
| 中间人替换 tag 指向的提交 | 签名覆盖 tag 对象（含它指向的提交哈希） |
| 下游无法追溯来源 | 用户可 `git tag -v` 自行验证 |

**本仓库的现状（诚实说明）**：`commit.gpgsign` 未设置，提交与 tag **都没有签名**。
对个人项目可接受；一旦涉及分发（npm 包、二进制），
**至少给发布 tag 签名**是低成本高收益的加固。

---

### 11.4 sourcepack 的发布链路

#### 触发方式：推 tag

```yaml
# .github/workflows/release.yml
on:
  push:
    tags: ['v*']
```

**为什么用 tag 而不是分支**：第 9 章实测过——**改写历史时 tag 不跟随**，
所以 tag 是稳定的"发布锚点"；分支会动，缓存/引用分支名会失效。

#### 三段式流水线（`needs` 串起来）

```
push tag v*  →  gate  →  goreleaser  →  npm-publish(+ 两段冒烟)
                ↓            ↓                  ↓
            全套检查     构建 6 个平台归档    发布 npm 并装回来验证
```

**① `gate`（发布闸门）**：

```yaml
    gate:
      steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with: { go-version: '1.24.x' }
        - run: go vet ./...
        - name: gofmt
          run: |
            unformatted=$(gofmt -l .)
            if [ -n "$unformatted" ]; then echo "$unformatted"; exit 1; fi
        - run: go test -race ./...
        - run: node --test test/
```

**为什么会有这个 job**：见 11.5 的事故复盘。它的注释里写着原因：

> `v1.8.0 就是"打 tag 直接构建发布"放出去的，当天暴露三个线上可见问题`

**② `goreleaser`（构建与发布归档）**：

```yaml
  goreleaser:
    needs: gate                      # ← 闸门没过就不构建
    permissions: { contents: write } # ← 权限按 job 最小化
    steps:
      - uses: actions/setup-go@v5
        with: { go-version: '1.24.x' }   # ← 固定版本，不用 'stable'
```

`.goreleaser.yaml` 的产物矩阵：

```yaml
builds:
  - env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    main: .
    ldflags: ['-X main.versionStr=v{{.Version}}']    # 版本号注入二进制
archives:
  - name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    wrap_in_directory: true
    format_overrides: [{ goos: windows, format: zip }]
checksum: { name_template: 'checksums.txt' }
```

**注意 `{{ .Os }}` 是 Go 的 GOOS（`windows`），不是 Node 的 `win32`**——
第 6 章的命名差异问题**正是这个模板与 npm 安装脚本不一致**造成的，
两个平台名混用直接让 Windows 用户装不上（v1.8.0 事故之一）。

**③ `npm-publish`（发布到 npm + 端到端冒烟）**：

```yaml
        - uses: actions/setup-node@v4
          with:
            registry-url: 'https://registry.npmjs.org'    # ← 官方写法
        - run: npm publish --access public
          env:
            NODE_AUTH_TOKEN: ${{ secrets.NPM_TOKEN }}     # ← 不再 echo token 到 ~/.npmrc
```

然后是**两段冒烟**（这是"发布闸门"的延伸，把验证做到用户路径上）：

| 阶段 | 做什么 | 覆盖什么 |
| :--- | :--- | :--- |
| 冒烟 1/2 | 用包里的 `download-binary.js` 拉**真实 Release 资产**，校验落地名/可执行/`--version`/`-h` | 不依赖 npm registry 传播，确定性最高；正好覆盖 v1.8.0 那类"归档名不一致" |
| 冒烟 2/2 | 轮询等 registry 传播后 `npm i -g`，再跑一遍 | 验证**发布出去的 tarball**（含 postinstall） |

**为什么分两段**：v1.8.2 首次上线时冒烟**因为 npm 传播慢而在 90 秒内一直 `ETARGET` 失败**——
那是脚本设计问题，不是包的问题。现在改成"先做不依赖传播的确定性检查，再做传播后的验证"。

---

### 11.5 事故复盘：v1.8.0 的"打 tag 直接发布"

| 时间 | 事件 |
| :--- | :--- |
| v1.8.0 发布当天 | 三个线上可见问题：**Windows 完全装不上**、`--staged` 计数恒为 0、`-h` 丢失自定义说明 |
| 当天 | 修完发 v1.8.1 |
| v1.8.2 | **补上 `gate` job + 固定 Go 版本 + 权限最小化 + npm 认证现代化 + 两段冒烟** |
| 同批 | CI 矩阵加入 **windows-latest**——加完当天就抓到两个真实平台问题 |

**这条链条的因果很清楚**：

```
发布流程缺闸门  →  问题留到用户手上
缺少 Windows CI →  平台问题根本没机会被发现
缺少安装冒烟    →  "装完即挂"这类问题只能靠用户反馈
```

**修复后的实测收益**（第 4/6 章引用的两件事）：

| 加的检查 | 抓到了什么 |
| :--- | :--- |
| `windows-latest` 矩阵 | `filepath.Match` 在 Windows 让 `*` 跨过 `/`；CRLF 让 `gofmt` 闸门假失败 |
| `go test -race` | 并发/管道问题（我们有 `io.Pipe` 与 goroutine） |
| 两段冒烟 | 归档名/落地名一致性（v1.8.0 的 Windows 事故） |

> **对你自己项目的启示**：发布前至少跑一遍"**用户实际会走的那条路**"
> （装包 → 跑命令 → 看输出），而不只是跑单元测试。
> v1.8.0 的三个问题里，有两个（Windows 安装、`-h`）都能被一条安装冒烟拦住。

---

### 11.6 发布史：42 个 tag 与"附注 tag 就是发布说明"

```bash
$ git tag -l 'v*' | wc -l
42
$ git for-each-ref --sort=creatordate --format='%(refname:short) %(creatordate:short) %(subject)' refs/tags | tail -4
v1.8.0 2026-10-11 v1.8.0: Git 变更打包（-g / -G），打包未提交改动与最近 N 个提交
v1.8.1 2026-10-12 v1.8.1: Windows 安装修复、整文件 UTF-8 判定、中文 Token 估算、godoc.go → main.go
v2.0.0 2026-10-12 v2.0.0: -d 非法路径明确报错（行为变更）；-h 新增 Examples 示例段；参数手册 PARAMETERS.md
v2.0.1 2026-10-12 v2.0.1: -h 末尾换成参数手册博客版（https://s.want.biz/sourcepack）
```

**注意第三列**：`%(subject)` 取的是 **tag 对象的说明**（第 3 章：附注标签是独立对象）。
所以 `git tag -a -m "..."` 写的那段话，就是**发布说明**——
`git tag -l` 看不到它，`git for-each-ref`/`git show <tag>` 才能。

**这也是为什么发布要用 `-a`（附注）而不是轻量 tag**：
轻量 tag 只是一个指向 commit 的引用，**没有地方放发布说明**，也无法签名。

---

### 11.7 回到 sourcepack：我们没有钩子，CI 就是我们的钩子

**实测现状**：

```bash
$ ls .git/hooks | grep -v '\.sample$' | wc -l
0
$ git config core.hooksPath || echo "(未设置)"
(未设置)
```

**为什么没有钩子**（有意的设计）：

| 原因 | 说明 |
| :--- | :--- |
| 钩子是**本地**的 | 克隆不带走（11.1 实测），团队规范必须放到 CI |
| 钩子**可绕过** | `--no-verify` 一句话失效 |
| 我们的质量闸门在**发布链路**里 | `gate` job + windows 矩阵 + 两段冒烟（11.4），这些谁都绕不过 |
| 工具本身**不该改仓库** | sourcepack 是只读工具（第 9 章）；给它装钩子等于污染用户仓库 |

**"我们的钩子"= CI 矩阵 + 发布闸门**，对应关系：

| 传统钩子 | sourcepack 的做法 |
| :--- | :--- |
| `pre-commit`（格式化/静态检查） | CI 里的 `go vet` + `gofmt -l`（**不可绕过**） |
| `pre-push`（跑测试） | CI 里的 `go test -race ./...` + `node --test test/` |
| 服务端 `pre-receive`（禁止坏提交） | 发布闸门 `gate` + 分支保护 |
| 发布后验证 | **两段式冒烟**（钩子做不到这一层） |

---

### 11.8 动手实验 `[lab]`

```bash
# A. 客户端钩子与它的边界
rm -rf /tmp/l11 && mkdir /tmp/l11 && cd /tmp/l11 && git init -q -b main . \
  && git config user.email t@t.c && git config user.name T
cat > .git/hooks/pre-commit <<'EOF'
#!/bin/sh
if grep -rn "FORBIDDEN" . --include="*.txt" >/dev/null 2>&1; then echo "❌ 拦截" >&2; exit 1; fi
EOF
chmod +x .git/hooks/pre-commit
printf 'ok\n' > ok.txt && git add ok.txt && git commit -m ok          # 通过
printf 'FORBIDDEN\n' > bad.txt && git add bad.txt && git commit -m bad ; echo "退出码 $?"   # 被拦
git commit --no-verify -m bypass ; git log --oneline -1              # 绕过成功
git ls-files .git/hooks | wc -l                                      # 0：不进版本库

# B. 服务端钩子（不可绕过）
git init -q --bare /tmp/l11-remote.git
cat > /tmp/l11-remote.git/hooks/pre-receive <<'EOF'
#!/bin/sh
while read o n ref; do
  case "$ref" in *forbidden*) echo "❌ 服务端禁止 $ref" >&2; exit 1;; esac
done
EOF
chmod +x /tmp/l11-remote.git/hooks/pre-receive
git remote add origin /tmp/l11-remote.git
git push origin main
git push --no-verify origin main:forbidden ; echo "退出码 $?"        # 仍被拒

# C. SSH 签名（本机无 gpg 也能做）
git config gpg.format ssh && git config user.signingkey ~/.ssh/id_ed25519.pub
git commit -S --allow-empty -m "签名提交"
git cat-file -p HEAD | head -5                                        # 看 gpgsig 段
echo "t@x.c $(cat ~/.ssh/id_ed25519.pub)" > /tmp/allowed_signers
git config gpg.ssh.allowedSignersFile /tmp/allowed_signers
git verify-commit HEAD
git tag -a -s v0.0.1-test -m "签名标签" && git tag -v v0.0.1-test

# D. 看自己项目的发布链路
cat .github/workflows/release.yml          # 或你项目的等价物
git tag -l 'v*' | wc -l
git for-each-ref --sort=creatordate --format='%(refname:short) %(subject)' refs/tags | tail -5
```

**自测题**：

1. 客户端钩子与服务端钩子，哪个 `--no-verify` 能绕过？为什么？
2. 为什么"团队规范"不能只靠 `pre-commit`？给出两条理由。
3. SSH 签名与 GPG 签名各有什么取舍？验证签名需要什么额外文件？
4. 为什么发布要用**附注 tag** 而不是轻量 tag？（提示：发布说明放在哪）
5. `.goreleaser.yaml` 的 `{{ .Os }}` 与 Node 的 `os.platform()` 有什么区别？这个区别在 v1.8.0 造成了什么后果？
6. sourcepack 的发布链路有哪三段？每段拦住什么类型的问题？

---

### 11.9 小结

- **钩子就是可执行文件**（`.git/hooks/`），启用＝去 `.sample` + 加执行权限；
- **客户端钩子可被 `--no-verify` 绕过、不进版本库**（克隆不带走）→ 只能当**便利**；
  **服务端钩子不可绕过**（实测 `! [remote rejected] (pre-receive hook declined)`）；
  团队规范应放在 **CI / 服务端 / 分支保护**；
- **签名**可以用 SSH（`gpg.format=ssh`，不需要 gpg）：实测 `gpgsig` 段落、
  `git verify-commit` / `git tag -v` 输出 `Good "git" signature`；
  验证需要 `allowedSignersFile`；**发布 tag 应当签名**（本仓库当前未签名，属已知短板）；
- sourcepack 的发布链路：**推 tag → gate（vet/gofmt/race/node）→ goreleaser（6 平台归档）
  → npm publish → 两段冒烟**；每段都用 `needs` 串起来，权限按 job 最小化；
- **发布用 tag 不用分支**：tag 不跟随历史改写（第 9 章实测），是稳定锚点；
  **附注 tag 自带发布说明**（42 个 tag 的第三列就是它）；
- **v1.8.0 事故的教训**：没有闸门、没有 Windows CI、没有安装冒烟，
  三个线上问题因此漏出——修复后同批加的 windows 矩阵当天就抓到两个真实平台 bug；
- sourcepack **有意不装钩子**：钩子本地/可绕过/会污染用户仓库，
  我们的闸门在 CI 与发布链路上。

**下一章（最后一章）**：把前 11 章的结论收拢回代码——
`gitRunner` 的每一条 hardening 对应本章/前章的哪个坑、`-z` 解析为什么必需、
`--src-prefix` 与 `--no-textconv` 防的是什么、只读命令清单，
以及"我们的测试夹具为什么要自己合成合并提交、中文路径与坏 textconv"。

---

<a id="第-12-章sourcepack-的-git-用法全解结课"></a>

# 第 12 章　sourcepack 的 Git 用法全解（结课）

---

> **本章目标**：把前 11 章的结论**逐条落回代码**。学完之后，你不仅能读 Git 的原理，
> 还能自己写一个"安全地读 Git"的工具——本章最后一节给出可直接照抄的 checklist。
>
> 这一章也是全课的收束：每一个 hardening 细节，都能指回某一章讲过的坑。

---

### 12.1 全景：我们与 Git 的接口只有一层

整个项目对 Git 的依赖被收在一个 40 行的类型里（`changeset.go:160-215` 附近）：

```
   ┌──────────────────────────────────────────────┐
   │  sourcepack                                  │
   │   collectChangeSet()  →  gitRunner           │
   │                          ├── command()        │  ← 所有 hardening 在这里
   │                          ├── run()            │
   │                          └── runLimited()     │  ← 带字节上限的读取
   └───────────────┬──────────────────────────────┘
                   │  exec.Command("git", ...)
                   ▼
             真正的 git 可执行文件
```

**为什么要收成一层**：只要所有调用都经过 `command()`，
加固措施就**不可能被漏掉**（新增调用自动继承）。这和后端服务里"所有出站请求走同一个 client"是同一个道理。

```go
func (g *gitRunner) command(args ...string) *exec.Cmd {
	full := make([]string, 0, len(args)+5)
	full = append(full, "--no-pager", "-c", "core.quotepath=false", "-C", g.dir)
	full = append(full, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(),
		"GIT_PAGER=cat",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_OPTIONAL_LOCKS=0",
	)
	cmd.Stdin = nil // 防止 git 去读 stdin 而挂住
	return cmd
}
```

---

### 12.2 每一条 hardening 对应哪一章的哪个坑

| 措施 | 没有它会怎样 | 对应章节 |
| :--- | :--- | :--- |
| `--no-pager` + `GIT_PAGER=cat` | `git log` 在非 TTY 环境下启动分页器，**进程挂住直到超时** | 第 1 章（git 调用细节） |
| `cmd.Stdin = nil` | git 尝试读 stdin（例如等待 `--edit` 或凭据），**永久阻塞** | 第 1 章 |
| `GIT_TERMINAL_PROMPT=0` | 需要认证时弹出提示，CI/agent 里**挂死**（宁可失败） | 第 8 章（协议与认证） |
| `GIT_OPTIONAL_LOCKS=0` | 只读操作去抢 `.git/index.lock`，与用户并发操作**互相干扰** | 第 4 章（索引） |
| `-c core.quotepath=false` | 中文文件名被转义成 `\344\275\240`，**路径解析全错** | 第 2/6 章（对象与 patch 格式） |
| `-C <dir>` | 需要 `os.Chdir`（进程级全局状态），并发/测试都不安全 | 第 1 章 |
| 把 stderr 包进 error | 用户看到 `exit status 128` 而**不知道 git 说了什么** | 第 1 章（"报错就是 git 的报错"） |
| `runLimited(limit, ...)` | 病态 diff（单行 20MB）把内存吃穿 | 第 6 章 + 第 10 章（体积上限） |

**注意最后一行的实现细节**：`runLimited` 用 `StdoutPipe` 流式读取并**在超限后继续排干管道**
（否则 git 会因写端阻塞而卡住）——这是"读子进程输出"的经典坑：

```go
	data, readErr := io.ReadAll(io.LimitReader(stdout, int64(limit)+1))
	clipped := len(data) > limit
	if clipped {
		data = data[:limit]
	}
	// 必须把剩余输出排干，否则子进程可能阻塞在写管道上导致 Wait 挂死。
	_, _ = io.Copy(io.Discard, stdout)
	waitErr := cmd.Wait()
```

**还有一条同样重要**：截断后**不能吞掉 `waitErr`**——因为管道已经排干，
此刻的失败一定是 git 自己出错（而不是"我们不读了导致它 SIGPIPE"）：

```go
	if waitErr != nil {
		// clipped 时同样要报错：上面已经把管道排干……
		return string(data), clipped, fmt.Errorf("git %s failed: %s", ...)
	}
```

> 这个细节曾经**真的错过**（截断时用 `if waitErr != nil && !clipped` 静默吞掉了失败），
> 修法与反向验证见第 12.7 节。

---

### 12.3 完整命令清单（我们到底调了哪些 git）

**只读命令一览**（生产代码，`changeset.go` + `changeoutput.go`）：

| 用途 | 完整参数（省略公共前缀） | 对应章节 |
| :--- | :--- | :--- |
| 找仓库根 | `rev-parse --show-toplevel` | 第 3 章（引用/工作区定位） |
| 处理 `-d 子目录` | `rev-parse --show-prefix` | 第 5 章（作用域） |
| 取分支名 | `symbolic-ref --short HEAD` | 第 3 章（符号引用；detached 会失败） |
| 取提交 | `rev-parse HEAD` / `rev-parse --short HEAD` | 第 2/3 章 |
| 取标题 | `log -1 --format=%s` | 第 2 章（commit 字段） |
| 校验 `--rev` | `rev-parse --verify <base>^{commit}` | 第 3 章（剥壳） |
| 变更清单 | `diff --no-ext-diff --name-status -z -M [--cached] [base] [-- pathspec]` | 第 4/6 章 |
| 暂存集合 | `diff --no-ext-diff --name-status -z -M --cached <base>` | 第 4 章（索引） |
| 未暂存集合 | `diff --no-ext-diff --name-status -z -M` | 第 4 章 |
| 未跟踪文件 | `ls-files --others --exclude-standard --full-name -z` | 第 7 章（忽略规则） |
| patch 正文 | `diff --no-ext-diff --no-textconv --no-color -M --src-prefix=a/ --dst-prefix=b/ [base]` | 第 6 章 |
| 提交遍历 | `log -N --date=short --format=%H<sep>%h<sep>%an<sep>%ad<sep>%P<sep>%s [--no-merges]` | 第 5 章（%P = 父边） |

**⚠️ 一个必须自己控制的细节**：`--format` 里我们用的分隔符是
**ASCII 的 Unit Separator（0x1F）**，而不是空格、逗号或 `|`：

```go
	logFieldSep = "\x1f"
	// 用法：
	"--format=%H" + logFieldSep + "%h" + logFieldSep + "%an" + logFieldSep +
	"%ad" + logFieldSep + "%P" + logFieldSep + "%s"
```

**为什么**：作者名与提交信息里**什么都可能出现**（含 `|`、含逗号、含换行），
但 0x1F 是"不可打印的控制字符"，实际提交信息里不会出现。
这与"清单一律用 `-z`"是同一条原则：

> **永远不要用可能出现在数据里的分隔符。** 要么用 NUL（`-z`），
> 要么用控制字符（0x1F），要么写个真正的解析器——绝不用空格/逗号/`|`。

---

### 12.4 为什么必须 `-z`：一个实测过的坑

`git diff --name-status` 的默认输出是**按行**的：

```
M	src/app.go
R100	src/old name.go	src/new name.go      # ← 制表符分隔，路径含空格
```

**问题**：路径里可以出现**空格、制表符、换行、引号**（Git 允许），
所以"按行 + 按空白切分"必然在某些仓库上崩掉。

`-z` 改成 **NUL 分隔**（NUL 是路径里唯一不可能出现的字节），于是解析变成：

```go
func parseNameStatusZ(s string) ([]*Change, error) {
	fields := strings.Split(s, "\x00")
	...
}
```

**测试里专门钉住了这些情形**（`changeset_test.go`）：

```go
"D\x00gone.md\x00C75\x00base.go\x00copy.go\x00M\x00中文 文档.md\x00"   // 中文 + 空格
{"a/hello world.txt\t": "hello world.txt"},        // git 给含空格路径加的尾 TAB
{"b/中文 文档.md\t":      "中文 文档.md"},
```

> **这里有一个真实修过的"死通道"**：`parseNameStatusZ` 起初签名里带 `error`，
> 但内部**永远不会返回非 nil**（畸形输入被静默当成正常数据）。
> 后果是"-z 输出被破坏时，我们既不报错也不丢弃，而是拿着垃圾继续跑"。
> 修法是让它真的校验（包括改名条目里空路径这种情形）并返回 error，
> 由调用方 `collectChangeSet` 抛出——**能被静默忽略的错误通道，等于没有错误处理**。

---

### 12.5 diff 相关的三个防御（每一个都有实测支撑）

| 参数 | 防的是什么 | 实测证据（哪一章） |
| :--- | :--- | :--- |
| **`--no-textconv`** | 用户配的 textconv 命令**改写 diff**，甚至让 `git diff` 直接失败 | 第 6 章：用户 textconv 失败时 `git diff` 退出码 **128**，而 `gx -g` 照常 exit 0 |
| **`--no-ext-diff`** | 用户配的**外部 diff 程序**（`diff.external`）会产生任意格式，我们的解析器必然错 | 第 6 章 |
| **`--src-prefix=a/ --dst-prefix=b/`** | `diff.mnemonicPrefix`（`i/`/`w/`）与 `diff.noprefix` 会改变头部，**路径还原不出来** → 幽灵条目 | 第 6 章：v1.8.1 出现假变更 `c/b/gone.go`，v1.8.2 修复 |
| **`-M`** | 不传则重命名显示为"删除+新增"（`A`+`D`），审查价值大幅下降 | 第 6 章：`R094` vs `A`/`D` 实测 |

**共同点**：这四条都是"**不去猜用户配置，而是把协议固定下来**"。
这是写工具时最重要的心态之一——**用户的配置是环境的一部分，不是你的接口**。

---

### 12.6 只读保证：实测我们能证明什么

```bash
$ grep -oE '"(diff|log|ls-files|rev-parse|symbolic-ref)"' changeset.go | sort -u | tr '\n' ' '
"diff" "log" "ls-files" "rev-parse" "symbolic-ref"
```

**五种子命令，全部只读**（第 9 章已展开）。这带来三个可验证的性质：

| 性质 | 为什么重要 |
| :--- | :--- |
| **零副作用** | 可以在任何仓库里跑，不需要备份、不会改索引/引用 |
| **不需要权限** | 只读 check-out 也能用（CI、agent 沙箱） |
| **可并发** | 不抢 `index.lock`（配合 `GIT_OPTIONAL_LOCKS=0`） |

---

### 12.7 测试策略：三种"以真值为准"的做法

这是全课最值得带走的部分——**我们怎么证明"我们理解对了 Git"**。

#### ① 以 git 自己为真值（差分测试）

`ignore_test.go` 用 **14 个模式 × 40 条路径**逐条对照 `git check-ignore`：

```go
var oraclePatterns = []string{
	"*.log", "**/*.log", "**/*.tmp", "src/**/*.go", "**/node_modules",
	"a/**/b", "abc/**", "build/", "/rooted.txt", "docs/**/*.md",
	"*.min.js", "temp?", "[ab]racket.txt", "keep/**/deep.txt",
}
```

**为什么强**：模式的组合空间（锚定 × `**` 位置 × 目录规则 × 负向）无法靠"想用例"覆盖；
把判定权交给 git，我们的任务就从"想全边界"变成"对齐一个已有实现"。
**第一次跑就抓到了 `a/**/b` 之下 `a/b/c` 的父目录规则**（第 7 章）。

#### ② 夹具合成"真实仓库里没有的形态"

第 5 章发现：**sourcepack 自己的 135 个提交里一个合并都没有**。
所以合并相关的代码路径只能靠夹具合成：

| 要覆盖的形态 | 夹具怎么造 | 对应章节 |
| :--- | :--- | :--- |
| 合并提交（两条父边） | `git checkout -b feat` + 两边各提交 + `git merge --no-ff` | 第 5 章 |
| 改名/删除/复制 | `git mv` + `git rm` + 内容微调（触发 `-M` 相似度） | 第 2/6 章 |
| 中文/空格/含 TAB 路径 | 文件名直接用中文与空格，断言用 `-z` 解析结果 | 第 12.4 |
| 坏 textconv | `.gitattributes` + `diff.<d>.textconv` 配一个会失败的命令 | 第 6 章 |
| 无提交仓库（unborn HEAD） | `git init` + 不提交 | 第 3 章 |
| detached HEAD | `git checkout --detach` | 第 3 章 |
| 跨 32KB 块的中文文件 | 直接写 183KB 中文+ASCII 文件 | 第 2 章（P0 bug） |

#### ③ 反向验证（Mutation Testing 的手工版）

**改完 bug 就把修复改回错误实现，确认测试真的变红**。本仓库的关键修复都做过：

| 反向改动 | 失败的用例 |
| :--- | :--- |
| 不标 `--staged` | `StagedCount = 0，期望 3` |
| 截断时吞掉 git 失败 | `截断时 git 失败仍必须报错` |
| 不挂包级 `pflag.Usage` | `-h 输出缺少 "Convenience shortcuts:"` |
| `DiffFull` 放回二进制检查之前 | `含裸字节的 diff 应丢弃且 DiffFull=0` |
| carry 拼接写回错误形式 | `中文注释+ASCII(chunk=2): isBinary=true，期望 false` |
| `printUsage` 恢复成默认输出 | `应恢复调用者原来的输出目标` |

**为什么必要**：不做这一步，你无法区分"测试通过"与"测试根本没跑到"。
**我自己就因为漏做而误判过一次**（补丁没打上却以为测试没覆盖），
所以后来固定成流程。

**规模**：Go **76 个用例 / 5 个文件** + Node **6 个用例**，CI 跑 3 OS × 3 Go 版本。

---

### 12.8 我们**没有**做的事（以及为什么）

| 没做 | 理由 | 章节 |
| :--- | :--- | :--- |
| 不引 go-git | 体积、`autocrlf`/filter/`.gitattributes` 的真实行为、报错可解释 | 第 1 章 |
| 不自己解析 revspec | 规则极多，交给 `rev-parse` + 包装报错 | 第 3 章 |
| 不自己实现 ignore 语义（除了 `.gdignore`） | git 模式由 git 给清单；`.gdignore` 是我们的机制才需要自实现 | 第 7 章 |
| 不改写历史 | 只读工具，零副作用 | 第 9 章 |
| 不动 repo 级优化（commit-graph/bitmap/repack） | 那是仓库所有者的选择 | 第 10 章 |
| 不装钩子 | 本地、可绕过、会污染用户仓库；闸门放 CI | 第 11 章 |
| 不做"自动拆分产物" | 按 token 装箱会把模块切碎，价值/复杂度不划算（已记 TODO） | — |

**这七条有一个共同点**：**先问"这件事该由谁负责"，再决定要不要自己实现**。
Git 已经负责的部分（对象、引用、合并、忽略、传输），我们只读不重写；
只有 Sourcepack 独有的概念（`.gdignore`、产物排版、变更集过滤）才自己实现。

---

### 12.9 结课 checklist：写"读 Git"的工具时照抄这 12 条

1. **所有 git 调用走一个封装**（`command()`），加固才不会被漏掉；
2. **禁分页、禁交互、禁 stdin、禁锁**：`--no-pager`、`GIT_PAGER=cat`、
   `GIT_TERMINAL_PROMPT=0`、`Stdin=nil`、`GIT_OPTIONAL_LOCKS=0`；
3. **`-c core.quotepath=false`**，否则中文路径变八进制转义；
4. **一切清单用 `-z`**；自定义字段用不可能出现的分隔符（别用空格/`|`）；
5. **固定协议、不猜配置**：`--src-prefix/--dst-prefix`、`--no-textconv`、`--no-ext-diff`；
6. **路径解析只剥一层前缀**（`a/` / `b/`），并记住仓库里可能真有 `b/` 目录；
7. **别用 `filepath.Match` 处理 `/` 分隔路径**（Windows 上它会跨过 `/`）；
8. **`-M` 才有重命名**，但要意识到它做内容相似度比较（大重构会变慢）；
9. **给子进程输出设体积上限**，并在截断后**排干管道**；
10. **只读命令清单化**，并在文档里承诺"不动仓库"；
11. **合并提交取第一父**，且把口径写进产物（用户才知道你在说什么）；
12. **以 git 为真值做差分测试 + 反向验证**，别只写"我想得到的用例"。

---

### 12.10 全课知识地图

```
第 1 章  设计哲学：快照 / 内容寻址 / 三棵树 ─────────────┐
                                                        │
第 2 章  对象模型：blob tree commit tag 的字节与哈希 ───┤
   ├─→ 空树 4b825dc… ────────► 用于"根提交/无提交"的 diff 基准
   └─→ blob 无文件名 ───────► 重命名免费、rename 靠 -M（第 6 章）
                                                        │
第 3 章  引用与 reflog：分支=41 字节 ───────────────────┤
   ├─→ detached HEAD ───────► symbolic-ref 失败的分支回退
   └─→ reflog ─────────────► 第 9 章的救援基础
                                                        │
第 4 章  索引：三棵树中间那棵 ─────────────────────────┤
   ├─→ staged/unstaged 两条边 ► 我们的三个计数
   └─→ stat 缓存 ───────────► 第 10 章的性能
                                                        │
第 5 章  提交图：DAG / 两条父边 / merge-base ──────────┤
   └─→ A..B vs A...B ───────► PR 审查口径（--base 的依据）
                                                        │
第 6 章  diff 与 patch：用时算 ─────────────────────────┤
   ├─→ 前缀可被配置改写 ────► --src-prefix 的由来
   └─→ textconv 会改/坏 diff ► --no-textconv 的由来
                                                        │
第 7 章  忽略规则：只管未跟踪 ─────────────────────────┤
   └─→ 父目录被排除不可救 ───► git 模式必须回溯祖先
                                                        │
第 8 章  远端与传输：增量 packfile ────────────────────┤
   └─→ 非快进保护 / force-with-lease
                                                        │
第 9 章  改写与恢复：造新对象 + 移动引用 ──────────────┤
   └─→ tag 不跟随改写 ──────► 发布锚点用 tag（第 11 章）
                                                        │
第 10 章 性能：delta / commit-graph / bitmap / partial ─┤
                                                        │
第 11 章 钩子 / 签名 / 发布链路 ───────────────────────┤
                                                        ▼
                                        第 12 章 收束回代码：
                                        gitRunner 的每条 hardening、
                                        命令清单、测试策略、checklist
```

---

### 12.11 结课自测（综合题）

1. 一个用户报告"我的中文文件名在快照里变成了乱码"，你会先查哪一层的哪个参数？
   （提示：第 12.2 与 12.5）
2. 产物里出现了两条一模一样的路径，其中一条是 `c/b/x.go`。可能是什么原因？
   （提示：第 6 章 mnemonicPrefix + 双剥前缀）
3. 用户说"我用 `-g --rev main` 审查我的分支，结果多出一个删除"。怎么解释、怎么建议？
   （提示：第 5 章双点/三点）
4. 为什么我们对 `git log --format` 用自定义分隔符，而不是空格？（第 12.3）
5. 如果要把 sourcepack 改成"同时支持 SVN 仓库"，`gitRunner` 这一层给你什么好处、
   又会有什么代价？（开放题）
6. 从 12.9 的 12 条里挑三条，说明它们各自对应前 11 章的哪个坑。

---

### 12.12 结课

这套课讲了一个工具与一个系统如何正确相处。回头看，最有价值的不是任何单条命令，而是三种态度：

1. **先理解机制，再写代码**——`4b825dc...` 不是魔法常量，是空树对象；
   `--src-prefix` 不是随手加的，是因为前缀可被用户配置改写；
2. **以真值为准，而不是以自己的想法为准**——
   14×40 的 `git check-ignore` 对照测试、反向验证、以及"没测出差异就写没测出差异"；
3. **先问该由谁负责**——Git 负责的（对象/引用/合并/忽略/传输）我们只读不重写，
   只有 Sourcepack 独有的概念才自己实现。

**课程文件**：`git-course/01-…` 到 `12-…`，全部示例都可在本机复现
（每章的"动手实验"小节给了完整命令）。**实验建议在隔离克隆里做**——
`git clone /path/to/sourcepack /tmp/git-lab`，这也是第 1 章第一件要做的事。

---

<a id="stub-back-10-appendix-a-glossary"></a>

# STUB back-10-appendix-a-glossary

占位内容，等待真实文件。

---

<a id="stub-back-20-appendix-b-commands"></a>

# STUB back-20-appendix-b-commands

占位内容，等待真实文件。

---

<a id="stub-back-30-appendix-c-map"></a>

# STUB back-30-appendix-c-map

占位内容，等待真实文件。

---

<a id="stub-back-40-appendix-d-reading"></a>

# STUB back-40-appendix-d-reading

占位内容，等待真实文件。

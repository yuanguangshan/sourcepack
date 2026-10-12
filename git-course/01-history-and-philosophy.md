# 第 1 章：起源与设计哲学

> **本章目标**：理解 Git 的"世界观"。学完你应该能回答：
> 为什么 Git 存快照而不是差异？为什么说它是个"内容寻址的文件系统"？
> 为什么"改历史"要造新对象？以及——为什么 sourcepack 选择调用 git CLI 而不是引库。
>
> 本章不需要背命令，只需要跟着跑几条，看清 Git 的骨架。

---

## 1.1 起源：一个被逼出来的工具（2005）

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

## 1.2 集中式 vs 分布式：本质区别

| | CVS / SVN（集中式） | Git（分布式） |
| :--- | :--- | :--- |
| 历史存在哪 | **只有服务器有** | **每个 clone 都有一份完整历史** |
| `commit` | 需要联网，写服务器 | 纯本地操作 |
| `log` / `diff` / `blame` | 通常要联网 | 本地，离线可用 |
| 分支 | 目录拷贝级别的重操作 | 一个 41 字节的文件 |
| 单点故障 | 服务器挂了就停摆 | 任何 clone 都能成为新的中心 |

**在标本仓库上验证"每个克隆都有完整历史"** `[lab]`：

```bash
$ git clone -q /path/to/sourcepack /tmp/git-lab && cd /tmp/git-lab   # 换成你自己的路径
$ git log --oneline | wc -l
135              # 写作时的数字，你的 clone 会不一样
$ git log --oneline -1
aa686d8 docs: v2.0.1 记录（-h 末尾换成参数手册博客版）；还原 v2.0.0 的 help 引用为当时真实输出
```

（这两行的具体数字只对当时那个快照成立：提交数只会增长，最新提交更是每次提交都变。
要看"结构"请自己数一遍——`git log --oneline | wc -l` 与 `git log --oneline -1`。）

**拔网线也一样能跑**——`git log`、`git diff`、`git blame` 全部是读本地
`.git/objects`，不需要任何网络。这就是"分布式"的实际含义。

> **回到 sourcepack**：正因为历史在本地，我们的 `-g`/`-G` 才能**纯离线**工作——
> 它们只是读本地对象库并渲染成 Markdown，从不联网（除了你显式用 `-p` 推送）。

---

## 1.3 快照 vs 差异：Git 的世界观

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

## 1.4 内容寻址与不可变性

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

## 1.5 三个核心抽象

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

## 1.6 为什么 sourcepack 调用 git CLI，而不是引入 go-git

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

## 1.7 动手实验 `[lab]`

```bash
# 0) 建实验克隆（如果你还没建）
#    把 /path/to/sourcepack 换成你本机的路径（或直接 clone 本仓库）
git clone -q /path/to/sourcepack /tmp/git-lab && cd /tmp/git-lab
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

### 自测题

1. `git hash-object` 算哈希时，哈希的输入里除了文件内容还有什么？（提示：`blob 6\0`）
2. 如果 Git 存的是快照，为什么改一个文件不会让仓库体积翻倍？
3. `git commit --amend` 之后，旧提交还在磁盘上吗？为什么？

<details>
<summary>点开看答案</summary>

1. **还有一段"类型 + 长度"头部**。哈希的输入是
   `类型` + 空格 + **内容的字节长度** + `\0` + 内容本身，即对 `blob 6\0hello\n`
   整体取 SHA-1。所以同一个字节串，作为 blob 与作为 tree 的哈希完全不同——
   **类型和长度都是哈希输入的一部分**。第 2 章会用 Python 把它逐字复现出来。

2. **因为快照是按内容寻址的，未改动的文件在新树里被"复用"而不是重存**。
   改一个文件只会产生一个新 blob；新 tree 里其余条目的哈希与旧 tree 完全一致，
   Git 直接指向已有对象，不复制数据。所以体积增长与"改了多少"相关，
   而与"仓库里有多少文件"无关。（第 10 章还会补一层：packfile 对相似对象做
   delta 压缩，进一步压掉重复。）

3. **还在。** `amend` 并不是"编辑"那个提交，而是**造一个新提交**、然后把分支指针移过去
   ——这正是第 1.4 节说的"对象不可变，改历史 = 造新对象 + 移动引用"。
   于是旧提交对象仍留在对象库里（对 `git reflog` 与 `ORIG_HEAD` 可见），
   要等到 `gc` 且不再被任何引用或 reflog 触及时才会被清掉。
   第 9 章的"三层救援"能成立，前提就是这一点。

</details>

---

## 1.8 小结

- Git 是为**内核级规模**设计的：速度、廉价分支、完全分布式；
- 它存**快照**（完整树）而不是差异；diff 是用时算的；
- 键是**内容的 SHA-1**，对象**不可变**，改历史 = 造新对象 + 移动引用；
- 内部只有三块：**对象库**（不可变）、**引用**（可变指针）、**索引**（候选快照）；
- sourcepack 选择"调 git CLI + 只读操作"，换来的是行为一致性与可解释的报错，
  代价是进程启动开销（第 8 章量化优化）。

**下一章**：把对象库拆开——看 blob/tree/commit 的真实字节、哈希的计算过程，
以及 `changeset.go` 里那个空树常量 `4b825dc6...` 的来历。

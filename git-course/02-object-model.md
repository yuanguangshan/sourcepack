# 第 2 章：对象模型

> **本章目标**：把 `.git/objects` 拆开看清它的字节。学完你应该能回答：
> blob/tree/commit 的哈希到底是怎么算的？磁盘上那个 21 字节的文件里是什么？
> `git diff` 面对一个"没有父提交的根提交"时拿什么当基准？（答案直接对应
> `changeset.go` 里的一个常量。）
>
> 本章是"设计哲学 → 具体实现"的转折：所有概念都开始有字节级的证据。

---

## 2.1 四种对象

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

## 2.2 哈希是怎么算的：动手复现 `[深潜]`

> **[深潜] 2.2–2.3 可以跳过。** 这两节用 Python 复现哈希、并拆开磁盘上的对象，
> 适合想弄清"到底是什么字节"的人。如果你只想先建立起整体模型，
> **直接跳到 2.4**：后面所有章节都不依赖这两节。

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

## 2.3 磁盘上的对象：zlib 压缩 + 目录分片 `[深潜]`

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

## 2.4 commit 对象：字段与"信息是怎么变成摘要的"

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

## 2.5 tree 对象与文件模式

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

## 2.6 空树对象：`4b825dc...` 从哪来

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

## 2.7 手工造一个提交（只用 plumbing）

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

## 2.8 blob 不含文件名的两个后果

### 后果一：重命名不改变 blob

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
> `git mv`/rename 支持靠的正是 `-M` 参数（见 `changeset.go` 里
> `diffCommon` 那一组参数，本版在 353 行附近）：
> `diff --no-ext-diff --no-textconv --no-color -M --src-prefix=a/ --dst-prefix=b/`。

### 后果二：内容相同的两个文件只存一份

```bash
$ git hash-object README.md
32c7dff26af3fa0745103d0104d0c8f58b3971f2
$ git ls-tree HEAD README.md | awk '{print $3}'
32c7dff26af3fa0745103d0104d0c8f58b3971f2
```

工作区文件的哈希 == 对象库里那个 blob 的哈希。**这就是 `git status` 的原理**
（第 4 章详细拆）：比较工作区文件、索引、HEAD 三方的哈希。

---

## 2.9 松散对象 vs packfile：`count-objects -v` 告诉我们什么

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

## 2.10 SHA-1 与 SHA-256：一个正在发生的迁移

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

## 2.11 动手实验 `[lab]`

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

### 自测题

1. 如果两个文件内容完全相同、文件名不同，`git add` 之后 `.git/objects` 里有几个对象？
2. `git commit` 之后索引变了吗？HEAD 变了吗？如果此时 `git status`，三个区域谁和谁比对？
3. 手工造的 commit 有什么办法让它出现在 `git log` 里？（提示：第 3 章，给它一个引用）

---

## 2.12 小结

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

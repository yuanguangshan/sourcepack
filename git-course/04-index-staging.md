# 第 4 章：索引（暂存区）

> **本章目标**：搞清 `.git/index` 到底是什么、`git add` 究竟写了哪些字节，
> 以及为什么"同一个文件可以既是 staged 又是 unstaged"。
> 学完你会理解 Git 的三棵树模型，并能解释 sourcepack 里
> `staged · unstaged · untracked` 这三个计数是怎么来的——
> 以及它在 v1.8.0 里为什么**恒为 0**。
>
> **本章的 `[深潜]` 节**：**4.2**（逐字节读索引的 `DIRC` 头）——**可以跳过**。
> 跳过后从 **4.3** 继续读，不影响本章其余部分，也不影响后面任何一章。
> （深潜节给的是"字节级证据"；先建立模型的话记住结论即可，需要时再回来。）


---

## 4.1 索引是"三棵树"里的中间那棵

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

**日常命令分别动哪棵树**（这张图能挡住 90% 的"我明明 add 了"类困惑）：

```mermaid
flowchart LR
    HEAD["HEAD 指向的 commit<br/>上次提交"]
    WT["工作区<br/>你此刻的文件"]
    IDX["索引 .git/index<br/>下次提交的候选快照"]
    NEW["新 commit 对象"]
    BR["分支指针<br/>refs/heads/main"]

    WT -->|"git add"| IDX
    HEAD -->|"git restore --staged<br/>（撤销 add：索引退回 HEAD 的内容）"| IDX
    IDX -->|"git commit"| NEW
    NEW -->|"成功后分支前移，HEAD 跟着走"| BR
    BR -->|"git checkout / switch<br/>用目标分支的内容同时改写索引与工作区"| WT
    HEAD -.->|"git diff --cached、git diff<br/>（只读：不改任何一棵树）"| IDX
```

**读法**：`add` 只写索引（所以 `git status` 第一列变了、第二列没变）；
`commit` 只读索引写对象，再挪分支指针；`checkout` 是唯一"一次改写两棵树"的日常命令。
`reset` 的三种模式是这张图的特例——第 9 章会用同一张图讲清楚它到底动了哪几棵。

> 记住这张图，"staged 是什么意思"就再也不会混了：
> **staged = 索引里的内容与 HEAD 不同**，而不是"我运行过 add"。

---

## 4.2 索引是二进制文件：逐字节看 `[深潜]`

> **[深潜] 本节可以跳过。** 它逐字节解剖 `.git/index` 的 `DIRC` 头，
> 是"为什么索引能那么快"的证据链。想直接学操作语义的，**跳到 4.3**。

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

## 4.3 `git add` 到底做了什么

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

## 4.4 三种状态与 porcelain 输出的两种列

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

## 4.5 三种 `git diff` 各看什么

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

## 4.6 `MM`：同一个文件的两份内容并存

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

## 4.7 索引的第二个身份：合并工作台（stage 1/2/3）

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

## 4.8 其它常用索引操作

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

## 4.9 回到 sourcepack：三个计数是怎么来的

### 4.9.1 取三个集合（代码原样，`changeset.go:366-388`）

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

### 4.9.2 用 git 的真值校验我们的输出

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

### 4.9.3 为什么 v1.8.0 里这三个计数**恒为 0**

这是一次真实事故，根因就在 4.9.1 那段的 `if` 条件里：

```go
	if !config.GitStaged && !cs.Unborn {      // ← --staged 模式时整段被跳过
```

`--staged` 模式下 `stagedSet` / `unstagedSet` **都不填充**（因为清单本身来自
`git diff --cached`，当时的作者认为"不必再算一遍"）。而计数函数是这么写的
（`changeset.go` 的 `summarizeChangeSet()`，本版在 583 行起）：

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

## 4.10 动手实验 `[lab]`

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
   〔细节在 4.2 节 `[深潜]`；跳过那节的读者按结论作答即可〕
5. sourcepack 的 `--staged` 模式为什么可以"直接标记 Staged=true"，而不必再跑 diff？

<details>
<summary>点开看答案</summary>

1. **staged = "索引里的内容与 HEAD 不同"，而不是"我运行过 `add`"。** 见 4.1 的
   三棵树图：`git diff --cached` 比对 HEAD ↔ 索引，有差异才叫 staged；`git diff`
   比对索引 ↔ 工作区，有差异才叫 unstaged。由此可推：若 `add` 进去的内容与 HEAD
   相同（文件本来就没改），索引与 HEAD 相等，`git status` 什么都不会显示。

2. **提交的是索引里那一版，也就是你 `add` 那一刻的内容。** `git commit` 只把
   **索引**写成 tree，工作区里之后改的东西完全不进这次提交（4.6）。所以
   `add` → 再改工作区 → `commit` 之后，提交里是第一次 add 的版本，而
   `git status --porcelain` 显示 ` M`（工作区与索引仍不同）——那部分改动要等下一次
   `add` + `commit` 才进历史。`MM`（两份内容并存）与 `git add -p` 的部分提交，
   都是这条规则的直接应用。

3. **因为冲突时索引是"合并工作台"，同一路径要同时保住三方版本**（4.7）。平时一个
   路径只有 stage 0 一条；冲突时出现三条：stage 1 = 共同祖先（merge base）、
   stage 2 = ours（当前分支）、stage 3 = theirs（被合并进来的）。4.7 实测的三条
   哈希（`df967b96…` / `b19a1e93…` / `950b81b7…`）我在 /tmp 沙箱用同样内容复现，
   逐字相同。工作区的 `<<<<<<<` 标记只是给人看的提示，真正的三方数据在索引里；
   解决冲突 = 一次 `git add` 把三条归一回 stage 0（实得 `2ab19ae6…`）。

4. **因为索引缓存了每个文件的 stat 信息，可以先用它做廉价筛选**（4.2）。
   `.git/index` 的每个条目含 `ctime / mtime / dev / ino / uid / gid / size` +
   mode + blob 哈希 + 路径；`git status` 先比 stat，只有对不上的才去读内容算哈希。
   这就是上万个文件的项目里 `git status` 依然飞快的原因（代价是 racy timestamp
   那类坑，留到第 10.5 节）。

5. **因为 `--staged` 的清单本身就来自 `git diff --cached`，"来源即证据"**
   （4.9.1、4.9.3）。清单里每一条必然满足"索引与 HEAD 不同"，所以不必再跑一次
   diff 去确认；v1.8.0 恒为 0 的根因正是清单来源与两个集合的填充被
   `if !config.GitStaged && !cs.Unborn` 割裂，导致 `summarizeChangeSet` 的 switch
   四个分支全落空。收益是省掉一次冗余 diff、语义上也不可能标错；**代价**是这个
   显式标记隐含依赖"清单只有这一个来源"的前提——将来若 `--staged` 与普通模式共用
   清单来源，这句赋值必须重新审视（同时 `--staged` 时 untracked 本应为 0，因为
   未跟踪文件不可能被暂存）。

</details>

---

## 4.11 小结

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

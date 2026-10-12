# 第 5 章：提交图与分支合并

> **本章目标**：把"历史"从"一串提交"升级为**有向无环图（DAG）**来看。
> 学完你能解释：合并提交为什么有两条父边、`^1`/`^2` 是什么、
> `A..B` 与 `A...B` 差在哪，以及为什么"和 main 比"在 PR 审查里是个坑。
>
> 本章最后一个实测会给出**量化证据**：同一个分支，用错基准会多出 1 个文件、
> 多算 1 行删除。

---

## 5.1 提交图：parent 边构成的 DAG

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

**一个真实观察**（写作时 sourcepack 有 135 个提交）：**没有任何合并提交**：

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

## 5.2 分叉与合并：在实验室里造一个

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

## 5.3 合并的两种形态

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

## 5.4 两条父边：`^1`、`^2` 与 `--first-parent`

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

## 5.5 对合并提交做 diff：三种都合法，含义完全不同

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

## 5.6 双点 `..` vs 三点 `...`（最容易踩的坑）

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

## 5.7 merge-base 是怎么算的

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

## 5.8 回到 sourcepack：三个具体问题

### 5.8.1 合并提交的 diff 取第一父

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

### 5.8.2 `-G` 的"最近 N 个提交"就是 `git log -N`

```bash
$ gx -G 3 --dry-run -d .
📦 Git changes: git log -3
  Commits:             3
  Files changed:       2 (+4 −0)
```

**注意它没有用 `--first-parent`**：所以 `-G 3` 里可能既有合并提交、
也有特性分支内部的提交（本例 3 个提交里含 1 个 merge + 2 个普通提交）。

### 5.8.3 **PR 审查的经典坑：`--rev main` 是双点语义**

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

## 5.9 动手实验 `[lab]`

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

## 5.10 小结

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
- 我们自己的提交历史里**一个合并都没有**（写作时自查：`git rev-list --count --merges HEAD` = 0），
  所以合并路径全靠夹具合成，
  这是"测试盲区"的一个真实来源。

**下一章**：diff 算法与 patch 格式——Myers 算法在做什么、unified diff 的每一行怎么读、
`a/` `b/` 前缀为什么会让我们的路径解析出错（第 2 章已经埋了 `--src-prefix` 的线），
以及 rename detection 的成本。

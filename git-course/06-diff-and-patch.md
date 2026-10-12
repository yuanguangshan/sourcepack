# 第 6 章：diff 算法与 patch 格式

> **本章目标**：读懂 unified diff 的每一行，理解 diff 是"**用时算出来的**"而不是存下来的，
> 并看清三个真实陷阱：**用户配置会改写前缀**、**textconv 能让 git diff 直接失败**、
> **`-M` 让 diff 需要做内容相似度比较**。
> 本章最后回到 `changeset.go`，解释两个幽灵条目 bug 的来历。

---

## 6.1 diff 的本质：两个快照的比较

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

## 6.2 Myers 算法：最小编辑脚本

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

## 6.3 unified diff 逐行解剖

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

## 6.4 hunk 头的数学

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

## 6.5 上下文与空白选项

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

## 6.6 文件末尾没有换行符

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

`changeset.go` 的 `splitPatch()` 里用 **`inHunk` 状态机**解决
（本版在 720–760 行；代码原样）：

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

## 6.7 前缀：`a/` `b/` 为什么会变

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

## 6.8 rename / copy 检测：`-M` / `-C`

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

## 6.9 二进制文件与 textconv：`--no-textconv` 的真实价值

Git 判定二进制的规则很简单：**前 8000 字节里有没有 NUL**
（`buffer_is_binary`）。命中就不给文本 diff，只给一行：

```bash
$ printf 'PNG\x00\x02changed' > logo.bin
$ git diff logo.bin
diff --git a/logo.bin b/logo.bin
index e65757e..b11395c 100644
Binary files a/logo.bin and b/logo.bin differ
```

> **与 sourcepack 的联系**：我们**早期**照这个思路写过 `binaryProbeSize = 8000`
> （原注释写着"与 git 的 buffer_is_binary 一致"）。但它后来被
> **改成整文件校验**——因为按字节截断的窗口会把"切在多字节字符中间"的合法 UTF-8
> 误判成二进制（v1.8.1 的真实 P0 bug，见 `PARAMETERS.md`）。
>
> 现在的实现在 `main.go` 的 `isBinaryBuffer()`（本版 766 行附近）——
> 判据只有两条：**含 NUL 就是二进制，否则必须是合法 UTF-8**。
> 这里值得记住的是：**搬 Git 的实现时要连它的假设一起搬**，Git 用 8000 字节探针是
> 因为它假定输入多半是 ASCII/mostly-clean；而我们既然会读任意中文文件，
> 就得把这条假设换掉。

### textconv：让用户配置"翻译"二进制

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

## 6.10 四种 diff 算法：差异出现在重复行上

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

## 6.11 回到 sourcepack：解析 patch 的四处硬骨头

### 6.11.1 从 `diff --git` 行提取路径

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

### 6.11.2 `parseDiffPath`：**只剥一层**前缀（v1.8.0 的双剥 bug）

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

### 6.11.3 按 `diff --git` 切块，再"贴"回权威清单

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

### 6.11.4 体积上限与二进制防线

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

## 6.12 动手实验 `[lab]`

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

<details>
<summary>点开看答案</summary>

1. **依次是：旧侧起始行 1、旧侧行数 4、新侧起始行 1、新侧行数 5。**
   格式是 `@@ -<旧起点>,<旧行数> +<新起点>,<新行数> @@`（6.4）：`-1,4` 指旧文件第 1–4 行，
   `+1,5` 指新文件第 1–5 行。起点是 1-based；行数为 1 时可省略（`@@ -5 +5,2 @@`）；
   行数为 0 时起点表示"插入位置前一行"（新增文件用 `-0,0`）。
   两侧行数差就是净增删行数（5−4=+1）——我在 6.12 的 lab 里复现，输出正是 `@@ -1,4 +1,5 @@`。

2. **因为它是协议行：一行里同时含旧、新两个路径，且路径本身可能带空格，任何按空格切分的解析都会错。**
   6.3 的例子 `a/my file.txt b/my file.txt`，用 `strings.Fields` 或 `Split("/")` 都还原不出来（6.11.1）。
   稳妥做法是双保险：用 `" b/"` 作锚点取新路径（前提是显式固定了 `--dst-prefix=b/`），
   而**权威路径来源是 `git diff --name-status -z`**（NUL 分隔，不怕空格），patch 只按路径"贴"回这份清单。

3. **`--- ` 会变成 `--- i/f.txt`、`+++ ` 变成 `+++ w/f.txt`（i=index、w=worktree）；若开 `diff.noprefix=true` 则干脆没有前缀。**
   防御是"不猜用户配置、把协议固定下来"：每次调用都显式带 `--src-prefix=a/ --dst-prefix=b/`
   （6.7 实测加上后恢复 `a/`/`b/`），并且 `parseDiffPath` **只剥一层**匹配到的前缀。
   两者缺一不可——v1.8.0 正是"不固定前缀 + 连剥两层"叠加出了幽灵条目。

4. **默认 `-M` 的相似度阈值是 50%，`-M100%` 要求两侧内容完全一致才认改名。**
   6.8 的实测（改名 + 改 1/20 行）：默认给 `R094 old_name.py new_name.py`；`-M100%` 退化成
   `A new_name.py` + `D old_name.py` 两条独立记录（我复现了同一现象：默认 `R086`、`-M100%` 变 `A`+`D`；
   而**纯改名不改内容**时 `-M100%` 仍给 `R100`）。**变慢的是大重构**：`-M` 要在"删除的文件 × 新增的文件"
   之间两两比较内容，一次删 500 加 500 就是约 25 万次比较；`-C` 更贵，它要在所有删除与新增之间找相似块。

5. **保护的是"工具不被用户配置里的外部命令拖垮"，不是洁癖式加固。**
   6.9 实测：`.gitattributes` 给 `blob.dat` 配了 `diff=mybin` 的 textconv 且命令执行失败时，
   `git diff blob.dat` 直接 `fatal: unable to read files to diff`、**退出码 128**；
   加上 `--no-textconv` 就回到 `Binary files a/blob.dat and b/blob.dat differ`、退出码 0。
   同一仓库里 `gx -g` 因为默认带 `--no-textconv`（同理 `--no-ext-diff`）照常退出 0，
   产物里标 `binary` 与 `+0 −0`。

6. **幽灵变更 = patch 里的块找不到权威清单里的主人，被 `appendUnmatchedChunks` 当成"新变更"追加（6.11.3）。**
   两个成因都在路径解析上：① `parseDiffPath` **连剥两层前缀**——仓库真有 `b/` 目录时
   `--- a/b/x.go` 被剥成 `x.go`（应为 `b/x.go`）；② **没固定前缀**——用户开 `mnemonicPrefix` 后
   `c/`（commit 前缀）被当成路径的一部分留了下来。两者叠加的实测产物就是 `c/b/gone.go modified +0 −1`
   这条假记录：6.11.2 的表格里 v1.8.1 变更数为 2，v1.8.2 修好后为 1（`b/gone.go deleted +0 −1`）。

</details>

---

## 6.13 小结

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

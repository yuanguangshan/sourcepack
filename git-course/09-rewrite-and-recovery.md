# 第 9 章：历史改写与恢复

> **本章目标**：把"改写历史"从玄学变成可推理的操作。
> 核心只有一句话：**对象不可变（第 2 章），所以改写 = 造一批新对象 + 把引用指过去**。
> 学完你能预测每个命令会动三棵树里的哪几棵、哪些哈希会变、
> 以及**事故之后按什么顺序救**。

---

## 9.1 改写的本质

| 你以为的 | 实际发生的 |
| :--- | :--- |
| "`amend` 修改了那个提交" | 造了一个**新**提交对象，把分支引用挪过去；旧的仍在对象库里 |
| "`rebase` 移动了提交" | 把每个提交**重新应用到新基底**，产生一串**全新**提交 |
| "`reset` 删掉了提交" | 只把引用往回挪；对象仍在（reflog 还引用着它） |

**这就是"几乎所有误操作都能恢复"的根基**：Git 从不修改对象，只移动引用（第 3 章）。

---

## 9.2 `reset` 的三种模式：动三棵树里的哪几棵

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

## 9.3 `revert`：只增不改的反向提交

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

## 9.4 `cherry-pick`：复制"改动"，不是复制提交

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

## 9.5 `amend`：最常用的"改写"

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

## 9.6 `rebase`：把提交"重放"到新基底

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

### 交互式 `rebase -i`：squash / reword / drop

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

## 9.7 改写之后，什么会"跟不上"

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

## 9.8 恢复的三层：ORIG_HEAD → reflog → fsck

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

## 9.9 清理历史的两把刀

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

## 9.10 回到 sourcepack：我们不改写历史，但会被它影响

### 9.10.1 只读工具：从不参与改写

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

### 9.10.2 但产物里记着哈希——改写会让它"对不上"

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

### 9.10.3 `--rev` 支持 reflog 语法，改写后特别好用

`HEAD@{1}` 是合法 revspec（第 3 章实测过），于是"刚 rebase 完，看看改了什么"变成：

```bash
$ gx -g --rev 'HEAD@{1}' --dry-run -d .
📦 Git changes: git diff HEAD@{1}  (working tree + index vs HEAD@{1})
```

**这比记住旧哈希方便得多**——因为 reflog 里就写着"上一步在哪"。

---

## 9.11 动手实验 `[lab]`

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

## 9.12 小结

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

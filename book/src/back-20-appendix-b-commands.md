# 附录 B　命令速查

> 本附录按**场景**而不是按字母序组织，收录书中出现过的全部 Git 命令
> （编排时共 **167 条**；每条都经 `git help -a` 确认存在）。
>
> 命令数量会随 Git 版本与本书版次变化，所以下面各分组只说明章节覆盖，不再报总数。
> 每条后面跟一句"**它到底干了什么**"，而不是只列命令名。
>
> **验证说明**：本附录里带 `[验]` 标记的命令，我都在本机
> （macOS / `git version 2.39.5 (Apple Git-154)`）用 `git help -a` 逐个确认存在，
> 并实际跑过一遍核对行为。少数需要特定状态才成立的（如 `verify-pack` 需要先有
> pack、`check-ignore` 需要先有忽略规则），命令说明里已注明前置条件。

---

## B.0　任务速查：我想干什么 → 用哪条命令

> B.1 起的分组表是**命令优先**的：你得先知道命令叫什么，才能去查它干什么。
> 这一节反过来——**从"我想干什么"出发**，先给命令，再指向讲原理的那一节。
> 它补的是"命令还没背下来"那个阶段；等背下来了，B.1 起的表更好用。
>
> 官方那张 [Git Cheat Sheet](https://git-scm.com/cheat-sheet)（git-scm.com，MIT）
> 用的也是这种"任务 → 命令"组织方式，值得对照着看；
> 本节是它的**中文重写版**，并按本书章节做了索引（版权与许可说明见附录 D.1）。

### 起手：拿到仓库 / 建仓库

| 我想干什么 | 命令 | 细节 |
| :--- | :--- | :--- |
| 新建一个仓库 | `git init` | 第 2 章 |
| 把别人的仓库弄到本地 | `git clone <url>` | 第 8.1 节 |
| 只要最近一次提交（省时间省流量） | `git clone --depth=1 <url>` | 第 10.6 节 |
| 只要历史、不要文件内容 | `git clone --filter=blob:none <url>` | 第 10.6 节 |

### 把改动放进"下一次提交"

| 我想干什么 | 命令 | 细节 |
| :--- | :--- | :--- |
| 暂存一个文件 | `git add <file>` | 第 4.3 节 |
| 暂存全部改动（含删除） | `git add -A` | 第 4.3 节 |
| 只暂存文件里的某几行 | `git add -p` | 第 4.3 节 |
| 改文件名 | `git mv <old> <new>` | 第 2.8 节（blob 不变） |
| 删文件 | `git rm <file>` | 第 4.8 节 |
| 让 Git 别再跟踪它（文件留着） | `git rm --cached <file>` | 第 4.8 节 |
| 把一个文件从暂存区撤下来 | `git restore --staged <file>`（旧写法 `git reset <file>`） | 第 4.8 节 |
| 现在到底是什么状态 | `git status`（第一列 staged、第二列 unstaged） | 第 4.4 节 |

### 提交

| 我想干什么 | 命令 | 细节 |
| :--- | :--- | :--- |
| 提交（开编辑器写信息） | `git commit` | 第 2.4 节 |
| 一句话提交 | `git commit -m 'msg'` | 第 2.4 节 |
| 跳过暂存直接提交（只对**已跟踪**文件有效） | `git commit -am 'msg'` | 第 4.3 节 |
| 补进上一次提交（改信息或补文件） | `git commit --amend` | 第 9.5 节（哈希会变） |
| 撤销最近一次提交、改动留着 | `git reset --soft HEAD~1` | 第 9.2 节 |

### 分支与切换

| 我想干什么 | 命令 | 细节 |
| :--- | :--- | :--- |
| 列分支 | `git branch` | 第 3.1 节 |
| 按最近提交时间排 | `git branch --sort=-committerdate` | 第 3.1 节 |
| 新建并切过去 | `git switch -c <name>`（旧写法 `git checkout -b <name>`） | 第 3.7 节 |
| 切分支 | `git switch <name>` | 第 3.7 节 |
| 删已合并的分支 | `git branch -d <name>` | 第 3.3 节 |
| 强删分支 | `git branch -D <name>` | 第 3.3 节 |
| 临时看一眼某个提交 | `git switch --detach <commit>` | 第 3.7 节（detached HEAD） |

### 看"改了什么"

| 我想干什么 | 命令 | 细节 |
| :--- | :--- | :--- |
| 看未暂存的改动 | `git diff` | 第 4.5 节 |
| 看已暂存的改动 | `git diff --staged` | 第 4.5 节 |
| 看相对上次提交的全部改动 | `git diff HEAD` | 第 4.5 节 |
| 看某个提交改了什么 | `git show <commit>` | 第 6.3 节 |
| 看两个提交之间的差异 | `git diff <a> <b>` | 第 5.5 节 |
| 只和分叉点比（PR 口径） | `git diff main...HEAD` | 第 5.6 节 |
| 只要统计 | `git diff --stat` | 第 6.3 节 |

### 看历史

| 我想干什么 | 命令 | 细节 |
| :--- | :--- | :--- |
| 一行一条 | `git log --oneline` | 第 5.1 节 |
| 画提交图 | `git log --graph --oneline --all` | 第 5.1 节 |
| 只看主干 | `git log --first-parent` | 第 5.4 节 |
| 某个文件的历史（跨改名） | `git log --follow -- <file>` | 第 2.8 节 |
| 谁改的某一行 | `git blame <file>` | 第 6.1 节 |
| 数提交个数 | `git rev-list --count HEAD` | 第 5.4 节 |
| 某段文字是哪次提交引入/删掉的 | `git log -G 'pattern'` | 第 6.2 节 |

### 撤销与救援

| 我想干什么 | 命令 | 细节 |
| :--- | :--- | :--- |
| 丢弃某个文件的工作区改动 | `git restore <file>` | 第 4.8 节 |
| 丢弃全部暂存+工作区改动（危险） | `git reset --hard` | 第 9.2 节 |
| 删未跟踪文件（先 `-n` 空跑） | `git clean -n` → `git clean -fd` | 第 4.8 节 |
| 临时收起手头改动 | `git stash` / `git stash pop` | 第 4.8 节 |
| 撤销一次提交但保留历史（安全） | `git revert <commit>` | 第 9.3 节 |
| 把某次提交复制到当前分支 | `git cherry-pick <commit>` | 第 9.4 节 |
| **提交或分支"丢了"** | `git reflog` → 找到哈希 → `git reset --hard <哈希>` | 第 3.8 节、第 9.8 节 |
| 回到上一次操作之前 | `git reset --hard ORIG_HEAD` | 第 9.8 节 |

### 合并与变基

| 我想干什么 | 命令 | 细节 |
| :--- | :--- | :--- |
| 把分支合进来 | `git merge <branch>` | 第 5.3 节 |
| 强制留下合并提交（保住特性边界） | `git merge --no-ff <branch>` | 第 5.3 节 |
| 压成一个提交合进来 | `git merge --squash <branch>` 然后 `git commit` | 第 5.3 节 |
| 把当前分支重放到新基底 | `git rebase <base>` | 第 9.6 节 |
| 整理最近 5 个提交（squash/reword/drop） | `git rebase -i HEAD~5` | 第 9.6 节 |
| rebase 搞砸了 | `git reflog` 找 rebase 之前的哈希 | 第 9.8 节 |

### 远端

| 我想干什么 | 命令 | 细节 |
| :--- | :--- | :--- |
| 加远端 | `git remote add <name> <url>` | 第 8.1 节 |
| 拉取但不改本地分支 | `git fetch origin` | 第 8.4 节 |
| 拉取并变基 | `git pull --rebase` | 第 8.2 节 |
| 推当前分支（首次用 `-u` 建立跟踪） | `git push` / `git push -u origin <name>` | 第 8.2 节 |
| 推标签 | `git push --tags` | 第 8.3 节 |
| 安全强推 | `git push --force-with-lease` | 第 8.6 节 |
| 看远端有哪些分支/标签 | `git ls-remote origin` | 第 8.1 节 |

### 配置与文件

| 我想干什么 | 命令 | 细节 |
| :--- | :--- | :--- |
| 设用户名/邮箱 | `git config user.name '...'` / `git config user.email '...'` | 第 8.8 节 |
| 写全局配置 | `git config --global ...` | 第 8.8 节 |
| 设别名 | `git config alias.st status` | 第 8.8 节 |
| 列出全部配置及其来源文件 | `git config --list --show-origin` | 第 8.8 节 |
| 忽略文件 | 编辑 `.gitignore`；排查用 `git check-ignore -v <path>` | 第 7.1 节、第 7.3 节 |
| 看某个版本里的某个文件 | `git show <rev>:<path>` | 第 2.5 节 |

---

## B.1　看历史与图

| 命令 | 它到底干了什么 |
| :--- | :--- |
| `git log --oneline -20` `[验]` | 用"短哈希 + 标题"一行一条地列最近 20 个提交。 |
| `git log --graph --oneline --all` `[验]` | 画出 ASCII 提交图（能看见分叉与汇合），`--all` 连所有引用一起看。 |
| `git log --first-parent --oneline` `[验]` | 只沿**第一父**链走，把特性分支内部的中间提交折叠掉——"主干推进史"视图。 |
| `git log --merges --oneline` `[验]` | 只列合并提交（parent 数 ≥ 2 的那些）。 |
| `git log --no-merges --oneline` `[验]` | 只列非合并提交，用来数"实际改动次数"。 |
| `git log -1 --format='%H%n%an%n%ad%n%s'` `[验]` | 按指定字段取**一个**提交（完整哈希/作者/日期/标题），做摘要最常用。 |
| `git log -1 --format=%P` `[验]` | 只取父提交列表（空格分隔）——判"是不是合并提交"就看它有几个字段。 |
| `git log --name-status -M --follow -- <path>` `[验]` | 追踪一个文件的历史（`--follow` 跨改名），并显示每次的状态（`R093` 这种就是改名 + 相似度）。 |
| `git log -z --name-only` `[验]` | 用 NUL 分隔文件名输出——路径含空格/换行时唯一可靠的解析方式。 |
| `git log --date=short --format=...` `[验]` | 把日期格式固定成 `YYYY-MM-DD`，避免不同环境输出不同。 |
| `git shortlog -sn HEAD` `[验]` | 按作者统计提交数，降序。 |
| `git rev-list --count --all` `[验]` | 只数提交个数，不渲染任何输出（比 `git log \| wc -l` 快得多）。 |
| `git rev-list --max-parents=0 HEAD` `[验]` | 找**根提交**（没有任何父提交的那个）。 |
| `git rev-list --objects --all` `[验]` | 列出所有可达对象（含 blob/tree）——观察"仓库到底有多少对象"的最直接方式。 |
| `git show --stat --oneline <rev>` `[验]` | 看单个提交的元信息 + 改动文件统计。 |
| `git show -m <merge>` `[验]` | 对合并提交**分别**按每个父提交显示 diff（否则 `git show <merge>` 默认什么都不显示）。 |
| `git blame -L1,20 <file>` `[验]` | 逐行显示"最后改这一行的是哪个提交"。 |
| `git describe --tags` `[验]` | 用最近的 tag 描述当前提交（形如 `v2.0.1-3-g1a2b3c4`），发布脚本常用来生成版本号。 |
| `git diff-tree --no-commit-id --name-only -r <rev>` `[验]` | 机器友好地列出某提交改动的文件；`-r` 必须加，否则不递归子目录。 |

**常见误用**：用 `git log | wc -l` 数提交。一旦历史里有合并提交，这个数字就不再等于"提交数"与"改动次数"中的任何一个；要数字就用 `git rev-list --count`。

---

## B.2　看差异与内容

| 命令 | 它到底干了什么 |
| :--- | :--- |
| `git diff` `[验]` | 比较**索引 ↔ 工作区**（即 unstaged 改动）。 |
| `git diff --cached` / `--staged` `[验]` | 比较**HEAD ↔ 索引**（即 staged 改动）；两者完全等价。 |
| `git diff HEAD` `[验]` | 比较**工作区 ↔ HEAD**，是上面两种改动之和。 |
| `git diff A...B` `[验]` | 从 `merge-base(A,B)` 比到 B——"我这条分支相对分叉点做了什么"。 |
| `git diff A..B` `[验]` | 两个尖端**直接**比；会把 A 自己的新增反向算成"B 删掉了它"。 |
| `git diff $(git merge-base A B) B` `[验]` | 三点语义的显式写法（不需要记 `...`），PR 审查场景最稳。 |
| `git diff --name-status -z -M` `[验]` | 只给"状态 + 路径"的权威清单，NUL 分隔、带重命名检测——机器解析的首选。 |
| `git diff --stat` `[验]` | 只给每个文件的增删行数小结。 |
| `git diff -U0` `[验]` | 上下文行数设为 0，只输出真正改动的行。 |
| `git diff -w` / `-b` `[验]` | 忽略全部空白差异 / 忽略空白**数量**变化。 |
| `git diff --word-diff` `[验]` | 按词而不是按行显示改动。 |
| `git diff --no-index <a> <b>` `[验]` | 比较两个任意文件（不必在仓库里）——做实验时最顺手；注意它退出码 **1** 表示"有差异"。 |
| `git diff --no-textconv` `[验]` | 禁用用户配置的 textconv 转换命令——否则用户的外部命令失败会让 `git diff` 直接退出 128。 |
| `git diff --no-ext-diff` `[验]` | 禁用用户配置的外部 diff 程序（`diff.external`）。 |
| `git diff --src-prefix=a/ --dst-prefix=b/` `[验]` | 把 diff 头部前缀**固定下来**，不受 `diff.mnemonicPrefix`/`diff.noprefix` 影响。 |
| `git diff -M` / `--no-renames` `[验]` | 开启/关闭重命名检测；`-M100%` 要求内容完全相同才算改名。 |
| `git diff --diff-algorithm=histogram` `[验]` | 换 diff 算法（可选 `myers`/`minimal`/`patience`/`histogram`）。 |
| `git diff --check` `[验]` | 检查"行尾空白""冲突标记残留"这类问题（比 diff 本身更适合当闸门）。 |
| `git diff --cached --name-only` `[验]` | 只列已暂存改动涉及的文件名。 |
| `git cat-file -t <rev>` `[验]` | 取对象的**类型**：`commit` / `tree` / `blob` / `tag`。 |
| `git cat-file -s <rev>` `[验]` | 取对象的**字节大小**。 |
| `git cat-file -p <rev>` `[验]` | 按类型"漂亮打印"对象内容：tree 给条目列表、commit 给字段、blob 给原始内容。 |
| `git cat-file --batch-check` `[验]` | 从 stdin 读一串 rev，逐行回"哈希 类型 大小"——批量查询对象的机器接口。 |
| `git cat-file --batch-all-objects --batch-check` `[验]` | 列出对象库里**所有**对象（含不可达的），排查"那个东西还在不在"。 |
| `git ls-tree HEAD` `[验]` | 列出某提交**顶层**的树条目（模式 类型 哈希 名字）。 |
| `git ls-tree -r HEAD` `[验]` | 递归列出所有条目——这就是"这次提交的完整快照"。 |
| `git grep -n <pattern> HEAD -- <path>` `[验]` | 在**某个版本**的文件内容里搜索（不必检出），`-n` 给行号。 |

**常见误用**：以为 `git diff` 是"看全部改动"。它默认**只看索引之外**（索引↔工作区）；刚 `add` 过的改动它一个字都不显示，必须加 `--cached`。

---

## B.3　引用与找回

| 命令 | 它到底干了什么 |
| :--- | :--- |
| `git rev-parse HEAD` `[验]` | 把任意 revspec 解析成**完整 40 位哈希**（递归解引用 HEAD → 分支 → 提交）。 |
| `git rev-parse --short HEAD` `[验]` | 解析成**无歧义的短哈希**（Git 自动决定长度，通常是 7 位）。 |
| `git rev-parse --verify <rev>^{commit}` `[验]` | 校验"这个 revspec 到底是不是一个提交"——附注标签会先被剥壳到 commit。 |
| `git rev-parse HEAD^{tree}` `[验]` | 剥到该提交的 tree 对象。 |
| `git rev-parse --show-toplevel` `[验]` | 输出仓库根的绝对路径（比自己找 `.git` 可靠）。 |
| `git rev-parse --show-prefix` `[验]` | 输出"当前目录相对仓库根"的前缀，是算子目录作用域的正确方式。 |
| `git rev-parse --abbrev-ref HEAD` `[验]` | 取当前分支名——但 detached 时会输出字面量 `HEAD`，要区分得配合 `symbolic-ref`。 |
| `git rev-parse --is-inside-work-tree` `[验]` | 判断当前目录是否在工作区里（裸仓库返回 false），脚本里做前置检查用。 |
| `git rev-parse --git-dir` `[验]` | 输出 `.git` 目录的实际位置（可能是相对路径或绝对路径）。 |
| `git symbolic-ref HEAD` `[验]` | 问"HEAD 这个名字指向哪个引用"——**detached 时会失败**，这正是它的价值。 |
| `git symbolic-ref --short HEAD` `[验]` | 同上，但只给短名（`main` 而不是 `refs/heads/main`）。 |
| `git symbolic-ref -q HEAD` `[验]` | 静默版：detached 时不给报错，只给非零退出码。 |
| `git for-each-ref --format='%(refname) → %(objectname:short)'` `[验]` | 遍历**所有**引用并格式化输出——不用自己读 `.git/refs/`（引用可能在 `packed-refs` 里）。 |
| `git for-each-ref --sort=creatordate --format='%(refname:short) %(creatordate:short) %(subject)' refs/tags` `[验]` | 按创建日期列 tag 及**标签说明**（轻量标签这里给提交标题，附注标签给 tag 对象说明）。 |
| `git show-ref --heads` `[验]` | 列出所有本地分支引用及其哈希。 |
| `git update-ref refs/heads/<name> <sha>` `[验]` | 直接创建/移动一个引用——"给任意提交起个名字"，包括你以为丢掉的提交。 |
| `git reflog -10` `[验]` | 看 HEAD 的移动史（每次 checkout/commit/reset/merge/rebase 一行）。 |
| `git reflog show <branch>` `[验]` | 看某个分支自己的移动史（对应 `.git/logs/refs/heads/<branch>`）。 |
| `git reset --hard HEAD@{1}` `[验]` | 用 reflog 回到"上一步"——`reset --hard` 之后最常见的救援动作。 |
| `git reset --hard ORIG_HEAD` `[验]` | 用"上一次 reset/merge/rebase 之前的位置"一枪救回（只有一格，会被覆盖）。 |
| `git fsck --lost-found` `[验]` | 校验对象库，并把**吊着的对象**写进 `.git/lost-found/`。 |
| `git fsck --unreachable` `[验]` | 只列出不可达对象，不写文件（想看不想动时用）。 |
| `git fsck --dangling` `[验]` | 只报告吊着的对象（dangling commit/tag/tree/blob）。 |
| `git branch rescue <sha>` `[验]` | 给一个孤立提交建分支，把它"捞回"可达世界。 |
| `git tag -a <name> -m <msg> <rev>` `[验]` | 建附注标签（独立对象，带说明、可签名）。 |
| `git tag -v <tag>` `[验]` | 校验标签的签名（未签名的标签会报 `error: no signature found`）。 |
| `git verify-commit <rev>` `[验]` | 校验提交签名；**未签名时退出码非零**，输出 `Good "git" signature for …` 才算通过。 |

**常见误用**：出事故后第一反应是 `git gc`。这**恰好会**把还能救的不可达对象清掉——正确顺序是 `ORIG_HEAD` → `reflog` → `fsck`，`gc` 越晚越好。

---

## B.4　临时保存与清理

| 命令 | 它到底干了什么 |
| :--- | :--- |
| `git stash push -u -m "<msg>"` `[验]` | 把工作区（`-u` 连未跟踪文件）暂存起来并还原到干净状态。 |
| `git stash list` `[验]` | 列出所有 stash 条目（含 `stash@{n}` 索引与说明）。 |
| `git stash show -p stash@{0}` `[验]` | 看某个 stash 的完整 diff。 |
| `git stash pop` / `apply` `[验]` | 恢复 stash；`pop` 恢复后会删掉该条目，`apply` 保留。 |
| `git stash drop stash@{0}` `[验]` | 只删掉某条 stash（内容仍在对象库里，短期还能按哈希找回）。 |
| `git clean -n` `[验]` | **演习**：列出会被删掉的未跟踪文件，但什么都不删。 |
| `git clean -ndX` `[验]` | 演习"只删**被忽略**的文件"会删掉什么（`-X` 只针对忽略项，`-d` 含目录）。 |
| `git clean -fd` `[验]` | 真正删掉未跟踪文件与目录（⚠️ 不可恢复，它们从没进过对象库）。 |
| `git rm --cached <file>` `[验]` | 从索引移除但**保留工作区文件**——让一个已跟踪文件"脱离版本控制"。 |
| `git rm -r --cached <dir>` `[验]` | 同上，递归处理目录（误提交 `node_modules` 后的补救）。 |
| `git gc` `[验]` | 常规垃圾回收：打包对象、清理不可达对象（**事故期间别跑**）。 |
| `git gc --prune=now` `[验]` | 立刻清掉不可达对象（更危险，只在确认不需要找回时用）。 |
| `git prune` `[验]` | 只删不可达对象，不打包。 |
| `git repack -adb` `[验]` | `-a` 全部重打、`-d` 删掉旧包、`-b` 同时写 bitmap。 |
| `git count-objects -v` `[验]` | 报告松散对象数、打包对象数、pack 个数与体积——判断"该不该 gc"的第一步。 |

**常见误用**：把 `git clean -fd` 当"清理构建产物"用。它删的是**所有未跟踪文件**，包括你刚写好但还没 `add` 的新代码——那些文件没进过对象库，删了就真没了。清理构建产物应该用 `git clean -fdX`（只删被忽略项）。

---

## B.5　忽略规则排查

| 命令 | 它到底干了什么 |
| :--- | :--- |
| `git check-ignore -v <path>` `[验]` | 回答"这个路径被**哪个文件的第几行、哪条模式**命中"，格式 `来源:行号:模式<TAB>路径`。 |
| `git check-ignore -q <path>` `[验]` | 静默判定：退出码 0 = 被忽略，1 = 没有被忽略（事故排查里最常用）。 |
| `git check-ignore -v --no-index <path>` `[验]` | 连**已跟踪**文件也按规则判定（平时已跟踪文件会直接返回"不忽略"）。 |
| `git check-ignore -v --stdin` `[验]` | 从 stdin 逐行读路径判定——一次查多个路径又不想被"整体退出 1 吞掉输出"时用。 |
| `git status --ignored --porcelain` `[验]` | 连被忽略的文件一起列（`!!` 前缀），看"哪些东西被挡在外面了"。 |
| `git ls-files --others --exclude-standard` `[验]` | 列出**未被跟踪且未被忽略**的文件——这正是 `git add .` 的候选集合。 |
| `git ls-files --others --ignored --exclude-standard` `[验]` | 列出被忽略的未跟踪文件（与上一条互补）。 |
| `git ls-files --error-unmatch <path>` `[验]` | 判断某文件是否**已被跟踪**（没跟踪就报错、退出码非零）——`.gitignore` 不生效时的第一诊断。 |
| `git rm --cached <path>` `[验]` | 让已跟踪文件回到"未跟踪"状态，忽略规则这才对它生效。 |
| `git config --show-origin --get core.excludesFile` `[验]` | 看全局忽略文件配置来自**哪个文件**（排查"这条规则是谁加的"）。 |
| `git config --list --show-origin` `[验]` | 列出所有生效配置**及来源文件**——`core.excludesFile`、`diff.external` 类问题都靠它定位。 |

**常见误用**：一次传多个路径给 `git check-ignore -v`。**只要其中有一个不被忽略，整条命令就退出 1 且不打印任何内容**——你会以为"全都正常"，其实输出被吞了。要一次查多个，用 `--stdin`。

---

## B.6　远端与传输

| 命令 | 它到底干了什么 |
| :--- | :--- |
| `git remote -v` `[验]` | 列出远端名及其 fetch/push URL。 |
| `git remote show origin` `[验]` | 显示某个远端的详情（分支映射、跟踪关系、是否会快进）。 |
| `git clone <url> <dir>` `[验]` | 克隆；本地路径会走硬链接优化，`file://` 才走真正的传输协议。 |
| `git init --bare <dir>` `[验]` | 建一个**没有工作区**的仓库——这就是"远端"的全部真相。 |
| `git fetch origin` `[验]` | 下载缺的对象并更新 `refs/remotes/*` 与 `FETCH_HEAD`；**不动本地分支与工作区，永远安全**。 |
| `git fetch origin main:tmp` `[验]` | 只取 main 并直接写进本地 `tmp` 分支（绕过 `refs/remotes`）。 |
| `git pull --rebase` `[验]` | fetch 后把本地提交**重放**到远端之上（历史保持线性）。 |
| `git push origin HEAD:refs/heads/<name>` `[验]` | 把当前提交推到远端指定分支——本地分支名与远端名不一致时的显式写法。 |
| `git push origin :<branch>` `[验]` | 源为空 → **删除**远端分支。 |
| `git push origin <tag>` `[验]` | 推一个标签（等价 `refs/tags/<tag>:refs/tags/<tag>`）——发布链路就是这么触发的。 |
| `git push --force-with-lease` `[验]` | 只有"远端仍是我上次看到的样子"才允许覆盖；远端变了就报 `stale info` 拒绝。 |
| `git push --force` `[验]` | 无条件覆盖远端引用（会**静默丢掉**别人的提交）。 |
| `git push --dry-run` `[验]` | 只报告"会推什么"，不真的推（没配 remote 会报 `No configured push destination`）。 |
| `git ls-remote <url>` `[验]` | 不克隆就问远端有哪些引用（附注标签会出现 `refs/tags/x` 与 `refs/tags/x^{}` 两行）。 |
| `git bundle create <file> --all` `[验]` | 把整个仓库（含全部引用）打包成**一个文件**，可在离线机器间搬运。 |
| `git bundle verify <file>` `[验]` | 校验 bundle 是否完整（输出 `The bundle records a complete history.`）。 |

**常见误用**：把 `git pull` 当成"安全的 `git fetch`"。`pull` = fetch + **merge/rebase**，它会移动你的分支、改你的工作区，冲突时还会留下半个合并状态。只想看看远端有什么，用 `fetch`。

---

## B.7　性能诊断

| 命令 | 它到底干了什么 |
| :--- | :--- |
| `GIT_TRACE_PERFORMANCE=1 git <cmd>` `[验]` | 打印 Git 内部各相位耗时，最后一行是总计——"到底慢在哪一段"。 |
| `GIT_TRACE2_PERF=1 git <cmd>` `[验]` | 更细的 trace2 事件流（含是否读了 commit-graph 这类信息）。 |
| `GIT_TRACE_PACKET=1 git fetch` `[验]` | 打印传输协议往返，能看到 `want`/`have` 协商的每一行。 |
| `GIT_TRACE=1 git <cmd>` `[验]` | 打印 Git 实际执行了哪些内部命令与环境——"它到底替我跑了什么"。 |
| `git count-objects -v` `[验]` | 松散对象数 / 打包对象数 / pack 个数 / 体积（`count` 只是**松散**对象数）。 |
| `git count-objects -vH` `[验]` | 同上，但体积用人类可读单位（KB/MB）。 |
| `git verify-pack -v .git/objects/pack/*.idx` `[验]` | 逐个对象的类型/大小/delta 链长，末尾给出 `chain length = N: M objects` 直方图与 `ok`。 |
| `git commit-graph write --reachable` `[验]` | 为所有可达提交写 commit-graph（实测让纯遍历类操作快 3.4×）。 |
| `git commit-graph verify` `[验]` | 校验现有 commit-graph 文件是否与当前对象库一致。 |
| `git multi-pack-index write` `[验]` | 为多个 packfile 建一层合并索引，避免逐个读 `.idx`。 |
| `git maintenance run --task=commit-graph` `[验]` | 按任务粒度做后台维护（`gc` 的现代化替代，可增量执行）。 |
| `git repack -adf --depth=<n>` `[验]` | 用指定 delta 深度重打全部对象——`--depth=1` 是"禁止深链"的实验开关（体积会涨 5.6×）。 |
| `git commit-graph write --reachable` 后对比 `git rev-list --count --all` `[验]` | 基准测试的"纯遍历"样本：只读提交元数据，最能体现 commit-graph 的收益。 |
| `/usr/bin/time -p git <cmd>` `[验]` | 命令级基准；**跑三次取最快**，避开首次的磁盘缓存抖动。 |
| `git fsmonitor--daemon status` `[验]` | 查看文件系统监视守护进程状态（十万级文件时 `git status` 的救命稻草）。 |

**常见误用**：在小仓库上做"性能优化"（altering、repack、调 depth）。本机 9 万对象规模下，bitmap 与 untracked cache 的收益**落在噪声里**——真正的瓶颈往往是进程启动（每次约 11 ms）与文件数，而不是这些机制。

---

## B.8　底层 plumbing

| 命令 | 它到底干了什么 |
| :--- | :--- |
| `git hash-object --stdin` `[验]` | 算 stdin 内容的 blob 哈希；**不写**对象库。 |
| `git hash-object -w --stdin` `[验]` | 同上并写入对象库。 |
| `git hash-object -t tree /dev/null` `[验]` | 复现空树常量 `4b825dc642cb6eb9a060e54bf8d69288fbee4904`。 |
| `git mktree` `[验]` | 从 stdin 的 `模式 类型 哈希<TAB>名字` 文本造一棵 tree（不碰索引和工作区）。 |
| `git commit-tree <tree> -p <parent> -m <msg>` `[验]` | 造一个 commit 对象；造出来的提交**不在 `git log` 里**，除非给它一个引用。 |
| `git write-tree` `[验]` | 把当前**索引**写成一棵 tree 对象（`git commit` 的第一步，但不创建提交）。 |
| `git read-tree HEAD` `[验]` | 把某棵树读进索引（会覆盖索引内容，慎用）。 |
| `git update-index --add --cacheinfo 100644,<sha>,<path>` `[验]` | 手工往索引里塞一个条目——这就是 `git add` 的第二步。 |
| `git update-index --refresh` `[验]` | 刷新索引里的 stat 缓存（文件时间戳变了但内容没变时消除假改动）。 |
| `git update-index --assume-unchanged <path>` `[验]` | 告诉 Git 别检查这个文件（性能手段，**不是**忽略规则）。 |
| `git update-index --skip-worktree <path>` `[验]` | 让 Git 忽略工作区版本（本地配置覆盖常用，同样不是忽略规则）。 |
| `git ls-files -s` `[验]` | 列出索引条目：模式 / blob 哈希 / **stage 号** / 路径。 |
| `git ls-files -u` `[验]` | 只列**未合并**（冲突）条目——冲突时同一路径会出现 stage 1/2/3 三条。 |
| `git ls-files --stage` `[验]` | 与 `-s` 等价，写全名更易读。 |
| `git diff-index --name-only HEAD` `[验]` | "HEAD ↔ 索引"的底层写法（不刷新 stat 缓存）。 |
| `git diff-files --name-only` `[验]` | "索引 ↔ 工作区"的底层写法。 |
| `git merge-tree <base> <b1> <b2>` `[验]` | 不碰工作区地做一次三方合并并打印结果——预演合并冲突。 |
| `git pack-refs --all` `[验]` | 把所有松散引用搬进 `.git/packed-refs`——"引用不一定在 `.git/refs/` 里"的成因。 |
| `git show-index` `[验]` | 读一个 `.idx` 文件（从 stdin）并列出其中每个对象的哈希与偏移。 |
| `git var GIT_AUTHOR_IDENT` `[验]` | 输出 Git 最终会使用的作者身份——排查"为什么提交作者不对"。 |
| `git version` `[验]` | 打印版本（脚本里据此判断某个选项是否可用）。 |
| `git help -a` `[验]` | 列出所有可用子命令——验证"某条命令是否真实存在"的最快方式。 |
| `git merge-base A B` `[验]` | 求两个提交的最近公共祖先；`--all` 会在 criss-cross 场景下返回多个。 |
| `git worktree list` `[验]` | 列出该仓库挂载的所有工作区（它们共享同一个 `.git`）。 |
| `git archive --format=tar HEAD` `[验]` | 把某棵树导出成归档（受 `.gitattributes` 的 `export-ignore` 影响）。 |
| `git format-patch -1 --stdout` `[验]` | 生成可邮寄的 patch（含作者与提交信息）——与 `git am` 配对使用。 |
| `git apply --check <patch>` `[验]` | 只检查补丁能不能干净应用，不改任何文件。 |
| `git fast-import` `[验]` | 用流式指令批量建对象（造几十万提交的测试仓库就靠它）。 |
| `git notes add -m "<note>" <rev>` `[验]` | 给提交附加一条**不进提交哈希**的注释。 |
| `git replace -l` `[验]` | 列出替换引用（`git replace` 能在不改历史的前提下"嫁接"对象）。 |
| `git sparse-checkout set --no-cone '<pattern>'` `[验]` | 只检出匹配的工作区文件（`--cone` 按目录，`--no-cone` 按模式）。 |
| `git bisect start` / `git bisect reset` `[验]` | 二分查找"哪个提交引入了问题"；`reset` 结束并回到原分支。 |
| `git bugreport -o <dir>` `[验]` | 生成缺陷报告模板（默认文件名含时间戳，**同一分钟内重复跑会因文件已存在而失败**）。 |
| `git diagnose --mode=all -o <dir>` `[验]` | 收集完整诊断包到一个 zip（含仓库结构信息，公开前先看内容）。 |
| `git filter-branch` `[验]` | 老的历史重写工具；启动时会自己打印"有大量坑，建议改用 `git filter-repo`"的警告。 |
| `git filter-repo` `[验]` | 官方推荐的历史重写工具（需单独安装；本机已可用，`--help` 可读 man 页）。 |
| `git credential` `[验]` | 凭据助手接口（自动化里更常用 `GIT_TERMINAL_PROMPT=0` 直接禁掉交互）。 |

**常见误用**：用 `git commit-tree` 造完提交就以为"提交好了"。它只写对象，**不移动任何引用**——那个提交在 `git log` 里根本不存在（第 2、3 章的实测都指向这一点）。

---

## B.9　给工具作者的固定前缀

写一个"读 Git"的工具时，下面这串参数应该**每一次调用都带上**
（第 12 章逐条解释过每个的来历）：

```bash
git --no-pager -c core.quotepath=false -C <dir> \
    diff --no-ext-diff --no-textconv --no-color \
         --src-prefix=a/ --dst-prefix=b/ \
         --name-status -z -M
```

配合的环境变量与进程约束：

```bash
GIT_PAGER=cat GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0
# 并且把子进程的 stdin 关掉（cmd.Stdin = nil）
```

**它到底干了什么**：把"用户配置"从你的输入里剔除出去——禁分页（不挂）、禁交互（不等密码）、禁抢锁（不干扰）、固定路径编码（不自作转义）、固定 diff 协议（前缀不被改写）、固定清单格式（NUL 分隔，不会被空格/换行击穿）。

**常见误用**：只加其中一部分。最容易被漏掉的是 `-c core.quotepath=false` 与 `--no-textconv`——前者让中文路径变八进制转义，后者让用户的外部命令失败时**整个 diff 挂掉**，而两者在开发者本机都"看起来正常"。

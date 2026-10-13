# 附录 E　进阶主题地图：没写进书里的那 60%

> 这本书**不追求覆盖 Git 的全部命令**（前言的"一点诚实的说明"里就交代过）。
> 但"没写进正文"不等于"你学不到"——学完前 12 章，你已经拿到了**推理未知特性的模型**。
> 这份附录把我**刻意略过**的主题列出来，每一条都告诉你两件事：
>
> 1. **它的本质是什么**（一句话讲清机制）；
> 2. **用书里哪一章的模型可以自己把它推出来**——这才是这份地图与"链接清单"的区别。
>
> 换句话说：**正文教你怎么想，这份附录教你把"怎么想"用到没学过的东西上。**

---

## E.1　先立一条规矩：所有链接都验证过

这份附录里出现的每一个 URL，都在 **2026-10-13** 用 HTTP 请求实测过（返回 200），
不是凭记忆写的。两个使用要点：

- **官方命令文档有官方中文版**：把 URL 里的命令名后面加 `/zh_HANS-CN` 即可，例如
  `https://git-scm.com/docs/git-bisect/zh_HANS-CN`。
  ⚠️ 但**概念类文档没有中文**（`merge-strategies`、`protocol-v2`、`reftable`、
  `gitformat-*` 这些加 `/zh_HANS-CN` 会 404），只有英文。
- **选项名与默认值会随 Git 版本变**。看文档时留意它的版本标注，
  并以本机 `git <cmd> --help` 为准（这也是全书第三条方法论）。

> 学会 `/zh_HANS-CN` 这个规律本身就有用：**你不用记住任何一条具体链接**，
> 只要知道命令名，就能自己拼出中文文档地址。

---

## E.2　复合仓库：`submodule` / `subtree`

**本质**：`submodule` 在**父仓库的 tree 里只是一个特殊条目**——模式为 `160000`
（叫 gitlink）的"提交指针"。父仓库**不存**子仓库的文件，只存"子仓库的某个提交哈希"。
所以 `git submodule update` 做的是"按父仓库记录的那个哈希，去子仓库检出对应提交"。

**用哪一章推理**：**第 2 章**。tree 条目本来就有"模式 + 类型 + 哈希"三要素
（你已经在 2.5 的模式表里见过 `100644`/`100755`/`40000`），`160000` 只是第四种：
"这是一个别的仓库的提交，不是我的 blob"。一旦看清这一点，`submodule` 所有的怪脾气
（父仓库不记录子仓库的未提交改动、切分支不会自动更新子模块、
`git status` 里显示的是"子模块指针变了"而不是文件变了）就都是这条定义的直接推论。

**`subtree` 相反**：它把子仓库的内容**真的并入**你的 tree——代价是你的历史里多了一份副本，
好处是使用者不需要懂 submodule（clone 下来就能用）。

| 资源 | 说明 |
| :--- | :--- |
| [git-submodule（中文）](https://git-scm.com/docs/git-submodule/zh_HANS-CN) | 官方文档，含 `.gitmodules` 格式与 `--init/--update/--foreach` |
| [git-subtree 文档源码](https://github.com/git/git/blob/master/contrib/subtree/git-subtree.adoc) | `subtree` 不属于核心命令，文档在源码 `contrib/` 下 |

---

## E.3　大文件：Git LFS 与 `clean`/`smudge` 过滤器

**本质**：LFS **不是** Git 的补丁，而是**第 6 章 `.gitattributes` 里那套 filter 机制的一个应用**。
`.gitattributes` 里为某类文件配 `filter=lfs`，于是：

- `clean`（`add` 时）：把大文件**替换成一段几十字节的指针文本**（内含真实文件的哈希与大小）；
- `smudge`（`checkout` 时）：拿到指针，去 LFS 服务器把真身取回来。

**所以对象库里从头到尾存的都是那个指针。** 这一句话解释了 LFS 的全部现象：
历史里为什么没有大文件、`clone` 为什么变小、为什么**改 filter 配置后要重新规范化历史**
（`git add --renormalize .`）才能修复曾经误提交的大文件。

**用哪一章推理**：**第 6.9 节**。你已经见过 textconv 与"外部命令参与 diff"的机制，
filter 是同一族东西——只是它作用在"进出对象库"而不是"渲染 diff"。
这也解释了第 6.9 节那个结论为什么重要：**用户配置能悄悄改变工具看到的字节**，
LFS 正是"合法地"这么干的。

| 资源 | 说明 |
| :--- | :--- |
| [gitattributes（中文）](https://git-scm.com/docs/gitattributes/zh_HANS-CN) | `filter.<driver>.clean/smudge` 的完整语义 |
| [git-lfs.com](https://git-lfs.com/) | LFS 官方站（安装、原理、迁移指南） |

---

## E.4　多工作区：`git worktree`

**本质**：一次 clone 只对应一个 `.git`，但 `worktree` 让你**同时挂多个工作目录到同一个 `.git`**。
关键在于哪些是共享的、哪些是各自的：

| 共享（一份） | 各自独立（每个工作区一份） |
| :--- | :--- |
| `.git/objects`（对象库） | 自己的 `HEAD` |
| `refs/`（分支、tag） | 自己的 `index`（暂存区） |
| 配置与钩子 | 自己的工作区文件 |

**用哪一章推理**：**第 3 章 + 第 4 章**。你已经知道 Git 内部就是"对象库（不可变）
+ 引用（可变指针）+ 索引（候选快照）"；`worktree` 做的事就是**把索引和 HEAD 复制一份、
把对象库和引用共用**。于是两个问题立刻有答案：

- 为什么不能在两个工作区同时检出**同一个分支**？——因为分支引用只有一份，
  两个 HEAD 指向它会让"当前在哪"变成有歧义的（Git 直接拒绝）。
- 为什么在一个工作区的提交，另一个工作区**马上就能看到**？——因为它们共享对象库与引用。

| 资源 | 说明 |
| :--- | :--- |
| [git-worktree（中文）](https://git-scm.com/docs/git-worktree/zh_HANS-CN) | `add`/`list`/`remove`/`prune` 与 `HEAD` 独立的语义 |

---

## E.5　冲突复用：`git rerere`

**本质**：**Re**use **Re**corded **Re**solution。它在你解决一次冲突后，把
"冲突前的三份内容 → 你解决后的结果"记下来；下次遇到**同样的冲突**，自动替你把结果写好
（仍然要你 `add` 确认）。它只改工作区与索引，**不碰对象库**。

**用哪一章推理**：**第 4.7 节**。你已经知道"冲突时索引里同一路径有 stage 1/2/3，
解决冲突就是一次 `add`"。rerere 的全部工作就是"识别出'同样的 stage 1/2/3 组合'，
然后自动替你执行那次 `add`"。它为什么对**反复 rebase 长分支**的人特别值钱，
也就能推出来了——那种场景下同样的冲突会出现很多遍。

| 资源 | 说明 |
| :--- | :--- |
| [git-rerere（中文）](https://git-scm.com/docs/git-rerere/zh_HANS-CN) | `rerere.enabled`、`rerere-train.sh`、`rerere.autoupdate` |

---

## E.6　二分定位：`git bisect`

**本质**：在提交图上做**二分查找**。你给一个"好"的提交与一个"坏"的提交，
Git 每次检出中间那个提交让你（或脚本）判定，逐步把范围缩到一个提交——即缺陷引入点。

**用哪一章推理**：**第 5 章（DAG）+ 第 9 章（detached HEAD）**。
DAG 的"祖先关系"天然可二分（沿第一父往回是一串线性历史）；
而它**必须能检出历史提交**这件事，正是第 3.7 节的 detached HEAD——
你并不在"哪个分支上"，只是在"看某个提交"，所以 HEAD 会断。
`git bisect run <脚本>` 只是把人工判定换成脚本退出码。

| 资源 | 说明 |
| :--- | :--- |
| [git-bisect（中文）](https://git-scm.com/docs/git-bisect/zh_HANS-CN) | `start/good/bad/run/reset`，`run` 的退出码约定 |

---

## E.7　邮件补丁工作流：`format-patch` / `am` / `send-email`

**本质**：**提交被序列化成文本，靠邮件（或任何文本通道）传递。**
`git format-patch` 把提交导成"第 6 章那种 unified diff + 一段带元信息的邮件头"，
`git am` 把它重新应用并**保留原作者与提交信息**。

**用哪一章推理**：**第 6 章 + 第 8 章**。补丁的正文就是 unified diff（第 6 章）；
而"为什么不直接用 `push`"的答案在第 8 章——`push` 要求你对**远端仓库有写权限**，
而邮件工作流让你**对上游没有任何写权限**也能贡献代码。
这是 Git 最初为 Linux 内核设计的场景，不是历史遗迹。

| 资源 | 说明 |
| :--- | :--- |
| [git-format-patch（中文）](https://git-scm.com/docs/git-format-patch/zh_HANS-CN) | `-N`/`--cover-letter`/`--rfc` 等 |
| [git-am（中文）](https://git-scm.com/docs/git-am/zh_HANS-CN) | 应用补丁、`--3way` 回退到三方合并 |

---

## E.8　合并策略与自定义合并驱动

**本质**：第 5 章讲了"合并提交有两条父边，`merge-base` 是共同祖先"。
**三方合并**就是在 merge-base 上拿着三方内容做 diff。所谓"策略"，
是在下面三种复杂情况下**怎么定候选**：

| 策略 | 何时用 | 它在解决什么 |
| :--- | :--- | :--- |
| `ort`（Git 2.33+ 默认） | 常规两分支合并 | `recursive` 的重写版，更快且历史更干净 |
| `recursive` | 老版本默认 | 出现**多个 merge-base** 时递归构造虚拟祖先 |
| `octopus` | 一次合多个分支 | 合并结果有多条父边；冲突时直接放弃 |
| `ours` | 只要"合并这个动作" | 保留当前分支内容，丢弃对方（**不是** `-X ours`） |

**自定义合并驱动**：`.gitattributes` 里给某类文件指定 `merge=<driver>`，
让 Git 调用**你自己的程序**去合并（例如锁文件、生成的 protobuf、Excel 表格）。

**用哪一章推理**：**第 5.7 节**。"多个 merge-base"不是玄学——它正是
"两个分支互相合并过多次"在图上的形态，而第 5.7 节算 merge-base 的方法
就是理解 `recursive` 的关键。

| 资源 | 说明 |
| :--- | :--- |
| [merge-strategies（英文）](https://git-scm.com/docs/merge-strategies) | 无中文版；策略与 `-X<option>` 的取舍 |
| [git-merge（中文）](https://git-scm.com/docs/git-merge/zh_HANS-CN) | `-s`/`-X`/`--no-ff` 的完整参数 |
| [gitattributes（中文）](https://git-scm.com/docs/gitattributes/zh_HANS-CN) | `merge=<driver>` 与 `%O %A %B` 占位符约定 |

---

## E.9　服务端与存储后端的演进

三个"看起来新、其实语义没变"的东西：

1. **协议 v2**：优化的是**一次会话内能问多少个引用**（v1 是"列出全部引用"，
   巨型仓库上开销很大）。它**没有改**第 8.7 节讲的 `want`/`have` 协商本质。
2. **引用事务**：把"多个引用一起更新"变成原子操作。用第 3 章理解——
   引用是文件，事务就是"这几个文件要么都改、要么都不改"。
3. **`reftable`**：一种新的**引用存储格式**，目标是替代 `packed-refs` 在大仓库上的性能问题。
   **它不改引用的语义**——第 3 章说的"引用就是'名字 → 哈希'的记录"依然成立，
   只是这些记录从"一个文本文件"换成了"一张紧凑的表"。

**用哪一章推理**：**第 3 章**。这一节的全部价值就是一句话：
**存储后端的更换不改变抽象**。你能区分"语义"与"实现"，就不会被新格式吓到。

| 资源 | 说明 |
| :--- | :--- |
| [protocol-v2（英文）](https://git-scm.com/docs/protocol-v2) | 无中文版 |
| [reftable（英文）](https://git-scm.com/docs/reftable) | 无中文版 |
| [gitformat-pack（英文）](https://git-scm.com/docs/gitformat-pack) | packfile 与 delta 的字节级格式（第 10.2 节的官方出处） |
| [gitformat-index（英文）](https://git-scm.com/docs/gitformat-index) | `.git/index` 的 `DIRC` 格式（第 4.2 节的官方出处） |
| [gitformat-commit-graph（英文）](https://git-scm.com/docs/gitformat-commit-graph) | 第 10.3 节的官方出处 |
| [gitformat-signature（英文）](https://git-scm.com/docs/gitformat-signature) | 第 11.3 节签名格式的官方出处 |

---

## E.10　还有这些（一句话 + 入口）

| 主题 | 一句话本质 | 入口 |
| :--- | :--- | :--- |
| `git bundle` | 把一个仓库打包成**单个文件**，可当远端用（离线传输） | [git-bundle](https://git-scm.com/docs/git-bundle/zh_HANS-CN) |
| `git replace` | 用别的对象**替身**某个对象（不改历史就"改写"看到的内容） | [git-replace](https://git-scm.com/docs/git-replace/zh_HANS-CN) |
| `git notes` | 给提交附加**不改变其哈希**的备注（引用命名空间 `refs/notes/*`） | [git-notes](https://git-scm.com/docs/git-notes/zh_HANS-CN) |
| `git sparse-checkout` | 只检出部分目录（第 10.7 节已讲原理，这里是用完整用法） | [git-sparse-checkout](https://git-scm.com/docs/git-sparse-checkout/zh_HANS-CN) |
| partial clone 的 filter 组合 | `blob:none` / `tree:0` / `blob:limit=<n>` 的取舍（第 10.6 节） | [partial-clone](https://git-scm.com/docs/partial-clone) |
| 钩子全集 | 每个钩子的触发时机（第 11 章讲了机制与边界） | [githooks](https://git-scm.com/docs/githooks/zh_HANS-CN) |
| 清理历史 | 重写历史的官方推荐工具（`filter-branch` 的替代品） | [git-filter-repo](https://github.com/newren/git-filter-repo) |
| 自定义传输助手 | 让 Git 走你自己的协议（第 8 章"远端只是另一个仓库"的延伸） | [gitremote-helpers](https://git-scm.com/docs/gitremote-helpers) |
| 术语总表 | 遇到任何不懂的 Git 词，先查它 | [gitglossary](https://git-scm.com/docs/gitglossary/zh_HANS-CN) |

---

## E.11　三份值得通读的材料

| 材料 | 为什么推荐 | 入口 |
| :--- | :--- | :--- |
| **Pro Git（中文版，免费）** | 覆盖最全的系统性教材；本书没写的主题它基本都有 | [git-scm.com/book/zh/v2](https://git-scm.com/book/zh/v2) |
| **Git 官方命令文档** | 最终真值源；`--help` 打不开时用它 | [git-scm.com/docs](https://git-scm.com/docs) |
| **Git 术语表** | 把"人话 ↔ Git 术语"对上，读文档不再卡在名词上 | [gitglossary（中文）](https://git-scm.com/docs/gitglossary/zh_HANS-CN) |

---

## E.12　最后一句：这份附录真正的用法

你大概已经注意到，上面每一节的结构都一样：**一句话本质 + 用哪一章推理**。
这不是排版偏好，而是这本书想留给你的东西。

> **没学过这本书的人**，看到 `git worktree` 只能当成一条新命令去背；
> **学过的人**会先问："它在既有模型里动的是哪一块？"——答案是"复制了索引和 HEAD、
> 共用对象库和引用"，于是它所有的限制都能自己推出来。
>
> 这也是为什么这份附录能存在：**剩下的 60% 不是另一套知识，而是同一套模型的落地。**

正文到此结束，附录到此结束。真正的工作在你自己的仓库里。

<details>
<summary>附：这份附录的链接是怎么验证的</summary>

对每个 URL 发一次 HTTP 请求（跟随重定向），检查响应码。实测方法与结果：

```bash
$ curl -s -o /dev/null -w '%{http_code}' -L https://git-scm.com/docs/git-bisect
200
$ curl -s -o /dev/null -w '%{http_code}' -L https://git-scm.com/docs/git-bisect/zh_HANS-CN
200
$ curl -s -o /dev/null -w '%{http_code}' -L https://git-scm.com/docs/merge-strategies/zh_HANS-CN
404                    # 概念类文档没有中文版，所以上面给的是英文链接
```

验证日期：2026-10-13。**链接会失效**——如果哪天你点开发现 404，
把命令名或主题名丢进搜索引擎即可，这比记住一条 URL 可靠。

</details>

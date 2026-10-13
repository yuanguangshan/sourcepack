# Git 深入课（以 sourcepack 为标本）

> 这套课程不背命令，而是**拆开 Git 看它怎么工作**：每一章都在这台机器上跑真命令、
> 留真输出，并回到 sourcepack 的源码里指出"我们为什么要这么调用 Git"。
>
> **👉 第一次来请先读：[前言：为什么写这套课](00-preface.md)**（动机、怎么读、三条方法论）。
>
> 标本仓库：**sourcepack**（150 个提交、42 个发布 tag，经历过 3 次改名，
> 有 CI 与完整的发布链路——麻雀虽小，五脏俱全）。

> 📘 **想当成一本书读？** 本目录的 12 篇已被装订成书（5 部 12 章 + 书名页/序/版本说明 +
> 5 份附录，末一份是《进阶主题地图》），产物在仓库 `book/dist/`（HTML / EPUB / Markdown 三种形态），
> 独立审查记录见 `book/REVIEW.md`。**这里的 12 篇散篇仍是唯一真值源**——
> 书由 `book/tools/build.py` 自动拼装；改了散篇只需重跑 build，不会出现两份内容漂移。

---

## 怎么用这套课

**1. 先建一个实验克隆**（重要：破坏性实验绝不在你真在用的仓库上做）

```bash
git clone /path/to/sourcepack /tmp/git-lab   # 换成你自己本机的路径
cd /tmp/git-lab
git config user.email lab@example.com
git config user.name Lab
```

为什么单独克隆：后面几章会做 `reset`、`rebase`、`filter-branch`、造"丢失的提交"
这类操作，它们会**改写历史**。克隆里随便折腾，`sourcepack` 本体毫发无损。

**2. 每章都有"动手实验"**：命令可以直接抄。实验都在 `/tmp/git-lab` 里做。

**3. 每章的"回到 sourcepack"小节**把 Git 的行为映射到我们自己的代码，
例如第 2 章会看到 `changeset.go` 里那个 `4b825dc642cb6eb9a060e54bf8d69288fbee4904`
到底从哪来的——它不是什么魔法常量，就是 Git 的空树对象。

---

## 目录

| 章 | 标题 | 核心问题 |
| :--- | :--- | :--- |
| [00](00-preface.md) | **前言：为什么写这套课** | 动机、为什么用 sourcepack 当标本、怎么读、三条方法论 |
| [01](01-history-and-philosophy.md) | 起源与设计哲学 | Git 为什么长这样？快照、内容寻址、三棵树 |
| [02](02-object-model.md) | 对象模型 | blob/tree/commit/tag 到底是什么字节？哈希怎么算的 |
| [03](03-refs-head-reflog.md) | 引用、HEAD 与 reflog | 分支只是 41 字节的文件；"提交丢了"是怎么回事 |
| [04](04-index-staging.md) | 索引（暂存区） | `add` 到底做了什么？staged/unstaged 的真实含义 |
| [05](05-commit-graph-and-merge.md) | 提交图与分支合并 | 父子链、merge 的两条父边、fast-forward、merge-base |
| [06](06-diff-and-patch.md) | diff 算法与 patch 格式 | Myers 算法、unified diff 头、`a/` `b/` 前缀与重命名检测 |
| [07](07-ignore-rules.md) | 忽略规则 | `.gitignore` 语义、嵌套、`core.excludesFile`、`check-ignore` |
| [08](08-remote-and-transfer.md) | 远端与传输 | refspec、fetch/push 语义、packfile 协商、force-with-lease |
| [09](09-rewrite-and-recovery.md) | 历史改写与恢复 | rebase/cherry-pick/revert/reset/reflog/fsck 找回丢失提交 |
| [10](10-performance-and-plumbing.md) | 性能与底层 | delta 链、bitmap、commit-graph、partial clone、sparse checkout |
| [11](11-hooks-signing-release.md) | 钩子、签名与发布 | hooks、SSH/GPG 签名、tag 发布链路（我们的 release.yml） |
| [12](12-sourcepack-git-internals.md) | sourcepack 的 Git 用法全解 | 逐条讲解我们发起的每个 git 调用与 hardening 细节 |

> 章节按"先原理、后操作、再回到自己的代码"编排。01–02 是地基，03–05 是日常
> 操作的真相，06–08 是传输与格式，09–12 是进阶与实战。

---

## 阅读约定

- 代码块里 `$` 开头的是命令，下一行是**真实输出**（不是示意）。
- 标注 `[lab]` 的命令在 `/tmp/git-lab` 里执行；标注 `[repo]` 的在
  `sourcepack` 本体执行。
- 涉及"为什么 sourcepack 这么写"的地方，会给出 `文件:行号`。

---

## 进度

| 章 | 状态 |
| :--- | :--- |
| 00 前言 | ✅ 已完成 |
| 01 起源与设计哲学 | ✅ 已完成 |
| 02 对象模型 | ✅ 已完成 |
| 03 引用、HEAD 与 reflog | ✅ 已完成 |
| 04 索引（暂存区） | ✅ 已完成 |
| 05 提交图与分支合并 | ✅ 已完成 |
| 06 diff 算法与 patch 格式 | ✅ 已完成 |
| 07 忽略规则 | ✅ 已完成 |
| 08 远端与传输 | ✅ 已完成 |
| 09 历史改写与恢复 | ✅ 已完成 |
| 10 性能与底层 | ✅ 已完成 |
| 11 钩子、签名与发布 | ✅ 已完成 |
| 12 sourcepack 的 Git 用法全解 | ✅ 已完成 |

**全 12 章已完成。** 建议按顺序读 01→12；每章末尾的"动手实验"可以直接抄命令，
实验请在隔离克隆里做（见上）。

---

## 各章要点（便于回看）

| 章 | 一句话要点 |
| :--- | :--- |
| 01 | Git 存**快照**而不是差异；键是**内容哈希**；内部只有对象库/引用/索引三块 |
| 02 | 对象哈希 = `类型 长度\0内容` 的 SHA-1（可用 Python 复现）；blob 不含文件名所以重命名免费；空树 `4b825dc...` 是普适常量 |
| 03 | 引用就是装着哈希的文件（41 字节）；HEAD 是符号引用；**reflog 能救回 `reset --hard` 丢掉的提交** |
| 04 | 索引 = 三棵树的中间那棵（`DIRC` 二进制）；staged/unstaged 是两条边；**冲突时同一路径有 stage 1/2/3** |
| 05 | 历史是 DAG；合并有两条父边；**`A..B` 会把 A 的改动反向算进来，PR 审查必须用 `A...B`/merge-base** |
| 06 | diff 是用时算的；**前缀可被用户配置改写**（sourcepack 显式固定）；`--no-textconv` 让工具不受用户外部命令影响 |
| 07 | `.gitignore` **只管未跟踪文件**；`abc/**` 不含自身；**父目录被排除后 `!` 救不回**——后者是 git 变更模式必须显式回溯祖先的原因 |
| 08 | 远端就是"没有工作区的仓库"；`fetch` 永远安全、`push` 有非快进保护；**传输是增量的**（want/have 协商，实测每次只多 3 个对象） |
| 09 | 改写 = 造新对象 + 移动引用；`reset` 三模式对应三棵树；**reword 一个提交会让下游哈希全变**；恢复三层 ORIG_HEAD → reflog → fsck |
| 10 | **delta 压缩**让快照不臃肿（禁深链体积涨 5.6×）；**commit-graph 让纯遍历快 3.4×**；partial clone 有"本地路径被忽略"与"服务端需 allowFilter"两个静默陷阱 |
| 11 | 客户端钩子可被 `--no-verify` 绕过、服务端钩子不可绕过；**SSH 也能签提交与 tag**；发布链路 = tag → gate → goreleaser → npm + 两段冒烟 |
| 12 | 所有 git 调用收在一层 `gitRunner`；**用 0x1F 而非空格做字段分隔符**；12 条"读 Git 的工具"checklist；全课知识地图 |

---

## 博客版（每章一篇）

本课程已发布到博客，每章一篇（标题形如 `Git 深入课（N/12）：…`）：

1. [Git 深入课（1/12）：起源与设计哲学](https://blog.want.biz/post/1791824832152)
2. [Git 深入课（2/12）：对象模型](https://blog.want.biz/post/1791824849922)
3. [Git 深入课（3/12）：引用、HEAD 与 reflog](https://blog.want.biz/post/1791824853662)
4. [Git 深入课（4/12）：索引（暂存区）](https://blog.want.biz/post/1791824856808)
5. [Git 深入课（5/12）：提交图与分支合并](https://blog.want.biz/post/1791824859915)
6. [Git 深入课（6/12）：diff 算法与 patch 格式](https://blog.want.biz/post/1791824865193)
7. [Git 深入课（7/12）：忽略规则](https://blog.want.biz/post/1791824876251)
8. [Git 深入课（8/12）：远端与传输](https://blog.want.biz/post/1791824881712)
9. [Git 深入课（9/12）：历史改写与恢复](https://blog.want.biz/post/1791824887196)
10. [Git 深入课（10/12）：性能与底层](https://blog.want.biz/post/1791824891617)
11. [Git 深入课（11/12）：钩子、签名与发布](https://blog.want.biz/post/1791824895739)
12. [Git 深入课（12/12）：sourcepack 的 Git 用法全解（结课）](https://blog.want.biz/post/1791824900511)

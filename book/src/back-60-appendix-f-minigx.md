# 附录 F　把 checklist 跑起来：一个 130 行的「读 Git」工具

> 第 12.9 节给了 12 条 checklist。**但"读过"和"能写出来"是两件事。**
> 这份附录是那 12 条的可执行版本：`book/tools/minigx.py`——约 130 行、零依赖、
> 只读，跑起来会打印一个仓库的分支、HEAD、三个计数和最近 5 个提交。
>
> 读它的方式：**别当示例代码看，当"checklist 的运行证据"看。**
> 代码里每个 `# (N)` 注释都对应书里的一条，而且每一条都可以被单独删掉、
> 亲眼看到后果（下面第三节就做了这件事）。

---

## F.1　怎么跑

它只用 Python 标准库——不需要装 Go、不需要 `pip install`、不需要编译：

```bash
$ python3 book/tools/minigx.py /path/to/some/repo
仓库      : /path/to/some/repo
当前分支  : main
HEAD      : 7fae53b
staged    : 0
unstaged  : 0
untracked : 1
最近 5 个提交：
  · 7fae53b  2026-10-13 yuanguangshan  book(附录E): 修正 E.11 里一处复制粘贴导致的链接错位
  · d631e4b  2026-10-13 yuanguangshan  book: 新增附录 E《进阶主题地图》—— 给未覆盖的高级主题一份带推理线索的入口
  · 66b1281  2026-10-13 yuanguangshan  book: 答案的节号精确化 + [深潜] 与自测题联动
```

**它对仓库完全只读**（从不写对象库、不动引用、不碰索引），所以你可以对任何仓库跑它，
包括你正在用的那个。这一点不是承诺，是设计——

> 顺带说：这个工具**故意只用 Python**，因为这本书承诺过"不需要会 Go"。
> 你在这里看到的加固，与 sourcepack 里 Go 版的 `gitRunner.command()` **一一对应**；
> 看完它再去读 `changeset.go`，不会有陌生感。

---

## F.2　12 条 checklist 落在哪一行

| # | checklist 条目 | 在 `minigx.py` 里的位置 |
| ---: | :--- | :--- |
| 1 | 所有 git 调用走一个封装 | `class Git` + `Git.run()`——**只有这一个地方起子进程** |
| 2 | 禁分页/交互/stdin/锁 | `run()` 里的 `--no-pager`、`GIT_PAGER=cat`、`GIT_TERMINAL_PROMPT=0`、`stdin=DEVNULL`、`GIT_OPTIONAL_LOCKS=0` |
| 3 | `-c core.quotepath=false` | `run()` 的全局参数（**注意边界**见 F.4） |
| 4 | 清单用 `-z`；字段用 0x1F | `split_z()`；`US = "\x1f"` 用于 `git log --format` |
| 5 | 固定协议、不猜配置 | `name_status()` 里固定的 `--no-ext-diff --no-textconv --src-prefix=a/ --dst-prefix=b/` |
| 6 | 路径只剥一层前缀 | `strip_one_prefix()`——只去一个 `a/`，因为仓库里可能真有 `b/` 目录 |
| 7 | 别用 `filepath.Match` 处理 `/` | 本工具不做路径匹配；书中第 7 章讲了为什么（Windows 上 `*` 会跨过 `/`） |
| 8 | `-M` 才有重命名 | `name_status()` 的 `-M`，以及 `R`/`C` 状态的三字段解析 |
| 9 | 输出设上限 + 截断后排干管道 | `MAX_OUTPUT` 与 `run()` 里的截断处理 |
| 10 | 只读命令清单化，并承诺不动仓库 | 全文只用 `rev-parse`/`symbolic-ref`/`diff`/`ls-files`/`log` 五个只读子命令 |
| 11 | 合并提交取第一父，并把口径写进产物 | `recent_commits()` 解析 `%P` 取第一个；输出里给合并提交标 `⑂` 并注明第一父 |
| 12 | 以 git 为真值做差分测试 | `book/tools/minigx_selftest.py`（下一节） |

---

## F.3　第 12 条：真的做了差分测试，不是"我想得到的用例"

第 12 条最容易被敷衍成"我手动试了试"。所以这份附录配了一个**可复跑的差分自测**：

```bash
$ python3 book/tools/minigx_selftest.py
═══ minigx 差分自测（以 git 为真值）═══
  ✅ 硬场景：中文名 + 真有 b/ 目录 + 三态 + 重命名 + 合并   git=(3, 0, 1) minigx=(3, 0, 1)
  ✅   b/x.go 未被多剥一层（保留原样）   解析到：['b/x.go', 'new_staged.txt', 'file with space.txt', 'renamed 文件.txt']
  ✅   中文路径未被八进制转义   含 CJK 的路径：['renamed 文件.txt']
  ✅   重命名被识别（状态 R）
  ✅   合并提交被标出（取第一父）
  ✅ 边界：unborn HEAD（刚 init，还没有任何提交）   git=(0, 0, 1) minigx=(0, 0, 1)
  ✅   没有因 rev-parse 失败而崩

✅ 全部通过：minigx 的计数与 `git status --porcelain` 完全一致
```

它的做法就是书里说的那件事：**不写"我想得到的用例"，而是造一个真实仓库，
然后用 `git status --porcelain` 的输出当标准答案**，逐项比对。夹具刻意踩满了坑：

- 文件名含**中文**与**空格**；
- 仓库里**真的有一个 `b/` 目录**（这样才能测出"多剥一层前缀"）；
- 库里同时存在 **staged / 重命名 / 未跟踪**；
- 历史里有一个**真正的合并提交**（多父）；
- 另有一个 **unborn HEAD** 的边界仓库。

> **一个诚实的插曲**：这个自测**第一版是错的**——它断言"输出里应该有 `b/x.go`"，
> 但 minigx 的输出只有计数、根本不打印路径；第二个版本又漏了夹具里
> `b/x.go` 已被合并提交、压根不在清单里。**三次修正都错在测试、不在被测代码。**
> 这恰好说明第 12 条为什么要强调"差分"：**如果只写"我想得到的用例"，
> 这些断言会一直绿着，而它们其实什么都没验证。**

---

## F.4　反向验证：把加固一条条拆掉

第 12 条的后半句是"**+ 反向验证**"——证明某个加固**不是装饰**。
下面每条都在本机实跑过（`git 2.39.5`）：

**quotepath（第 3 条）——注意它的边界在哪**

```bash
$ git diff | head -1
diff --git "a/\344\270\255\346\226\207\346\226\207\344\273\266.txt" "b/..."
$ git -c core.quotepath=false diff | head -1
diff --git a/中文文件.txt b/中文文件.txt
```

但**这条只对非 `-z` 的输出生效**——`-z` 本身就是"原样输出"，天然免疫：

```bash
$ git ls-files -z | tr '\0' '\n' | head -1
中文文件.txt                    # -z：原样，不需要 quotepath
$ git ls-files | head -1
"\344\270\255\346\226\207..."   # 非 -z：被转义
```

所以第 3 条真正非加不可的地方是 **patch 正文的 `diff --git` 头**，
不是"读 `-z` 清单"。**把这个边界搞错，就会以为自己在防一件不会发生的事。**

**固定前缀（第 5 条）——用户配置真的会改你的输出**

```bash
$ git -c diff.mnemonicPrefix=true diff | head -1
diff --git "i/\344\270\255..." "w/\344\270\255..."        # ← 前缀变成了 i/ 和 w/
$ git -c diff.mnemonicPrefix=true diff --src-prefix=a/ --dst-prefix=b/ | head -1
diff --git "a/\344\270\255..." "b/\344\270\255..."        # ← 显式固定压过配置
```

**只剥一层前缀（第 6 条）——仓库里真的有 `b/` 目录时**

```bash
$ git diff --name-status -M --cached -z | tr '\0' '\n' | grep x.go
b/x.go          # 正确：只剥一层
x.go            # 错误：多剥了一层——路径已经不是你仓库里的路径了
```

**只读（第 10 条）——可验证的承诺**

```bash
$ git status --porcelain > /tmp/before.txt
$ python3 book/tools/minigx.py . > /dev/null
$ git status --porcelain > /tmp/after.txt
$ cmp /tmp/before.txt /tmp/after.txt && echo "跑前跑后，仓库状态一字未变"
跑前跑后，仓库状态一字未变
```

---

## F.5　从这 130 行到 sourcepack：差的是什么

minigx 与 sourcepack 共用同一套加固，差别只在**覆盖面**：

| | minigx（130 行） | sourcepack |
| :--- | :--- | :--- |
| 读什么 | 分支 / HEAD / 三计数 / 最近 5 个提交 | 全量快照 + 变更集 + 历史 + 忽略规则 |
| 忽略规则 | 直接用 `ls-files --exclude-standard`（交给 git） | 自实现 `.gdignore` 匹配器（第 7 章），因为那是它**独有**的概念 |
| 输出 | 终端文本 | Markdown 快照 / 变更集 / 推送 |
| 加固 | **一样** | **一样** |

**这张表想说的是第 12.8 节那句话**：先问"这件事该由谁负责"。
忽略规则在 minigx 里交给 git 就够了——因为 minigx 没有 `.gdignore` 这种自有概念；
而 sourcepack 有，所以它必须自己实现那一层。

> **换句话说：这两个工具的区别不在"加固水平"，在"谁负责什么"。
> 而判断该谁负责，靠的正是前 11 章那些原理。**

---

## F.6　拿它继续做实验

这份附录最好的用法，是**把它改坏**。三个建议：

1. **删掉第 (3) 条**（`-c core.quotepath=false`），找一个文件名含中文、
   且内容被修改过的仓库跑它——看 `diff --git` 头变成什么样；
2. **删掉第 (5) 条的前缀固定**，然后 `git config diff.mnemonicPrefix true`，
   看输出是否开始出现 `i/`、`w/`；
3. **把第 (6) 条的"只剥一层"改成 `while` 循环剥到底**，在真有 `b/` 目录的仓库上跑，
   看 `b/x.go` 变成 `x.go`——这就是 sourcepack v1.8.0 那个"幽灵条目"的形状。

改坏了再跑 `minigx_selftest.py`，看它是怎么把你的改动抓出来的。
**这比读十遍 checklist 有用。**

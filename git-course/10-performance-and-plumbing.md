# 第 10 章：性能与底层

> **本章目标**：理解"Git 为什么不慢"，以及**什么时候它真的会慢**。
> 学完你能：判断某个操作是"启动开销主导"还是"数据量主导"、
> 知道 commit-graph / bitmap / delta / partial clone 各自加速哪一段，
> 并且能用 `GIT_TRACE_PERFORMANCE` 自己做诊断。
>
> 本章所有数据都来自本机实测（含一个用 `git fast-import` 造的 3 万提交仓库），
> 并且**包括我没能测出差异的部分**——那同样是有价值的信息。

---

## 10.1 先看结论：三件事决定了 Git 快不快

| 层 | 加速手段 | 加速的是什么 | 本章实测 |
| :--- | :--- | :--- | :--- |
| **对象存储** | **delta 压缩** | 磁盘体积（快照模型仍然省空间） | `--depth=1` → 体积 **5.6×** |
| **提交图遍历** | **commit-graph** | 历史 walk（`rev-list`、`log --graph`） | **3.4×** |
| **打包发送** | **bitmap** | 服务端算"要发哪些对象" | 机制 + 文件证据（本机规模下未测出差异） |
| **工作区状态** | stat 缓存 / untracked cache | `git status` | 启动开销主导，本机未测出差异 |
| **网络传输** | **partial clone** | 首次克隆体积 | 对象数 90k → **60k**（去掉全部 blob） |

**一句话**：Git 的默认实现已经把"常见操作"优化到**几十毫秒**；
优化手段真正生效的地方是**大历史**（几十万提交）与**大传输**（几 GB 仓库）。

---

## 10.2 delta 压缩：快照模型为什么仍然省空间 `[深潜]`

> **[深潜] 10.2–10.4 可以跳过。** 这三节是量化与机制（delta 链、commit-graph、bitmap），
> 结论在 10.1 已经给了。只想知道"该不该开、开了有什么用"的，**跳到 10.5**。

第 2 章说过 Git 存**快照**；第 2 章的 `verify-pack` 也露出了 `chain length = 4`。
这里把它讲透：packfile 会把**相似对象**串成 delta 链——只存"与基准对象的差异"。

**实测**（本机 3 万提交 / 9 万对象的合成仓库）：

| 打包方式 | pack 大小 | 相对默认 |
| :--- | ---: | :--- |
| 默认（`--depth` 未指定，实际 50） | **8,964,390** 字节 | 1.0× |
| `--depth=50 --window=50` | **7,400,287** 字节 | 0.83× |
| **`--depth=1`（禁止深链）** | **49,806,711** 字节 | **5.6×** ← 灾难 |

**`--depth=1` 的 5.6 倍**就是"如果没有 delta"的代价：
每次提交都完整存一份文件内容。**这就是 delta 存在的意义**，
也是"Git 存快照却不臃肿"的关键。

**delta 链长直方图**（`git verify-pack -v`，同一个仓库）：

```
chain length = 1: 402 objects
chain length = 2: 438 objects
...
chain length = 49: 3400 objects        # 长链是"改了很多次的小文件"
chain length = 50: 1186 objects        # 深度上限（默认 50）
非 delta: 60430 个 / delta: 29620 个
```

**读法**：深链省空间，但**读取时要把链上的 delta 依次应用**——
所以 Git 用 `--depth` 限制链长（空间换时间的旋钮）。
日常你不需要调它；只有"仓库特别大又很少读旧对象"（如归档镜像）才考虑加深。

---

## 10.3 commit-graph：加速历史遍历 `[深潜]`

`commit-graph` 是一个**预计算的提交元数据文件**（父提交、提交时间、根树位置），
让 Git 不必为每个提交去解压 commit 对象。

**实测**（3 万提交 / 9 万对象，三次取最快）：

| 操作 | 无 commit-graph | 有 commit-graph | 提升 |
| :--- | ---: | ---: | ---: |
| `git rev-list --count --all` | **110 ms** | **32 ms** | **3.4×** |
| `git log --graph --oneline -n 1000` | **112 ms** | **32 ms** | **3.5×** |
| `git log --oneline`（全量、要输出 3 万行文本） | 144 ms | 140 ms | ~0 |
| `git log -1 --format=%H` | 27 ms | 28 ms | ~0 |

```bash
$ git commit-graph write --reachable
$ ls -lh .git/objects/info/commit-graph
1.7M          # 3 万提交 → 1.7MB
```

**怎么解释"有的快 3 倍、有的没变"**：

- `rev-list`/`-graph` 是**纯遍历**（只读提交元数据）→ 直接受益；
- `git log --oneline` 要**逐个解压 commit 对象取出标题** → 瓶颈在对象读取；
- `git log -1` 只读一个提交 → 整个耗时几乎全是**进程启动**（约 27ms 固定开销）。

> **这条"固定启动开销"对 sourcepack 特别重要**——见 10.8。

---

## 10.4 bitmap：加速"要发哪些对象" `[深潜]`

服务端在 `fetch`/`clone` 时要回答："客户端缺哪些对象？"
朴素做法是从提交一路走到每个 tree/blob（可能几百万次），
**bitmap** 预先把"某个提交可达的对象集合"编码成位图（每个对象一位），
于是这一步变成**位运算**。

```bash
$ git repack -adb          # -a 全部重打、-d 删旧包、-b 写 bitmap
$ ls -lh .git/objects/pack/*.bitmap
399K                        # 3 万提交 / 9 万对象 → 399KB
```

**诚实的实测结果**：在这台机器、这个规模下，我没能测出 bitmap 带来的差异：

| 操作 | 无 bitmap | 有 bitmap |
| :--- | ---: | ---: |
| `git rev-list --objects --all`（本地遍历） | 282 ms | 279 ms |
| file:// 克隆 | 962 ms | 1026 ms |

**原因**：bitmap 优化的是 **`pack-objects` 在服务端的可达性计算**（大仓库上能省掉几百万次对象访问），
而本机 9 万对象、本地克隆的瓶颈在别处（进程与磁盘），差异落在噪声里。
**所以：机制确定存在，量化收益需要几十万对象以上的仓库才显出来**——
这本身是个重要认知：**别在小仓库上做"性能优化"**。

---

## 10.5 `git status` 为什么快：stat 缓存（第 4 章那条线）

第 4 章拆过索引条目里的 `ctime/mtime/dev/ino/size`。它的用途就是这里：
`git status` **先比 stat**，只有对不上的才去读内容算哈希。

**实测**（2000 文件仓库）：`git status --porcelain` 首次 31ms、二次 32ms、
开启 `core.untrackedCache` 后 30ms —— **同样落在噪声里**。
原因：2000 个文件的 stat 检查本身就只要几毫秒，其余是进程启动。

**什么时候这块会真的痛**：

| 场景 | 症状 | 手段 |
| :--- | :--- | :--- |
| 十万级文件 | `git status` 变成秒级 | `core.untrackedCache=true`、`core.fsmonitor=true`（macOS 有 fsmonitor） |
| 网络文件系统（NFS/SMB） | stat 极慢 | `core.checkStat=minimal` |
| 巨型 monorepo | 检出本身很慢 | `git sparse-checkout`（见 10.7） |

---

## 10.6 partial clone：只要历史，不要 blob

`--filter=blob:none` 让 clone **不下载文件内容**，只在需要时按需拉取（promisor 机制）。

**实测**（3 万提交 / 9 万对象；**注意服务端必须开 `uploadpack.allowFilter`**）：

```bash
# 服务端（提供克隆的那个仓库）
$ git config uploadpack.allowFilter true

# 客户端
$ git clone --bare --filter=blob:none file:///tmp/big part.git
$ du -sh part.git ; git -C part.git count-objects -v | grep in-pack
8.9M
in-pack: 60000                    # ← 完整克隆是 90000（少了 3 万个 blob）
$ git -C part.git log --oneline -1
66f65d24 c29999                   # ← 历史在本地，log 正常
$ git -C part.git config --get remote.origin.promisor
true
```

**两个我踩到的坑（都很常见）**：

1. `git clone --filter=blob:none /local/path` →
   `warning: --filter is ignored in local clones; use file:// instead.`
   本地路径克隆会走硬链接优化，**过滤被忽略**；要用 `file://` 才会走真正的传输路径；
2. 即使 `file://`，服务端没开 `uploadpack.allowFilter` 时也只是
   `warning: filtering not recognized by server, ignoring` —— **静默退化成完整克隆**。

**用途**：CI 里只需要读历史/跑 `git log`、大仓库的代码搜索
（配合 `git grep` 时按需取 blob）。

---

## 10.7 sparse checkout：只检出需要的部分

`--filter` 省的是**对象**，`sparse-checkout` 省的是**工作区文件**（大 monorepo 常配一起用）。

```bash
$ git clone -q /tmp/big sp && cd sp && ls | wc -l
200
$ git sparse-checkout set --no-cone '/f1*.txt'
$ ls | wc -l
111                                  # ← 工作区只留下匹配的文件
```

| 模式 | 说明 |
| :--- | :--- |
| `--cone`（默认） | 按**目录**（`git sparse-checkout set dirA dirB`），最常用 |
| `--no-cone` | 按 gitignore 风格模式（更灵活，也更容易写错） |

---

## 10.8 回到 sourcepack：我们的性能账本

前几章的结论在这里汇总成一张"账"：

| 事实 | 数据 | 来源 |
| :--- | :--- | :--- |
| **每次 git 调用的固定开销** | **约 11 ms**（进程启动） | 第 8 章实测 |
| `-g` 一次要发起几次 git 调用 | **约 10 次**：`rev-parse ×4`（show-toplevel / show-prefix / HEAD / verify）、`symbolic-ref`、`log -1`、`diff --name-status ×2`（暂存/未暂存）、`ls-files --others`、`diff --patch`（大块输出） | `changeset.go:282-393` |
| 于是"纯 spawn 开销" | **≈ 110 ms**（10 × 11ms） | 第 1 章 + 第 8 章 |
| `-g` 全流程（500 文件变更） | **≈ 210 ms** | 第 1 章 |
| 目录扫描耗时（2 万文件 / 10 万行） | **3.85 s** | TODO.md 实测 |
| 同一规模的 100 万行（5000 文件） | 仅 190 ms | 同上 |

**三个工程结论**：

1. **不要为小项目做 git 层优化**：`-g` 里近一半时间就是"启动 8 个 git 进程"，
   而这是"调 CLI 换取行为一致性"的既定代价（第 1 章的设计取舍）；
2. **真正的瓶颈是"文件数"**：2 万文件的仓库扫描要 3.85 秒，
   而 100 万行但只有 5000 文件的仓库只要 190ms——
   所以扫描并发的收益只在"文件数极多"时才出现（已记入 `TODO.md`，暂不做）；
3. **我们用到的 git 优化都是"免费"的**：
   - 只读命令 + 一次性拿清单（`--name-status -z` 一次拿全）→ 减少 spawn 次数；
   - `--no-ext-diff --no-textconv` 避免外部程序（第 6 章）；
   - 体积上限（`--max-diff-lines` + 字节上限）防止"单行 20MB"这类病态输入；
   - **反向的**：我们**不**动 commit-graph / bitmap / repack——
     那是仓库所有者的选择，我们只读、不改（第 9 章）。

**诊断工具**（我们自己在排查时用的）：

| 命令 | 看什么 |
| :--- | :--- |
| `GIT_TRACE_PERFORMANCE=1 git <cmd>` | Git 内部各相位耗时（最后一行是总计） |
| `GIT_TRACE2_PERF=1 git <cmd>` | 更细的 trace2 事件（含是否读 commit-graph） |
| `git count-objects -v` | 松散/打包对象数、体积 |
| `git verify-pack -v <idx>` | delta 链分布 |
| `/usr/bin/time -p` + 多次取最快 | 命令级基准（避开单次抖动） |

> **诚实提醒**：本章有**三项没能测出差异**（bitmap、stat 缓存、`log --oneline`）。
> 原因已分析（规模不够 / 启动开销主导），而不是"这些技术无效"。
> 把"没测出差异"如实写出来，比编一个好看的数字有用——

---

## 10.9 动手实验 `[lab]`

```bash
# 0) 用 fast-import 快速造大仓库（这也是一个值得知道的工具）
mkdir -p /tmp/l10 && cd /tmp/l10 && git init -q -b main .
python3 - <<'PY' > stream.fi
files=200; commits=5000
print('blob\nmark :1\ndata <<EOM\nf0\nl0\nEOM')
print('commit refs/heads/main\nmark :1000000\nauthor P <p@x.c> 1700000000 +0800\ncommitter P <p@x.c> 1700000000 +0800\ndata <<EOM\ninit\nEOM')
for i in range(files): print(f'M 100644 :1 f{i}.txt')
prev=':1000000'
for c in range(1,commits):
    idx=(c*7)%files; m=2000000+c*2
    print(f'blob\nmark :{m}\ndata <<EOM\nf{idx} rev {c}\nl0\nEOM')
    print(f'commit refs/heads/main\nmark :{m+1}\nauthor P <p@x.c> {1700000000+c} +0800\ncommitter P <p@x.c> {1700000000+c} +0800\ndata <<EOM\nc{c}\nEOM\nfrom {prev}')
    print(f'M 100644 :{m} f{idx}.txt'); prev=f':{m+1}'
print('done')
PY
git fast-import --quiet < stream.fi && git reset -q --hard main
git rev-list --count HEAD

# 1) delta 深度对体积的影响
git repack -adf -q --depth=1   ; ls -l .git/objects/pack/*.pack | awk '{print "depth=1:  "$5}'
git repack -adf -q --depth=50  ; ls -l .git/objects/pack/*.pack | awk '{print "depth=50: "$5}'

# 2) commit-graph 的效果
rm -f .git/objects/info/commit-graph
time git rev-list --count --all
git commit-graph write --reachable
time git rev-list --count --all          # 应明显更快

# 3) delta 链分布
git verify-pack -v .git/objects/pack/*.idx | grep '^chain length' | head

# 4) partial clone（服务端要开 allowFilter）
git config uploadpack.allowFilter true
git clone -q --bare --filter=blob:none file://$PWD ../l10-part.git
git -C ../l10-part.git count-objects -v | grep in-pack   # 比完整克隆少掉全部 blob

# 5) 诊断
GIT_TRACE_PERFORMANCE=1 git rev-list --count --all 2>&1 | tail -2
```

**自测题**：

1. `--depth=1` 为什么会让仓库体积暴涨 5 倍以上？
2. 为什么 commit-graph 让 `rev-list` 快 3 倍，却几乎不影响 `git log -1`？
3. bitmap 加速的是哪一侧的哪一步？为什么小仓库测不出差异？
4. `--filter=blob:none` 在本地路径克隆时为什么被忽略？怎么让它生效？
5. `sparse-checkout` 与 `partial clone` 分别省的是什么？
6. sourcepack 的 `-g` 里，最大的固定开销是什么？为什么这是"刻意接受"的代价？

<details>
<summary>点开看答案</summary>

1. **因为 `--depth=1` 禁止了 delta 深链，每次提交只能完整存一份文件内容。**
   见 10.2：packfile 本应把相似对象串成 delta 链、只存"与基准对象的差异"，
   `--depth=1` 把它关掉，"如果没有 delta"的代价就暴露出来。本章实测（3 万提交 /
   9 万对象）：默认打包 **8,964,390 字节 → `--depth=1` 49,806,711 字节，即 5.6×**。
   本机用同规模合成仓库复现：`--depth=1` 为 **49,791,699 字节**，`--depth=50` 为
   7,400,102 字节（与书中 7,400,287 几乎一致）。`git verify-pack -v` 还能看到
   chain length 直方图与 50 的深度上限，长链正是"改了很多次的小文件"。

2. **因为 commit-graph 只预计算提交元数据，受益的是"纯遍历"；而 `git log -1`
   的耗时几乎全是进程启动。** 见 10.3：commit-graph 存了父提交、提交时间、根树位置，
   `rev-list` / `log --graph` 不必逐个解压 commit 对象；`git log --oneline` 还要取出
   每条标题（瓶颈在对象读取），`git log -1` 只读一个提交，约 27ms 的固定启动开销就是
   全部。本章实测：`rev-list --count --all` **110ms → 32ms（3.4×）**、`-graph` 112ms →
   32ms，而 `log --oneline` 144→140ms、`log -1` 27→28ms（≈0）。本机同样复现方向：
   `rev-list` 91ms → 12ms，`log -1` 始终 8ms；commit-graph 文件同为 1.7MB。

3. **加速的是服务端 `pack-objects` 计算"客户端缺哪些对象"（可达性）这一步，
   把逐对象遍历变成位运算。** 见 10.4：bitmap 预先把"某个提交可达的对象集合"编码成
   位图（3 万提交 / 9 万对象 → 399KB），服务端在 clone/fetch 时就不必从提交一路走到
   每个 tree/blob。小仓库测不出差异，是因为**这 9 万对象、本地克隆的瓶颈在进程与磁盘，
   差异落在噪声里**：本章诚实记录的表格是 `rev-list --objects --all` 282ms vs 279ms、
   file:// 克隆 962ms vs 1026ms。所以"机制存在"不等于"现在就该开"——它的收益要几十万
   对象以上的仓库才显出来，**别在小仓库上做性能优化**。

4. **因为本地路径克隆走的是硬链接优化、根本不经过传输协议，过滤器无从施加。**
   见 10.6 的坑 1：必须把 URL 写成 `file://` 才会走真正的传输路径。实测
   `git clone --bare --filter=blob:none /local/path` 给出
   `warning: --filter is ignored in local clones; use file:// instead.`，对象数仍是
   90000；换成 `file://` 但服务端没开 `uploadpack.allowFilter` 时，得到
   `warning: filtering not recognized by server, ignoring`，**静默退化成完整克隆**
   （实测同样是 90000）。两个条件都满足后，实测 in-pack **90000 → 60000**（去掉全部
   blob）、体积 8.9M、`remote.origin.promisor=true`，与本章数字完全一致。

5. **`--filter`/partial clone 省的是对象库（不下载 blob），`sparse-checkout` 省的是
   工作区文件（只检出匹配的路径）。** 见 10.6 与 10.7：前者把首次克隆的对象数从 9 万
   降到 6 万，但工作区仍是全量，blob 靠 promisor 机制按需拉取；后者不动对象库，只把
   工作区限制到需要的目录或模式（`--cone` 按目录、`--no-cone` 按 gitignore 风格模式，
   本章实测 200 个文件 → 111 个）。两者常配合用于大 monorepo：一个治"下载多大"，
   一个治"磁盘上摊开多少"。

6. **最大的固定开销是 git 进程启动本身：单次约 11ms，`-g` 大约要发起 10 次 git 调用，
   于是约 110ms 花在 spawn 上。** 见 10.8 的账本：`rev-parse ×4`、`symbolic-ref`、
   `log -1`、`diff --name-status ×2`、`ls-files --others`、`diff --patch`，而 `-g`
   全流程（500 文件变更）约 210ms——**近一半时间不是扫描文件，而是启动进程**。
   这是第 1 章"调 git CLI 换取行为一致性与可解释报错"的既定取舍：换成库内实现能省掉
   这部分，但要承担语义偏差与行为不一致的风险。真正的瓶颈其实是**文件数**（2 万文件
   3.85s vs 100 万行 5000 文件 190ms），而 commit-graph/bitmap/repack 这类仓库级优化
   我们**不碰**——只读、不改（10.8 结论 3）。

</details>

---

## 10.10 小结

- **delta 压缩**是"快照模型不臃肿"的原因：禁止深链（`--depth=1`）会让体积涨 **5.6×**；
  链长直方图可用 `verify-pack -v` 观察（本机 3 万提交仓库链长 1–50）；
- **commit-graph** 预计算提交元数据：纯遍历类操作（`rev-list`、`log --graph`）
  实测 **3.4×** 提升，而"读对象取内容"和"单次短命令"不受益（后者被进程启动主导）；
- **bitmap** 让服务端算可达集合变成位运算；**本机规模下未测出差异**——
  它是为大仓库/大传输准备的；
- **stat 缓存 / untracked cache** 是 `git status` 快的原因（第 4 章那条线），
  本机 2000 文件规模同样测不出差异，十万级文件才显出来；
- **partial clone**（`--filter=blob:none`）能把对象数从 90k 降到 60k（去掉全部 blob），
  但有**两个静默陷阱**：本地路径克隆会忽略过滤（要用 `file://`）、
  服务端要开 `uploadpack.allowFilter`；**sparse-checkout** 省的是工作区文件；
- 诊断三件套：`GIT_TRACE_PERFORMANCE` / `git count-objects -v` / 多次取最快的计时；
- **sourcepack 的性能账本**：`-g` 的约 110ms 花在约 10 次 git 进程启动上（10 × 11ms，
  逐条清单见 10.8 的账本；这是刻意的设计取舍），
  真正瓶颈是**文件数**（2 万文件 3.85s vs 100 万行 5000 文件 190ms），
  而我们**不碰** repo 级的优化（commit-graph/bitmap/repack）——只读、不改。

**下一章**：钩子、签名与发布——`pre-commit`/`pre-push` 能做什么与不能做什么、
为什么钩子不能替代 CI、GPG/SSH 签名的意义，以及 **sourcepack 自己的发布链路**
（`release.yml`：tag → gate → goreleaser → npm publish → 冒烟）。

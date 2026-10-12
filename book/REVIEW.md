# 《Git 深入课》独立审查报告

- 审查人：book-reviewer2（未参与写作，纯外部审查）
- 审查对象：`book/dist/BOOK.md`（审查期间从 6352 行增至 **6404 行**，+52 行全在**附录 D**）、`book/dist/book.html`、`book/dist/book.epub`；源：`book/src/*.md`、`git-course/01..12`
- 工作目录：`/Users/ygs/ygs/sourcepack`
- 审查时间：2026-10-13（仓库 HEAD = `260c431`，157 提交 / 42 tag / 最新 tag v2.0.1）
- 命令环境：`export PATH="/opt/homebrew/bin:$PATH"`，git 2.39.5 (Apple Git-154)，Python 3，node v26.8.1
- 约束遵守：只写本文件；未运行 `book/tools/build.py`；未改动 `book/src`、`book/dist`、`book/tools`、`git-course` 及仓库其他任何文件（全程只读检查，脚本写在 `/tmp`）。

### ⚠️ 审查期间的并发变更（重要，影响取证口径）

审查过程中（约 02:10→02:26）`book/dist/*`、`book/tools/build.py`、`mdmini.py` 被**并发重建/修改**（mtime 变为 02:25–02:26，非本人操作，本人只写了本 REVIEW.md 与 `/tmp` 脚本）。表现为 `BOOK.md` 增长 +52 行、`.epub/.html` 重建。经比对：

- **源内容未变**：`book/src/*`、`git-course/*`、`*.go` 的 mtime 均停留在 01:xx（附录 B 源 back-20 仍为 01:54）。
- **+52 行全部追加在附录 D**（延伸阅读与工具）：全书所有章/附录 A/B/C 的起始行与审查开始时**逐行一致**（ch1@408 … 附录A@5332、B@5658、C@5936），故本报告引用的所有行号（950/1258/1919/2625/2739/4978/5145/5152/5214/5298/5313/5660/6183/6203）**依旧有效**。
- **所有结论已在"并发重建后的当前产物（02:26）"上重新复核**：附录 B 仍 167/"92 条"仍在、merge-base 仍在 changeset.go:328、Test 仍 79、summarizeChangeSet@583/splitPatch@720 未变；EPUB 重验仍全部通过；BOOK.md 结构（12 章/4 附录/围栏奇数→偶数/0 占位）仍健全。
- **附录 D 是全书最未充分审查的部分**（审查期间才被扩写 +52 行）：本次仅做了结构与占位符层面的核对 + 内容预览（其"不编造 URL、给搜索关键词、版本以 `git <cmd> --help` 为准"的写法可取），**未逐条核对延伸资源**。若附录 D 的准确性重要，建议单独再审一遍。

---

## 1. 结论

**需修**——但**无阻断（blocking）项、无高严重度项**。三大构建产物（BOOK.md / book.html / book.epub）在**结构、链接、Markdown 健康、EPUB 有效性四类机械检查上全部通过**，可直接使用；Git 语义层面的核心断言经 `git` 实证**逐字正确**。

需要"需修"的原因是若干**内容准确性层面的数字口径与源码行号锚点漂移**（中/低级）：最突出的是**附录 B 自报"92 条命令"、实际 167 条**（6 处），以及第 12 章"五种子命令"漏了 `merge-base`。这些都**不破坏成书**，但属于"读者一核就发现对不上"的事实性偏差，建议发版前修正。

一句话：**格式与构建健康，机理准确；输在几处会漂移的硬编码数字上。**

---

## 2. 缺陷清单

> 严重度定义：阻断＝使书不可用/产物损坏；高＝实质性错误、明显误导读者；中＝事实性偏差或可核对性受损，但不破坏可用性；低＝措辞/陈旧小瑕疵。

### D1 ［中］附录 B 命令数"92 条"与实际 167 条不符（6 处）

- **位置**：
  - `book/src/back-20-appendix-b-commands.md:3`
  - `book/src/back-30-appendix-c-map.md:248`、`:268`
  - `book/dist/BOOK.md:5660`、`:6183`、`:6203`
- **问题**：正文自报"共 **92 条**真实可用的 Git 命令"，但附录 B 实际有 **167 个命令条目行**（165 个不同命令串 / 80 个不同命令名），无论哪种口径都不是 92。"92 条"是表格扩张到 167 条后遗留的陈旧数字。
- **命令与真实输出**：
  ```
  $ awk '/^# 附录 B/{f=1;next} /^# 附录 C/{f=0} f&&/^\| `/ {c++}END{print c+0}' book/src/back-20-appendix-b-commands.md
  167
  # 去重命令名(归一) = 80；去重命令串 = 165；均非 92
  ```
- **建议修法**：把 6 处"92 条"改为"167 条（按场景分组）"；或改用一句不写死数量的表述（"覆盖书中全部命令，按场景分组"），彻底免漂移。此项可在 `book/src` 改后随 build 同步（注意我没跑 build）。

### D2 ［中］第 12 章"五种子命令，全部只读"漏了第 6 个 `merge-base`

- **位置**：`book/dist/BOOK.md:5145-5152`（12.6 只读保证，`grep` 论证在 `:5148-5152`）、`book/dist/BOOK.md:5056-5073`（12.3 命令清单，均未列 `merge-base`）
- **问题**：12.6 用 `grep '"(diff|log|ls-files|rev-parse|symbolic-ref)"'` 得出"**五种子命令，全部只读**"作为"我们到底调了哪些 git"的审计结论。但生产代码在 `--base` 路径下还会调用第 6 个命令 `merge-base`（该正则把它排除在外）。因此"五种子命令"的**计数不完整**（应为：默认路径 5 个；启用 `--base` 时 6 个）。
- **命令与真实输出**：
  ```
  $ grep -n "merge-base" changeset.go | grep -v _test
  changeset.go:328: mb, mbErr := g.run("merge-base", config.GitBase, "HEAD")
  # 复现书中 grep（确实只出 5 个，因为正则不含 merge-base）：
  $ grep -oE '"(diff|log|ls-files|rev-parse|symbolic-ref)"' changeset.go | sort -u | tr '\n' ' '
  "diff" "log" "ls-files" "rev-parse" "symbolic-ref"
  ```
- **说明/定性**：`merge-base` **是只读命令**，故"全部只读"的定性结论仍然成立；受影响的只是"计数完整性/审计完备性"。`--base/--first-parent` 特性来自提交 `81647fe`，第 12 章写于此特性之前，未回填。（另：`changeoutput.go:412` 的 `"diff"` 是 `writeFenced(bw,"diff",...)` 输出代码围栏的**标签**，非 git 调用，不构成遗漏。）
- **建议修法**：12.3 命令清单补一行 `merge-base <base> HEAD`（对应 `--base`），12.6 把"五种子命令"改为"五/六个子命令（`--base` 时多一个 `merge-base`），全部只读"。

### D3 ［中］源码行号锚点漂移（多处引用的行号已对不上源码）

- **问题**：书的核心卖点之一是"结论可指回 `changeset.go`/`ignore.go` 逐行核对"，但源码在成书后被继续修改，导致若干被引用的具体行号失效。下列 4 处经核对确认**代码本身真实存在、只是行号已偏移**：
  | 书中引用 | 位置 | 实际所在 |
  |---|---|---|
  | `changeset.go:561-577` 指 `summarizeChangeSet` | `book/dist/BOOK.md:1919` | `summarizeChangeSet` 实际在 **583**；561-577 是 `shortSHA`/`countUniquePaths` |
  | `changeset.go:701-737` 指 `inHunk` 状态机 | `book/dist/BOOK.md:2625` | `splitPatch`@**720**、`inHunk`@**723-757** |
  | `changeset.go:321` 指 `^{commit}` 校验 | `book/dist/BOOK.md:1258` | `rev-parse --verify <base>^{commit}` 实际在 **334** |
  | `changeset.go:337` 指 `-M` | `book/dist/BOOK.md:950` | `-M` 调用在 **353 / 361**（337 为附近空行） |
- **命令与真实输出**（示例）：
  ```
  $ grep -n "func summarizeChangeSet\|func shortSHA\|func splitPatch\|inHunk :=" changeset.go
  564:func shortSHA(s string) string {
  583:func summarizeChangeSet(cs *ChangeSet) {
  720:func splitPatch(patch string) []patchChunk {
  723:	inHunk := false
  $ sed -n '561,565p' changeset.go      # 书中称此处是 summarizeChangeSet，实为 shortSHA
  func shortSHA(s string) string {
  ```
- **影响**：多数锚点仍落在**正确函数的邻域（±约 20 行内）**，读者能定位到相关代码；但"逐行核对"的精确性受损。`book/tools/build.py` 的 `find_dangling_section_refs()` 只校验**正文小节交叉引用**（如"见 3.8"），**不校验源码行号锚点**，故这类漂移无人把守。
- **建议修法**：二选一——(a) 在 build 加一步校验，把书中 `*.go:行号` 引用解析出来核对目标函数名/特征是否仍在（源码变则报警）；(b) 成本更低：把表述从精确行号改为"约/附近"（如"`changeset.go` 的 `summarizeChangeSet()`"），并与源码版本号绑定说明。

### D4 ［中］"Go 76 个用例"与实际的 79 不符

- **位置**：`book/dist/BOOK.md:5214`（12.7 测试策略）
- **问题**：正文写"Go **76 个用例 / 5 个文件**"。实际 79 个 `func Test*`（无 TestMain）。文件数 5 正确。多出的 3 个与 `--base/--first-parent`（`TestGitBaseUsesMergeBase`）及剪贴板/iCloud 用例改动同期引入（提交 `81647fe`、`5b35376`）。
- **命令与真实输出**：
  ```
  $ grep -rn "^func Test" *_test.go | grep -iv "TestMain" | wc -l
  79
  $ grep -rc "^func Test" *_test.go      # changeset 46 + ignore 5 + main 22 + nosubdirs 4 + streaming 2
  ```
- **建议修法**：改为"Go 79 个用例 / 5 个文件"；或同样改"约 80 个用例"免漂移。Node 6 个用例未逐一核对（见"未能验证"）。

### D5 ［低］`binaryProbeSize = 8000` 常量已不存在

- **位置**：`book/dist/BOOK.md:2739-2742`（6.9）
- **问题**：正文以现在时写"我们的 `binaryProbeSize = 8000` 就是照这个来的……后来改成整文件校验"。当前源码中已无 `binaryProbeSize` 符号；现实现为 `main.go` 的 `isBinaryBuffer()`（NUL 判定 + UTF-8 有效性，整文件）。源码注释（`changeset.go:908-909`、`main.go:755-765`）仍承载该概念，故**论述语义仍成立**，只是所引符号名过时。
- **命令与真实输出**：
  ```
  $ grep -rin "binaryProbeSize" *.go      # 无输出（符号已移除）
  $ sed -n '766,774p' main.go
  func isBinaryBuffer(buf []byte) bool {
      if len(buf) == 0 { return false }
      if bytes.IndexByte(buf, 0) >= 0 { return true }
      return !utf8.Valid(buf)
  }
  ```
- **建议修法**：把"`binaryProbeSize = 8000`"改为历史叙述（"早期用 8000 字节探针，与 git `buffer_is_binary` 对齐；后改为整文件 `isBinaryBuffer()`"）。属轻微过时，非硬伤。

### D6 ［低］gitRunner "40 行的类型"措辞不精确

- **位置**：`book/dist/BOOK.md:4978`
- **问题**："整个项目对 Git 的依赖被收在一个 **40 行**的类型里（`changeset.go:160-215` 附近）"。`gitRunner` **结构体本身仅 157-160 共 4 个字段**；40 行指的是"结构体 + 其方法簇（command/run/runLimited/scoped）"的整体。实质结论（所有 git 调用必经 `command()`）经核对成立、`command()` 的 hardening 逐字无误，仅"40 行的类型"对"类型(结构体)"而言夸大。
- **命令与真实输出**：`grep -n "type gitRunner struct" changeset.go` → `157`（结构体 157-160）。
- **建议修法**：改"收在一层薄封装 `gitRunner` 里（结构体 4 字段，方法集中在 `command()`）"。

### D7 ［低］冻结的提交计数会漂移；`0 合并` 定性仍成立；课程源与书本口径分叉

- **位置**：`book/dist/BOOK.md` 含 `135 个提交` ×3（`37/*、390/*、5184`）、`143 个提交` ×1（`3611`）、`153 个提交` ×3（扉页类）；课程源 `git-course/00-preface.md`、`git-course/README.md` 写 `150 个提交`。
- **核验**：仓库 HEAD 现为 **157 提交、42 tag、0 合并提交、最新 tag v2.0.1**。
  - "写作时有 153 个提交"**这个限定站得住**：153 正是当前倒数第 5 条提交（`5b35376`），其后 4 条（`880b56d` 前言改进、`ea7227c` 撤回 book 跟踪、`b9d57f8`+`260c431` gitignore）全是成书期的 `chore/docs`——与任务描述一致。
  - `0 个合并提交`/线性历史：`git rev-list --count --merges HEAD` = **0**，现仍成立，故"135/143 个提交里一个合并都没有"的定性判断不受数字冻结影响。
  - `135`/`143` 均以历史实验快照口径出现（如"用 sourcepack 的真实历史，143 个提交"），作为定性观察可接受，但硬编码具体数字会继续漂移。
  - **跨格式分叉**：书（front matter）用 153，课程源 `00-preface.md`/`README.md` 用 150。二者都**不在同一产物内**（书不含 00-preface/README），故不影响成书一致性；但已发布的博客版与书数字口径不一致，值得留意。
- **建议修法**：正文实验快照数字改用"约"或随文标注"写作时"；课程源 `00-preface.md`/`README.md` 的 150 建议顺手对齐（超出本次成书范围，仅提示）。

### D8 ［低］第 12 章收尾块命名与全书不一致（关键词扫描会漏报）

- **位置**：`book/dist/BOOK.md:5298`（`### 12.11 结课自测（综合题）`）、`:5313`（`### 12.12 结课`）
- **问题**：第 1–11 章收尾块统一为"**动手实验 / 自测题 / 小结**"；第 12 章改为"结课自测 / 结课"。**三类收尾块内容其实齐全**（12.6 有动手实验项、12.11 综合自测题、12.12 结课总结），纯命名差异。但任何按"自测题/小结"关键词做的自动校验都会把第 12 章**误判为缺失**（本人初版脚本即如此，人工复核才发现是别名）。
- **建议修法**：若要保持命名统一便于机器校验，可把 12.11/12.12 改为"12.11 自测题（综合）""12.12 小结（结课）"；或维持现状但在 build/校验里登记别名。

### D9 ［工具瑕疵·非书稿缺陷］build.py `verify()` 有重复告警块

- **位置**：`book/tools/build.py:565-570`。
- **问题**：`if leaked:` 判断连续写了两遍，一旦触发会把同一条"shell 注释被当标题"的问题**追加两次**。不影响本次产物（当前 `leaked` 为空），但会重复告警、略掩盖真实问题数。属构建脚本小瑕疵，**不属于书稿缺陷**，按要求仅记录、未改动。

---

## 3. 已核实无误的部分（含覆盖面）

**A. 结构与完整（通过）**
- `BOOK.md` 恰有 **12 个 `## 第 N 章`，N=1..12 连续**；**5 个部**（一~五）；**附录 A–D 四个齐全**。
- 每章「动手实验 / 自测题 / 小结」三类收尾块**全部存在**（第 12 章用别名，见 D8；"12 组动手实验"与 12 章一一对应成立）。
- 占位符扫描：**无 STUB / 占位内容 / 待补 / FIXME / xxx/XXX**。4 处 `TODO` 经逐条读上下文**均为对项目 `TODO.md` 的正常引用**（数据来源、已登记改进），**非 stub**。

**B. 链接与锚点（通过）**
- `book.html`：**258 个 `id`，303 个 `href="#..."`，失效 #锚点 = 0**。
- **无任何指向书外 `.md` 或 `git-course/` 的相对链接**（BOOK.md 与 HTML 双向确认，均为 0）；HTML 除 Mermaid CDN 外**无外部资源**，离线可读。

**C. Markdown 健康（通过）**
- **23 个文件围栏全部成对**（BOOK.md 392 个围栏标记=196 块；各源文件 0–44 个标记均偶数）。
- **表格分隔行列数与表头全部一致，0 处坏表**（跨 BOOK.md + 全部源文件）。
- 逻辑段落级未闭合括号/引号 **0 处**（逐物理行看似 46 处未闭合 `（`，均为 Markdown 软换行、下一行即闭合，**非截断**）；各源文件结尾均正常收尾。

**D. EPUB 有效性（通过）**
- `testzip()` = **None**；**mimetype 为首个条目且 `ZIP_STORED`(compress_type=0)**，内容 `application/epub+zip`。
- **27 个 `.xhtml/.opf/.xml` 全部被 `xml.etree.ElementTree.fromstring` 解析通过**。
- `content.opf`：manifest 26 项 / **spine 24 项**；**spine↔manifest 双向无缺**（spine 每个 idref 都在 manifest，manifest 每个非 nav/xhtml 都在 spine）。已修复的"spine 只有 7 项/缺附录"**确已修复**——**附录 A–D 均在 spine**（ch020–ch023）。
- `nav.xhtml` **24 条目 == spine 24**，且 nav 每个 href 都落在 spine 内。

**E. 内容准确性（机理层面高度准确；问题集中在数字/锚点，见上）**
- **Git 哈希断言逐字对上**（用真 git/python 实证）：`hello\n` blob = `ce013625030ba8dba906f756967f9e9ca394464a`；`hello`(无换行)=`b6fc4c62...`；`hashlib.sha1(b'blob 6\x00hello\n')` 与 git **完全一致**；空树 = `4b825dc642cb6eb9a060e54bf8d69288fbee4904`；本机 git 2.39.5 确实支持 `--object-format=sha256`。
- **附录 B 抽样 44 条（超出要求的 20 条）全部为真实 git 命令**（用 `git help -a` 比对，0 未命中）；`filter-repo` 被书正确标注为"需单独安装"的外部工具，且本机确可 `git filter-repo --help` 读 man 页。
- **附录 A 术语数精确**：自报 145，实测 145（146 个含"—"定义行 − 1 行文件头格式说明）。抽查 6 条术语（对象/空树/松散对象/packfile/delta/链长）定义准确。
- **附录 C C.2 困惑表精确**：自报 47 行，实测 47（156–202）。C.1 Mermaid 静态校验：`graph TD`、26 节点/37 边、**双引号全配对、无 subgraph/end 失衡、虚线边 `|标签|` pipe 均合法**；HTML 已正确渲染为 `<div class="mermaid">` 且有 CDN 失败兜底。
- **14×40 对照测试精确**：`oraclePatterns` 实为 14、`oraclePaths` 实为 40。
- **源码引用（正面）逐条坐实**：`changeset.go:44`=`emptyTreeSHA` 空树常量；`:282-297` HEAD/symbolic-ref/detached/unborn 检测；`gitRunner.command()`（188-198）的 hardening（`--no-pager`/`-c core.quotepath=false`/`GIT_PAGER`/`GIT_TERMINAL_PROMPT`/`Stdin=nil`/`GIT_OPTIONAL_LOCKS`）**逐字无误**；`capDiff/capDiffBytes/truncateUTF8/diffBodyLooksBinary/splitPatch/applyChunk/appendUnmatchedChunks/parseDiffGitHeader` **符号均存在**；`ignore.go:101-111/275-279/357`（祖先目录排除 / `**` 语义 / `path.Match`）准确；`parseDiffPath` 确为"只剥一层前缀"；`--max-diff-lines` 默认 **2000** 准确。
- **硬事实核对**：157 提交 / 42 tag / 最新 v2.0.1 / **0 合并提交**，均与仓库一致。
- **Go 规模口径**：全书含测试 6376 行、非测试 3860 行，对应书中"约 6400 / 约 3900"，吻合。
- **第 6 章机理**：`diff.mnemonicPrefix→i//w/`、`diff.noprefix`、`--src-prefix/--dst-prefix` 固定前缀、`-M` 默认 50% 阈值、二进制"前 8000 字节含 NUL"、textconv 失败使 git diff 退出码 128——均为正确 Git 行为。
- **命令名 `gx` 属实**：`package.json` bin 含 `gx`、`install.sh` 建立 `gx` 快捷方式、README 首推 `gx`。

---

## 4. 未能验证的部分（如实说明）

- **附录 D（延伸阅读与工具）逐条资源未核对**：该附录在审查期间被并发扩写 +52 行，是全书最未充分审查的部分。仅做了结构/占位符核对与内容预览（写法严谨：不编 URL、给搜索关键词、提示版本以 `git <cmd> --help` 为准），未逐条验证其中资源/命令的准确性。建议单独再审。
- **第 6 章若干"实测"精确数值**（6.5 的 `U0:9/U1:11/U3:11`、`-w` 后 0 行；6.10 的 `patience 16 行 vs 其余 6 行`）：需要书中那套夹具文件（`f.txt`/`g.txt`/`r.py` 等）才能复现，本次未重建夹具，故作为"书中自报实测"记录、未独立复算。机理判断无误。
- **第 8 章 `143 个提交 / 697 个对象 / 700(+3)`**：属历史快照，现仓库对象数已变，无法按当前 HEAD 复核；仅确认其为历史口径表述。
- **Mermaid 渲染**：本机无 mermaid CLI（`mmdc` 未装，其 puppeteer/Chromium 依赖较重，未安装）。故仅做**人工静态语法核对**（见 E 节），**未真正渲染出图**，不构成"能渲染"的保证。
- **Node 6 个用例**：书中 12.7 提到 Node 6 个用例，未逐一核对 `test/` 与 `index.js`。
- **"经历 3 次改名"**：指仓库/目录改名历史，未逐条考证 git 历史中的 rename 记录。
- **附录 D（延伸阅读）链接可达性**：未联网逐一访问外链（属引用清单，非书内逻辑）。
- **`git-course/00-preface.md`、`README.md` 的"150 个提交"**：这两份**不在成书范围内**，仅作跨格式漂移提示，不列入书稿缺陷定级。

---

## 5. 给 lead 的一句话处置建议

书可以发，但**发版前请至少修 D1（附录 B "92 条"→167，6 处）与 D2（12 章补 `merge-base`）**；D3/D4 建议一并改（源码行号锚点改用"函数名/约"表述，"76"→"79"）。D5–D8 可择要处理。D 项/机械类质量很高，无需返工。

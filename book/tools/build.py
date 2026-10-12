#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
build.py —— 把 git-course/ 的散篇 + book/src/ 的元内容与附录，整合成"一本书"。

产出（都在 book/dist/）：
  BOOK.md     单一 Markdown 书稿（可直接投喂给 LLM / 阅读器）
  book.html   单文件 HTML 书（带侧栏目录、深色模式、离线可读）
  book.epub   EPUB 3 电子书

设计要点：
  1. **单一真值源**：12 章正文来自 git-course/*.md，不复制、不改写。
     构建时只做"排版级"处理（层级下沉、加部标题、修正章内相对链接）。
     → 课程继续更新时，重跑 build.py 就能出新版书，不会出现两份内容漂移。
  2. **部（Part）结构**：把 12 章编入 5 个部分 + 附录，见 PARTS。
  3. **交叉引用修正**：正文里形如 `第 3 章` 的文字保持不变（书与课一致），
     形如 `](03-refs-head-reflog.md)` 的相对链接改成书内锚点。
"""

import html
import os
import re
import shutil
import subprocess
import sys
import unicodedata
import zipfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from mdmini import Renderer, inline, slugify, esc  # noqa: E402

ROOT = Path(__file__).resolve().parents[2]      # sourcepack 仓库根
COURSE = ROOT / "git-course"
SRC = ROOT / "book" / "src"
DIST = ROOT / "book" / "dist"

BOOK_TITLE = "Git 深入课"
BOOK_SUBTITLE = "以 sourcepack 为标本"
BOOK_VERSION = "1.0"
BOOK_AUTHOR = "yuanguangshan"

# ---------------------------------------------------------------- 书的结构
# (部标题, 部导语, [(章文件名, 章标题)])
PARTS = [
    ("第一部　地基", "先把 Git 的世界观看清楚：它存什么、怎么定位内容、为什么不可变。", [
        ("01-history-and-philosophy.md", "起源与设计哲学"),
        ("02-object-model.md", "对象模型"),
    ]),
    ("第二部　日常操作的真相", "你每天都在敲的 add / commit / diff / branch，底下到底发生了什么。", [
        ("03-refs-head-reflog.md", "引用、HEAD 与 reflog"),
        ("04-index-staging.md", "索引（暂存区）"),
        ("05-commit-graph-and-merge.md", "提交图与分支合并"),
    ]),
    ("第三部　格式与传输", "差异是怎么算出来的、忽略规则的真实语义、以及数据怎么在仓库间流动。", [
        ("06-diff-and-patch.md", "diff 算法与 patch 格式"),
        ("07-ignore-rules.md", "忽略规则"),
        ("08-remote-and-transfer.md", "远端与传输"),
    ]),
    ("第四部　改写、性能与工程", "会改写历史才敢用 Git；知道它为什么快，才敢在大仓库里用它。", [
        ("09-rewrite-and-recovery.md", "历史改写与恢复"),
        ("10-performance-and-plumbing.md", "性能与底层"),
    ]),
    ("第五部　实战", "把前四部收束到一个真实项目上：发布链路，以及一个「读 Git 的工具」该怎么写。", [
        ("11-hooks-signing-release.md", "钩子、签名与发布"),
        ("12-sourcepack-git-internals.md", "sourcepack 的 Git 用法全解"),
    ]),
]

FRONT = ["front-00-title.md", "front-10-preface.md", "front-20-copyright.md"]
BACK = ["back-10-appendix-a-glossary.md",
        "back-20-appendix-b-commands.md",
        "back-30-appendix-c-map.md",
        "back-40-appendix-d-reading.md"]

APPENDIX_TITLES = {
    "back-10-appendix-a-glossary.md": "附录 A　术语表",
    "back-20-appendix-b-commands.md": "附录 B　命令速查",
    "back-30-appendix-c-map.md": "附录 C　知识地图与学习路径",
    "back-40-appendix-d-reading.md": "附录 D　延伸阅读与工具",
}


# ---------------------------------------------------------------- 工具函数

def read(p: Path) -> str:
    return p.read_text(encoding="utf-8")


def _map_outside_fences(md: str, fn) -> str:
    """
    只在**围栏代码块之外**应用 fn(line)。

    这是本构建脚本最容易被自己咬到的地方：源稿里大量 shell 注释以 `#` 开头
    （`# 1) 建实验克隆`、`#!/bin/sh`），若不跳过代码块，它们会被当成
    Markdown 标题，把书的层次彻底打乱。
    """
    out, in_fence, fence_char = [], False, ""
    for line in md.split("\n"):
        m = re.match(r"^\s*(`{3,}|~{3,})", line)
        if m:
            # 见围栏即翻转：围栏内部不可能出现合法的围栏行。
            # 用 startswith(fence_char*3) 当闭合条件会漏掉带语言标记的围栏。
            if not in_fence:
                fence_char = m.group(1)[0]
            in_fence = not in_fence
            out.append(line)
            continue
        out.append(line if in_fence else fn(line))
    return "\n".join(out)


def demote(md: str, levels: int = 1) -> str:
    """把正文标题整体下沉 levels 级，跳过代码块内部。"""
    def fix(line: str) -> str:
        h = re.match(r"^(#{1,6})(\s+)", line)
        if not h:
            return line
        return "#" * min(len(h.group(1)) + levels, 6) + line[len(h.group(1)):]

    return _map_outside_fences(md, fix)


def collect_section_ids(md_text: str) -> set:
    """
    收集书稿里实际存在的**章内小节编号**（形如 `3.7`、`4.9.1`）。

    用于校验正文交叉引用（`见 3.8`、`第 12.7 节`）没有指向不存在的小节。
    这是"重编号事故"留下的保险丝：一旦有人再改小节编号，
    build 会立刻报出失效引用的行号，而不是等读者发现。
    """
    ids, in_fence = set(), False
    for line in md_text.split("\n"):
        if re.match(r"^\s*(`{3,}|~{3,})", line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        m = re.match(r"^#{2,6}\s+(\d+(?:\.\d+){1,2})[\s　:：]", line)
        if m:
            ids.add(m.group(1))
            # 同时登记上一级，`4.9.1` 的引用允许被 `4.9` 兜住
            if "." in m.group(1):
                ids.add(m.group(1).rsplit(".", 1)[0])
    return ids


def find_absolute_path_leaks(md_text: str) -> list:
    """
    找出泄漏进书稿的**作者本机绝对路径**。

    这种泄漏隐蔽且伤人：实验命令写成 `/Users/<name>/...` 在作者机器上能跑，
    读者照抄第一条就 `fatal: repository '<path>' does not exist`。
    审查时实证过：全书因此混着三套口径（绝对路径 / `/path/to/sourcepack` /
    `<本仓库或你自己的仓库>`），而它恰好砸在"全书第一件要做的事"上。

    判据：命令行与正文里出现 `/<Users|home>/<用户>/` 这类宿主路径。
    报告里保留 `/Users/ygs` 字样是正常的（那是内审文档，不是给读者的命令）。
    """
    bad, in_fence = [], False
    # 宿主目录前缀（macOS/Linux 的 home）。/tmp、/usr 等系统路径不算泄漏。
    pat = re.compile(r"(?:^|[\s`'\"(=])(/(?:Users|home)/[A-Za-z0-9._-]+/)")
    for i, line in enumerate(md_text.split("\n")):
        if re.match(r"^\s*(`{3,}|~{3,})", line):
            in_fence = not in_fence
            continue
        for m in pat.finditer(line):
            bad.append((i + 1, m.group(1)))
    return bad


def find_stale_go_anchors(md_text: str) -> list:
    """
    校验书里引用的**源码行号锚点**（如 `changeset.go:583`）仍然指向该函数。

    这类锚点必然会漂移：书写完之后只要有人往这个文件加一行，全部下移。
    审查时已经实证了 4 处漂移（`561-577`→583、`701-737`→720-760、
    `321`→334、`337`→353），都是"代码还在、行号对不上"。

    所以这里只做**弱校验**：把 `文件名:行号` 或 `文件名:起-止` 解析出来，
    看看那一行/那个区间内是否还包含书中提到的函数名（如果书里提到了的话）。
    提不出函数名的，就把行号本身和"该文件当前总行数"比对——
    越界的一律报出来。宁可漏报（行号对但函数错），不可把明显越界的放过。
    """
    bad = []
    go_files = {}
    for p in ROOT.glob("*.go"):
        go_files[p.name] = len(p.read_text(encoding="utf-8").split("\n"))
    # 形如 `changeset.go:561-577` / `changeset.go:583:` / `ignore.go:101-111`
    pat = re.compile(r"\b([A-Za-z0-9_]+\.go):(\d+)(?:-(\d+))?\b")
    for i, line in enumerate(md_text.split("\n")):
        if line.startswith("```"):          # 代码块内的引用不校验（示例性质）
            continue
        for m in pat.finditer(line):
            fname, start, end = m.group(1), int(m.group(2)), m.group(3)
            if fname not in go_files:
                continue
            total = go_files[fname]
            last = int(end) if end else start
            if start > total or last > total + 5:
                bad.append((i + 1, f"{fname}:{start}" + (f"-{end}" if end else ""),
                            f"该文件当前仅 {total} 行"))
    return bad


def find_bad_local_anchors(md_text: str) -> list:
    """
    检查书稿内部 `[文字](#锚点)` 是否指向真实存在的标题锚点。

    锚点由 mdmini.slugify 生成，判断口径必须与它一致（含全角空格折叠），
    否则会出现"看起来对、实际链到空"的错。[...](#c.3三种阅读路径) 就曾
    写错成带点形式，而真实锚点是 `#c-3三种阅读路径`。
    """
    from mdmini import slugify          # 局部导入，避免与构建顺序耦合

    # 现有锚点：来自 `<a id="...">` 注入 与 标题行
    anchors = set(re.findall(r'<a id="([^"]+)"', md_text))
    in_fence = False
    for line in md_text.split("\n"):
        if re.match(r"^\s*(`{3,}|~{3,})", line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        m = re.match(r"^#{1,6}\s+(.*)$", line)
        if m:
            anchors.add(slugify(m.group(1)))

    bad, in_fence = [], False
    for i, line in enumerate(md_text.split("\n")):
        if re.match(r"^\s*(`{3,}|~{3,})", line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        for m in re.finditer(r"\]\(#([^)]+)\)", line):
            if m.group(1) not in anchors:
                bad.append((i + 1, m.group(1)))
    return bad


def find_dangling_section_refs(md_text: str) -> list:
    """返回 [(行号, 原文)]，指向不存在小节的交叉引用。"""
    known = collect_section_ids(md_text)
    # 形如「见 3.8」「第 12.7 节」「对照 4.1」「6.11.3」
    pat = re.compile(r"(?:见|参见|对照|第)\s*\*?\*?(\d+\.\d+(?:\.\d+)?)")
    bad, in_fence = [], False
    for i, line in enumerate(md_text.split("\n")):
        if re.match(r"^\s*(`{3,}|~{3,})", line):
            in_fence = not in_fence
            continue
        if in_fence or line.startswith("#"):
            continue
        for m in pat.finditer(line):
            ref = m.group(1)
            if ref not in known and not ref.endswith(".0"):
                bad.append((i + 1, m.group(0)))
    return bad


def strip_first_h1(md: str):
    """取出正文首个 H1 作为章标题，返回 (标题, 去掉该行的剩余正文)。"""
    lines = md.split("\n")
    for idx, line in enumerate(lines):
        if line.startswith("# "):
            title = line[2:].strip()
            rest = lines[:idx] + lines[idx + 1:]
            return title, "\n".join(rest).lstrip("\n")
        if line.strip() and not line.startswith(("<!--", "---")):
            break
    return "", md


# 章内相对链接 → 书内锚点。build 时按标题重建映射。
_LINK_RE = re.compile(r"\]\((?:\./)?(\d{2}-[a-z-]+\.md)(#[^)]*)?\)")


def rewrite_links(md: str, slug_by_chapter: dict) -> str:
    target_slug = slug_by_chapter.get("00-preface.md", "")

    def repl(m):
        fname = m.group(1)
        if fname == "00-preface.md":
            return f"](#{slug_by_chapter.get(fname, target_slug)})"
        # 指向某一章：定位到该章标题锚点
        if fname in slug_by_chapter:
            return f"](#{slug_by_chapter[fname]})"
        return m.group(0)

    return _LINK_RE.sub(repl, md)


# ---------------------------------------------------------------- 组装书稿

def build_book_md() -> tuple:
    """返回 (markdown 文本, 目录结构, 锚点映射)。"""
    parts_md = []
    toc = []                 # [(level, text, slug, kind)]
    slug_by_chapter = {}
    seen_slugs = {}

    def uniq(slug: str) -> str:
        n = seen_slugs.get(slug, 0)
        seen_slugs[slug] = n + 1
        return slug if n == 0 else f"{slug}-{n}"

    def heading(level: int, text: str, explicit_slug: str = "") -> str:
        slug = uniq(explicit_slug or slugify(text))
        toc.append((level, text, slug, ""))
        # 用 HTML 锚点而不是 "## 目录" 那种伪标题，避免污染书稿自身的标题体系
        return f'<a id="{slug}"></a>\n\n' + "#" * level + " " + text + "\n"

    def add_front(fname):
        md = read(SRC / fname)
        if fname == "front-00-title.md":
            # 书名页：正文照旧，但它的 H1 记为书名
            title, rest = strip_first_h1(md)
            parts_md.append(f'<a id="book-title"></a>\n\n# {title}\n\n{rest.strip()}\n')
            return
        title, rest = strip_first_h1(md)
        parts_md.append(heading(2, title) + "\n" + rest.strip() + "\n")
        slug_by_chapter[fname] = toc[-1][2]

    def add_chapter(fname, fallback_title, chapter_no):
        path = COURSE / fname
        if not path.exists():
            print(f"  ! 缺章：{fname}", file=sys.stderr)
            return
        raw = read(path)
        title, rest = strip_first_h1(raw)
        title = title or fallback_title
        # 章标题统一为「第 N 章　标题」
        norm = re.sub(r"^第\s*\d+\s*章[：:]\s*", "", title).strip()
        display = f"第 {chapter_no} 章　{norm}"
        # 部 = H1，章 = H2，章内小节 = H3/H4，形成三级层次
        parts_md.append(heading(2, display) + "\n")
        slug_by_chapter[fname] = toc[-1][2]
        # 章内小节**保留原编号**（`3.7`、`4.9.1`），不做重编号。
        #
        # 早期版本把 `## 3.7` 重编成 `### 7.`，理由是"章号已在章标题里重复"。
        # 但这个"美化"破坏了全书 **109 处**正文交叉引用（`见 3.8`、`第 12.7 节`），
        # 且与已发布的 12 篇博客版（用的就是 3.7 编号）对不上。
        # 代价（一个数字前缀）远小于收益风险，所以保留原编号 —— 教科书里
        # `## 第 3 章` 下面写 `### 3.7` 本就是正常样式。
        parts_md.append(demote(rest, 1).strip() + "\n")

    def add_back(fname):
        md = read(SRC / fname)
        title, rest = strip_first_h1(md)
        title = title or APPENDIX_TITLES.get(fname, fname)
        parts_md.append(heading(2, title) + "\n" + rest.strip() + "\n")
        slug_by_chapter[fname] = toc[-1][2]

    # --- 前置 ---
    for f in FRONT:
        add_front(f)

    # --- 正文：5 部 12 章 ---
    ch_no = 0
    for part_title, part_lead, chapters in PARTS:
        parts_md.append(heading(1, part_title) + "\n" + f"> {part_lead}\n")  # 部 = H1
        for fname, fb in chapters:
            ch_no += 1
            add_chapter(fname, fb, ch_no)

    # --- 附录 ---
    for f in BACK:
        add_back(f)

    body = "\n\n---\n\n".join(p.strip() for p in parts_md if p.strip()) + "\n"

    # 第二轮：修正章内相对链接（此时锚点已全部就绪）
    body = rewrite_links(body, slug_by_chapter)

    # 前置部分的 H1 不该出现在目录里（书名/序/说明按惯例不进正文目录，书名单独入口）
    return body, toc, slug_by_chapter


def build_toc_md(toc) -> str:
    """
    生成书的目录。

    层次不能直接用标题层级 —— 那会出两个错：
      * 序/版本说明用 H2（与章同级）→ 被塞进"部"下面缩进两级；
      * 四个附录用 H2，紧跟第五部的章之后 → 看起来像第五部的子章节。
    所以这里按**语义**分三档：
        顶层（不缩进）：部、序、版本说明、附录
        缩进一级   ：章
    这正是纸质书目录的样子。
    """
    lines = ["## 目录", ""]
    for level, text, slug, _ in toc:
        if text.startswith(BOOK_TITLE):
            continue
        # 章标题的唯一可靠特征是「第 N 章　」前缀（部是「第X部」、附录是「附录 X」）
        indent = "  " if re.match(r"^第 \d+ 章", text) else ""
        lines.append(f'{indent}- [{text}](#{slug})')
    return "\n".join(lines)


# ---------------------------------------------------------------- 导出

CSS = """
:root{--bg:#fdfdfb;--fg:#22252a;--muted:#6b7280;--line:#e3e5e8;--accent:#2f6f4f;
--code-bg:#f5f6f4;--quote:#f3f6f3;--sidebar:#f7f8f6;--maxw:46rem}
@media (prefers-color-scheme:dark){:root{--bg:#16181c;--fg:#dfe3e8;--muted:#9aa2ad;
--line:#2c3038;--accent:#7fc9a2;--code-bg:#1e2126;--quote:#1c201f;--sidebar:#1a1d21}}
*{box-sizing:border-box}
html{-webkit-text-size-adjust:100%}
body{margin:0;background:var(--bg);color:var(--fg);
font:16px/1.85 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Hiragino Sans GB","Microsoft YaHei",sans-serif}
.wrap{display:flex;align-items:flex-start;gap:2.2rem;max-width:82rem;margin:0 auto;padding:0 1.2rem}
nav.toc{position:sticky;top:0;flex:0 0 19rem;max-height:100vh;overflow:auto;
padding:1.6rem 0 3rem;font-size:.82rem;line-height:1.6;border-right:1px solid var(--line)}
nav.toc h2{font-size:.78rem;letter-spacing:.12em;text-transform:uppercase;color:var(--muted);
margin:1.1rem 0 .5rem;padding:0 1rem}
nav.toc a{display:block;color:var(--fg);text-decoration:none;padding:.16rem 1rem;
border-left:2px solid transparent;opacity:.82}
nav.toc a:hover{opacity:1;border-left-color:var(--accent);background:var(--sidebar)}
nav.toc a.lv1{font-weight:650;margin-top:.35rem}
nav.toc a.lv2{padding-left:1.6rem;font-size:.79rem}
nav.toc a.lv3{padding-left:2.3rem;font-size:.76rem;color:var(--muted)}
main{flex:1 1 auto;min-width:0;max-width:var(--maxw);padding:1.4rem 0 6rem}
h1,h2,h3,h4{line-height:1.35;font-weight:680}
h1{font-size:1.72rem;margin:3.2rem 0 1.2rem;padding-bottom:.5rem;border-bottom:2px solid var(--line)}
h1:first-child{margin-top:.4rem}
h2{font-size:1.28rem;margin:2.4rem 0 .9rem}
h3{font-size:1.08rem;margin:1.9rem 0 .7rem}
h4{font-size:.98rem;margin:1.5rem 0 .6rem;color:var(--muted)}
h5,h6{font-size:.94rem;margin:1.2rem 0 .5rem;color:var(--muted)}
p{margin:.85rem 0}
a{color:var(--accent)}
a.anchor{margin-left:.4rem;opacity:0;text-decoration:none;font-weight:400}
h1:hover a.anchor,h2:hover a.anchor,h3:hover a.anchor,h4:hover a.anchor{opacity:.4}
code{background:var(--code-bg);padding:.13em .38em;border-radius:4px;
font:.88em/1.5 "SF Mono",Menlo,Consolas,"Liberation Mono",monospace}
pre{background:var(--code-bg);border:1px solid var(--line);border-radius:8px;
padding:.9rem 1rem;overflow-x:auto;line-height:1.62}
pre code{background:none;padding:0;font-size:.84rem}
blockquote{margin:1.1rem 0;padding:.5rem 1rem;border-left:3px solid var(--accent);
background:var(--quote);border-radius:0 6px 6px 0;color:var(--fg)}
blockquote p{margin:.35rem 0}
.table-wrap{overflow-x:auto;margin:1.1rem 0}
table{border-collapse:collapse;font-size:.85rem;min-width:60%}
th,td{border:1px solid var(--line);padding:.42rem .68rem;text-align:left;vertical-align:top}
th{background:var(--sidebar);font-weight:650;white-space:nowrap}
tr:nth-child(even) td{background:color-mix(in srgb,var(--sidebar) 45%,transparent)}
hr{border:0;border-top:1px solid var(--line);margin:2.6rem 0}
ul,ol{padding-left:1.5rem}
li{margin:.3rem 0}
details{background:var(--code-bg);border:1px solid var(--line);border-radius:8px;padding:.5rem 1rem;margin:1.1rem 0}
summary{cursor:pointer;font-weight:600;padding:.3rem 0}
details[open] summary{border-bottom:1px solid var(--line);margin-bottom:.6rem}
.mermaid{background:var(--code-bg);border:1px solid var(--line);border-radius:8px;
padding:1rem;margin:1.1rem 0;overflow-x:auto}
#top{position:fixed;right:1.2rem;bottom:1.2rem;background:var(--accent);color:#fff;
border-radius:50%;width:2.4rem;height:2.4rem;display:grid;place-items:center;
text-decoration:none;opacity:.45;font-size:1.1rem}
#top:hover{opacity:.9}
@media(max-width:60rem){nav.toc{display:none}.wrap{padding:0 .9rem;max-width:46rem}}
@media print{nav.toc,#top{display:none}.wrap{display:block;max-width:none}
body{font-size:11pt}pre,table{page-break-inside:avoid}h1{page-break-before:always}}
"""

HTML_TPL = """<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{title}</title>
<meta name="author" content="{author}">
<meta name="description" content="{desc}">
<style>{css}</style>
</head>
<body>
<div class="wrap">
<nav class="toc">
<div style="padding:0 1rem .6rem;font-weight:700;font-size:.95rem">{title}</div>
<div style="padding:0 1rem 1rem;color:var(--muted);font-size:.75rem">{subtitle} · v{version}</div>
{toc}
</nav>
<main>
{body}
</main>
</div>
<a id="top" href="#book-title" title="回到顶部">↑</a>
<script type="module">
// Mermaid 只在真的有图时才加载；离线时优雅退化为代码块。
if (document.querySelector('.mermaid')) {{
  import('https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.esm.min.mjs')
    .then(m => m.default.initialize({{startOnLoad:true,theme:document.documentElement
      .style.colorScheme==='dark'?'dark':'default'}}))
    .catch(() => document.querySelectorAll('.mermaid').forEach(e =>
      e.style.fontFamily='monospace'));
}}
// 侧栏高亮当前章节
const links=[...document.querySelectorAll('nav.toc a')];
const byId=new Map(links.map(a=>[a.getAttribute('href').slice(1),a]));
const io=new IntersectionObserver(es=>{{for(const e of es){{if(e.isIntersecting){{
  links.forEach(a=>a.style.background='');const a=byId.get(e.target.id);
  if(a)a.style.background='var(--sidebar)';}}}}}},{{rootMargin:'-10% 0px -80% 0px'}});
document.querySelectorAll('main h1,main h2').forEach(h=>io.observe(h));
</script>
</body>
</html>
"""


def build_html(md_text: str, toc) -> str:
    r = Renderer(shift=0)
    body = r.render(md_text)
    # 侧栏目录：用我们自己收集的 toc（比重新解析 HTML 更可靠）
    nav = []
    for level, text, slug, _ in toc:
        if text.startswith("Git 深入课"):
            continue
        cls = "lv1" if level == 1 else "lv2" if level == 2 else "lv3"
        nav.append(f'<a class="{cls}" href="#{slug}">{html.escape(text)}</a>')
    return HTML_TPL.format(
        title=html.escape(f"{BOOK_TITLE}：{BOOK_SUBTITLE}"),
        author=html.escape(BOOK_AUTHOR),
        desc=html.escape("以 sourcepack 为标本的 Git 原理教程，12 章 + 4 附录"),
        css=CSS, toc="\n".join(nav), body=body,
        subtitle=html.escape(BOOK_SUBTITLE), version=BOOK_VERSION,
    )


def build_epub(md_text: str, toc, out: Path):
    """
    EPUB 3：每章一个 XHTML + nav.xhtml 目录 + 最小 OPF。

    切分点不靠"数井号"，而是用 build 时记录的 **toc**（它知道哪一行是部、
    哪一行是章、哪一行只是章内小节或序的小标题）。早期版本按 `## ` 裸切，
    后果有两个：序的「一、二、三」和附录的每个分组都变成独立 XHTML
    （切成 66 份，阅读器目录里全是碎片）；而只按 `# ` 切时第五部单文件 173KB、
    四个附录直接不在 spine 里。
    """
    # 需要切分的标题文本集合 = 部 + 章 + 附录 + 前置的独立篇目。
    # toc 里的 (level, text, slug)；部是 level 1，其余正文单元是 level 2。
    break_texts = set()
    for level, text, slug, _ in toc:
        if text.startswith(BOOK_TITLE):
            continue
        if level <= 2:
            break_texts.add(text.strip())

    chunks, cur, cur_title, cur_slug = [], [], "封面", "cover"
    in_fence = False
    for line in md_text.split("\n"):
        if re.match(r"^\s*(`{3,}|~{3,})", line):
            in_fence = not in_fence
            cur.append(line)
            continue
        is_break = False
        if not in_fence:
            m = re.match(r"^#{1,2}\s+(.*?)\s*$", line)
            if m and m.group(1).strip() in break_texts:
                is_break = True
        if is_break:
            if cur:
                chunks.append((cur_title, cur_slug, "\n".join(cur)))
            cur_title = re.sub(r"^#+\s+", "", line).strip()
            cur_slug = slugify(cur_title)
            cur = [line]
        else:
            cur.append(line)
    if cur:
        chunks.append((cur_title, cur_slug, "\n".join(cur)))

    # 去重 slug
    used, files = {}, []
    for title, slug, md in chunks:
        n = used.get(slug, 0)
        used[slug] = n + 1
        fid = f"ch{len(files):03d}-{slug if n == 0 else slug + '-' + str(n)}"
        files.append((fid, title, md))

    if out.exists():
        out.unlink()
    with zipfile.ZipFile(out, "w") as z:
        # mimetype 必须是第一个条目且不压缩
        z.writestr(zipfile.ZipInfo("mimetype"), "application/epub+zip",
                   compress_type=zipfile.ZIP_STORED)
        z.writestr("META-INF/container.xml", """<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
<rootfiles><rootfile full-path="OEBPS/content.opf"
media-type="application/oebps-package+xml"/></rootfiles></container>""")
        z.writestr("OEBPS/style.css", CSS)

        spine, navlis = [], []
        for fid, title, md in files:
            # EPUB 关掉"悬停显示 #"的锚点链接：电子书阅读器不会悬停，
            # 那个 # 会变成标题后面一个可见的井号，纯噪音。
            r = Renderer(shift=0, anchors=False)
            body = r.render(md)
            xhtml = ('<?xml version="1.0" encoding="utf-8"?>\n'
                     '<!DOCTYPE html>\n<html xmlns="http://www.w3.org/1999/xhtml" '
                     'xml:lang="zh-CN" lang="zh-CN"><head><meta charset="utf-8"/>'
                     f'<title>{html.escape(title)}</title>'
                     '<link rel="stylesheet" type="text/css" href="style.css"/></head>'
                     f'<body>{body}</body></html>')
            z.writestr(f"OEBPS/{fid}.xhtml", xhtml)
            spine.append(f'<itemref idref="{fid}"/>')
            navlis.append(f'<li><a href="{fid}.xhtml">{html.escape(title)}</a></li>')

        z.writestr("OEBPS/nav.xhtml",
                   '<?xml version="1.0" encoding="utf-8"?>\n<!DOCTYPE html>\n'
                   '<html xmlns="http://www.w3.org/1999/xhtml" '
                   'xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="zh-CN" lang="zh-CN">'
                   '<head><meta charset="utf-8"/><title>目录</title></head><body>'
                   '<nav epub:type="toc" id="toc"><h1>目录</h1><ol>'
                   + "".join(navlis) + "</ol></nav></body></html>")

        manifest = "".join(f'<item id="{f}" href="{f}.xhtml" '
                           f'media-type="application/xhtml+xml"/>' for f, _, _ in files)
        z.writestr("OEBPS/content.opf", f"""<?xml version="1.0" encoding="utf-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid"
 xml:lang="zh-CN">
<metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
<dc:identifier id="bookid">sourcepack-git-course-{BOOK_VERSION}</dc:identifier>
<dc:title>{html.escape(BOOK_TITLE + '：' + BOOK_SUBTITLE)}</dc:title>
<dc:creator>{html.escape(BOOK_AUTHOR)}</dc:creator>
<dc:language>zh-CN</dc:language>
<meta property="dcterms:modified">2026-10-13T00:00:00Z</meta>
</metadata>
<manifest><item id="nav" href="nav.xhtml" media-type="application/xhtml+xml"
 properties="nav"/><item id="css" href="style.css" media-type="text/css"/>
{manifest}</manifest>
<spine>{''.join(spine)}</spine>
</package>""")


# ---------------------------------------------------------------- 校验

def verify(md_text: str, expected_chapters: int = 12):
    problems = []
    # 代码块必须成对
    fences = re.findall(r"^\s*```", md_text, re.M)
    if len(fences) % 2:
        problems.append(f"代码围栏数量为奇数（{len(fences)}），有未闭合代码块")

    # 表格分隔行必须跟表头一致
    lines = md_text.split("\n")
    for i, line in enumerate(lines):
        if re.match(r"^\s*\|?[\s:|-]+\|[\s:|-]*$", line) and "-" in line:
            if i == 0 or "|" not in lines[i - 1]:
                problems.append(f"第 {i+1} 行像表格分隔行但上一行不是表头：{line[:40]}")

    # 章数
    h = re.findall(r"^## 第 (\d+) 章", md_text, re.M)
    if len(h) != expected_chapters:
        problems.append(f"章标题数量为 {len(h)}，期望 {expected_chapters}")
    if [int(x) for x in h] != list(range(1, expected_chapters + 1)):
        problems.append(f"章号不连续：{h}")

    # 代码块里的 shell 注释绝不能被当成标题（本构建最易复发的 bug）。
    #
    # 判据要精确，否则误报满屏：
    #   * 合法标题形如  `### 7. 动手实验`  —— 编号后跟 **`.` + 空格 + 中文/标题文字**
    #   * 泄漏的注释形如 `# 1) 建实验克隆` 或 `# 0) ...` —— 编号后跟 **`)`**
    # 所以只把"`)` 结尾的编号标题"当成泄漏；`1.`/`7.1` 这类是正常章节编号。
    # 另外这些行若真泄漏，说明它们逃过了 demote()，通常仍带着顶层 `# `，
    # 因此再叠加一条：泄漏行的层级必须是 H1/H2（真正的注释大多是 `# ` 单井号）。
    leaked = []
    in_fence = False
    for i, line in enumerate(lines):
        if re.match(r"^\s*(`{3,}|~{3,})", line):
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        if re.match(r"^#{1,6}\s+\d+\)\s", line):
            leaked.append(i + 1)
    if leaked:
        problems.append(f"有 {len(leaked)} 处 shell 注释形态（`# N)`）被当成了标题，"
                        f"首个在第 {leaked[0]} 行")

    # 残留的相对链接
    left = re.findall(r"\]\((?:\./)?\d{2}-[a-z-]+\.md", md_text)
    if left:
        problems.append(f"仍有 {len(left)} 处指向课程目录的相对链接未改写")

    # 指向书外文件系统的相对链接：在单文件书里必然失效（书是自包含的）。
    # 形如 `](../README.md)` 或 `](published-urls.md)`。
    outside = [m for m in re.findall(r"\]\((?!https?://|#)([^)]+)\)", md_text)
               if m.endswith(".md")]
    if outside:
        problems.append(f"有 {len(outside)} 处指向书外 .md 的相对链接（单文件书里会失效）："
                        + ", ".join(sorted(set(outside))[:5]))

    # 章内小节交叉引用（`见 3.8` / `第 12.7 节`）必须指向真实存在的小节。
    # 重编号事故的直接后果就是这里全线飘红（实测 109 处）。
    dang = find_dangling_section_refs(md_text)
    if dang:
        problems.append(
            f"{len(dang)} 处小节交叉引用指向不存在的小节（确认改过小节编号？）："
            + "; ".join(f"第 {ln} 行「{txt}」" for ln, txt in dang[:6]))

    # 内部锚点链接（`[文字](#锚点)`）必须指向真实存在的标题。
    bad_anchor = find_bad_local_anchors(md_text)
    if bad_anchor:
        problems.append(
            f"{len(bad_anchor)} 处内部锚点链接指向不存在的锚点："
            + "; ".join(f"第 {ln} 行 → #{a}" for ln, a in bad_anchor[:6]))

    # 作者本机绝对路径绝不能泄漏进给读者的命令（读者照抄就会 fatal）。
    leak = find_absolute_path_leaks(md_text)
    if leak:
        paths = sorted({p for _, p in leak})
        problems.append(
            f"{len(leak)} 处作者本机绝对路径泄漏（读者照抄会 fatal：repository 不存在）："
            f"{', '.join(paths)}；首个在第 {leak[0][0]} 行")

    # 源码行号锚点必须落在该 .go 文件的行数范围内（越界一定是漂移）。
    # 注意这只抓"越界"这种硬漂移；行号对但函数错仍可能漏过，属已知边界。
    stale = find_stale_go_anchors(md_text)
    if stale:
        problems.append(
            f"{len(stale)} 处源码行号锚点越界（源码已变，行号漂移）："
            + "; ".join(f"第 {ln} 行 {ref}（{why}）" for ln, ref, why in stale[:6]))

    return problems


# ---------------------------------------------------------------- main

def main():
    DIST.mkdir(parents=True, exist_ok=True)
    missing = [f for f in FRONT + BACK if not (SRC / f).exists()]
    if missing:
        print("缺少元内容/附录文件（书仍可生成，只是不完整）：", file=sys.stderr)
        for m in missing:
            print("  -", m, file=sys.stderr)

    body, toc, slug_map = build_book_md()
    toc_md = build_toc_md(toc)
    # 目录插在书名页之后、序之前
    marker = '<a id="book-title"></a>'
    if marker in body:
        i = body.index(marker)
        j = body.index("\n---\n", i)
        full = body[:j] + "\n\n" + toc_md + "\n\n---\n" + body[j + len("\n---\n"):]
    else:
        full = toc_md + "\n\n" + body

    # 书稿头部元信息
    header = (f"<!-- {BOOK_TITLE}：{BOOK_SUBTITLE} v{BOOK_VERSION} "
              f"（由 book/tools/build.py 生成，正文源：git-course/） -->\n\n")
    full = header + full

    problems = verify(full)

    (DIST / "BOOK.md").write_text(full, encoding="utf-8")
    (DIST / "book.html").write_text(build_html(full, toc), encoding="utf-8")
    build_epub(full, toc, DIST / "book.epub")

    stats = {
        "行": len(full.split("\n")),
        "字符": len(full),
        "中文字": len(re.findall(r"[\u4e00-\u9fff]", full)),
        "代码块": len(re.findall(r"^\s*```", full, re.M)) // 2,
        "表格": len(re.findall(r"^\s*\|", full, re.M)),
        "章": len(re.findall(r"^## 第 \d+ 章", full, re.M)),
        "部": len(re.findall(r"^# 第[一二三四五]部", full, re.M)),
        "附录": len(re.findall(r"^## 附录 [A-D]", full, re.M)),
    }
    print("=== 构建完成 ===")
    for k, v in stats.items():
        print(f"  {k}: {v}")
    for p in problems:
        print("  [校验] ! " + p)
    if not problems:
        print("  校验：全部通过")
    print(f"\n产出目录：{DIST}")
    return 1 if problems else 0


if __name__ == "__main__":
    sys.exit(main())

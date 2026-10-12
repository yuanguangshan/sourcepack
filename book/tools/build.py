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


def demote(md: str, levels: int = 1) -> str:
    """
    把正文标题整体下沉 levels 级，但**跳过代码块内部**。
    源稿里有些 shell 注释以 `#` 开头，若不跳过会被误当成标题。
    """
    out, in_fence, fence_char = [], False, ""
    for line in md.split("\n"):
        m = re.match(r"^\s*(`{3,}|~{3,})", line)
        if m:
            if not in_fence:
                in_fence, fence_char = True, m.group(1)[0]
            elif line.strip().startswith(fence_char * 3):
                in_fence = False
            out.append(line)
            continue
        if not in_fence:
            h = re.match(r"^(#{1,6})(\s+)", line)
            if h:
                line = "#" * min(len(h.group(1)) + levels, 6) + line[len(h.group(1)):]
        out.append(line)
    return "\n".join(out)


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
        parts_md.append(heading(1, title) + "\n" + rest.strip() + "\n")
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
        parts_md.append(heading(1, display) + "\n")
        slug_by_chapter[fname] = toc[-1][2]
        parts_md.append(demote(rest, 1).strip() + "\n")

    def add_back(fname):
        md = read(SRC / fname)
        title, rest = strip_first_h1(md)
        title = title or APPENDIX_TITLES.get(fname, fname)
        parts_md.append(heading(1, title) + "\n" + rest.strip() + "\n")
        slug_by_chapter[fname] = toc[-1][2]

    # --- 前置 ---
    for f in FRONT:
        add_front(f)

    # --- 正文：5 部 12 章 ---
    ch_no = 0
    for part_title, part_lead, chapters in PARTS:
        parts_md.append(heading(1, part_title) + "\n" + f"> {part_lead}\n")
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
    lines = ["## 目录", ""]
    for level, text, slug, _ in toc:
        if text.startswith("Git 深入课"):
            continue
        indent = "  " * max(level - 1, 0)
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
    """EPUB 3：一本 XHTML 一章 + nav.xhtml 目录 + 最小 OPF。"""
    # 按 H1 切章
    chunks, cur, cur_title, cur_slug = [], [], "封面", "cover"
    for line in md_text.split("\n"):
        if line.startswith("# "):
            if cur:
                chunks.append((cur_title, cur_slug, "\n".join(cur)))
            cur_title = line[2:].strip()
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
            r = Renderer(shift=0)
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
    h = re.findall(r"^# 第 (\d+) 章", md_text, re.M)
    if len(h) != expected_chapters:
        problems.append(f"章标题数量为 {len(h)}，期望 {expected_chapters}")
    if [int(x) for x in h] != list(range(1, expected_chapters + 1)):
        problems.append(f"章号不连续：{h}")

    # 残留的相对链接
    left = re.findall(r"\]\((?:\./)?\d{2}-[a-z-]+\.md", md_text)
    if left:
        problems.append(f"仍有 {len(left)} 处指向课程目录的相对链接未改写")

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
        "章": len(re.findall(r"^# 第 \d+ 章", full, re.M)),
        "部": len(re.findall(r"^# 第[一二三四五]部", full, re.M)),
        "附录": len(re.findall(r"^# 附录 [A-D]", full, re.M)),
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

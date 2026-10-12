#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
mdmini —— 一个够用的 Markdown → HTML 转换器（零依赖，纯标准库）。

为什么自己写：这个仓库只装了 Go/Python/Node，没有 pandoc、没有 calibre。
为了"把课程整合成一本书"而引入一堆构建依赖不划算，何况本书的 Markdown
子集是可控的：标题、段落、围栏代码块、表格、引用块、列表、行内样式。

它**不追求** CommonMark 全兼容。遇到不认识的写法会退化成段落，而不是崩掉。
"""

import html
import re

# ---------------------------------------------------------------- 行内解析

def esc(text: str) -> str:
    """转义 HTML 元字符（代码块用）。"""
    return html.escape(text, quote=False)


def slugify(text: str) -> str:
    """
    生成稳定的锚点 id。

    中文标题无法照搬 GitHub 的算法（那套以 ASCII 为中心），这里采用
    "保留字母数字与 CJK、其余折叠为连字符"的策略，并且**只用标题文本**，
    因此同一个标题在任何构建里得到同样的 id —— 交叉引用才能靠它。
    """
    s = text.strip().lower()
    s = re.sub(r"`([^`]*)`", r"\1", s)          # 去掉行内代码的反引号
    s = re.sub(r"\*\*([^*]*)\*\*", r"\1", s)     # 去掉粗体标记
    s = re.sub(r"\*([^*]*)\*", r"\1", s)         # 去掉斜体标记
    s = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", s)  # 链接只留文字
    out = []
    for ch in s:
        if ch.isalnum() or ch == "_" or "\u4e00" <= ch <= "\u9fff":
            out.append(ch)
        elif ch in " \t-/.":
            out.append("-")
        # 其余字符（标点、`、* 等）直接丢弃
    slug = "".join(out)
    slug = re.sub(r"-{2,}", "-", slug).strip("-")
    return slug or "section"


_INLINE_CODE = re.compile(r"(`+)(.+?)\1", re.S)


def inline(text: str) -> str:
    """
    行内 Markdown → HTML。

    处理顺序很关键：先把行内代码"挖"出来占位，避免代码里的 `*` `_` `|`
    被后面的规则误伤 —— 这正是源书稿里出现 `*` 通配符时会踩的坑。
    """
    stash = []

    def _stash(m):
        stash.append(m.group(2))
        return f"\x00{len(stash)-1}\x00"

    text = _INLINE_CODE.sub(_stash, text)

    text = esc(text)

    # 图片先于链接（语法前缀相同）
    text = re.sub(r"!\[([^\]]*)\]\(([^)\s]+)(?:\s+\"([^\"]*)\")?\)",
                  lambda m: f'<img src="{m.group(2)}" alt="{m.group(1)}"'
                            + (f' title="{m.group(3)}"' if m.group(3) else "") + "/>",
                  text)
    text = re.sub(r"\[([^\]]+)\]\(([^)\s]+)(?:\s+\"([^\"]*)\")?\)",
                  lambda m: f'<a href="{m.group(2)}"'
                            + (f' title="{m.group(3)}"' if m.group(3) else "") + f'>{m.group(1)}</a>',
                  text)
    # 粗体要先于斜体，否则 **x** 会被当成两个 *
    text = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", text, flags=re.S)
    text = re.sub(r"(?<!\*)\*([^*\n]+?)\*(?!\*)", r"<em>\1</em>", text)
    text = re.sub(r"~~(.+?)~~", r"<del>\1</del>", text)

    def _unstash(m):
        return f"<code>{esc(stash[int(m.group(1))])}</code>"

    return re.sub(r"\x00(\d+)\x00", _unstash, text)


# ---------------------------------------------------------------- 块级解析

FENCE = re.compile(r"^(\s*)(`{3,}|~{3,})\s*([\w+-]*)\s*$")
HEADING = re.compile(r"^(#{1,6})\s+(.*?)\s*#*\s*$")
UL_ITEM = re.compile(r"^(\s*)[-*+]\s+(.*)$")
OL_ITEM = re.compile(r"^(\s*)(\d+)[.)]\s+(.*)$")
TABLE_SEP = re.compile(r"^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$")


def _split_row(row: str):
    row = row.strip()
    if row.startswith("|"):
        row = row[1:]
    if row.endswith("|"):
        row = row[:-1]
    # 支持 \| 转义
    parts = re.split(r"(?<!\\)\|", row)
    return [p.strip().replace("\\|", "|") for p in parts]


class Renderer:
    """把 Markdown 渲染成 HTML 片段。会记录标题，供生成目录用。"""

    def __init__(self, shift: int = 0, collect_headers: bool = True,
                 anchors: bool = True):
        self.shift = shift              # 标题整体下沉层级（书名页 H1 → 书的 H1 之下）
        self.headers = []               # [(level, text, slug)]
        self.collect = collect_headers
        # HTML 版要"悬停显示 #"的锚点链接；EPUB 版不要（那是可见的噪音文字）
        self.anchors = anchors
        self._used = {}

    def _uniq(self, slug: str) -> str:
        n = self._used.get(slug, 0)
        self._used[slug] = n + 1
        return slug if n == 0 else f"{slug}-{n}"

    def render(self, md: str) -> str:
        lines = md.replace("\r\n", "\n").replace("\r", "\n").split("\n")
        out = []
        i = 0
        n = len(lines)

        while i < n:
            line = lines[i]

            # ---- 围栏代码块 ----
            m = FENCE.match(line)
            if m:
                indent, fence, lang = m.group(1), m.group(2), m.group(3)
                char = fence[0]
                body = []
                i += 1
                while i < n and not re.match(rf"^\s*{re.escape(char)}{{3,}}\s*$", lines[i]):
                    body.append(lines[i])
                    i += 1
                i += 1  # 吃掉收尾围栏
                code = "\n".join(body)
                if lang == "mermaid":
                    out.append(f'<div class="mermaid">{esc(code)}</div>')
                else:
                    cls = f' class="language-{lang}"' if lang else ""
                    out.append(f"<pre><code{cls}>{esc(code)}\n</code></pre>")
                continue

            # ---- 裸 HTML 锚点（build 注入的 <a id="..."></a>）----
            # 必须原样输出成真正的 HTML，否则会被转义成可见文字
            # `&lt;a id="..."&gt;&lt;/a&gt;`，在 EPUB 里满页乱码。
            if re.match(r'^\s*<a\s+id="[^"]*"\s*>\s*</a>\s*$', line):
                out.append(line.strip())
                i += 1
                continue

            # ---- 少量 HTML 标签原样透传 ----
            # `<details>`/`<summary>` 承载"不依赖网络的纯文本版图表"，是 Mermaid
            # 的离线兜底；若被当成段落转义，书页上会出现 `&lt;details&gt;` 这种乱码。
            if re.match(r'^\s*</?(details|summary|div|table|thead|tbody|tr|td|th)\b',
                        line.strip()):
                out.append(line)
                i += 1
                continue

            # ---- 空行 ----
            if not line.strip():
                i += 1
                continue

            # ---- HTML 注释（构建头部元信息）----
            # 必须显式吃掉：否则会被当成段落转义输出，书里出现一行
            # `&lt;!-- Git 深入课 ... --&gt;` 的乱码文字。多行注释也要一起吞掉。
            if line.lstrip().startswith("<!--"):
                while i < n and "-->" not in lines[i]:
                    i += 1
                i += 1
                continue

            # ---- 分隔线 ----
            if re.match(r"^\s*([-*_])(\s*\1){2,}\s*$", line):
                out.append("<hr/>")
                i += 1
                continue

            # ---- 标题 ----
            m = HEADING.match(line)
            if m:
                lvl = min(len(m.group(1)) + self.shift, 6)
                text = m.group(2)
                slug = self._uniq(slugify(text))
                if self.collect:
                    self.headers.append((lvl, text, slug))
                mark = (f'<a class="anchor" href="#{slug}" aria-hidden="true">#</a>'
                        if self.anchors else "")
                out.append(f'<h{lvl} id="{slug}">{inline(text)}{mark}</h{lvl}>')
                i += 1
                continue

            # ---- 引用块 ----
            if line.lstrip().startswith(">"):
                buf = []
                while i < n and (lines[i].lstrip().startswith(">") or
                                 (buf and lines[i].strip())):
                    buf.append(re.sub(r"^\s*>\s?", "", lines[i]))
                    i += 1
                inner = Renderer(shift=0, collect_headers=False).render("\n".join(buf))
                out.append(f"<blockquote>{inner}</blockquote>")
                continue

            # ---- 表格 ----
            if "|" in line and i + 1 < n and TABLE_SEP.match(lines[i + 1]):
                head = _split_row(line)
                i += 2
                rows = []
                while i < n and "|" in lines[i] and lines[i].strip():
                    rows.append(_split_row(lines[i]))
                    i += 1
                t = ['<div class="table-wrap"><table>', "<thead><tr>"]
                t += [f"<th>{inline(c)}</th>" for c in head]
                t.append("</tr></thead><tbody>")
                for r in rows:
                    r = r + [""] * (len(head) - len(r))
                    t.append("<tr>" + "".join(f"<td>{inline(c)}</td>" for c in r[:len(head)]) + "</tr>")
                t.append("</tbody></table></div>")
                out.append("".join(t))
                continue

            # ---- 列表 ----
            if UL_ITEM.match(line) or OL_ITEM.match(line):
                ordered = bool(OL_ITEM.match(line))
                items = []
                while i < n:
                    mm = (OL_ITEM.match(lines[i]) if ordered else UL_ITEM.match(lines[i]))
                    if not mm:
                        # 允许列表项之间的续行（缩进 2+ 的非空行）
                        if items and lines[i].strip() and lines[i].startswith(("  ", "\t")):
                            items[-1] += "\n" + lines[i].strip()
                            i += 1
                            continue
                        break
                    items.append(mm.group(3) if ordered else mm.group(2))
                    i += 1
                tag = "ol" if ordered else "ul"
                li = []
                for it in items:
                    parts = it.split("\n")
                    body = inline(parts[0])
                    # 用 <br/> 而不是 <br>：EPUB 是 XHTML，必须 XML 合法。
                    # 只写 <br> 会让 epubcheck 直接判错（曾导致 28 个 xhtml 解析失败）。
                    for extra in parts[1:]:
                        body += "<br/>" + inline(extra)
                    li.append(f"<li>{body}</li>")
                out.append(f"<{tag}>" + "".join(li) + f"</{tag}>")
                continue

            # ---- 段落（连续非空行，且不被上面的规则截断） ----
            buf = [line]
            i += 1
            while i < n and lines[i].strip():
                nxt = lines[i]
                if (FENCE.match(nxt) or HEADING.match(nxt) or UL_ITEM.match(nxt)
                        or OL_ITEM.match(nxt) or nxt.lstrip().startswith(">")
                        or re.match(r"^\s*([-*_])(\s*\1){2,}\s*$", nxt)):
                    break
                if "|" in nxt and i + 1 < n and TABLE_SEP.match(lines[i + 1]):
                    break
                buf.append(nxt)
                i += 1
            out.append("<p>" + inline("\n".join(buf)) + "</p>")

        return "\n".join(out)

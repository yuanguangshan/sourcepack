#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
minigx_selftest —— 用"以 git 为真值"的方式验证 minigx.py。

这就是第 12.9 节第 (12) 条说的**差分测试**：不写"我想得到的用例"，
而是**拿 git 自己的输出当标准答案**，逐项比对。你在自己机器上跑一次就能复现。

    $ python3 minigx_selftest.py

覆盖的硬场景（每一条都对应书里的一处坑）：
  * 中文文件名        → 第 (3) 条 quotepath
  * 仓库里真有 b/ 目录 → 第 (6) 条"只剥一层前缀"
  * 三种状态同时存在   → 第 (4) 条 -z 清单
  * 重命名（R）        → 第 (8) 条 -M
  * 合并提交（多父）   → 第 (11) 条取第一父
  * unborn HEAD       → 第 2.6 节的空树常量
"""

import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
MINIGX = os.path.join(HERE, "minigx.py")


def sh(cmd, cwd):
    return subprocess.run(cmd, cwd=cwd, shell=True, stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE, text=True)


def git_truth(repo):
    """用 `git status --porcelain` 的两列当标准答案。

    第 1 列 = 索引 vs HEAD（staged），第 2 列 = 工作区 vs 索引（unstaged），
    `??` = 未跟踪。这与 minigx 输出的三个计数口径一致。
    """
    out = sh("git status --porcelain", repo).stdout
    staged = unstaged = untracked = 0
    for line in out.split("\n"):
        if not line:
            continue
        c1, c2 = line[0], line[1]
        if c1 == "?" and c2 == "?":
            untracked += 1
        else:
            if c1 not in (" ", "?"):
                staged += 1
            if c2 != " ":
                unstaged += 1
    return staged, unstaged, untracked


def minigx_counts(repo):
    """跑 minigx，把它打印的三个计数解析出来。"""
    r = subprocess.run([sys.executable, MINIGX, repo],
                       stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
    if r.returncode != 0:
        raise RuntimeError("minigx 失败：%s" % r.stderr.strip())
    got = {}
    for key in ("staged", "unstaged", "untracked"):
        m = re.search(r"^%s\s+:\s+(\d+)$" % key, r.stdout, re.M)
        got[key] = int(m.group(1)) if m else None
    return got, r.stdout


def build_fixture(root):
    """造一个把书里那些坑全都踩一遍的仓库。"""
    sh("git init -q .", root)
    sh("git config user.email lab@example.com", root)
    sh("git config user.name Lab", root)
    # 故意让 Git 用默认的 core.quotepath=true，制造八进制转义的环境
    sh("git config core.quotepath true", root)

    # 真有 b/ 目录：这是"多剥一层前缀"的陷阱
    os.makedirs(os.path.join(root, "b"), exist_ok=True)
    open(os.path.join(root, "b", "x.go"), "w").write("x\n")
    open(os.path.join(root, "中文文件.txt"), "w").write("中文\n")
    open(os.path.join(root, "file with space.txt"), "w").write("space\n")
    sh("git add . && git commit -qm base", root)

    # 分叉 + 合并（制造多父提交）
    sh("git checkout -qb feat", root)
    open(os.path.join(root, "b", "x.go"), "a").write("feat\n")
    sh("git commit -qam 'feat change'", root)
    sh("git checkout -q -", root)
    open(os.path.join(root, "中文文件.txt"), "a").write("main\n")
    sh("git commit -qam 'main change'", root)
    sh("git merge -q --no-ff feat -m 'merge feat'", root)

    # 同时制造 staged / 重命名 / 未跟踪
    #
    # 关键：这里必须**再改一次 b/x.go 并暂存**。否则被多剥一层前缀的陷阱
    # 根本没被踩到——合并已经把 b/x.go 的改动提交掉了，它不会出现在清单里。
    # （第一版自测就是这么漏掉的：断言在找一个根本不该出现的路径。）
    open(os.path.join(root, "b", "x.go"), "a").write("staged-mod\n")
    sh("git add b/x.go", root)
    open(os.path.join(root, "new_staged.txt"), "w").write("s\n")
    sh("git add new_staged.txt", root)
    sh("git mv 'file with space.txt' 'renamed 文件.txt'", root)
    open(os.path.join(root, "untracked_新文件.txt"), "w").write("u\n")
    return root


def case_hard(root, results):
    label = "硬场景：中文名 + 真有 b/ 目录 + 三态 + 重命名 + 合并"
    truth = git_truth(root)
    got, out = minigx_counts(root)
    ok = (truth == (got["staged"], got["unstaged"], got["untracked"]))
    results.append((label, ok, "git=%s minigx=%s" % (truth, tuple(got.values()))))

    # 注意：minigx 的**输出**只有计数，没有路径清单。
    # 所以"路径是否被多剥一层"必须直接查它解析出来的条目，而不是 grep stdout
    # ——第一版自测就是在这里写错了断言（在打印内容里找 b/x.go，永远找不到）。
    sys.path.insert(0, HERE)
    import minigx
    entries = minigx.name_status(minigx.Git(root), cached=True)
    # 重命名条目的三个字段是 (状态, 旧路径, 新路径)，两边都要收进来看
    paths = []
    for (_st, p, new) in entries:
        if p:
            paths.append(p)
        if new:
            paths.append(new)

    results.append(("  b/x.go 未被多剥一层（保留原样）", "b/x.go" in paths,
                    "解析到：%s" % paths))
    # 判据要写准：不能只找"中文"两个字——本夹具里的中文路径是改名后的
    # `renamed 文件.txt`。所以判"存在 CJK 字符"且"不存在八进制转义（\\3xx）"。
    has_cjk = any(re.search(r"[\u4e00-\u9fff]", p) for p in paths)
    has_octal = any(re.search(r"\\[0-7]{3}", p) for p in paths)
    results.append(("  中文路径未被八进制转义", has_cjk and not has_octal,
                    "含 CJK 的路径：%s" % [p for p in paths if re.search(r"[\u4e00-\u9fff]", p)]))
    # 重命名必须被识别（状态以 R 开头）
    results.append(("  重命名被识别（状态 R）",
                    any(st.startswith("R") for (st, _p, _n) in entries), ""))
    # 合并提交必须被认出来
    results.append(("  合并提交被标出（取第一父）", "⑂" in out, ""))


def case_unborn(root_parent, results):
    label = "边界：unborn HEAD（刚 init，还没有任何提交）"
    root = tempfile.mkdtemp(dir=root_parent)
    sh("git init -q .", root)
    sh("git config user.email lab@example.com", root)
    sh("git config user.name Lab", root)
    open(os.path.join(root, "new.txt"), "w").write("n\n")
    truth = git_truth(root)
    got, out = minigx_counts(root)
    ok = (truth == (got["staged"], got["unstaged"], got["untracked"]))
    results.append((label, ok, "git=%s minigx=%s" % (truth, tuple(got.values()))))
    results.append(("  没有因 rev-parse 失败而崩", "尚无提交" in out, ""))


def main():
    parent = tempfile.mkdtemp(prefix="minigx-selftest-")
    results = []
    try:
        case_hard(build_fixture(parent), results)
        case_unborn(parent, results)
    finally:
        shutil.rmtree(parent, ignore_errors=True)

    print("═══ minigx 差分自测（以 git 为真值）═══")
    bad = 0
    for label, ok, detail in results:
        print("  %s %s%s" % ("✅" if ok else "❌", label,
                             ("   " + detail) if detail else ""))
        if not ok:
            bad += 1
    print()
    if bad:
        print("❌ %d 项不符 —— 说明 minigx 或本自测需要修" % bad)
        return 1
    print("✅ 全部通过：minigx 的计数与 `git status --porcelain` 完全一致")
    return 0


if __name__ == "__main__":
    sys.exit(main())

#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
minigx —— 一个最小可运行的「读 Git」工具（约 130 行）。

它是第 12.9 节那 12 条 checklist 的可执行版本：每一处加固都标了它对应第几条。
读它的方式是这样的——**别把它当示例代码看，把它当"checklist 的运行证据"看**：
下面每一个 `# (N)` 注释，都对应书里的一条，且都可以被单独删掉、看到后果。

    $ python3 minigx.py /path/to/repo

输出：当前分支、HEAD 短哈希、staged/unstaged/untracked 三个计数、最近 5 个提交。

设计取舍（都是刻意的）：
  * **零依赖**：只用 Python 标准库。不需要装 Go、不需要 pip install。
  * **只读**：它从不写对象库、不动引用、不碰索引（第 (10) 条）。
    你可以对任何仓库跑它，包括正在用的。
  * **与 sourcepack 同构**：这里的 `git()` 就是 sourcepack 里 `gitRunner.command()`
    的 Python 版，加固项一一对应——所以看完它再去读 `changeset.go` 不会陌生。
"""

import os
import subprocess
import sys

# ── 第 (9) 条：给子进程输出设体积上限。
# 一个恶意/超大仓库可能让 `git log` 吐出几百 MB。设上限，超了就截断。
MAX_OUTPUT = 8 * 1024 * 1024          # 8 MiB


class Git:
    """第 (1) 条：**所有** git 调用走这一个封装。加固只写在这里。"""

    def __init__(self, repo):
        self.repo = os.path.abspath(repo)

    def run(self, *args, limit=MAX_OUTPUT):
        # 第 (2) 条：禁分页、禁交互、禁 stdin、禁锁。
        #   --no-pager          别启动 pager（否则可能挂住等你按键）
        #   GIT_PAGER=cat       兜底：某些路径只看环境变量
        #   GIT_TERMINAL_PROMPT=0  需要凭据时**直接失败**，别弹交互
        #   stdin=DEVNULL       把子进程的 stdin 关掉，否则它可能等输入
        #   GIT_OPTIONAL_LOCKS=0    别去抢 index.lock（我们不需要写）
        env = dict(os.environ)
        env["GIT_PAGER"] = "cat"
        env["GIT_TERMINAL_PROMPT"] = "0"
        env["GIT_OPTIONAL_LOCKS"] = "0"

        cmd = [
            "git",
            "--no-pager",
            # 第 (3) 条：不加它，中文路径会变成 "\344\270\255" 这种八进制转义。
            #
            # 实测的边界（别记错）：这条只对**非 `-z`** 的输出生效——
            #   `git ls-files`             → "\344\270\255..."（转义）
            #   `git ls-files -z`          → 中文文件.txt   （`-z` 本身就是"原样输出"）
            # 真正非加不可的地方是 **patch 正文的 `diff --git` 头**：
            #   `git diff`  → diff --git "a/\344\270\255..." "b/\344\270\255..."
            #   （加了这条）→ diff --git a/中文文件.txt b/中文文件.txt
            # 所以它是给"读 patch 正文"用的，不是给"读 -z 清单"用的。
            "-c", "core.quotepath=false",
            "-C", self.repo,
            *args,
        ]
        proc = subprocess.run(
            cmd, env=env, stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE,
        )
        out = proc.stdout
        if len(out) >= limit:
            # 第 (9) 条：截断之后**必须把管道排干**，否则子进程可能因为
            # 写不进管道而卡死（书里的 sourcepack 同样处理）。
            out = out[:limit]
        if proc.returncode != 0:
            # 只读工具要给出可解释的报错，而不是让栈追溯糊用户一脸。
            raise RuntimeError(
                "git %s 失败（exit %d）：%s"
                % (" ".join(args), proc.returncode, proc.stderr.decode("utf-8", "replace").strip())
            )
        return out

    def try_run(self, *args):
        """给"允许失败"的调用用（例如 unborn HEAD 下 rev-parse 会失败）。"""
        try:
            return self.run(*args)
        except RuntimeError:
            return None


# ── 第 (4) 条：一切清单用 `-z`；自定义字段用不可能出现的分隔符。
#
# `-z` 让 git 用 NUL 分隔条目——文件名里**什么都可能出现**（空格、换行、`|`），
# 唯 NUL 不可能，因为它是路径的终止符本身。
#
# 而 `git log --format` 没有 `-z`，所以我们自己定分隔符：
# 用 ASCII 的 Unit Separator（0x1F），同样不会出现在作者名或提交信息里。
# 书里 sourcepack 的常量就是 `logFieldSep = "\x1f"`。
US = "\x1f"


def split_z(data):
    """把 `-z` 输出切成条目列表（末尾那个空串要丢掉）。"""
    text = data.decode("utf-8", "replace")
    return [x for x in text.split("\0") if x != ""]


def split_lines(data):
    return [l for l in data.decode("utf-8", "replace").split("\n") if l != ""]


# ── 第 (6) 条：路径解析**只剥一层**前缀。
#
# `--name-status` 与 `diff --git` 行里的路径可能带 `a/`、`b/` 前缀。
# 只剥一层：仓库里完全可能真的有一个叫 `b/` 的目录，多剥就把 `b/x.go` 变成 `x.go`。
# 这正是 sourcepack v1.8.0「幽灵条目」的根因（见第 6.11.2 节）。
def strip_one_prefix(path, prefix):
    if path.startswith(prefix):
        return path[len(prefix):]
    return path


def name_status(git, *extra, cached=False):
    """取一份「状态 + 路径」清单。第 (4) 条：用 -z；第 (8) 条：-M 才有重命名。"""
    args = [
        # 第 (5) 条：固定协议、不猜配置。
        #   --no-ext-diff  别让用户配的外部 diff 程序参与
        #   --no-textconv  别让 .gitattributes 里的 textconv 参与
        #   固定 --src-prefix/--dst-prefix，否则前缀会被 diff.mnemonicPrefix 改写
        "diff", "--no-ext-diff", "--no-textconv",
        "--src-prefix=a/", "--dst-prefix=b/",
        "--name-status", "-z", "-M",
    ]
    if cached:
        args.append("--cached")
    args.extend(extra)
    fields = split_z(git.run(*args))

    # --name-status -z 的输出是「状态\0路径\0」成对（重命名/复制是「状态\0旧\0新\0」三连）。
    entries, i = [], 0
    while i < len(fields):
        status = fields[i]
        kind = status[0]
        if kind in ("R", "C"):                    # 重命名 / 复制：多一个路径
            if i + 2 >= len(fields):
                break
            old = strip_one_prefix(fields[i + 1], "a/")
            new = strip_one_prefix(fields[i + 2], "b/")
            entries.append((status, old, new))
            i += 3
        else:
            path = strip_one_prefix(fields[i + 1], "a/") if i + 1 < len(fields) else ""
            entries.append((status, path, None))
            i += 2
    return entries


def head_info(git):
    """当前分支 + HEAD 短哈希。注意 unborn HEAD（刚 init、还没有提交）要能处理。"""
    # 分支名：问"名字"用 symbolic-ref；detached 时它会失败，退回"detached"。
    branch_raw = git.try_run("symbolic-ref", "--short", "HEAD")
    branch = branch_raw.decode("utf-8", "replace").strip() if branch_raw else "(detached)"

    # HEAD 短哈希：unborn 状态下 rev-parse 会失败——这不是错误，是合法状态。
    sha_raw = git.try_run("rev-parse", "--short", "HEAD")
    sha = sha_raw.decode("utf-8", "replace").strip() if sha_raw else "(尚无提交)"
    return branch, sha


def counts(git):
    """三个计数。第 (11) 条：合并提交取第一父的功课，体现在下面的 base 选择上。"""
    # base：有 HEAD 就拿 HEAD；unborn 时用空树（第 2.6 节的 4b825dc...）。
    #
    # 为什么 unborn 要特判：`git diff --cached` 在还没有 HEAD 时会报错，
    # 而语义上「与空树比」正好等价于「全部都是新增」。这个常量不是魔法，
    # 就是那棵没有条目的 tree（书里 sourcepack 的 `emptyTreeSHA`）。
    empty_tree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
    has_head = git.try_run("rev-parse", "--verify", "HEAD") is not None
    base = [] if has_head else [empty_tree]

    staged = name_status(git, *base, cached=True)
    unstaged = name_status(git)
    untracked = split_z(git.run(
        "ls-files", "--others", "--exclude-standard", "--full-name", "-z"))
    return staged, unstaged, untracked


def recent_commits(git, n=5):
    """
    最近 n 个提交。

    第 (4) 条：字段之间用 0x1F 而不是空格/逗号/`|`——作者名与提交信息里什么都可能有。
    %P 是**所有**父提交（合并提交会有两个），这里取第一个：第 (11) 条。
    """
    fmt = US.join(["%H", "%h", "%an", "%ad", "%P", "%s"])
    # 注意**故意不加** `--no-merges`：合并提交正是第 (11) 条要处理的场景，
    # 滤掉它们就看不到"取第一父"这件事了。
    raw = git.try_run("log", "-n", str(n), "--date=short", "--format=" + fmt)
    if raw is None:
        return []
    out = []
    for line in split_lines(raw):
        parts = line.split(US)
        if len(parts) < 6:
            continue
        sha, short, author, date, parents, subject = parts[:6]
        first_parent = parents.split(" ")[0] if parents else ""
        is_merge = len(parents.split(" ")) > 1 if parents else False
        out.append({
            "sha": sha, "short": short, "author": author,
            "date": date, "subject": subject,
            "is_merge": is_merge,
            "first_parent": first_parent[:7],
        })
    return out


def main(argv):
    if len(argv) != 2:
        print("用法：python3 minigx.py <仓库路径>", file=sys.stderr)
        return 2

    repo = argv[1]
    if not os.path.isdir(os.path.join(repo, ".git")) and not os.path.isdir(repo):
        print("不是一个目录：%s" % repo, file=sys.stderr)
        return 2

    git = Git(repo)
    # 第 (10) 条：只读承诺。这里先确认能读，读不到就明确报错。
    if git.try_run("rev-parse", "--git-dir") is None:
        print("不是一个 git 仓库：%s" % repo, file=sys.stderr)
        return 2

    branch, sha = head_info(git)
    staged, unstaged, untracked = counts(git)
    commits = recent_commits(git)

    print("仓库      : %s" % git.repo)
    print("当前分支  : %s" % branch)
    print("HEAD      : %s" % sha)
    print("staged    : %d" % len(staged))
    print("unstaged  : %d" % len(unstaged))
    print("untracked : %d" % len(untracked))

    # 第 (11) 条：把口径写进产物——用户才知道"1 个"指的是什么。
    if staged or unstaged:
        print("            （staged/unstaged 口径：`--name-status -M` 的条目数，"
              "与 `git status --porcelain` 的两列对应）")
    print("最近 %d 个提交：" % len(commits))
    for c in commits:
        mark = "⑂" if c["is_merge"] else "·"
        extra = ("   [合并提交，第一父 %s]" % c["first_parent"]) if c["is_merge"] else ""
        print("  %s %s  %-10s %s  %s%s"
              % (mark, c["short"], c["date"], c["author"], c["subject"][:46], extra))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))

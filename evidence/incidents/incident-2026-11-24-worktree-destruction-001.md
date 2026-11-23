# 事件记录 #1（2026-11-24 00:00 前后）· dev worktree 部分摧毁事件（归因未定，E4）

## 0. 一句话
2026-11-23 23:58（S2 收口）至 2026-11-24 00:0x 之间，`/private/tmp/culler-worktrees/dev`（shz/dev@6ae57e3 工作树）被一次**未归因操作**删除了 1944 个受跟踪文件与 `.git` 指针文件，并创建一个空的 `.culler/` 目录。**零数据损失**（全部内容已在提交中），已完整恢复（`git worktree repair`＋`git checkout -- .`，00:09）。

## 1. 时间线（全部实读证据）
| 时刻（本地） | 事件 | 证据 |
|---|---|---|
| ~23:48 | S2 改名在 a09s2 worktree 执行（dev 未涉） | 本会话操作记录 |
| ~23:52–23:58 | S2 提交 6ae57e3 ff 进 dev；a09s2/a09note worktree 回收（父目录 mtime=23:52） | git log＋目录 mtime |
| **00:00** | **事故窗口**：dev worktree 内 1944 受跟踪文件被删除（LICENSE/README.md/VERSION/CHANGELOG.md/.gitattributes/.gitignore/.mise.toml/.github/ISSUE_TEMPLATE/*/.culler/Dockerfile/Dockerfile.culler-project/ANALYSIS-*/GIT-DELIVERY.md/SECURITY.md/ITERATION-ROADMAP/skills-lock.json/dashboard 构建文件族…）；`.git` 指针文件被删；空 `.culler/` 目录出现（=受跟踪 `.culler/Dockerfile` 被删后的残壳） | 目录 mtime=00:00；git status " D"×1944；物理计数 1042（盘）vs 2501（提交） |
| 00:0x | 日期翻转；所有者指令"继续剩余任务推进"到达 | 会话记录 |
| 00:09 | 发现（ff merge 报 "not a git repository"）→`git worktree repair`（重建 .git 指针）→`git checkout -- .`（恢复全部受跟踪文件）→status=0 行（净树）→ff 进 2ef514c（AM v0.2.5）→blcheck 72/0 | 本会话操作记录 |

## 2. 已排除的假设（逐条实读证据）
- **macOS 开机清 /private/tmp**：否。`uptime`=6 天（boot 于 11-17），机器未重启。
- **本机 Culler 部署行为**：否。`~/.culler`/`~/.shorizon` 均不存在；launchd 无 culler/shorizon/mc- 单元。
- **所有者 shell 操作**：否（现有证据内）。zsh_history 尾部（最近条目 epoch≈11-21/22）无任何指向 culler-worktrees 的命令。
- **我的操作**：否。本会话对 dev worktree 的最后一次写=23:5x 的 S2 ff merge（内容=提交字节，已哈希验证 MATCH）；AM v0.2.5 登记全部在 am-v025 worktree 内完成（该 worktree 全程完好，48 条目完整检出）。
- **内容篡改**：否。事故前抽查的 7 个"mtime 被刷新"文件字节=提交值逐字 MATCH；事故后幸存文件集无一新内容（空 `.culler/` 目录除外）。

## 3. 归因现状（E4=未知，不猜）
删除集合的选择性（受跟踪文件中删 1944、留 ~557；幸存集=恰好 S1/S2 触碰的文件+部分目录）与 `rsync -a --delete <部分源>/ dev/` 或"按文件清单 find -delete"的签名相容，但**无直接证据指向任何具体进程/主体**。若所有者在 11-23 23:58–11-24 00:10 之间在本机做过操作（另一个 agent 窗口、IDE、清理脚本），即可解释；执行侧不越权断言。

## 4. 影响与处置
- **数据**：零损失。全部工作（S1/S2/A06/A34/隔离/AM 登记）在提交中（branch ref＋object database 在主仓库，不受 worktree 目录影响）。
- **恢复完整性**：`git checkout -- .` 后 status=0 行净树；AM 文件哈希=登记值；blcheck 72/0。
- **安全面**：无新文件注入、无内容篡改、无远端变更（外发面仅白天的 ci/routing-base 推进，全程本会话实读）。
- **预防**：`/private/tmp` 工作树约定（AGENTS.md）本身含"macOS 重启即失"风险，本次事件证明**"提交到分支"纪律是唯一可靠防线**——本会话全部工作当时均已提交，故损失为零。后续每波收口时增加一次"branch tip vs 远端/主仓库 object DB 可达性"抽查（`git fsck` 轻量面）不必常做，但**任何 worktree 异常先 `git log <branch>` 在主仓库侧验证提交幸存，再谈恢复**。

## 5. 登记义务
本事件入 AM 下一登记轮（v0.2.6）①行；不属于任何工单面的产品缺陷（DEFECT-REGISTER 不收环境事件）。

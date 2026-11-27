# 外发登记 · push shz/dev → origin/main（2026-11-27）

- 所有者裁定＝「push，然后继续推进工作」（轮五十三 F1 确认包收口）。
- 远端＝`https://github.com/warlockedward/Culler.git`（本机唯一 origin；硬边界记忆在位：绝不 upstream vinchenkov/Culler——本仓库未配置该 remote，推送命令显式 refspec 仅 origin）。
- 形态＝**纯 fast-forward**：origin/main（登记前 bba39ef）为 shz/dev 祖先；无 force、无分支创建、本地 main 不受触碰（保持 PRISTINE b22df40）。
- 体量＝dev tip 至本登记提交（115 commits / 636 files +27k/−1.3k 区间，含本登记文件）。
- 门状态如实记录＝Mimosa 完整审计已执行并封印（scanId scan-2026-11-27T01-59-14.124Z-2e2724cedeb1，seal sha256:6957518e…49056）；扫描器 runStatus=inconclusive（其设计使然：静态发现无自动 verdict）；campaign 面 11 条经 agent E1 逐条真值否定（evidence/A32/mimosa-full-audit-2026-11-27.md）；上游既有 548 条＋依赖 advisory 4 包未处置（归 owner 面，非本 push 之阻断项，依裁定推进）。
- 风险接受权威＝所有者（本登记系对裁定的执行记录，非本代理之背书）。

## 执行结果（push 后补记）

- 第一次尝试＝SSL 传输瞬断（unexpected eof，errno 0）；**副作用以服务端实证而非本地感知**：ls-remote 读得 main 仍在 bba39ef＝push 未生效，git ref 更新原子＝无部分态。重试成功。
- 重试输出＝`bba39ef..877c2a0  shz/dev -> main`（`..`＝fast-forward，非 `+` 强推；PUSH_EXIT=0）。
- 服务端复核＝`git ls-remote origin refs/heads/main` → `877c2a08b2ff7c5ff092b85ace60d15f6c1ecc9d`（与 dev tip 逐字相等，2026-11-27 实测）。
- 本地 main 未触碰（仍 PRISTINE b22df40）；upstream vinchenkov 未配置亦未指向。
- 本验证文件随下一轮推送入远端（当前远端 tip＝877c2a0）。

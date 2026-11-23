# CI 首跑发现 #1（所有者启用 fork Actions 后，run 45097992173 / PR #1；严格分"已证/未证"）

背景：fork Actions 此前从 `enabled:true` 但首跑横幅未点⇒**建库至今 run=0**；所有者 2026-11-23 浏览器点启用后，我以 PR #1（`ci/wave2-evidence`←main，**未碰 main**，base/head 均已核实=warlockedward/Culler）触发**历史首次真实 CI**。这轮的价值=把"从没在 Linux/CI 上验过的 job"第一次跑出来。

## 已证（有单变量/日志直读）
1. **Actions 门确已通**：run 对象生成、job 真调度（此前 push main 连 awaiting 都无⇒确认是横幅门，非我没配）。
2. **`deployment-linux` + `deployment-linux-amd64` 失败=先前既有、非本轮改动**：13 条 `--- FAIL` 全在 `agentsview_supervision_test.go`/`supervision_activate_test.go`/`lifecycle_test.go`（launchd 语义），这些测**无 `//go:build darwin` 标签**⇒在 Linux runner 上编译到不同平台路径、fake 激活"job is not loaded"。**单变量对照**：同一容器（alpine，纯 Go modernc sqlite）跑 `dd087c7`（A02/A25b **之前**）与 `bfa22b0` 的 deployment 测试二进制，两者对这 3 测**同样红同样因**⇒与 A02/A25b 零因果。**根因=监督平台测缺构建标签/缺 Linux 跳过，属上游既有测试对 Linux 的覆盖缺陷**（我未改这些文件；"不改他人钉测"律下不擅动，登记为候选独立票）。
3. **`opt-in-lanes`（我 A32 新增）失败=我的 job 定义缺陷**：首步 `docker_boundary` 报 `Unable to find image 'mc-prod:latest' locally / pull access denied`⇒**boundary/e2e 需预建 `mc-prod`，我的 job 没有构建步**。更根本：**`mc-prod` 仅由 `build-prod.sh` 产，其 `:68-82` 的 `systemctl --user list-units` 守卫在 headless runner（无用户 bus）必 `exit 3`**⇒A32"把 Docker 道放进 PR CI"的形状被它本该验证的首跑**证伪**（我当初标 E2 未实测、要首跑见分晓，答案是否定的）。⇒ **A32 停 CODED，不标 EVIDENCED**。

## 未证（据实标注，不推断）
4. **`application-package-linux` 失败=未归因**：错误="packaged installer did not complete the expected local check: Download and verify…"（安装器自检）。我**未改**此 job 定义，但其红**既未证明与我相关、也未证明既存**（该 job 无历史 CI 基线可比）。需一次基线 run 或所有者知晓上游此 job 在本 fork 是否曾绿。**不并入结论 2 的"既有"类**。

## 净影响（诚实）
- **A31（host-gateway add-host）/A32（runner 首跑）/A02（Linux 凭据端到端）的延付证据本轮均未获得**：opt-in-lanes 在触及这些路径前就因缺镜像失败⇒**三票维持 CODED/IN_PROGRESS，一律不升 EVIDENCED**。
- 反而获得一条更硬的战略事实：**Linux CI 的重 Docker 道在当前 job 拓扑下无法直接跑**（mc-prod 构建依赖有守卫的 build-prod + 既存监督测无 Linux 标签）——这改变"靠 Actions 一键解锁全部 Linux 延付"的乐观预期，需 A32 重做形状 + 一个监督测平台标签的独立修复。

## 候选后续（各自需正清单，非本件擅动）
- T-a：opt-in-lanes 修正形状（三选一，属 A32 自身返工）：①内联 buildx 构 mc-prod（仿 release-images，重）；②改调 update 式轻量镜像；③把 Docker 道移出 PR、改 schedule/manual（尊重"这些本就是 opt-in 重道"的原意）。
- T-b：监督平台测加 `//go:build darwin` 或 Linux 正确跳过（独立测试健康票；触他人测，需专票）。
- T-c：`application-package-linux` 归因（基线 run）。

## 最终 job 全表（run completed / conclusion=failure，收尾补全）
| 结论 | job | 定性 |
|---|---|---|
| **success** | **fast-suite-macos** | ★**权威交接门绿**（AGENTS"hand off 前必跑"的整套 check，含 mc/deployment/dashboard/resident/runner 全快套件）⇒**合入 dev 的 A02/A25b/A25a/A23 等在 Mac 权威门上健康**，这是本轮对"我的代码对不对"的核心正面信号 |
| success | security-acceptance / application-package(mac) | 既有门绿 |
| failure | deployment-linux / -amd64 | **已证=既有**（§已证2，基线容器单变量） |
| failure | opt-in-lanes | **已证=我 job 缺陷 + 形状被证伪**（§已证3） |
| failure | release-checks | **级联**：该 job=“Require every release check to succeed”聚合门，因上游 Linux job 红连带红，非独立缺陷 |
| failure | release-images | "Check the Project image with the runtime volume"（Linux 镜像校验步，我未碰；无基线→未归因，疑首跑环境） |
| failure | application-package-linux | §未证4，未归因 |
| failure | dashboard-browser / -amd64 | 真实 Chromium×真实 mc smoke 报 **34 fail + 2 errors**（含一条 Discord 主机命令卡 30004ms 超时）。**我没改 dashboard/或 smoke**；但**无法用基线 run 证既有**（此 fork 首跑无历史）⇒**未归因，初判=首跑环境性（真实浏览器/Docker/provider 依赖面），但明确不假装已证**。若后续要治 fork CI，此项须专门复跑定性 |

## 净战略事实（修正我先前对 Actions 的乐观预期）
"启用 Actions 即一键解锁全部 Linux 端到端证据"**不成立**：fork 的重 Linux/浏览器/镜像 job 首跑即红（多数既有/环境性，少数未归因），runner 现阶段跑不了需要 mc-prod/Chromium/provider 的道。⇒ **拿 A02/A31/A25 的 Linux 端到端证据，现实中只剩两条**：路 A=218 真宿主（需扩权令，见用户决策）；路 B=先修 build-prod 守卫让 runner 能造 mc-prod（敏感发布件，独立票）。快套件 macOS 绿已保证"不回归"，Linux "已跑通"仍待真域证据。

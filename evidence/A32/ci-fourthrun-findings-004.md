# CI 第四跑发现 #4（run 45370177577，dispatch 于 `ci/routing-base`=`3444103`，2026-11-24 09:58Z–10:12Z）

## 0. 使命与一句话结论
S4a（CLI 命令名面 culler→shz，59d141d）＋AM v0.2.6＋S4b/S3 裁定包（3444103）的首次宿主 CI 验证。**结论：零 job 级回归；决定性绿面达成（fast-suite-macos＋218 专用栈双硬门）；全部红面逐件归因到既有登记家族，零新红**（O1 §3 "新红＝停并上报"义务＝未触发）。

## 1. Runner 拓扑（E1）
- `fast-suite-macos`/`security-acceptance`/`application-package`/`dashboard-browser(-amd64)`/`deployment-linux(-amd64)`/`application-package-linux`/`release-images`/`release-checks`＝GitHub hosted runners（日志路径 `/home/runner/work`）。
- `opt-in-lanes`＝**218 自托管 runner**（日志 env：`DOCKER_HOST=unix:///run/shz-ci-docker.sock`＋`/home/shzci/actions-runner/…`＝A32/S0–S6 专用隔离栈在位）。

## 2. job 级结论（终态实读）
| job | 结论 | 对照 main push 基线 run 45127097061（bba39ef，findings-003 §2 表） |
|---|---|---|
| fast-suite-macos | **success** | success（本 run＝S4a 时代全 Go 套件在 macOS 宿主绿＝**S4a 非回归决定性信号**） |
| security-acceptance | success | success |
| application-package | success | success |
| opt-in-lanes | **success** | main 上 failure；本 run 的 success＝分支侧工作流（软门＋A34 N1 修复＋专用栈路由）之效果，与第三跑同形 |
| dashboard-browser(-amd64) | failure | failure（main 既有家族） |
| deployment-linux(-amd64) | failure | failure（main 既有，在册） |
| application-package-linux | failure | failure（main 既有） |
| release-images→release-checks | failure | failure（main 既有，下游） |

⇒ **本 run 相对 main 基线的 job 级差值＝零新增红**（唯一差值项 opt-in-lanes failure→success＝分支侧改进，方向为绿）。

## 3. 红面逐件归因（每行证据级别 E1＝实读日志）
### 3.1 deployment-linux（job 138362140313；amd64 变体 138362140493 FAIL 集合机器 diff＝逐测试名等同，仅 job 名栏/时间戳差）
19 FAIL 行＝13 顶层＋6 子测，全在 supervision/activation 家族：`agentsview_supervision_test.go`／`supervision_activate_test.go`／`lifecycle_test.go`。错误签名＝`supervision activation rolled back: verify com.culler.<suffix>.agentsview: job is not loaded`（"job is not loaded"×10 行）。**与首跑 findings-001 §2 同族同签名**（13 条顶层 supervision 测，根因＝监督平台测缺 `//go:build darwin` 标签/Linux 跳过＝launchd 语义在 Linux 宿主编译到平台路径后 fake 激活不载 job）＝**既有（T-b 候选独立票，findings-001 §候选2 在册）**。S4a 敏感行（shz/cli_install/argv/culler-command）在日志中＝**0 命中**。注：失败行中 `com.culler.<suffix>` 标签为 S3 单元面现值（未改），非 S4a 相关。

### 3.2 application-package-linux（job 138362140649）
`The packaged installer did not complete the expected local check: … Installed release: unknown GitHub refused access. Repair the GitHub credential, then run install.sh again.`＝安装器自检需 GitHub 凭据/发布资产（本 fork releases=0、test_releases=false）⇒ **既有**（首跑 findings-001 §4"未归因"项＋第三跑 §2 分类"main 既有"；签名逐字同族）。

### 3.3 dashboard-browser（job 138362140611；amd64 变体同族）
bun test 权威计数＝**226 pass / 32 fail / 258 tests**（首跑签名＝34 fail + 2 errors）。失败家族＝Collie/Discord/会话 smoke（"the real host commands … disconnect Discord"、"one conversation continues between Discord and the dashboard…"、26×"(unnamed)"）。失败日志中 `shz`／`S-Horizon` 命中＝**0**（S4a 零 dashboard 文件改动，smoke 断言面不涉 CLI 文本）⇒ **既有家族**（计数 34+2→32+0＝家族内环境性浮动，Discord 宿主命令 30s 超时类）。

### 3.4 release-images（job 138364073037）
"Check the Project image with the runtime volume" 步 exit-1（runtime probe JSON 输出后失败：deployment_uuid＋project 列表在场，探针后续断言败）。**既有**（首跑"未归因→疑首跑环境"＋第三跑 §2"main 既有，下游"）。

### 3.5 release-checks（job 138370240218）
`A required release check failed or remains unverified.`＝聚合门（"Require every release check to succeed"）级联，无独立缺陷（findings-001 §表"级联"同形）。

### 3.6 opt-in-lanes 软门道（218 专用栈；红被 continue-on-error 吞，job 仍 success）
- **boundary**：`FAIL mc/boundarydocker 83.784s`＝18 FAIL 行（14 顶层＋4 子测）＝**与 A34 修复后的登记基线逐测试等同**（第三跑 20 行中 N1 家族 `TestAttachmentCanaryPublisherHasNoProductionCollieRouteDockerBoundary`＋2 子测两行＝A34 已消除，本 run 不在集；余 14 顶层全部在 findings-003 §3.1 表内：T1 族 arm64 硬编码×6／T2 族 uid/VirtioFS 语义×4／N2 族产品码 arm64-only×1／N3 容器名残留×1／WikiExport EPERM×1＝登记归属不变）。
- **e2e**：`FAIL mc/e2e 2173.310s`（36min）＝25 FAIL 行（21 顶层＋4 子测）＝**与第三跑 §3.2 登记集合同形**：单根因链（resident 启动健康门＝attachment canary ⇒ `exit status 41` 本 run×25〔第三跑×22〕⇒ `timed out (1m30s) waiting for resident dispatch`×20）＋唯一异签名 `TestSpineLockDomainGuardDockerBoundary`（5.76s 非超时＝T2 族挂载定义域，findings-003 §3.2 在册）。
- **update_docker 硬门**：`ok mc/deployment 29.685s`＝**绿**（连续第 4 次独立确认：A27 双宿主×2＋第三跑 43.198s＋本 run；含 S4a 时代二进制）。
- **property 硬门**：`ok mc/property 2.557s`＝绿（第三跑 2.668s 同形）。

## 4. 新红判定（O1 §3 字面义务）
**零新红**：本 run 每一个红面都逐字落入既有登记家族（findings-001/003 表＋T-b/T1/T2/N1/N2/N3 归属）；boundary/e2e FAIL 集合相对登记基线无新增成员、无新增签名。⇒ 无上报停止项。

## 5. S4a 宿主 CI 验证结论
- 非回归信号达成：fast-suite-macos success（全 Go 套件含 S4a 提交）＋macOS application-package/security-acceptance success。
- Linux docker 面：S4a 不触 docker lane 的被测面（argv[0]/CLI 记录/操作者文本均宿主 CLI 面）；boundary/e2e 红面归因零 S4a 敏感行＝S4a 与既有红零因果。
- 218 专用栈双硬门第 4 次绿＝隔离栈稳态再确认。

## 6. 纪律执行记录
- `--log-failed` 对 in-progress run 不可用（BlobNotFound 家族）⇒ 候 run 终态后取（既知限制，findings-003 §6 同条）。
- gh jobs JSON 的 runnerName 字段空⇒runner 归属从日志 env（DOCKER_HOST/shzci 路径）判定＝E1。
- 本 run 期间本机发生 boot（~14:32）⇒ `/private/tmp` worktree 再毁（incident-002 在案，零数据损失）；CI 不受影响（runner 在 218/hosted）。

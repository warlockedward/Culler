# CI 第三跑发现 #3（run 45153594551 / job 137620872629，dispatch 于 `ci/routing-base`=sha `8bfc831`，218 自托管 runner，x86_64）

## 0. 使命与一句话结论
验证 8bfc831 引入的 continue-on-error 软门版工作流：boundary/e2e 红而不阻塞 + update_docker/property 硬门首跑。**机制验证达成**（§1 三重证据）；**归因义务履行**（§3 全表源码级闭合）；**当前证据下零项构成产品回归**，但清单外三个新类（N1–N3，§4）按 O1 §3 字面义务须所有者确认纳入归因清单后方可视为"已归因"。

## 1. 步骤表实况（全实读，UTC；job 日志 113796B / 干净版 1130 行）
| 步 | 时间窗 | API conclusion | 日志事实 |
|---|---|---|---|
| Build mc-prod | 10:10:55→10:12:00 | success | 连续第 3 次现场建图（堵旧镜像假绿风险，findings-002 §0） |
| Build mc-fake-e2e | 10:12:00→10:12:43 | success | T3 修复第 2 次真机验证 |
| boundary（软门） | 10:12:43→10:14:37 | **success**（被吞） | `FAIL	mc/boundarydocker	88.819s` + `##[error]Process completed with exit code 1.`（10:14:37.776Z）⇒ **continue-on-error 实证**：exit 1 被吞、后续步照跑 |
| e2e（软门） | 10:14:37→10:51:39 | **success**（被吞） | `FAIL	mc/e2e	2220.108s`（37min，未触 45min 上限）+ `##[error]exit code 1.`（10:51:39.221Z）⇒ 同上 |
| update_docker（硬门，首跑） | 10:51:39→10:52:31 | success | **`ok	mc/deployment	43.198s`** ⇒ 真绿 |
| property（硬门，首跑） | 10:52:31→10:52:35 | success | **`ok	mc/property	2.668s`** ⇒ 真绿 |

job `opt-in-lanes` conclusion=**success**；轮询日志 `OPT_IN_LANES_EXIT=completed/success`（本地 18:52:56，§H-7 证据门）。

## 2. run 级 job 差值（双基线对照，全实读）
| job | main push run 45127097061（bba39ef） | 上探针 run 45136188809（cffa1d9） | 本 run（8bfc831） |
|---|---|---|---|
| opt-in-lanes | failure | failure | **success** |
| fast-suite-macos | success | success | success |
| security-acceptance | success | success | success |
| application-package | success | success | success |
| dashboard-browser(-amd64) | failure | failure | failure（**main 既有**） |
| deployment-linux(-amd64) | failure | failure | failure（**main 既有**，在册） |
| application-package-linux | failure | failure | failure（**main 既有**） |
| release-images→release-checks | failure | failure | failure（**main 既有**，下游） |

⇒ **本 run 相对 main 的 job 级差值仅一项：opt-in-lanes failure→success**（软门生效本身）。8bfc831 只动 workflow 软门 + A33 票行 + O1 决定件，零产品码。

**main 既有红的家族注**：`dashboard-browser(-amd64)` 红签名 = `the run envelope names release "" but this binary is release "development": different releases`——与 N1（§4.1）**同根因家族**：G-B bind（413880a）经 `git merge-base --is-ancestor` 实证为 origin/main（bba39ef）祖先 ⇒ 该家族红在 main 上已存在，非本票引入。

## 3. 红项归因全表（O1 §3 义务；每行标注证据级别 E1=实读工件/源码，E2=推断）
### 3.1 boundary 道——20 FAIL 行（14 顶级＋6 子测）；与 run 45136188809 **集合逐字同集**（机器 diff：两侧去耗时后 `diff` 零差异）
| 测试 | 清单归属 | 失败签名（实读日志） | 根因（源码级） | 定类 |
|---|---|---|---|---|
| TestProductionImageCarriesPinnedRealAdapterRuntime | T1 在列 | `codex-linux-arm64/vendor/aarch64-unknown-linux-musl/bin/codex: not found` | 测试写死 arm64 包路径（production_image_test.go:107）；镜像双架构分派本身正确（runtime.Dockerfile 成对 manifest，findings-002 §1 已证） | T1 |
| TestProductionCodexShippedArgvIsAcceptedByThePinnedBinary | T1 在列 | 同路径 `ENOENT posix_spawn` | 同上（:149） | T1 |
| TestDoctorContainerRuntimeCapabilityProbeDockerBoundary（＋1 子测） | T1 在列 | `probe detail lacks the "native arm64" leg` | 证据串要求 arm64 腿（doctor_probe_test.go:93） | T1 |
| TestFinalUidCanWriteTheVirtioFSProjectDockerBoundary | T2 在列 | `uid 10002 could not write the operator-owned VirtioFS bind: Permission denied` | VirtioFS=Desktop 专有共享实现；Linux 原生 bind 属主呈现规则不同（landing_envelope_test.go:115） | T2 |
| TestMergeMountModesGovernInsideTheContainerDockerBoundary | T2 在列 | `missing SOURCE_RW` | 同上（:321） | T2 |
| TestClaudeAgentUsesMCAsTheModelUserDockerBoundary | **清单外→N2 家族** | `the endpoint saw 0 requests`（agent_user_test.go:523 触发，断言 :460-461） | **产品代码** arm64-only：`runner/agent-runner/adapters/claude.ts:37-38` CLAUDE_BIN 写死 `claude-agent-sdk-linux-arm64/claude`（注释自认 "The runtime holds the Linux arm64 build only."）；amd64 镜像按 `package.linux-x64.json` 装 `-linux-x64` 包 ⇒ SDK spawn 即败 ⇒ 0 请求。`codex.ts:30-31` 同款。镜像烘焙副本与源逐字节同（子代理 diff 实证） | T1 族（产品代码层，见 §4.2） |
| TestAgentProfileClaudeModelDockerBoundary（＋2 子测） | **清单外→N2 族** | harness.ts:49 宿主 bun `JSON Parse error: Unexpected EOF` | `harnessVersions()`（native-harness.ts:12）读 **arm64 manifest**（runner/agent-runner/package.json）的包名集 ⇒ pins 探针 `require` 两个 arm64-only 包名（harness.ts:46-49）⇒ amd64 镜像 `/app/node_modules` 无之 ⇒ 探针必败。"Unexpected EOF"=Bun 对空输入 JSON.parse 的专属报错（子代理本机 Bun 实测复现同款文本）⇒ 指向 :49 拿到空 stdout；该精确子机制标 **E2**（需 218 运行时日志钉死），平台根因本身源码闭环 | T1 族（测试绑定 arm64 manifest） |
| TestAgentProfileCodexModelDockerBoundary（＋2 子测） | 同上 | 同上 | 同上 | T1 族 |
| TestAttachmentCanaryWarmComposedBoundaryDockerBoundary | **清单外→T2 族** | `warm uid-10002 attachment read failed: exit status 41`（attachment_canary_test.go:65） | 宿主侧 `published`/分片目录 operator-owned **0700**（attachment_publisher.go:766-804 强制）；容器 uid-10002 经 Linux 原生 bind 穿越 ⇒ EACCES ⇒ 2.5s（100×25ms）重试读不到 ⇒ 容器内脚本 `process.exit(41)`（attachment_canary_prod.go:601-625，exit(41) 在 :613）。macOS 绿 = VirtioFS 属主折叠语义（本仓库 ownership_unix_test.go:16-20 明文记录 "host inode is created with the sharing user's ownership, NOT the container uid"） | T2 族（uid 呈现语义）；**兼记 Linux 生产域隐患**（见 §4.2 末） |
| TestAttachmentCanaryPublisherHasNoProductionCollieRouteDockerBoundary（＋1 子测） | **清单外→N1** | `the run envelope names release "" but this binary is release "8bfc831…"：different releases`（attachment_canary_test.go:133） | **A13 G-B bind（413880a，verbs.go:309-321）× 陈旧 fixture**：子测在 :126 写**无 `release_build_id` 字段**的 collie envelope；CI 镜像从被测提交现场构建并烙 release=<sha>（build-prod.sh:100）⇒ bind 在场 ⇒ 拒语被 release 检查抢先；macOS 镜像旧于 413880a（**E2**：由"macOS 绿"前提反推的唯一源码一致解释）无此检查 ⇒ 报 "host scope only" ⇒ 绿。fixture 遗漏实证：4fa3863（"Give the agent run.json fixtures a matched-pair identity"，自认 "added to every agent envelope fixture"）的 --stat 只含 `mc/cmd/mc/` 下 7 文件，**不含** boundarydocker/attachment_canary_test.go；该文件最后改动=9dbc043（2026-11-14，早于 413880a）。**安全性质未失**：envelope 仍被拒、无 host-scope 泄漏；生产 collie envelope 恒带身份（resident/src/effects.ts:637 `requiredReleaseBuildId`） | 新类 N1（身份语义×fixture 卫生，非平台问题，见 §4.1） |
| TestAttachmentCanaryRecoversExactInterruptedAttemptDockerBoundary | **清单外→N3** | `container name … already in use by container "96d3ddcb068c…"`（attachment_canary_test.go:178） | **环境残留**：generation 写死 32×'a'（:144-145）⇒ 确定性容器名；`docker create --name` 前**无 `docker rm -f`**（对照 precedent：landing_envelope_test.go:152 有）；前次 run 在本 run 的 warm-read 失败（T2 族）后中断、清理未跑到 ⇒ fixture 容器残留在共享 daemon（218 实读：96d3ddcb068c Up 3h，标签 `mc.attachment-canary=aaa…`+镜像 revision=4f4fcfd=二跑现场件） | 新类 N3（runner 状态残留，见 §4.3） |
| TestCollieOperatorTransportDockerBoundary | **清单外→T1 族** | `/opt/culler-runtime/commands/mc: 1: ELF: not found`（operator_transport_test.go:145） | 测试**自身** `GOOS=linux GOARCH=arm64` 硬编码交叉编译 dispatch shim+mc-real（:26-28）盖掉镜像内命令路径（:94-110）⇒ amd64 引擎 execve 得 ENOEXEC ⇒ shell 回退把 ELF 首行字节当脚本解释（dash 报错格式；"哪一个 shell 执行了回退"标 **E2**，不影响根因）。镜像内该路径本是 ELF（build-prod.sh:103 编 mc_dispatch.go ⇒ runtime.Dockerfile:108 COPY） | T1 族（测试自编译参数） |
| TestA21AgentUserSpineReadsOnProductionLayout | **清单外→T1 族** | `/mnt/mc: line 1: syntax error: unexpected ";;"`（spine_mount_a21_test.go:103） | 同上（:36-38 `GOARCH=arm64`）：arm64 mc ELF 被 ro bind 为 /mnt/mc，amd64 alpine 的 ash 执行它 ⇒ ENOEXEC ⇒ 按脚本解释 ELF 字节 ⇒ exit 2 | T1 族（测试自编译参数） |
| TestWikiExportRunsInTheRunnerImageDockerBoundary | **清单外→T2 族** | `chmod …/output/client-lib.js: operation not permitted`（wiki_export_test.go:59） | **宿主侧** `os.Chmod(path,0600)`（mc/wiki/export.go:441）作用于容器 uid-10002 写出的文件；Linux 原生 bind 保 uid-10002 属主 ⇒ 非 root CI 用户 EPERM（EPERM 本身即证明非 root）；macOS = VirtioFS 属主折叠 ⇒ 宿主属 operator ⇒ 绿。**产品注释 export.go:235-236 把"容器用户不拥有宿主文件"当作跨平台前提——该前提在 Linux 原生部署下不成立** | T2 族（uid 语义）＋产品域注记（见 §4.2 末） |

### 3.2 e2e 道——25 FAIL 行（21 顶级＋4 子测）；**Linux 首跑、无已知红基线**（前两次 run 的 e2e 都因 boundary 红被跳）
- **共同根因定量证实**：e2e 区域日志中 `warm uid-10002 attachment read failed: exit status 41` 出现 **22 次**；`timed out (1m30s) waiting for resident dispatch` 出现 **20 次**。
- **链路（实读）**：resident 启动健康检查 = 同一 attachment canary（resident/src/main.ts:146 `attachment confinement canary refused (exit …)`、:598 `resident: attachment confinement unhealthy:`）⇒ canary 红（机制同 §3.1 warm 行，T2 族 uid 语义）⇒ resident 永不健康 ⇒ **所有**等 dispatch 的管道测试在 90s 处超时 ⇒ **21 顶级红中 20 个 = 此单一根因的下游**（非 20 个独立缺陷）。
- **唯一异签名**：`TestSpineLockDomainGuardDockerBoundary`（8.58s，非超时）= **T2 族挂载定义域**（实读 e2e_test.go:340-404）：guard 的拒绝谓词建立于"Desktop 的 bind 挂载非 block-device-backed（virtiofs/fakeowner 两种呈现）"前提（:340-344 注释明文）；原生 ext4 上 host 目录 bind **就是** block-device-backed ⇒ guard 正确放行 ⇒ 继续开 DB ⇒ 空目录无文件 ⇒ `unable to open database file (14)`。轮八 Q7 重钉版本（断言拒绝结果属性而非 fstype 名）的定义域边界在 Linux 上由此显形——属 A33 参数化范围（"按宿主共享实现分派、各按正确期望断言"）。

## 4. 清单外新类（O1 §3：须所有者确认纳入归因清单）
### N1 · A13 release-bind × 陈旧 fixture（1 个 boundary 测试 ＋ dashboard-browser 道 main 既有红同家族）
- 事实链全 E1：413880a 引入 loader 侧 G-B bind（无 `release_build_id` 的 agent envelope 被拒）；4fa3863 更新 7 个 cmd/mc fixture（自认 "every agent envelope fixture"）漏 boundarydocker/attachment_canary_test.go:126（该文件 git log 实证最后改动=9dbc043/2026-11-14）；CI 现场镜像烙 release=<sha>（build-prod.sh:100）⇒ 拒语从 "host scope only" 变为 "different releases"。
- **安全性质未失**：envelope 仍被拒（拒绝发生得更早，不是更晚）；无 host-scope 泄漏路径产生。生产路径恒带身份（resident/src/effects.ts:637）⇒ fixture 纯陈旧。
- 修复方向（候裁，执行侧不擅动边界测试）：fixture 补动态读取的镜像 release-id（同型 precedent：update_checkpoint_docker_test.go:65 `docker run … /bin/cat RuntimeRoot+"/release-id"`）；或裁定检查次序。触边界测试 ⇒ 需明确授权（转写≠放宽纪律）。
### N2 · 产品代码 arm64-only harness（amd64 生产镜像的 claude/codex agent harness 实际不可用）
- `claude.ts:37-38` / `codex.ts:30-31` 包路径 arm64-only＋注释自认 "holds the Linux arm64 build only"；镜像按架构装 x64 包 ⇒ 路径错配。产品部署域=一台 Mac（AGENTS.md 首段）⇒ arm64-only 与当前域一致，**今日不构成缺陷**；若 Linux 部署成为目标则升级为产品缺陷。
- **A33 票面影响**：A33 现票面="参数化测试的 T1/T2 宿主假设"；N2 表明跨平台化面延伸到**产品代码层**（harness 双架构适配）⇒ A33 票面需扩或另立产品面票（候裁）。
- 末段产品域注记：warm canary（§3.1）与 wiki export（§3.1）两项同证**"Linux 原生部署下 uid-10002 读不了 operator-owned 0700 目录/收不了权"是真实产品域问题**（今日域=Mac 故不激活）——与 N2 一并作为"若部署域扩展到 Linux"的前置缺陷登记。
### N3 · 环境残留类（确定性容器名＋无幂等前置清理）
- `mc-attachment-canary-aaaa…` 写死名（:144-145）＋ create 前无 `docker rm -f` ⇒ 任何前次中断都在共享 daemon 上留下"下次必红"的雷。修复方向=测试侧幂等（platform-independent）；本次 run 的残留已清（§7 前后差）。
- **与隔离方案的联动**：独立 data-root（S2）使新栈天生零容器 ⇒ N3 类雷在隔离后结构性消失；但测试侧幂等仍建议做（隔离不消除"同栈内前次 run 残留"的可能）。

## 5. 判定（诚实，不越级）
- **软门机制**：验证达成——O1 §4 的设计（exit-1 被吞＋后续步照跑＋硬门真跑）与实际测量一致。
- **硬门首跑**：update_docker `ok 43.198s`（A27 的 218 绿第 3 次独立确认）＋ property `ok 2.668s` ⇒ **A32 的 Linux 有效信号核心达成**。
- **A32 票面验收**：① release.yml 唯一必改 ✓（8bfc831 --stat：release.yml＋DEV-BASELINE＋O1 决定件，零产品码）② 三守卫零转写 ✓（8bfc831 未触守卫测试）③ 自托管路由试跑 ✓（本 run 即实跑数据）④ job 绿＋fast-suite-macos 绿（含 dashboard 道三守卫测试，AGENTS.md Checks 节定义）＋发布枚举零漂移 ✓ ⇒ **A32=EVIDENCED**（登记按票面验收文本；EVIDENCED≠红道绿宣称，红道实况全部在 §3 表）。
- **真产品回归**：当前证据下零项。N1–N3 待所有者认类（认类后软门归因清单更新为 T1/T2/N1/N2/N3）。
- **fast-suite-macos=success**：T1/T2 断言在其定义域 macOS 侧仍被真跑且绿（O1 §3 边界条款达成）。
- **未验证项（不藏）**：N1 的"macOS 镜像旧于 413880a"是 E2 反推（未重开 macOS 侧镜像构建记录）；AgentProfile 两测试的精确 EOF 子机制（E2，需 218 运行时日志）；operator_transport 的 ENOEXEC 回退执行者（E2，不影响根因）。

## 6. 运行环境事实（供后续 run 对照）
- 本 run 在**共享主 daemon**（/var/run/docker.sock，data-root=/data-shunhong/docker_data）上跑——S3 切换前的预期态（隔离方案 §0/§4 在册）；跑时实测与 ≥8 套他人项目容器并存（18:44 本地实读 62 容器含 `mc-e2e-helper-*`）。
- 218 残留容器 `mc-attachment-canary-aaaa…`（96d3ddcb068c）= 二跑遗留，处置见 §7。
- 证据件：上 run 日志 /tmp/oil3.log（99507B）；本 run 全 job 日志 /tmp/shz-run3-job.log（113796B）/ 干净版 /tmp/shz-run3-clean.log（1130 行）；机器比对件 /tmp/fails-boundary-run3.txt vs /tmp/fails-boundary-run4.txt（20=20 同集）；轮询日志 /tmp/shz-run-45153594551-poll.log。
- 下载注意（上轮 grep 歪因已修）：GitHub job 日志行带时间戳前缀，FAIL 行匹配用 `Z +--- FAIL:` 而非 `^\s*--- FAIL`；进行中 job 的日志 blob 不可下载（404 BlobNotFound）；`gh api …/logs` 需 `--allow-escape-sequences`。

## 7. 残留容器处置（前后差，218 实读）
- **前态**（删除前实读）：主 daemon `docker ps -q | wc -l`=**61**；`docker ps -a --filter name=mc-attachment-canary` = `96d3ddcb068c mc-attachment-canary-aaaa… Up 3 hours`，标签 `mc.attachment-canary=aaaaaaaa…` ＋ `org.opencontainers.image.revision=4f4fcfd…`（=二跑现场镜像）。
- **处置**：`docker rm -f mc-attachment-canary-aaaaaaaa…`（首次 ssh 断连于执行前，重连核实状态未变后重试；`docker rm -f` 幂等，无部分成功风险）⇒ `RM_OK`。
- **后态**（同命令族实读）：`docker ps -q | wc -l`=**60**；filter 结果**空**。前后差=恰好移除该 1 容器，主 daemon 其余容器零扰动。

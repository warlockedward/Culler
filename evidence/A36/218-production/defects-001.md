# 218 生产部署轮缺陷登记 · 001（轮六十三）

## D-1 · CLI PATH 验证硬依赖 /bin/zsh＝Linux 无 zsh 时安装器永久拒绝（真机抓取）

- 症状：`install-source.sh --activate` 走到 `mc __onboard home` 拒语 `A new zsh shell cannot resolve shz. Check its shell configuration.`（domain-rejection）。218 实况＝Ubuntu 22.04 root shell=bash，无 zsh。
- 根因：`mc/deployment/cli_install.go` 的 `installCLIPath` 对四个壳环境条目逐一 `verify`（`verifyCLIShell` = 真实 spawn `/bin/zsh -ic/-lic`）。二进制不存在时 spawn 失败＝verify 失败＝编辑循环反而往 `~/.zshrc`/`~/.zprofile` 写无人读的 PATH 导出，终验循环再失败→拒。macOS 默认面（zsh+bash 俱在）掩盖了该假设。
- 定性＝产品缺陷（非品牌面，非本部署配置可绕）：Linux 生产形态下向导 home 段不可达＝A25/A03 系 Linux 端到端前置。既有测试全用 stub verify（读 rc 文件、不 spawn 真壳）＝零覆盖该行为＝缺陷逃逸原因（如实）。
- 修复（最小面）：`verifyCLIShell` 开头对壳二进制做存在性判定，缺失＝视为通过（该壳家族不在本机＝不是操作者的登录环境；不 spawn、不写其 rc）。bash 面验证强度不变；zsh 在场（macOS）行为逐字不变。附带收益：Linux 不再产生无意义 .zshrc。
- 回归测试：新增直接单测＝不存在的壳路径→verifyCLIShell 返回 nil；真实 `/bin/sh`（或宿主必有壳）spawn 失败面保持错误语义。
- 影响面：`mc/deployment`（生产码，已在 host_release 清单）；无 wire/状态机/schema 触面；无文档面变更（拒语措辞不动）。
- 验证：`cd mc && ./test.sh ./deployment/`＋gofmt/vet＋全道后台跑；218 拉新提交重跑安装器第三轮过闸＝真机终证。

## 处置与授权边界

- 本修复属 owner「按建议顺序执行」部署轮内阻塞缺陷（预案 §3 步骤 2 的安装路径自身）＝执行域最小修复，不改任何已裁不变量。
- 拒语文案未动＝AGENTS 终端设置对抗评审门不触发（非 setup 文案变更）。

## D-1 验证记录（2026-11-27）

- 定向：`./test.sh ./deployment/ -run 'TestVerifyCLIShell|TestCLIPath'`＝ok（新 3 面单测通过：缺失壳跳过/在场壳不解析仍拒/解析成功通过）。
- 静态：gofmt 清零＋`go vet ./deployment/` exit 0。
- **全道**：`./mc/check.sh` → `MC_CHECK_EXIT=0`，29 包 ok、零 FAIL、零 UNVERIFIED（含新测试行）。
- 真机终证＝218 安装器第 4/5 轮 `home` 段过闸（spine provisioned＝缺陷路径实战通过）。

## D-2 · 入站附件树目录 0700＝Linux 容器无法读取（真机抓取，激活闸拦截）

- 症状：`shz activate` 于 supervision 段回滚——`resident produced no successful ... tick within 90 seconds`；resident.stderr 反复 `attachment confinement canary refused (exit 41): warm uid-1002 attachment read failed`。
- 根因：容器把 `.../in/published` bind 成 `/mc/attachments/in`，以 uid 10002 读已知摘要路径；但该链各级＝`openOrCreateAttachmentDirectory` 建的 **0700 root 目录**（attachment_publisher.go），容器（"other"）无 x 位 → 逐级 EACCES → 读循环 2.5s 耗尽 exit 41。生产 Collie 容器同 `-v .../in/published:/mc/attachments/in:ro`（effects.ts:715）＝**真实入站附件在 Linux 同样读不到**，canary 非误报。
- 逃逸原因＝macOS CI 快线：Docker Desktop virtiofs 文件共享无视 unix 权限位，恒绿；Linux 部署道（deployment-linux）既有红（H-3）未覆盖此格＝未验证继承面现形。与 D-1（硬编码 /bin/zsh）同族＝"宿主平台假设"类缺陷。
- 修复（owner 甲案，2026-11-27）：**入站读取链平台化**——Linux 上 `in/published`+`sha256`+shard 三级 = **0711**（execute-only：容器可穿越已知摘要路径、不可列目录），macOS 保持 **0700**。published 文件仍 0444；outbound 链与 `/mc/private` 闸、宿主侧父链（home/attachments/session/in）一律 owner-only 0700（canary 双向断言"private gate EACCES""outbound EACCES"原样）。
  - 为何平台化（首轮教训）：首版 dafb3a6 硬编 0711 全域，真机暴露两层不一致——① canary 的 `trustAttachmentCanaryDirs`→`boundary.TrustHomeDir` 谓词按"无 group/other 位"判，把 0711 拒成 `mode 0711 grants group or other access`（错误从"读不到 exit41"变"信任被拒"＝修复生效但触发下游严格闸）；② 测试 `TestTrustHomeDirTraversalAllowsExecuteOnlyChain` 在 macOS 上连 0701/0711 都被 `trustedNoGrantingACL` 拒。根因再认识＝**权限位强制本是 Linux 特有**（macOS virtiofs 无视位＝bug 只在 218 现形的原委），故放宽亦须只在 Linux，macOS 保持严格 0700＝不制造本地不一致、不扰动 CI 快线。
  - 实现面：`verbs` 加 `inboundAttachmentReadMode()`（`runtime.GOOS=="linux"→0711` 否则 0700）＋导出 `AttachmentInboundReadMode()` 供发布者/canary/测试共用同一真值；`openOrCreateAttachmentDirectory`/`requireAttachmentDirectory` 加 mode 参数＋**建后 Fchmod**（mkdir 受 umask 影响，部署 umask 077 会削 o+x＝不显式 chmod 则白改）；`verifyPublishedAttachment` 按层级传期望 mode；canary 独立 `os.Mkdir(shard)` 改用 `AttachmentInboundReadMode()`；shard 创建四处入站化。
  - boundary 新谓词：`TrustHomeDirTraversal`（＝`TrustHomeDir` 但容忍 group/other 的 **execute** 位，仍禁 read/write、仍拒 symlink 与非 owner、仍走 macOS `trustedNoGrantingACL`）——canary 信任链改调它，保留完整 ACL 语义（不重造、不丢 macOS 原生 ACL 检测＝否则＝真弱化，被 `...RefusesGrantingNativeACL` 测试逮住）。
  - 治理门登记：新增导出 API 触发 `codes_test` 两道门＝导出错误返回 API 计数 14→15 ＋"每个公开 mount rejector 需有编码拒绝 fixture"→补 `TrustHomeDirTraversal` map 项与 tests 表项（复用 `missing` 触发 `CodeAllowlistUntrusted`）。
  - 测试转写：树走查/canary 预建改用 `AttachmentInboundReadMode()` 断言（Linux 得 0711、macOS 得 0700，跨平台各自成立）；新增**入站加宽（0755 含 group/other read）必拒**负向量＝锁死"可穿越不可列"边界不随改漂移；`...RefusesGrantingNativeACL`（macOS chmod +a 授予 list）红→改后**必须仍拒**（故走完整 ACL 检测的新谓词而非纯 mode 判断）。
- 验证：`go build` 0／`go vet` 0／gofmt 净；`mc/boundary` ok（含新谓词向量＋codes_test 双门）；`mc/verbs` ok；曾红两测定向复跑 ok；**全道 `./mc/check.sh` → MC_CHECK_EXIT=0，29 包 ok、零 FAIL/零 UNVERIFIED/零 gofmt**；提交＝见本轮（前置 KEY-CLEAN 硬门＝全树 grep 无 key 字面量方 add）；深度扫描封号见 AM v0.2.46（**560＝基线 559+1**，增量＝C11 http 客户端引入的 `provider-proxy→provider-transport:282` SSRF 面，已代码路径级定性与缓解，残余登记不吞并）；**真机终证＝218 重建后 canary 过、resident tick 成、supervision 激活、F1 双臂跑通（r12 段，Linux 平台）**。
- 等级如实：canary 激活前拦截＝安全护栏按设计工作（它先拦下"读不到"的 Linux 生产故障，又拦下我首版"0711 未过信任闸"的半成品），非误报；我改的是被护栏揭示的缺陷本体。macOS 与 Linux 两台必须各自成立（owner 明令），已用 `AttachmentInboundReadMode()` 单一真值保证代码/测试/canary 三处同步。

## 环境性测试脆弱（登记非本轮引入，A35 全道唯一红）

`TestReleaseSkillWorksForExternalAgentWithoutSourceOrDevelopmentTools` 在全道并发下于 `agent_skill_external_test.go:54` 的 `mise exec -- bun -p process.execPath` 子进程返回 exit 1（耗时 120s）；隔离重跑同树（含 A35 改动）`ok`（但该步本身耗时 173s）。判定＝该测试对宿主机 mise/bun 外部工具的固有脆弱＋并发放大，**不触 attachment/canary/broker 任何代码路径**，非 A35 回归。归 A32 上游既有面候命清单式处理（不借 A35 域顺手改他人断言）；如后续要根治＝把该 fixture 的 bun 解析改为预取常量或加超时余量，属独立测试卫生票。
**复验**：同树重跑全道 → `MC_CHECK_EXIT=0`、29 包 ok＝该 bun 步并发脆弱自愈，坐实"非 A35 回归"判定（提交 4e641fd）。

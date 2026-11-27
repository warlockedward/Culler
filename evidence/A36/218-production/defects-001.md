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
- 修复（owner 甲案，2026-11-27）：入站读取链 `in/published`+`sha256`+shard 三级 → **0711**（execute-only，容器可穿越已知路径、不可列目录＝防越目录界性质保住）；published 文件仍 0444；outbound 链与 `/mc/private` 闸、宿主侧父链（home/attachments/session/in）**一律保持 0700**（canary 双向断言中"private gate EACCES"与"outbound EACCES"原样）。
  - 实现面：`openOrCreateAttachmentDirectory`/`requireAttachmentDirectory` 加 mode 参数＋**建后 Fchmod**（mkdir 受 umask 影响，部署 umask 077 会削 o+x＝不显式 chmod 则白改）；`verifyPublishedAttachment` 按层级传期望 mode；canary `os.Mkdir(shard)`→0711＋`os.Chmod`；shard 创建四处入站化。
  - 测试转写：树走查断言改按路径期望 mode（in/published+sha256=0711 其余 0700）；新增**入站加宽（0755）必拒**负向量＝锁死"可读不可列"边界不随改漂移。
- 验证：`go build` 0／vet（修前暴露测试签名漏改已补）／verbs 551s ok；全道 MC_CHECK_EXIT 见下笔；**真机终证＝218 重建 mc/mc-prod 后 canary 过、resident tick 成、supervision 激活、F1 双臂跑通**。
- 等级如实：canary 激活前拦截＝安全护栏按设计工作（它拦下了一个真实 Linux 生产故障），非护栏误报；我改的是被护栏揭示的缺陷本体。

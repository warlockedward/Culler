# A35 设计件 · diff-plan-001 — Linux 原生 bind 下的容器身份移植（修到底）

日期 2026-11-27（轮六十三）。立票授权＝owner「激活 A35，修到底」（＝L2/L3 安全边界移植新令，兑现票面"激活需所有者新令"）。范围＝A35 票面预枚举的三缺陷，非开放式。授权＝实施进码；但**每碰一次宿主防火墙（双臂 gateway-allow）仍逐字呈报遵 R-a**，不在此令内自动扩。

## §0 为什么这是"补齐主平台"而非"加功能"

owner 愿景（AGENTS 首段）＝**primary deploy = x86_64 Linux**。`main.ts:604`：canary 不过 → resident `process.exit(2)`（fail-closed 硬门，非可降级项）。resident = dispatch tick + agent 生命周期 + provider-proxy + dashboard 的宿主。⇒ **canary 不过＝Linux 上 agent 一个都跑不起来、spine 空转、dashboard 无监听**＝主平台运行时全瘫。修 A35 ＝让 Linux 与 macOS 对等，不是可选增强。

## §1 根因链（全部 E1，探针直证）

Linux 原生 bind 不重映射、不容"镜像内 owner"被宿主覆盖；macOS Docker Desktop virtiofs 无视 unix owner/mode（＝缺陷只在 218 现形、CI 快线恒绿的原因）。同一类根因三面：

- **②入站**（published 链 root:0700 → uid-10002 读不到，warm canary exit-41）＝**已由 D-2 修**（8bf944b）：入站读链 Linux 0711 execute-only／macOS 0700，单一真值 `verbs.AttachmentInboundReadMode()`。
- **②出站**（本票当前卡点）＝**探针直证**：容器内 publisher（fsuid/euid 10001）打开 bind 自宿主 `root:0700` 的 out 根 → `open fixed attachment canary outbound root: permission denied`。探针（218，`--network none --user 10001:10001` 挂 mc-prod）：
  - 宿主 `0:0 700` 源 → 容器 uid-10001 `OPEN-ERR:EACCES WRITE-ERR:EACCES`（逐字复现）
  - 宿主 `10001:10001 700` 源 → 容器 uid-10001 `OPEN-OK WRITE-OK`（root 宿主侧仍 override 回读）
- 设计本就期望 out＝`10001:10001:0700`（canary `attachment_canary_prod.go:671` 私有门断言＋镜像 Dockerfile `install -d -o 10001 -g 10001 -m 0700 /mc/private/attachments/out` 双证）。**bug＝`PrepareAttachmentTree` 用 `openOrCreateAttachmentDirectory(..., attachmentOwnerMode=0700)` 建成创建者(root)所有，bind 时遮蔽了镜像内 10001 所有**。生产 Collie 容器同形（effects.ts:716 `-v ${outbound}:/mc/private/attachments/out`）＝**Linux 出站附件投递同样坏**，canary 只是提前在激活前拦住。
- **①amd64 harness**＝镜像内 `adapters/claude.ts:37`/`codex.ts:30` 自认"holds the Linux arm64 build only"⇒ amd64 镜像 harness 缺。
- **③wiki export Chmod**＝`mc/wiki/export.go:441` 宿主 `os.Chmod` 在原生 bind 下 EPERM（:235 前提"容器用户不拥有宿主文件"在 Linux 不成立）。

## §2 出站修复设计（O1：对齐所有权，不放宽 access）

把 out 子树（out, out/staging, out/journal, out/published, out/published/sha256）在 **Linux** 上建成 **owner=容器写者 uid（10001:10001）、mode 0700**；**macOS 保持 root:0700 逐字不变**（virtiofs 无视，零 CI 扰动）。

关键不变量守护（这是安全边界，红线）：
1. **不放宽 access**：仍是 0700（owner rwx、group/other 无位）。root 宿主读经 DAC override，无需给 other 任何位。
2. **owner 只能是固定常量 10001**，不可任意：见 §3 验证耦合。
3. 入站链维持 D-2（root 写、0711 供容器穿越）＝方向正确性对称：入站宿主写→root 所有 + o+x；出站容器写→10001 所有 + 0700。

### §3 必处理的验证耦合（否则改一处坏一处）

`requireAttachmentDirectory` 现断言 `uint32(stat.Mode&0o777)==mode && int(stat.Uid)==os.Getuid()`（root）。若 out 改 10001 所有，重复 `PrepareAttachmentTree`（复用既有树）会在 open 时拒 `uid!=root`。故 `openOrCreateAttachmentDirectory`/`requireAttachmentDirectory` 需增**期望 owner uid 参数**（Linux 出站传 10001、其余传 root=Getuid()）：
- 新 `AttachmentOutboundUID() (int, bool)`＝Linux→(10001,true)，macOS→(0,false 表示 root/不特殊)。
- `openOrCreateAttachmentDirectory(parent, name, mode, ownerUID)`：ownerUID==Getuid() 走现逻辑；否则建后 `fchown(fd, ownerUID, ownerUID)`（root 有 CAP_CHOWN）且验证接受 `uid==ownerUID`。
- `verifyPublishedAttachment`（bytes.go）只走**入站** published 链（root 所有）＝不受影响；无需改动（出站不经此校验）。
- 幂等：复用树时若已 10001 所有 → fchown no-op；若历史遗留 root（升级路径）→ 本次 fchown 归正。
- macOS：ownerUID 传 Getuid()＝root＝行为与今完全一致。

## §4 正清单（进码蓝本；行位以届时代码为准）

`mc/verbs/attachment_publisher.go`（AttachmentOutboundUID＋open/require 增 ownerUID 参＋out 子树 Linux 归 10001；in 链保持 D-2）；`mc/verbs/attachment_canary.go`（若 canary 树构造需同步）；`mc/cmd/mc/attachment_canary_prod.go`（私有门断言已容 10001，核 publish 步过即可）；**①** `runner/agent-runner/adapters/claude.ts`+`codex.ts`（去 arm64-only 硬钉，按 `bun`/`node` 架构选择或支持 amd64；属 runner 域，触 host_release/runner_release 清单义务）；**③** `mc/wiki/export.go`（宿主 Chmod 改"尽力 chown/权限、EPERM 非致命"或按容器用户所有权模型，勿静默吞错）。测试＝§5。

## §5 验收（票面 F1，Linux 硬门）

- **②出站**：`openOrCreateAttachmentDirectory` Linux 单测＝建后 Lstat uid==10001 ∧ mode==0700 ∧ 复调用幂等；**负例**＝owner 非 10001/非 root 必拒、给 group/other 位必拒（锁死不放宽）；218 真机＝`mc __attachment-canary` 过（入站+出站全绿）→ `shz --activate` supervision ok → resident tick 在 → dashboard 7333 监听。
- **①**：Linux 净机 `--confirm-live-spend` 一次真实 agent 回合（本地 new-api）跑通＝harness 在 amd64 可用。
- **③**：wiki export 在 Linux 真机无 EPERM（或降级语义明确）＋单测。
- 全道 `MC_CHECK_EXIT=0`＋`gofmt/vet` 各 tag＋**Linux amd64 交叉 build/vet**；`boundary`/`verbs` 包绿（含 D-2 已有）；blcheck 无违规；扫描无新面。
- **双臂随其后**：A36-F1（代理在场→gateway-allow scoped [逐字呈报 R-a]→真容器 dial 往返→revoke 零残留）＋A31-F1③。

## §6 风险与回滚

- 风险＝受信所有权模型放宽（root-only → root-or-固定10001，仅 Linux 仅出站）；缓解＝owner 常量化不可任意＋group/other 永拒＋负向量＋canary 双向断言原样。
- macOS 零改（平台门控）＝不引入回归面。
- 回滚＝出站 owner 改回 root＝Linux 监督回退到卡 canary（即今日态），无数据损失；①③ 各自独立可退。

## §7 边界

本票不碰：宿主防火墙（双臂 gateway-allow 属 R-a 另呈）、spine schema、wire 形状、CI 工作流、租户任何资源。激活即实施进码；双臂的 ufw 单条规则在 §5 双臂步骤届时逐字请示。

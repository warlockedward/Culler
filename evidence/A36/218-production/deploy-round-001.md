# 218 生产部署轮 · 真机账 001（轮六十三，2026-11-27）

依据＝owner「按建议顺序执行（部署→E→F0a）＋零干扰宿主租户（端口/网络/镜像/容器）＋磁盘走 /data-shunhong」＋后续「C11 进码授权，边界照 §2，验收用本地 new-api」＋「选 B、不要影响宿主任何应用与服务含网络与容器」。

## 1. 已达面（全部 E1 真机，Linux x86_64 生产形态）

| 向导段 | 结果 | 意义 |
|---|---|---|
| preflight | ok | linux/amd64 识别 |
| home | ok/done | **spine 建成**（deployment 7b75278f，schema 身份 4786047e…，helper 实算）；runner/host release ok；**CLI 装上**（/root/.local/bin/mc、shz，D-1 zsh 修复实战生效）|
| runtime-auth | ok | **C11 配置链在产**（newapi 过全部缺省拒）＋**key 经 A02 宿主凭据库录入**（refresh-grants/newapi.json；录入走 C11 新明文登记目的地活连 new-api 验 key 通过＝C11 连接面真机首用）；"no replay token spent"＝零外发 |
| routing / container / project / tunables / surfaces | ok | routing.md 生；**native 信封＋内核能力探针健康**；Project culler 采纳、sandbox v3 绑定、VISION.md 就位；tunable 一致；dashboard 127.0.0.1:7333 令牌面策略验证 |
| **supervision** | **fail（回滚）** | 见 §2 |

构建面（非激活）：mc 二进制＋mc-prod/mc-project 镜像 native amd64（release-id=8bf944b）＋agentsview 0.47.0＋Playwright 浏览器双 smoke ok。

## 2. 卡点根因链（证据，非猜测）

`supervision`＝`--activate` 起 systemd 托管 resident → mc 探针"90s 无 release-bound tick"→回滚。下钻：

1. **resident 起不来＝canary 健康闸崩溃**：`main.ts:596 proveAttachmentConfinement` catch 后 **`process.exit(2)`（:604）**＝canary 不过则 resident **崩溃退出**（非带病运行）。
2. **canary 第一层＝入站读不到（D-2）**：`in/published` 链 root:0700，容器 uid-10002 无 x 位→逐级 EACCES→warm read exit-41。**已修**（owner 甲案：入站读链 Linux→0711 execute-only，macOS 保持 0700；platform 化 `verbs.AttachmentInboundReadMode()`＋建后 Fchmod 抗 umask；`boundary.TrustHomeDirTraversal` 新谓词容忍 execute 仍禁 r/w 且保留 macOS ACL 检测；codes_test 双治理门登记）。全道 MC_CHECK_EXIT=0、macOS 与 **Linux amd64 交叉 build/vet 双净**；扫描 560（基线 559+1＝C11 http 客户端引入的 provider-proxy→transport SSRF 面，已代码路径级定性、残余不吞并）。提交 8bf944b。
3. **canary 第二层（当前卡点）＝出站 publisher 宿主所有权被 bind 遮蔽**：修好入站后 canary 进到出站，报 `open fixed attachment canary outbound root: permission denied`。根因＝镜像 Dockerfile `install -d -o 10001 -m 0700 /mc/private/attachments/out`（镜像内 owner=10001），但运行时该路径被 **bind-mount 覆盖**为宿主侧 `PrepareAttachmentTree` 以 **root:0700** 建的 out 目录；容器内 setuid publisher 以 euid 10001 打开 root:0700 目录→EACCES（`attachment_canary_publisher_linux.go:35` 要求 ruid10002/euid10001，:56 打开该根）。macOS virtiofs 无视 owner/mode＝恒绿；Linux 逐层现形。

**这族＝A35 在册票**：DEV-BASELINE A35 早登记"Linux 三缺陷：① amd64 harness arm64-only ② attachment 目录 operator-owned 0700 在原生 bind 下 uid-10002 不可穿越（warm canary exit-41）③ wiki export 宿主 Chmod EPERM……**仅登记不激活，激活需所有者新令**"。D-2＝②的入站面已修；出站 publisher＝②的第二面；③未碰。逐层"部署推着修 A35"＝在**无 A35 激活令**下间接激活它，越往下越是真安全边界（容器 setuid 写域宿主所有权），不该在部署会话里零敲碎打自主推进。

## 3. 对 owner 上轮"B（双臂走非激活捷径）"的诚实纠正

我提 B 时假设"代理本体在场"可经非激活路径达成。**取证证伪**：provider-proxy 是 resident 内模块，resident 被 canary 崩溃闸挡死→无 resident 即无代理→双臂（A36-F1 需代理在场＋gateway-allow 经 ufw；A31-F1③ 需容器经宿主 INPUT 达代理）**没有非激活捷径**。故 B 的"双臂"一半不成立，收回。B 的另一半（监督激活立专票）成立且升级为 A35。

## 4. 零干扰终证（同格式/交叉，五条）

1. **ufw diff 空**，ALLOW 规则数 before=after=28＝**未碰防火墙**（本轮双臂未跑＝无 gateway-allow＝ufw 全程零改动）。
2. 宿主 iptables `shz-ci-gw` 标记规则＝3（既有 CI，未增）。
3. 无本部署常驻：resident/provider 进程 0、shorizon systemd 单元 0、环回代理口/7333-5 **均未在听**（激活回滚＝ surfaces 未起）。
4. 当前监听端口 169 个 **< 窗始 188**（租户 crash-loop/临时容器自身波动）；**本部署净新增监听＝0**。
5. 本部署 footprint 容器仅 `mc-helper-5fefdccd2682`（helper 自有产物）；docker 网络/netns 未新增（netns 仅既有 shz-ci）；镜像 mc-prod/mc-project/mc-*-e2e 与三命名卷 mc-{opencode,runtime,spine}-5fefdccd2682＝本部署自有、落大盘 data-root、未运行、无与租户同名冲突。

**取证疏忽如实登记**：窗始 docker-ps/docker-net 快照未统一 `--format`（存了默认表 ID 列），致与 after（名字列）无法逐行对齐——不影响上述结论（其它维度同格式交叉印证），但记为下轮纪律（窗始快照一律固定 --format 列）。

## 5. 状态判定（不虚标）

- **C11＝IN_PROGRESS**：三切片代码在 dev、config 链+录入+入站传输真机在产，但端到端真实 agent 回合未成（依赖监督激活＝待 A35）；不升 CODED/EVIDENCED（EXEC-2：CODED 证据须端到端合并后成立）。
- **A36＝CODED 不升、A31＝IN_PROGRESS 不变**：F1/F1③ 验收臂未跑（代理未在场、未碰 ufw）＝维持原状，不假 PASS。
- **D-1/D-2＝已修在册**（defects-001.md），D-2 端到端待 A35 激活后于 Linux 全链复验。
- 建议下一步＝**owner 新令激活 A35**（我出 diff-plan：入站已修复用＋出站 publisher 宿主 out 目录 owner 归 10001（不放宽 access、符合镜像本意）＋wiki export Chmod＋amd64 harness；Linux boundary/e2e 硬门）→ 双臂随之。

## §6 r13 逐层真机账（② 通过后暴露 C11-TS 遗漏，随即补）

- **② 出站修复真机生效**＝r13（HEAD 4e641fd）resident.stderr 报错从 `attachment confinement canary refused` 变为 `The runtime catalog connection is invalid.`＝canary（入站+出站）已过、监督推进到下一层。ufw 仍 ALLOW 28＝防火墙全程未碰（r13 刻意不含 gateway-allow）。
- **新边界＝C11 切片3 的 TS 侧遗漏**：`resident/src/credential-projector.ts:48` 的 runtime-catalog kind 白名单只放行 `<channel>-oauth`/`<channel>-api-key`，未含 `openai-compatible`；`:55` 对非 opencode 目的地强制 https（self-host 是环回 http）。⇒ 真机第一次把 self-host 连接喂进 resident 消费路径才现形——**此前 resident 789 全绿是因测试 row 全是 codex-api-key，未覆盖 C11 新 kind 走 parseRuntimeCatalog**（"未验证继承面"又一例，与 D-1/D-2 同族＝macOS/CI 面掩盖 Linux/新路径）。
- 修复＝:48 白名单加 openai-compatible；新增 `kind=openai-compatible ⟹ channel=codex` 路由锁；scheme 对 self-host 放行 http（目的地 host 类=环回/私有字面 IP 由 Go validateRegistry + C02 连接期双重把守，此处不重复）；其余 kind 保持 https-only 不变。补 credential-projector.test 四向量（http 环回落／https 落／claude 路由拒／userinfo 拒）。resident 790/0（+1）。

## §7 干净重装验证（轮六十四·dev-reset 全清→向导 r17，决定性）

owner 选「干净重装验证」。前置核实（不猜破坏性工具）：本轮窗口 `3e5d6a3..4d499c9` 内 schema 三文件**零改动**（git diff 空）→ `mc __reset` 不因 schema 拒绝；mc/mc-prod/HEAD 全=4d499c9→免 --rebuild；dry-run PLAN 作用域确认仅 `/data-shunhong/shorizon-prod`。

**dev-reset --confirm --project 执行**：spine 销毁＋**全新大脑**（新 deployment `a087f5534fe…`，schema 仍 4786047e 三度确证不变），home/container/tunables 重建成；project 段被 dev-reset 自身 `mc onboard project` 未带 `--accept-generated-sandbox` 挡住（工具局限）。

**发现＝dev-reset 真实缝隙（登记，不在 A35 域内顺手改）**：dev-reset 有意**保留 `onboard-intents/`**（注释：清它会丢失重登记路径），**却又销毁 `deployment.uuid` 换全新大脑**→保留的 posture receipt（`project-<sha>.json` 记 `deployment_uuid:7b75278f…`＝已销毁旧脊）与新脊 `a087f5534fe…` 不符→向导 project 段按设计"跨 deployment 引用一律拒隐式改写"→卡死。**＝恢复工具两处设计（保留 intents + 换 uuid）自相矛盾**，净重装后首次 project 重登记必卡。手工清除该孤儿 receipt（我 MC_HOME 内、指向已死 deployment）后放行。

**干净脊向导 r17 全段（决定性）**：preflight/home/runtime-auth/routing/container/**project/tunables/surfaces/supervision/verify 全 ok/done**；`supervision=done`→resident+dashboard+agentsview active、**tick 回执落（release 4d499c9）**、127.0.0.1:7333 在听。⇒ **A35 核心目标达成：Linux 运行时不再残疾——装配/凭据/网关许可/监督/心跳/代理在场全通**（① ② C11 载体三项真机证实）。

**唯一未闭＝acceptance 真实回合**：`--confirm-live-spend` 下 codex harness 启动并拨 new-api（status=running＝进程确实起了、① 生效旁证），但该回合未干净收尾→残留 running→后续报 `ambiguous after a prior start`。new-api 后端存活（`/v1/models`→401 正确鉴权拒绝）。清理 running 记录后，acceptance 失败使 `enabled_bindings=[]` 而 routing.md 仍引 newapi→tick 刷 `unresolved binding`（产品 DisableOnboardRuntimeBinding 未同步 routing.md＝**疑似第二个产品缺陷：acceptance 失败禁用 binding 不清理 routing.md 引用→部署自锁死循环**，待干净复现定性）。已 `disable --now` 停我的 user 单元→干净静默（loaded 0）。

**零干扰全程终证**：CI 栈 4 单元 active、宿主 shz-ci-gw 3、ufw ALLOW 恒 28（双臂未跑＝从未碰防火墙）、租户容器 56 在跑、new-api 存活。本部署仅余自有 MC_HOME（大盘）＋三命名卷＋镜像，无残留运行体、无与租户撞名。

**净结论**：Linux"不残疾"已由干净重装证实到"监督激活+心跳+代理在场"这一层；差最后"一次真实 agent 回合成功返回"，其卡点是 **codex CLI ↔ new-api 协议兼容**（外部工具面）或上述**疑似产品自锁缺陷**，均属需单独定性的新边界，非 A35 移植本体——A35 三缺陷（① amd64 路径 ② bind owner 双向 ＝真机已修已证）。

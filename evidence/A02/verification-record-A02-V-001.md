# A02 验证记录 #1（TM §7.1 · Linux 主机凭据后端 · 真宿主 CLI 动词端到端）

- verification_id: **A02-V-001**
- target_component: Linux 主机凭据后端（`host_secret_linux.go`：HKDF(/etc/machine-id‖salt)+AES-256-GCM；写后读回/摘要校验在通用 `host_secret.go`；`HostCredentialCommandsAvailable()` linux=true）——即 DEFECT 之外的 A01 批文件2 落地件
- tested_version: dev tip `accbf32`（含 A02 提交 `f65147d`）；被测二进制=本机交叉编译 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ./cmd/mc`，30559228 字节 ELF x86-64
- 运行域：**218.84.111.20**（root:3479；Ubuntu 22.04.4 / **x86_64**；预检实测 `systemd-creds` 缺席、`/dev/tpm*` 无、`/etc/machine-id` 33 字节在位、**无任何 com.culler.* 活动单元、无生产 home**）——route A 经所有者"我也同意你的倾向"明确扩权至凭据动词端到端
- 隔离：`MC_HOME=/tmp/a02probe/home`（0700 全新临时 home）、`XDG_STATE_HOME=/tmp/a02probe/state`（盐/blob 落此，不触 /root）；结束 `rm -rf` 全清

## 结果（逐件实读，无预写）
1. **真 CLI 动词跑通**：`mc agent-profile proxy-tls` **rc=0**，stdout 返回 CA 证书 PEM（宿主代理取用，符合设计——明文只回给调用方、不落盘）。此动词经 `agentprofiles.go:144→EnsureProviderProxyTLS(home)→WriteHostSecret/ReadHostSecret/DeleteHostSecret`，即**真实产品路径**（非单测驱动）。
2. **默认后端路径生效**：`HostCredentialCommandsAvailable()=真`（未撞"require operator's host"拒语）；状态树=设计形态：`culler-host-credentials/credential-salt`（盐首建一次）+ `culler-host-credentials/blobs/Culler-<homehash>/proxy-tls:<ref>.enc`；machineIDPath/stateRoot 空⇒读真 `/etc/machine-id` + 默认 XDG 分支。
3. **加密属实**：blob 1825 字节，`grep 明文证书标记=0`（`plaintext_leak=0`）⇒ 盘上无明文。
4. **幂等/授权保全**：二次 proxy-tls rc=0、`idempotent=YES cert stable`（复用同一 proxy-tls 授权，§7.4 保全不变量）。
5. **删盐=重授权（U-6/件3 真硬件坐实）**：移走 `credential-salt`→proxy-tls **rc=1**、`saltloss_old_unrecoverable=YES`（无原盐则旧料不可解，须重新授权，绝不静默重生成假可解）；与容器单测 `TestLinuxHostSecretRefusesMissingSaltAndNeverRegenerates` 互证。
6. 前置门（此前已在案）：双平台编译 / Linux 容器 6/6 / 扫描分域 0 落行 / 审方 5 语 ACCEPT / mc 全道 26-0-0。

## 残留（诚实具名，不影响后端 EVIDENCED）
- `connection-save`/`credential-read`（provider kind）CLI 动词**未单独驱动**（需初始化 project/spine，重且非必要）；但其经 `provider_credential.go:165→WriteHostSecret`=**与 proxy-tls 完全同一后端代码路径**，本记录①-⑤已在真宿主证该后端。读回校验/幂等/删盐不可解均通用路径，kind 无关。
- A31（host-gateway add-host）**不由本记录证**（proxy-tls 不启动带 provider-access 挂载的 agent 容器）——仍候 route B/CI。
- A25 向导 UI 全程（step2 起）不属 A02；但"Linux 第 2 步撞 `ErrHostCredentialUnavailable`"的根因（后端=桩）已由本记录证实**在真宿主可用**⇒A25 的 store 依赖解阻（其 UI 端到端另计）。

- reviewed_by: host-agent（agent E1；真宿主操作经所有者 route-A 明确扩权；非独立复验）
- verdict: **A02 = EVIDENCED**（Linux 主机凭据后端真宿主 CLI 动词端到端五项全绿）；义务完成判定仍归 F0a Exit+治理链。

# ADR-0 · 主机凭据后端（O1）— 定稿文本草案 #1

> 状态：**DRAFT-FOR-SIGNATURE**（执行侧据所有者八格批文起草的定稿文本；**尚未生效**——生效=下方"定稿批准"行由 Authority 落笔。执行侧不代签，见 §9）。
> 取代：`docs/contracts/IDENTITY-GOVERNANCE.md §5.1 表 B〔待:O1 ADR〕` 占位；兑现 `MASTER-ARCHITECTURE D17/D20`、`PROVIDER-SECURITY §1（L1 凭据层）`、`DATA-CONSISTENCY §7.4（密钥环归属，见件 7）`。
> 批文沿革：八格=`全按推荐`（轮十六 D1–D11 + 轮二十一"全按推荐 a准"）；本轮补 **P1（C01 字段闭集另案，不属本 ADR 的凭据选型，见 §8）**、**P7（备份加密密钥环机制归 A06，复用本后端）**。

## 1. 决策
主机凭据后端采用**抽象层同构、载体分 OS** 的模型：
- **抽象层跨 OS 完全一致**——`hostSecretStore` 三法接口（Read/Write/Delete）+ 写后读回校验 + 摘要比对 + 引用生命周期 + 错误语义 + 可用性谓词，对上层（provider 授权、连接读写、代理 TLS、viewer、connection 删除）呈同一契约。
- **载体层分 OS**——darwin 保留既有 login Keychain 实现（不重写已验证机制，MASTER §9 护栏 3）；linux 新增主机密钥封装实现（本 ADR 核心新增）。
- **"同一凭据后端"（U-7）据此定义为抽象层同构**，非载体层同一；dev 与 prod 同用该抽象，CI 经 `test_fake_routing` 构建标签下的 fixture 后端持**零真凭据**（结构成立，见 §5）。

## 2. Linux 主机密钥封装（实现已由 A02 落地并 EVIDENCED）
- 密文落**用户态状态根**（`$XDG_STATE_HOME`，默认 `~/.local/state/` 下 `culler-host-credentials/`），blob 按 `<service>/<account>.enc` 隔离；**引用元数据落 MC_HOME**（`host-credential-references/`），与载体无关。
- 密钥派生 = `HKDF-SHA256(IKM = /etc/machine-id ‖ 安装期 salt, salt = 每次写入随机 nonce, info = 域分离标签 + service + account)` → AES-256-GCM。全部标准库（`crypto/hkdf`，工具链 pin go 1.25），**零外部依赖**。
- salt 首次存储时一次性生成（`O_EXCL`），**丢失绝不静默再生**（再生将使旧密文永久不可解却表面正常）；缺失即报不可用。
- **信任锚（显式承认，不夸大）**：解密需同时具备 root（读 machine-id 与用户态 salt/blob）；此为**与文件权限同层的机密性**，**不宣称硬件级强度**（218 实测无 TPM；见 §6）。真凭据**永不入容器、永不入 spine、盘上仅密文**（PROVIDER-SECURITY P3 / IDENTITY-GOVERNANCE 表 B 固化）。

## 3. 恢复叙事（件 3 写死）
换机 / OS 重装 / salt 丢失 / machine-id 变更（含克隆 VM）⇒ 密文不可解 ⇒ **恢复 = 重走向导凭据步骤（重新授权）**，与 U-6=否（备份不含可用凭据）一致。salt 置于 MC_HOME 之外，使"仅恢复 MC_HOME"亦触发重授权——**从严、无例外的刻意选择**（代价：OS 重装必重授权；收益：备份语义无暗门）。未来具 TPM 的宿主可在同接口内将 salt 换为 PCR 绑定，属独立后续票，不改本契约的恢复语义。

## 4. 拒外部 KMS（件 4）
不采用运行时外部 KMS：① CI 服务身份本身即凭据，正面违反"PR CI 零凭据"不变量（AGENTS）；② 宿主→KMS 出口 = PROVIDER-SECURITY §2.7 未写的目的地治理新面；③ 设施可用性（U-2）未知，不为未证依赖扩面。**翻案前置**=先修订 PROVIDER-SECURITY 目的地治理（Authority 事件），再立独立票。

## 5. 三环境后端一致性与 CI（件 1 × U-7 落实）
- dev 与 prod 同用 §1 抽象（各按其 OS 载体）。
- CI（含 GitHub runner）经 `test_fake_routing` 构建标签选入 fixture 后端，**不触任何真凭据存储、不需 keychain/salt/machine-id**——"CI 零凭据"由构建标签结构保证，非约定。

## 6. 探针与降级声明（诚实）
218（Ubuntu 22.04 / systemd 249 / x86_64）实测：`systemd-creds` 二进制缺席（该工具 systemd 251+ 才有）、无 `/dev/tpm*`、`/etc/machine-id` 在位。⇒ 原设想"调用 systemd-creds `--with-key=host`"**不可用**，本 ADR 采 §2 的 Go 原生等价封装，**信任锚等同、不更高、不假称硬件背书**。A02 已在该宿主以真·CLI 动词 EVIDENCED（明文不上盘 / 幂等保授权 / 删盐=需重授权）。

## 7. 备份加密密钥环（件 7，本轮 P7 裁定）
本 ADR **不定义**备份加密的密钥环机制；**归 A06** 自定，且**复用 §1–§2 的同一 `hostSecretStore` 抽象后端**（备份加密密钥本身作为一条主机凭据存取，不另起秘密存储权威）。DATA-CONSISTENCY §7.4 的"机制归 O1 ADR"据此收敛为：**O1 提供凭据存取抽象，A06 在其上实现密钥环生命周期**——两文档不再各造后端。

## 8. 与 C01 的边界（本轮 P1 另案，防混淆）
本轮 **P1 批准的是 C01 的 L2 Provider Registry 配置字段闭集**（`runtimes.registry` 端点登记 schema），属 PROVIDER-SECURITY §1 的**目的地登记**，与本 ADR（O1 的**凭据载体**）正交。本 ADR 不定义 registry 字段；C01 依其 evidence/C01/diff-plan-001.md §1 闭集进码。二者共用"目的地/凭据不来自 agent"（P1 不变量）但不共用代码面。

## 9. 签发（两事件，本 ADR 生效的唯一门）
- **所有者批文（八格 + P1/P7）**：**已落**=`全按推荐`（轮二十一）+ 本轮"P1 认草案 / P7 自定复用后端 / P6 帮我落笔"。
- **定稿批准（Authority 对本 §1–§8 文本落笔）**：☐ **待笔**——执行侧起草止于此；此格未填 ⇒ 本 ADR 为草案、IDENTITY-GOVERNANCE 表 B 占位不销、不据此宣称"凭据后端契约已定"。落笔后：本件转 `docs/contracts/ADR-0-credential-backend.md` 入登记册，A01→EVIDENCED，AM 记独立签发事件。

*（起草：host-agent 执行侧；**起草 ≠ 批准**。证据/实测引用见 evidence/A02/*、evidence/A01/o1b-comparison-001.md、evidence/A01/a01-decision-support-001.md。）*

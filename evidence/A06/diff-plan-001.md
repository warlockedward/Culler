# A06 备份加密密钥环 + 恢复演练框架 + 三层证据 + Z5 预留 — diff-plan #1

## 0. 授权与形状
- 所有者轮三十三 grill-me **Q4"全按推进"=有界扩权令发出**（A06 218 演练域：scratch MC_HOME+XDG 隔离、专用 docker 栈、凭据 env/密钥服务、零残留前后差；禁区=他人项目状态/主 daemon 容器卷/shzci 账户面）。
- 形状=P7 裁定原话（PENDING-DECISIONS-002 §P7）："备份 spine（用 A27 StageSpineCopy）+ 恢复演练框架 + 三层证据 TM §7.1 + Z5 预留"。
- 侦察结论（本轮子代理 8 节事实件，file:line 锚全在）：**备份/恢复机制已在仓内**（mc backup crossing、resident BackupChore、update checkpoint）⇒ 本票纯 additive，**零改现有 backup/restore wire 语义**（AGENTS.md "wire shape change = design decision" 纪律：本票不产生 wire shape 变更）。

## 1. 改动面（逐文件）
| 文件 | 改动 | 断言/行为影响 |
|---|---|---|
| mc/deployment/backup_keyring.go（**新**） | ① 密钥环元数据 `MC_HOME/backups/keyring.json`（version=1、key_id、account ref、fingerprint=sha256(key)、created_at；DC §7.4① "key_id×覆盖时段×指纹" 的 Class A 记录面）② `EnsureBackupKeyring(home)`：缺则 `WriteHostSecret(home,"backup-key",32B CSPRNG)`（A02 抽象，service=`Culler-<sha256(home)>` 既有派生）→读回+SHA256 校验（WriteHostSecret 内建）→原子落 keyring.json（temp+rename+sync，`requireSecureBackupDir` 式 0700 owner-only 治理）；在则读+`ReadHostSecret` 验证 ref 可解+指纹比对（漂移即拒，不静默换新钥） | 新面；复用 A02 后端=**不另起秘密存储权威**（ADR-0 §7 件7 边界） |
| mc/deployment/recovery_drill.go（**新**） | ① `EncryptedBackupSpine(ctx, home, image, volume, run RuntimeDocker)`：`StageSpineCopy`（A27 复用，WAL/SHM sidecar 故意随行）→ `substrate.OpenSnapshotReadOnly`（A27 guard：见 -wal/-shm 即拒=快照未 settle 的诚实失败）→ AES-256-GCM 加密（密钥=密钥环）→ 文件形 `SHB1`：magic(4B)+"SHB1"+version(1B)+key_id 长度前缀+key_id+nonce(12B)+ciphertext（含 GCM tag）→ 落 `MC_HOME/backups/encrypted/spine-<UTC ts>.db.enc`（**独立命名，不触现有 `spine-*.db` 正则/pruneBackups/RestoreSpine 路径**）② `RunRecoveryDrill(ctx, home, image, volume, run)`：取最新 `.enc` → 解密（nonce/key_id 回验密钥环指纹）→ `OpenSnapshotReadOnly` → schema/deployment_uuid/表计数+`PRAGMA quick_check`+聚合根抽样（meta 行+dispatch/activity 计数=DC §7.5 "integrity_check+聚合根抽样"）→ **TM §7.1 全字段记录**（verification_id/target_component/tested_version/environment/attack_scenario=演练剧本引用/expected/actual/evidence_ref/evidence_digest/test_script_digest/execution_id/supersedes/verdict/reviewed_by=执行侧显式兼任记录/verified_at；场景不适用字段按空值规则留空**不删列**）→ `MC_HOME/backup-drill/<runID>/record.json`+`.enc` 摘要③ Z5 预留=record 的 `z5_replication` 字段（本版恒 `NOT_APPLICABLE`+注释：独立信任域通道未实现，TM §2 Z5/D22 四要求待专票） | 新面；对生产 spine **零写**（演练=读备份文件+读 volume staging 副本，绕开 RestoreSpine"只准入空槽"的破坏性约束） |
| mc/cmd/mc/main.go | `__recovery-drill` 进 if-chain（→runLocal）+ dispatchVerb case：`LoadIdentity`+`RequireHostScope`（host 专用动词，agent 容器结构性不可见=setuid+run.json 门）→ `configuredCanonicalHome` → EncryptedBackupSpine+RunRecoveryDrill 一周期 → JSON receipt 出 stdout（exit 0/1/2 语义照惯例） | 新 verb；现有 verb 零改 |
| mc/deployment/recovery_drill_test.go（**新**，无 build tag 无 docker） | ① 密钥环：首建=ref 可解+指纹匹配；二次调用幂等不换钥；密钥环指纹漂移（hostSecretStore 换值）=拒绝不静默；keyring.json 被 symlink/放宽 mode=拒（secureBackupDir 式治理测试）② SHB1 round-trip：加密→解密字节全等；篡改密文/换 key_id/截断=拒③ 演练主路径：scratch home+`substrate.Init` 真建 spine+插 meta/dispatch/activity 行→**fake run RuntimeDocker 注入**（StageSpineCopy 的容器步被 fake 成宿主 cp，同 update_checkpoint_test.go 的注入缝模式）→ 断言 record.json TM §7.1 **全字段存在**+verdict=PASS+evidence_digest 可复算+聚合根抽样值正确④ hostSecretBackend swap 隔离（host_secret_linux_test.go:156-158 先例） | 证明新行为，不改既有测试 |
| tools/blcheck/status.json | A06=BACKLOG(缺省)→IN_PROGRESS | — |
| DEV-BASELINE.md A06 行 | 备注追加轮三十三 CODED 实况（本件路径+复用面+具名残留=DC §7.4 双事件轮换/自动周期 resident 集成/Z5 通道实现，各自候裁） | — |

## 2. 明确不做（守界，防 scope 蔓延）
- **不改** `mc backup`/`__backup-spine`/`RestoreSpine`/`backup_retain` 任何现有语义与命名正则（两套备份并存=迁移期事实，明文穿越备份继续有效；加密备份是新通道）。
- **不实现** DC §7.4 双事件轮换协议（rotation_prepared/effective/reverted）=具名残留候裁（密钥环元数据已含 key_id×指纹×时段面，轮换=在其上加事件账本）。
- **不做** resident 周期自动演练集成（BackupChore 先例在，集成=候裁小项）。
- **不实现** Z5 异地通道（仅 record 字段预留+注释）。
- **不触** launchd/部署/发布件；`host_release.go` 清单不涉（mc 单二进制，新 .go 编入二进制）。

## 3. 验证路径
1. 本机：`go vet` 各 tag + `./mc/check.sh` 全链（新单测无 docker 必跑必过）+ 相关既有备份测试族零回归（spine_transfer_test.go/update_checkpoint_test.go）。
2. 218 演练域（Q4 有界扩权，S4 run 结束后执行以免栈争用）：交叉编译 amd64 mc→ssh 218→scratch MC_HOME+`XDG_STATE_HOME` 隔离→专用 docker 栈（`DOCKER_HOST=unix:///run/shz-ci-docker.sock`，摘组后 shzci/root 均可达专用栈；主 daemon 60 容器零触碰）→`mc __recovery-drill` 真 volume 全流程→**三层证据 TM §7.1 记录落盘**→零残留前后差（scratch 域整体删除+专用栈容器/卷计数前后差）。
3. evidence/A06/verification-record-A06-V-001.md 按 TM §7.1 全字段（218 实测面+本机单测面双记录）。

## 4. 风险
- `StageSpineCopy` 的 `run` 缝在生产=`runRuntimeDocker`（真 docker）：218 演练即真路径；本机单测用 fake run=**不证明 docker 臂**（诚实标注，docker 臂=218 域）。
- 密钥环 key 存 OS 密钥库（darwin keychain / Linux 文件态+machine-id+salt）⇒ "备份+密钥环同宿主失陷=两者同失"的威胁模型边界**不扩称**（备份加密防的是备份文件离宿主后的裸露面，不防宿主整体失陷——TM B10/RR 语义照旧）。
- SHB1 是**新持久格式身份**：若日后纳入 distribution Formats（update_policy.go 先例），需格式版本治理；本版自含 version 字节，预留该缝。

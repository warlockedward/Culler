# D07 DIFF-PLAN-001 · 锚定发布（链头集→Merkle root→Z5 复制；四要求）

票面（DEV-BASELINE）：锚定发布（链头集→Merkle root→Z5 复制；四要求）｜deps B07✓,D06✓｜契约锚 D22；DC §3.4 输出｜验收 P4.1：锚定四要求离线验证。
基线：dev tip eb12243；schema 基线 identity＝9e322e35…01a0（S10）——本票触 schema 面（S11⇒新基线 identity）。

## 0. 契约形态 recon（先列后改的事实基础）

- **D22 四要求**：①签名密钥与 spine 不共享安全域；②checkpoint 序号单调；③新 checkpoint 链于前者（防分叉/回滚）；④定期复制至独立信任域支持离线验证。sigstore 可选路径只发布签名 Merkle root＋验证元数据，**永不发布签署内容/项目名/内部事件**。
- **Z5 独立信任域**（TM §2/Z 行、B10 行）：备份异地副本＋锚点 checkpoint（D22 四要求）。
- **DC §3.4 验证器输出（协议形状，B07 已实现）**：`VerifyAuditChain`⇒per-aggregate 链头集 `{(aggregate_id, seq_max, chain_digest)}`（Errors 空＝层一干净）；`ComputeRoot(heads)`⇒(algorithm_id, leaf_count, root)（F0b 冻结算法，输出必须携带 algorithm_id；空集＝EMPTY_ROOT 域常量）。**层二（整根消失）的预期依据＝已锚定的聚合根链头集合（D22/IG 产物）**。
- **D06 既有消费面**：sign_requests 状态机＋signnode 过渡节点（密钥在节点环/A02 store＝结构上不在 spine 内）＋能力门 anchor.sign.request＋receipt 三重验证。
- **外部完整性分工（DC §3.4 末条）**：内部链不自证对外可信；锚定＝D22（IG 消费 DC 输出），DC 不引入签名机制。

## 1. 设计裁定

- **D0＝schema S11 anchor_checkpoints（4 对象）**：checkpoint_seq INTEGER PK UNIQUE≥1（单调，D22②）；prev_digest/root_digest/heads_digest/artifact_digest 64hex；algorithm_id；leaf_count≥0；heads_json（≤65536，规范化链头集全文——checkpoint 可完全由 spine 重建，Z5 副本丢失可恢复）；published_by FK users；sign_request_id FK sign_requests（D06 签名路径绑定）；created_at SpineTime。**no_update/no_delete 双 trigger＝防回滚的结构性承载（D22③）**＋guarded_insert（published_by 归属复验，D01 模板）。
- **D1（checkpoint 协议，shz-anchor-v1 规范形）**：核 = `shz-anchor-checkpoint-v1\n` + join("\n", seq 十进制, prev_digest, root_digest, algorithm_id, heads_digest)——节点签署的 payload 即此字节串；heads_digest＝sha256(链头集规范渲染)（聚合根 id 升序＝DC §3.4 canonical 序；每行 `id|seq_max|chain_digest`）；prev_digest＝前一 checkpoint **核**的 sha256（链于已签内容——D22③ 最强形）；genesis prev＝sha256("shz-anchor-genesis-v1")；seq＝事务内 MAX+1（与 audit seq 同纪律，D22②）。
- **D2（签名，D22①）**：经 D06 路径 create⇒execute（payload＝核字节串）——密钥在节点环/A02 store，**结构上不在 spine 内**；receipt 三重验证后 checkpoint 行才落；artifact 记录 key_id＋carrier=transitional（§5.4 生产锚定门可审计）。
- **D3（Z5 复制，D22④）**：artifact＝`MC_HOME/anchors/checkpoint-<seq>.json`（复制单元；内容＝root＋链头集＋验证元数据＋签名＋attestation＋公钥——sigstore 纪律：**零签署内容/项目名/内部事件**）。跨机传输（异地副本）＝运维面（A06 域），本票产可复制单元＋离线验证器。**恢复路径**：publish 入口先查最新 checkpoint 的 artifact 是否在位，缺⇒由 spine 行（heads_json＋sign_requests receipt）重建写盘（幂等复制，不另占 seq）。
- **D4（离线验证器，验收句载体）**：`mc anchor verify <dir>`——**零 spine、零 MC_HOME 依赖**：逐 artifact 验（格式/链头集规范序无重复/heads_digest 绑定/leaf_count/核重建+ed25519 签名验证[嵌入公钥]/root 可由链头集离线重算[ComputeRoot 纯函数]/attestation 验/绑定元组）＋跨 artifact 验（seq 自 1 严格递增无缺口/prev_digest 链闭合）。输出验证报告＋已验证至 seq N。
- **D5（诚实边界，入册）**：①异地传输＝A06 域（本票产复制单元）；②sigstore/外部 transparency log＝可选路径未建（artifact 形态已守其纪律）；③层二整根消失**检测器**不在票面——本票交付 DC §3.4 层二所要求的**预期依据**（已锚定链头集）；④生产锚定接受＝§5.4 载体 B 的所有者基础设施决策——本票 transitional 签名支撑 P4.1 离线验证，artifact 的 key_id/carrier 字段使载体门可审计；⑤锚定不解决内容可信，只锚完整性（D22 语义）。

## 2. 正清单（文件）

- `mc/substrate/schema.sql`：S11 块（表＋3 trigger，4 对象）。
- `mc/substrate/migrate_queries.go`：＋anchorCheckpointsDDL（逐字节等同）。
- `mc/substrate/migrate.go`：＋d06SigningIdentity（9e322e35…01a0 钉定）＋S10.to 钉＋S11 add-anchor-checkpoints（⇒CurrentSchemaID 符号）。
- `mc/anchor/anchor.go`（新包）：shz-anchor-v1 规范形（CoreCanonical/HeadsDigest/heads 渲染）＋BuildArtifact＋VerifyArtifact＋VerifyDir（离线验证器核心，纯文件，零 spine 零 MC_HOME）。
- `mc/verbs/anchor.go`：＋AnchorPublish（层一预检[Errors 非空⇒拒锚]⇒ComputeRoot⇒恢复分支⇒D06 create/execute⇒artifact 构建+落盘⇒checkpoint 行+Class A anchor.published）。
- `mc/cmd/mc/anchor_command.go`＋main.go（runLocal family＋switch）＋systemstatus.go（anchor-publish case）：`mc anchor publish`（withSpine）／`mc anchor verify <dir>`（host 本地，无 spine）。
- `mc/substrate/anchor_checkpoints_test.go`（新）：stripAnchorCheckpoints＋d06Spine＋S11 族。
- `mc/anchor/anchor_test.go`＋`mc/verbs/anchor_t22_test.go`（新）：§4 四要求族。
- registry 11 步全族更新（同 D03/D05/D06 机械模式）。

## 3. 负清单

- 无异地传输/备份通道（A06 域）；无 sigstore/外部 log（可选路径未建）；无层二消失检测器（只交付其预期依据）；无 KMS/HSM 集成（§5.4 B 载体＝所有者基础设施决策）；无 TS/dashboard 面；锚定不声明内容可信（只锚完整性）；B05 gate 源枚举面不动。

## 4. 四要求离线验证测试族（验收句＝"锚定四要求离线验证"）

- **R1（密钥域分离）**：checkpoint 签名在 ring 公钥下验证通过，而 spine 内无任何密钥材料（断言 sign_requests/signnode ring 之外 spine 无 key 列被用于验证）；离线验证只依赖 artifact 自含公钥。
- **R2（seq 单调）**：两次 publish⇒seq 1,2；伪造 artifact（seq 重复/倒退/缺口）⇒离线验证拒。
- **R3（链于前者/防回滚）**：checkpoint 2 的 prev_digest＝checkpoint 1 核摘要；篡改 artifact 1 任一核字段⇒artifact 2 离线验证拒（回滚被检出）；spine 侧 no_update/no_delete trigger 拒改拒删。
- **R4（复制＋离线）**：artifact 落 MC_HOME/anchors/；**拷贝 anchors 目录到孤立位置（模拟异地），零 spine 零 MC_HOME 访问**⇒VerifyDir 全绿；artifact 内容断言：无项目名/无内部事件载荷（sigstore 纪律）；root 可由链头集离线重算相符。
- **行为面**：层一不干净（注入断链）⇒拒锚 fail-closed；新事件后重锚⇒root 移动；恢复路径（删 artifact 后重 publish⇒幂等复制不占新 seq）；能力门（无 grant⇒不锚）；节点不可达⇒publish 失败零 checkpoint 落库。

## 5. 验证 lane

- L1 定向（substrate S11 族＋anchor 包＋verbs R 族＋既有回归）。
- L2 mc/check.sh 全量⇒干净 lane EXIT=0（29 包[新 anchor 包]）。
- L3 登记轮⇒blcheck。

## 6. STOP 条件

- S1 锚定需触生产锚定门（§5.4 载体 B/A 的基础设施集成）⇒停（所有者基础设施决策域；本票 transitional+离线验证为票面）。
- S2 层二检测器被要求实现⇒停（票面＝锚定发布＋四要求验证；检测器＝DC §3.4 层二的独立消费面）。
- S3 artifact 需携带内部事件内容/项目名⇒停（D22 sigstore 纪律红线）。
- S4 节点密钥需移入 spine 或共享域⇒停（D22①红线，结构上不可能——D06 设计已隔离）。

## 7. 登记与收尾

实施＝少提交于 shz/TD07；全道绿后 ff dev；登记轮＝AM v0.2.26＋status.json D07=EVIDENCED＋blcheck；新基线 identity 入册；诚实边界（D5 五项）入登记。

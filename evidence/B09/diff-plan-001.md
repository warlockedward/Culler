# B09 diff-plan-001 — usage 表 v1→v2 首迁移＋blob 回填＋回填证明（先列后改）

- 基线：shz/TB09 @ e9e39ec（dev tip，C03 收口后）
- 契约锚：MASTER D10（"v1 基线极简；usage 表=首迁移 v1→v2（含 blob 回填）｜回填须数据正确性证明（D-G6.3）"）＋DC §3.2（迁移原子性契约：可恢复事务边界/确定性中间态，禁"状态更新而证据缺失"分裂）＋票面 F1（回填证明＝计数/digest/双向核验，失败原文不丢）
- 依赖已证：B02✓（v1 基线冻结）／C03✓（usage v1 积累形：usage_events＋usage_observations＋S4 步骤＋identity Z）
- 悬空锚登记：D-G6.3 正文不在现行树（roadmap 已被 superseded；grep 零命中）——不虚构其语义，以票面验收句（计数/digest/双向核验，失败原文不丢）为操作规范，呈登记轮。

## 0. 现面事实（E1）

1. v1 形（C03 落地）：`usage_events`（raw 归一化片段积累，PK (request_identity, event_seq)，native 唯一索引，append-only）＋`usage_observations`（版本槽 PK (request_identity, event_seq, source_version)，state∈reported/unknown/reconciliation，payload_digest hex64，append-only，FK→usage_events）。
2. 当前 identity Z = 6ce879af…d085（AM v0.2.16 登记；S4.to＝CurrentSchemaID 符号）。
3. 迁移机制：`applyMigrationStep` 单事务五元组（步骤 SQL＋schema_history 行＋activity 事件＋audit 链行＋meta stamp）；四态 runner（缺步骤⇒执行／记录+checksum 匹配⇒跳过／冲突⇒fail-closed／序列异常⇒停止）；registry 末端唯一引用 CurrentSchemaID 符号（历史步骤端点钉字面量）。
4. `MigrationStep{Name, Version, FromSchemaID, ToSchemaID, Checksum, Statement}`；Checksum=sha256(name+"\n"+statement)——**不含**任何新增可选字段（台账语义不变）。
5. B07/C03 先例：步骤语句文本与 schema.sql 载体块逐字节一致（sqlite_master 哈希同参照脊）；U13/U14 测试形态（仅新步应用＋DDL 字节一致）。
6. 稳态写入面：`RecordUsageObservation`（substrate/usage_observation.go）已在单事务内写 observation 行＋链行——v2 行维护加入同一事务（R-03：状态与其证据同提交）。

## 1. 设计裁定（本票决策面）

- **D1 v2 形状＝`usage` 表（BL 结算消费形，Class B 可变状态）**：每槽（request_identity, event_seq）当前版本一条——`source_report_id`（=usage_observations.observation_id，稳定不可复用，BL §4 来源身份四元组位 1）主键；request_identity/local_request_id/provider_request_id/source_version（四元组位 2/4＋槽键）；origin/usage_family/model_id/output_tokens/input_tokens/report_kind/finality/identity_provability/state/payload_digest/supersedes_report_id（BL §4 supersedes 链位）；occurred_at（27 字符围栏）。v2 是消费投影：历史版本证据永远在 v1（append-only＋链），v2 只承"当前事实"——可变性归 Class B，审计归 v1 链（DC §3.2 分层）。
- **D2 回填规则（S5 步骤内，纯 SQL，确定性）**：`INSERT INTO usage SELECT … FROM usage_observations o WHERE o.source_version = (SELECT MAX(source_version) FROM usage_observations WHERE 同槽)`——每槽取 max 版本行提升；raw-only（无 observation）事件**不回填**（不可完成的事实不造行），计数证明中显式申报（非静默）。
- **D3 失败原文不丢（结构性）**：S5 **不 DROP 任何 v1 对象**——v1 两表与链行原样保留；"原文不丢"由"v1 不删＋步骤单事务全有全无"双重保证，不靠清理。
- **D4 同事务回填验证（DC §3.2 迁移原子性）**：`MigrationStep` 增可选字段 `Verify func(tx *sql.Tx) error`（S1-S4 为零值）；`applyMigrationStep` 在步骤 SQL 执行后、账本行/链行/stamp 之前于**同一事务**内调用 Verify；失败⇒整步回滚（无"表建了数据错"的中间态）。Checksum 不含 Verify（台账字节语义不变，历史步骤重算一致）。
- **D5 回填证明三验（Verify 内容，F1 票面逐条）**：
  - P1 计数：`usage` 行数 == v1 有 observation 的槽数；raw-only 事件数（usage_events 总行数 − 有 observation 的槽数）显式读出入报告（申报不静默）。
  - P2 digest：v2 payload_digest 多重集摘要 == v1 所选 max 版本行 payload_digest 多重集摘要（ORDER BY digest ASC 逐条拼接后 SHA-256＝序无关的多重集语义）。
  - P3 双向：v2→v1 方向每行 source_report_id 在 usage_observations 存在（anti-join=0）；v1→v2 方向每个 max 版本 observation 行在 usage 有对应（anti-join=0）。
  - 违反任一项⇒ErrBackfillProof 上抛⇒步骤回滚。
- **D6 稳态写入（最小面）**：`RecordUsageObservation` 同事务内维护 v2 行——新 max 版本⇒REPLACE（按 source_report_id）；同版本重投（幂等重入）⇒v2 不动；旧版本（不可发生于写入路径，版本单调）⇒拒（ErrUsageVersionCollision 既有路径）。v2 行与 observation 行＋链行同提交（R-03）。
- **D7 命名/钉定**：`c03UsageIdentity` 常量＝Z（S4 产出的 identity，历史事实钉定）；S4.to 由 CurrentSchemaID 符号改钉 Z 字面量；S5 `migrate-usage-to-v2`（c03UsageIdentity⇒CurrentSchemaID 符号，语句＝usage 表 DDL＋索引＋回填 INSERT＋（无 trigger——v2 是 Class B 可变表，不套 append-only））。
- **D8 审计链**：S5 迁移事件自动经 applyMigrationStep 既有五元组入 schema 聚合链（零新机制）；usage 聚合链不受 v2 影响（v1 是证据权威）。

## 2. 正清单（只触这些文件）

1. `mc/substrate/schema.sql` — 新增 `usage` 表 DDL 块（v2，含注释块）。
2. `mc/substrate/migrate_queries.go` — `usageV2DDL` 常量（含回填 INSERT，语句文本与 schema.sql 的表定义块逐字节一致；回填 INSERT 不在 schema.sql＝迁移专属语句，schema.sql 只载 DDL——sqlite_master 比对仅覆盖表/索引对象，先例同 S2 的 trigger 改写）。
3. `mc/substrate/migrate.go` — `c03UsageIdentity` 常量（Z）＋S4.to 钉 Z＋S5 步骤（struct 字面量带 Verify）＋`applyMigrationStep` 增 Verify 调用位（D4）。
4. `mc/substrate/usage_v2.go`（新）— `VerifyUsageBackfill(tx)`（P1/P2/P3 三验）＋`ErrBackfillProof`＋稳态写入 `UpsertUsageCurrent(tx, in)`（D6，供 RecordUsageObservation 同事务调用）。
5. `mc/substrate/usage_observation.go` — RecordUsageObservation 事务内接 UpsertUsageCurrent（D6）。
6. `mc/substrate/usage_v2_test.go`（新）— M1-M6 测试族（§5）。
7. `mc/substrate/migrate_test.go` — 随 5 步 registry 更新（preLedger 5 步/b01 4 步/no-op 5 行 ledger；stripUsageV2 助手）。
8. `mc/substrate/usage_observation_test.go` — U13/U14 随 5 步更新（b07 基线迁移⇒应用 S4+S5）＋U5 等涉 usage 表计数断言面核对。
9. `mc/verbs/onboard_migration_test.go` — 两处 strip 助手补 usage 表剥离＋ledger 行数 4→5。
10. `mc/substrate/auchain_test.go` — W11/W12 随 5 步更新（schema 聚合链行 2→3；S5 映射行载荷断言）。

## 3. 负清单（不触）

- v1 两表 DDL/trigger/索引（回填只读 v1）；audit_events 机制；auchain.go 序列化器
- resident/TS 面（verb wire 形状不变，v2 为脊内部投影）
- dispatch/Decide/seam；openwiki/；docs/contracts（治理域）
- B05 registry（本票零新 verb）；release 面（零新生产文件⇒host_release 不动）
- BL 结算（usage_settle 族/C05）；usage 表不预建 settled 列（BL 状态归 BL 票面，避免第二权威）

## 4. v2 DDL 草案（实施时列级细化）

```sql
-- usage — the v2 settlement-consumption projection of the usage
-- observations (B09). One row per (request_identity, event_seq) slot at
-- its current (max) source_version. Class B: mutable by design — the
-- append-only history and its chain evidence stay in usage_observations
-- (v1); this table is the readable "current fact" for settlement.
CREATE TABLE usage (
    source_report_id      TEXT PRIMARY KEY,   -- = usage_observations.observation_id
    request_identity      TEXT NOT NULL,
    local_request_id      TEXT NOT NULL,
    provider_request_id   TEXT,
    event_seq             INTEGER NOT NULL CHECK (event_seq >= 1),
    source_version        INTEGER NOT NULL CHECK (source_version >= 1),
    origin                TEXT NOT NULL CHECK (origin IN ('provider','local')),
    usage_family          TEXT NOT NULL,
    model_id              TEXT NOT NULL,
    output_tokens         TEXT NOT NULL,
    input_tokens          TEXT NOT NULL,
    report_kind           TEXT NOT NULL CHECK (report_kind IN ('cumulative','incremental')),
    finality              TEXT NOT NULL CHECK (finality IN ('interim','final')),
    identity_provability  TEXT NOT NULL CHECK (identity_provability IN ('native','cursor','none')),
    state                 TEXT NOT NULL CHECK (state IN ('reported','unknown','reconciliation')),
    payload_digest        TEXT NOT NULL /* hex64 fence */,
    supersedes_report_id  TEXT /* hex64 fence or null */,
    occurred_at           TEXT NOT NULL /* 27-char fence */
);
CREATE INDEX usage_request ON usage (request_identity, event_seq);
CREATE INDEX usage_local ON usage (local_request_id);
```

S5 步骤语句＝上述 DDL ＋回填：
```sql
INSERT INTO usage (source_report_id, request_identity, local_request_id, provider_request_id,
  event_seq, source_version, origin, usage_family, model_id, output_tokens, input_tokens,
  report_kind, finality, identity_provability, state, payload_digest, supersedes_report_id, occurred_at)
SELECT o.observation_id, o.request_identity, o.local_request_id, o.provider_request_id,
  o.event_seq, o.source_version, o.origin, o.usage_family, o.model_id, o.output_tokens, o.input_tokens,
  o.report_kind, o.finality, o.identity_provability, o.state, o.payload_digest,
  o.supersedes_observation_id, o.occurred_at
FROM usage_observations o
WHERE o.source_version = (
  SELECT MAX(o2.source_version) FROM usage_observations o2
  WHERE o2.request_identity = o.request_identity AND o2.event_seq = o.event_seq);
```

## 5. 测试族（usage_v2_test.go）

- **M1 回填正确性（max 版本提升）**：种子＝槽 A（v1 interim＋v2 final⇒v2 取 final）＋槽 B（单版本）＋raw-only 事件（不提升）⇒usage 恰 2 行，字段逐值等于 v1 所选行；raw-only 显式申报数=1。
- **M2 三验全过**：S5 迁移后 P1/P2/P3 逐断言（多重集摘要相等、双向 anti-join=0、计数相等）。
- **M3 证明失败⇒整步回滚（失败原文不丢）**：篡改 registry（S5 语句加 WHERE 过滤制造缺行）⇒Migrate 报 ErrBackfillProof 族；usage 表空（未提交）＋ledger 无 S5 行＋meta 未钉新 identity＋v1 种子逐行未动。
- **M4 四态复跑**：迁移后重跑 Migrate⇒no-op（S5 已记录+checksum 匹配⇒跳过）。
- **M5 DDL 字节一致**：迁移后脊与 fresh 脊的 usage 对象 sqlite_master 逐字节相等（U14 先例形态）。
- **M6 稳态写入（D6）**：新 observation（版本+1）⇒v2 行 REPLACE 至新 source_report_id；同版本重投⇒v2 不动；v2 行与 observation 同事务（回滚则两者俱无）。

## 6. 验证 lane

- L1 substrate 测试族（M1-M6＋更新的 U13/U14/W11/W12/migrate 族）定向绿。
- L2 `mc/check.sh` 全量⇒最终干净 lane EXIT=0（含 shell 契约/closure/全 Go 包）。
- L3 blcheck：图零改动⇒标准门禁复跑 0 violations。
- 诚实边界：真实数据量回填性能＝B06 压测域（不在此宣称）；usage 表稳态消费方＝C05/BL（本票只保证"当前事实可读且被证明"，不预建结算状态列）。

## 7. STOP 条件（STOP-1 呈报，不猜）

- S1 v2 形状需要 BL 域字段（settled/authorization 等）⇒停（负清单：不预建第二权威）；以票面"usage 表=首迁移"最小形落地。
- S2 同事务 Verify 钩子触及 applyMigrationStep 既有五元组顺序语义⇒停，呈报（D4 的插入位＝步骤 SQL 后/账本行前，若与 B01 钉死测试冲突则改）。
- S3 D-G6.3 若被登记轮裁定须有正文出处⇒呈报悬空锚处置（本 plan §0.4 已登记，不虚构）。

## 8. 登记与收尾

- 实施＝少提交于 shz/TB09；全道绿后 ff 并 dev（合并前全道硬规则）；登记轮＝AM v0.2.17＋status.json B09=EVIDENCED＋blcheck 留痕。
- 本票落地后：B-chain 余 B06（门 O3 候令）＋B08（ready）；C05（BL 核心）依赖 C03✓ 可立。

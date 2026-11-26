# B08 diff-plan-001 — activity/存量域入链（第二数据迁移实战＋零改写互验）（先列后改）

- 基线：shz/TB08 @ 5d2ac82（dev tip，B09 收口后，identity W）
- 契约锚：DC §3.2（Class A 含 activity「入链」＋R-03 原子审计＋迁移原子性）＋IG §7（历史 Actor 不改写：迁移不得把原始 actor 改写为具名 user，关联经映射表）＋票面 F1（P4.0：第二迁移实战＋零改写比对）
- 依赖已证：B07✓（链机制＋AppendAuditEvent＋seq_prefix 触发器）＋B03✓（payloadSchemas 登记制：addition-only v1 附录家族先例）
- 与 B09 的关系：B09 是第一数据迁移（usage v1⇒v2 纯 SQL 回填）；B08 是第二数据迁移，且首次引入**步骤内 Go 数据移动钩子**（Migrate 字段）——因链行摘要 SQLite 无法计算（schema.sql 注释明文："SQLite has no SHA-256"）。

## 0. 现面事实（E1）

1. `activity` 表：id INTEGER PK AUTOINCREMENT／created_at TEXT（SpineTime `2006-01-02 15:04:05`，DEFAULT datetime('now')）／actor NOT NULL／kind NOT NULL／subject／detail／dispatch_key（hex64 双围栏+UNIQUE）／dispatch_request_id（16hex+UNIQUE）／dispatch_result（json object ≤64KiB，与 request_id 成对 CHECK）／project REFERENCES projects(id)；append-only 双触发器。
2. **activity 写入面＝35 处分散 `INSERT INTO activity`**（verbs/ 14 文件＋substrate/migrate_apply.go 等，grep 实数；无单一咽喉；5 处带可选列）。
3. 链机制：AppendAuditEvent（tx/conn 同事务，seq=max+1，prev fail-closed）；seq_prefix 触发器＝DB 侧空洞兜底；VerifyAuditChain 为 db 域（未提交 tx 不可见⇒步骤内验证须 tx 域自做）。
4. B09 先例：MigrationStep.Verify 同事务证明钩子（checksum 不含钩子＝台账记"应用了什么"非"如何证明"）。
5. payloadSchemas 登记制：`// addition-only v1 appendix families registered by the chain implementation — their field tables exist, their vectors do not yet (registered-pending-vectors, not verified)`（B07 先例）。
6. IG §7：历史 `'operator'` 保留为遗留主体；任何迁移不得改写原始 actor；关联经 legacy_actor_mapping（本票不涉该表——它是 P4 身份域的映射，入链照录原值）。

## 1. 设计裁定（本票决策面）

- **D0 范围裁定（先于一切）**：本票＝**存量域入链迁移**（票面 F1「第二迁移实战＋零改写比对」的字面交付）。稳态接线（未来 activity 行自动入链，需改 35 处调用点含 dispatchseam 设计决策面）**登记为具名后续票**，不在本票——理由：票面 F1 只验收迁移与零改写；35 点改接的风险面（dispatchseam 冻结 JSON、逐点 NULL/空串语义）超出本票的最小安全变更面。边界诚实登记＝迁移后 'activity' 聚合为迁移时点快照，后续行候稳态票入链（DC §3.2 activity 入链义务的稳态半＝具名残余，不冒充已闭）。
- **D1 入链语义**：activity 每行⇒`activity` 聚合一条链行（Class A 证据入链，DC §3.2 明文）；链行 payload=activity 行的规范投影（原值照录，含 legacy actor——零改写＝不改写＋不改值）。
- **D1b 无独立 aggregate_open**：'activity' 聚合首行即 genesis（DC §3.4「首事件即存在证明」，同 'schema' 聚合先例——B07 S3 首迁移行 prev=genesis，无 open 事件）⇒链行与 activity 行一一对应，覆盖证明更简。
- **D2 payload 家族 `activity`（declared-pending-vectors，B07 先例）**：字段表（声明序）＝`id(dec_int,req)／actor(str,req)／kind(str,req)／created_at(time,req，SpineTime⇒规范 RFC3339Z 微秒形确定性转换)／subject(str,opt)／detail(str,opt)／dispatch_key(str,opt)／dispatch_request_id(str,opt)／dispatch_result(str,opt)／project(str,opt)`；无向量（NOT verified 登记）；event_type=activity.kind（既有点分 slug，event_type 索引可按 kind 查）。
- **D3 event_id 确定性**：sha256("shz-activity-chain-v1|<activity_id>")——回填与稳态同一式，重放身份稳定（C03 open 事件 ID 先例）。
- **D4 稳态接线＝具名后续票（D0）**：本票不建 `AppendActivity`、不改 35 处调用点；后续票面＝集中 `AppendActivity`（activity 行＋链行同事务）＋35 点机械改接＋源级覆盖闸（B05 两阶段先例）。本票只交付存量迁移＋证明。
- **D5 覆盖闸（随 D4 后续票）**：源级测试枚举全仓（非测试）`INSERT INTO activity`，断言仅 `AppendActivity` 含之——未来旁路必红（fail-closed）。本票不建（无稳态写入面可闸）。
- **D6 S6 步骤形态**：Statement＝`activity_chain_map` DDL（activity_id INTEGER PRIMARY KEY REFERENCES activity(id)＋event_id hex64 围栏 UNIQUE）——合法 schema delta⇒identity 跨 W⇒V；**Migrate 钩子**（新可选字段，B09 Verify 同族，checksum 不含）＝Go 回填：按 id 升序逐行 AppendAuditEvent（D1b：无 open 事件，首行 prev=genesis）＋map 行（本地 prev 递推＝EventDigest/ChainDigest 预计算，免逐行回查）；**Verify 钩子**＝覆盖三验（map 行数==activity 行数；map.event_id anti-join audit_events=0；'activity' 聚合链行数==activity 行数）。
- **D7 零改写证明（双层面）**：步骤内＝S6 不写 activity（结构性，Verify 不依赖它）；测试面＝Migrate 前后 activity 全列 digest 相等＋payload 中 actor 原值（含 'operator'）逐字断言（IG §7 票面互验）。
- **D8 顺序即 rowid 序**：回填按 activity.id 升序⇒链 seq 与行序一致（单写者＋seq 分配 max+1 天然吻合；seq_prefix 触发器兜底）。
- **D9 诚实边界**：declared-pending 家族无向量（NOT verified 登记，同 B07 migration/aggregate_open 族）；activity 链行不承载"历史归因"（IG §7 更强归因仅经映射行证据追加，本票不建该表）；**稳态入链＝具名后续票（D0/D4）——迁移后 'activity' 聚合为迁移时点快照，DC §3.2 稳态半候后续票，不冒充已闭**。

## 2. 正清单（只触这些文件）

Go（mc/）：
1. `mc/substrate/schema.sql` — 新增 `activity_chain_map` DDL 块。
2. `mc/substrate/migrate_queries.go` — `activityChainMapDDL` 常量（与 schema.sql 逐字节一致）。
3. `mc/substrate/migrate.go` — `b09ChainIdentity` 常量（W 字面量，独立复算钉定）＋S5.to 钉 W＋S6 `migrate-activity-to-chain`（W⇒CurrentSchemaID 符号，Statement=activityChainMapDDL，Migrate=回填钩子，Verify=覆盖三验）＋`MigrationStep.Migrate func(tx *sql.Tx) error` 字段（additive）。
4. `mc/substrate/migrate_apply.go` — applyMigrationStep 步骤 SQL 后调 Migrate 钩子、Verify 前（既有 verifyStep 位不动）。
5. `mc/substrate/auchain.go` — payloadSchemas 增 `activity` 家族字段表（addition-only 附录，注释同先例）。
6. `mc/substrate/activity_chain.go`（新）— `migrateActivityToChain`（S6 Migrate 钩子：按 id 升序逐行 AppendAuditEvent＋map 行，本地 prev 递推免回查）＋`VerifyActivityBackfill`（S6 Verify 三验）＋SpineTime⇒canonical 转换＋确定性 event ID。
7. ~~35 处调用点改接 AppendActivity~~（**移出本票＝D4/D0 具名后续**）。
8. `mc/substrate/activity_chain_test.go`（新）— N1-N5 测试族（§5）。
9. ~~覆盖闸测试~~（**随 D4 后续票＝D5**）。
10. `mc/substrate/migrate_test.go` — 随 6 步 registry 更新（preLedger 6/b01 5/no-op 6 行）＋strip 助手补 activity_chain_map。
11. `mc/substrate/usage_observation_test.go` — U13（b07 基线⇒S4+S5+S6 三步）＋U14（strip 面）更新。
12. `mc/substrate/auchain_test.go` — W11（b02 基线⇒S3-S6 四步，schema 聚合链行 4）＋W12（preLedger⇒5 applied… 更正：S1-S6 全链⇒audit 链行 4[S3,S4,S5,S6]，S6 载荷断言）更新。
13. `mc/verbs/onboard_migration_test.go` — 两处 strip 助手补 activity_chain_map＋ledger 5⇒6。

## 3. 负清单（不触）

- dispatch.Decide／prepare-attest-commit 冻结 JSON（dispatchseam 只换 activity 记录行）
- activity 表本体（零 DDL 改动＝IG §7 零改写的结构面）／audit_events 机制／auchain 序列化器
- resident/TS（usage/observation verb wire 不变）；openwiki/；docs/contracts
- B05 registry（零新 verb——AppendActivity 是既有 verb 事务内的 substrate 函数）
- release 面（零新生产文件⇒host_release 不动）

## 4. activity_chain_map DDL 草案

```sql
-- activity_chain_map — the cross-reference between the activity ledger and
-- its chain projection (B08). One row per activity record: the activity id
-- and the chain event id (deterministic per D3). The backfill is idempotent
-- by the primary key; steady-state writes add one row per activity row in
-- the same transaction (R-03).
CREATE TABLE activity_chain_map (
    activity_id INTEGER PRIMARY KEY REFERENCES activity (id),
    event_id    TEXT NOT NULL UNIQUE
               CHECK (length(event_id) = 64 AND
                      length(CAST(event_id AS BLOB)) = 64 AND
                      event_id NOT GLOB '*[^0-9a-f]*')
);
```

## 5. 测试族（activity_chain_test.go）

- **N1 回填正确性**：种子 activity 行（含可选列变体×3＋legacy actor 'operator'）⇒S6 应用⇒'activity' 聚合链行=种子数（一一对应，D1b 无 open 事件）；seq 按 id 升序（首行 prev=GenesisHash("activity")）；payload 字段逐值（含 actor 原值、created_at 规范形转换）；map 行齐（activity_id↔event_id 确定性）。
- **N2 零改写互验（IG §7 票面）**：Migrate 前后 activity 全列多重集 digest 相等＋payload_json 中 actor 与活动行原值逐字相等（含 'operator'）。
- **N3 证明失败⇒整步回滚**：篡改 registry（Migrate 钩子只回填一半）⇒ErrBackfillProof 族⇒map/链行/DDL 全回滚＋ledger 无第六行＋meta 钉 W 不动＋activity 零触。
- **N4 幂等**：已全映射的脊重跑 S6 语义＝无新行（map 主键挡重放；四态 runner 层面=已记录⇒跳过）。
- **N5 既有 registry 测试随 6 步全绿**（migrate/usage/auchain/onboard 族更新）。

## 6. 验证 lane

- L1 substrate 定向（N1-N6＋更新族）。
- L2 `mc/check.sh` 全量⇒干净 lane EXIT=0。
- L3 blcheck：图零改动⇒标准门禁。
- 诚实边界：真实数据量回填耗时＝B06 域；declared-pending 无向量＝NOT verified（登记）；覆盖闸是源级静态面（同 B05 纪律），非运行时全量证明。

## 7. STOP 条件（STOP-1 呈报，不猜）

- S1 35 处调用点中任一处改接改变其错误处理语义（非纯机械）⇒停，呈报该点。（随 D0 稳态移出本票，条件解除）
- S2 dispatchseam 冻结 JSON 面被改接波及⇒停（负清单红线）。（随 D0 稳态移出本票，条件解除）
- S3 payload 字段表与 B03 登记制冲突（如需改既有家族表）⇒停（addition-only 纪律）。

## 8. 实施补记（先列后改的追加登记）

- **D10 项目删除覆盖（测试门咬住的强制集成，非扩面）**：`activity_chain_map.activity_id REFERENCES activity(id)`＋`activity.project`⇒新表进入 projects 的 FK 传递闭包，`TestProjectDeleteCoversEveryDependentTable` 按设计变红。处置＝`verbs/projectdelete.go` 删除清单在 activity 条目**之后**增 `activity_chain_map`（删除逆序执行⇒map 行先于 activity 行移除，FK 无悬挂）；activity 的 where 谓词提为 `projectActivityWhere` 常量两处复用（零语义变化）。lane 第一跑唯一红点＝此门，修复后 projectdelete 族 8/8 绿。
- **D11 fieldSpec 第三元＝optional 语义再确认**：activity 家族表初版把必填/可选写反（dispatch_key 等必填⇒首跑 NOT_CONFIRMED），B03 provider_request_id 同型教训第二次实证——新家族表落地必跑种子数据。
- **D12 'activity' 聚合无 aggregate_open**（D1b）：与 'schema' 聚合同先例（首行即 genesis），覆盖证明一一对应更简；S6 步骤自身簿记 activity 行（kind=schema.migration.step）生成于回填之后，不入 'activity' 聚合（其证据在 'schema' 聚合 S6 行）——迁移时点快照语义，候稳态票（D0）统一。

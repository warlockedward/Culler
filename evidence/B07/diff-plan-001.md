# B07 链实现 · diff-plan-001（audit_events 链载体＋shz-canon-v1 Go 序列化器＋shz-merkle-v1 根算法＋verify-chain verb＋链测试全套）

日期 2026-11-25。shz/dev @ 582ac7e（B01=B02=B05=EVIDENCED，AM v0.2.14，blcheck 0 violations）。授权链＝所有者"继续剩余任务推进"＋DEV-BASELINE:76 B07 票面（所有者设计件）＋DC §3.4 全节（链结构最低契约权威）＋B03 设计件（docs/design/f0b-serializer.md＋vectors.json 30 例）＋B04 设计件（docs/design/f0b-2-root-algorithm.md＋root_vectors.json 11 例）。依赖 B03+B04 已 EVIDENCED（blcheck 边满足）。

## 0. 票面分解（DEV-BASELINE:76）
| 要素 | 权威 | 本 plan 落点 |
|---|---|---|
| prev_hash 触发器 | B07 票面 | §2.2 audit_events DDL＋append-only/结构触发器（SQL 可表达面）；摘要复算归 Go 路径（§2.5 诚实边界） |
| aggregate_open | DC §3.4 检测义务两层界定＋§3.5 | §2.4 OpenAggregate：只登记生命周期，**不充当**被删对象的唯一消失检测依据（层二不宣称能力，D31①） |
| 序列化器按 B03 | F0b-1 shz-canon-v1 | §2.5 Go 实现＋vectors.json 30 例符合性测试（A5 教训：解析器存在≠口径正确） |
| `shz verify-chain` | B07 票面＋DC §3.4 验证器输出条 | §2.6 只读 verb：层一五检出＋per-aggregate 链头集＋{algorithm_id, leaf_count, root_hex}（shz-merkle-v1，root_vectors.json 11 例全过含错误例） |
| F1：链测试全套 | 票面 | §4 W1-W12 |

## 1. 现状事实（E1，file:line，@582ac7e 树）
1. **链在 b22df40 不存在**（DC §3.2 固化→缺口：NOT_FOUND 已证）⇒本票是链机制首建；DC §3.1-3.5 全部 [新增] 语义为唯一上位规范。
2. **摘要关系冻结**（F0b-1 §1 复述 DC §3.4）：event_digest=SHA-256(CanonicalEvent)；chain_digest_i=SHA-256(chain_digest_{i-1}_bytes ‖ event_digest_i_bytes)（原始 32 字节拼接）；prev_hash_i:=chain_digest_{i-1} 存事件行**且入哈希域**；GENESIS(type)=SHA-256(UTF8("shz-genesis-v1:"‖NFC(type)))。
3. **序列化编码冻结**（F0b-1 §2-§4＋reference_encoder.py 权威源）：presence 标签 0x00 ABSENT/0x01 NULL/0x02 VALUE；envelope 14 字段声明序（serializer_id enum{shz-canon-v1|shz-canon-v0}、event_id hex32、aggregate_id str、aggregate_type str、seq dec_int、event_type str、occurred_at/recorded_at time、effective_at?、supersedes_event_id? hex32、causation_id?/correlation_id? str、prev_hash hex32、payload obj:<family>）；str=NFC+u64be 长度前缀（无转义层）；dec/dec_int 词法原文文法校验；time=RFC3339 UTC 定宽微秒；hex32=64 小写 hex；enum 封闭词表；obj 位置化按声明序（未知字段=NOT_CONFIRMED 错误，必需字段显式 NULL=错误）；payload v1 已注册族＝usage_observation/usage_settle/admission（有向量）。
4. **根算法冻结**（F0b-2＋reference_root_encoder.py）：leaf=SHA-256(0x00‖u64be(len(idNFC))‖idNFC‖u64be(seq_max)‖chain_digest32)；序=NFC(aggregate_id) UTF-8 原始字节字典序升序（重复 id=畸形）；node=SHA-256(0x01‖left‖right)；奇层>1 复制末节点（RFC-6962）；EMPTY_ROOT=SHA-256(UTF8("shz-root-empty-v1"))；单叶树 root=叶本身；输出形状 {algorithm_id="shz-merkle-v1", leaf_count, root_hex}。
5. **迁移基础设施已就位**（B01/B02）：schema 变更⇒注册 S3 步骤（pre-X=eb7ebcf9…⇒new）；audit_events DDL 进基线 schema.sql；**今日零已部署 spine**（B01 diff-plan §1.9 先例）⇒S3 无活对象。
6. **原子审计映射表**（DC §3.2 R-03）：本波落地行＝"schema 当前值（meta.schema_id, Class B）↔迁移事件（Class A）"：B01 applyMigrationStep 的单事务扩展为五者同提交（步骤 SQL＋schema_history 行＋activity 事件＋meta stamp＋**audit_events 链行**）⇒映射表首行建成。
7. **shz CLI 面**（main.go:39 `filepath.Base(os.Args[0]) == "shz"`）：同一二进制操作者改名⇒`shz verify-chain`＝main.go dispatch 面新增顶层命令（operator/host scope，只读 spine）。
8. **activity 表 hex 双 fence 纪律**（schema.sql:684-700 dispatch_key 注释先例）：length＋GLOB 双检防 NUL 截断绕过⇒audit_events 各 hex/dec/time 列同族纪律。
9. **高频记录不入审计链**（DC §3.2 R-10 收紧）：payload_digest 用于数据一致性检查，不构成独立防篡改证明⇒audit_events 只收 Class A 链合格事件（低频、有审计责任声明者）。

## 2. 设计
### 2.1 audit_events 链载体表（schema.sql 新增，紧随 schema_history）
```sql
CREATE TABLE audit_events (
    aggregate_id   TEXT NOT NULL,
    seq            INTEGER NOT NULL CHECK (seq >= 1),
    event_id       TEXT NOT NULL UNIQUE CHECK (hex64 双 fence),
    aggregate_type TEXT NOT NULL,
    event_type     TEXT NOT NULL,
    serializer_id  TEXT NOT NULL CHECK (serializer_id IN ('shz-canon-v1','shz-canon-v0')),
    occurred_at    TEXT NOT NULL CHECK (time 定宽微秒形状),
    recorded_at    TEXT NOT NULL CHECK (time 定宽微秒形状),
    effective_at   TEXT CHECK (time 定宽微秒形状),              -- 可空
    supersedes_event_id TEXT CHECK (hex64 双 fence),            -- 可空
    causation_id   TEXT,                                        -- 可空
    correlation_id TEXT,                                        -- 可空
    prev_hash      TEXT NOT NULL CHECK (hex64 双 fence),
    event_digest   TEXT NOT NULL CHECK (hex64 双 fence),
    chain_digest   TEXT NOT NULL CHECK (hex64 双 fence),
    payload_family TEXT NOT NULL,
    payload_json   TEXT NOT NULL CHECK (json_valid),
    PRIMARY KEY (aggregate_id, seq)
);
CREATE INDEX audit_events_event_type ON audit_events (event_type);
-- append-only：任何 UPDATE/DELETE ABORT（DC §3.5）
CREATE TRIGGER audit_events_no_update BEFORE UPDATE ON audit_events
BEGIN SELECT RAISE(ABORT, 'audit_events is append-only'); END;
CREATE TRIGGER audit_events_no_delete BEFORE DELETE ON audit_events
BEGIN SELECT RAISE(ABORT, 'audit_events is append-only'); END;
-- seq 前缀完整性：插入 seq=k 要求 k=1 或存在 seq=k-1（防空洞=层一"事件缺失"的 DB 侧兜底）
CREATE TRIGGER audit_events_seq_prefix BEFORE INSERT ON audit_events
WHEN NEW.seq > 1 AND NOT EXISTS (SELECT 1 FROM audit_events WHERE aggregate_id = NEW.aggregate_id AND seq = NEW.seq - 1)
BEGIN SELECT RAISE(ABORT, 'audit_events seq prefix gap'); END;
```
（event_digest/chain_digest 的**复算验证不在触发器**——SQLite 无 SHA-256 函数⇒由 Go append 路径写入前计算＋verify-chain 复算承担，§2.5 诚实边界。）
### 2.2 链语义映射（DC §3.4 → 实现）
- 序列分配：单写者事务内 `SELECT COALESCE(MAX(seq),0)+1 FROM audit_events WHERE aggregate_id=?`（BEGIN IMMEDIATE 下确定性，§3.1①）；UNIQUE(aggregate_id,seq) 主键＋event_id UNIQUE 去重（§3.1②③）；链头＝最后成功追加行（§3.1④）。
- genesis：seq=1 行 prev_hash 必须＝GENESIS(aggregate_type)（Go 路径校验＋verify-chain 复验；SQL 侧无法算 SHA⇒诚实登记）。
- 冗余绑定：验证器双关系同核（字段值=前序 chain_digest 且递推式成立，DC 原文）。
### 2.3 Go 包面（mc/substrate/auchain.go，新文件）
- `AppendAuditEvent(db, tx 或 db, ev AuditEvent) error`：事务内 seq 分配＋GENESIS/递推 prev_hash 校验＋canonical 序列化＋event_digest/chain_digest 计算＋INSERT（调用方事务已开启时加入之＝原子审计同事务承诺，§3.2）。
- `VerifyAuditChain(db) (ChainReport, error)`：层一五检出——断链（双关系）、重复（event_id/UNIQUE 复验）、分叉（同一 (aggregate_id,seq) 双摘要＝DB 主键下不可能出现⇒检出面＝验证器自算摘要 vs 存储摘要不符，单写者下=损坏信号报 ERROR）、聚合根内事件缺失（seq 空洞）、链头回退（同 aggregate_id 出现 seq 倒退行序＝主键序下不可能⇒检出面＝chain_digest 递推在 max(seq) 处不闭合）；输出 per-aggregate 链头集。
- `ComputeRoot(heads) (algorithm_id, leaf_count, root_hex)`：shz-merkle-v1 实现（§1.4 规则逐条）。
- canonical 序列化器 `encodeCanonicalEvent`：F0b-1 §2 编码表 Go 形态（presence 标签/声明序位置化/文法守卫/NOT_CONFIRMED 错误上抛）。
### 2.4 aggregate_open（DC §3.4 两层界定的诚实形态）
`OpenAggregate(db, aggregateID, aggregateType, actor)`＝AppendAuditEvent(event_type="aggregate_open", payload_family="aggregate_open"——**该族字段表在 B03 件为 declared-pending**⇒本波 payload 只含声明序两字段 {actor, reason?}，**登记为 v1 附录只增族**，向量补齐属后续事件，不标 verified）。层二声明：整根消失检测**不宣称具备**（无锚定/独立登记在位，D31① 设计能力≠当前能力）。
### 2.5 verify-chain verb（mc/verbs/verifychain.go＋main.go dispatch 面）
`shz verify-chain [--aggregate <id>]`：OpenSpineRead 只读入口⇒VerifyAuditChain⇒ComputeRoot⇒JSON 输出 {algorithm_id, leaf_count, root_hex, aggregates:[{aggregate_id, seq_max, chain_digest, status}], errors:[...]}。非空链头集 evidence_digest=链头集文件摘要（§7.1 绑定形状＝输出字段预留，验证记录落盘属 §7.1 流程非本 verb）。
### 2.6 S3 迁移步骤（registry 第三步）
S3=add-audit-events-chain：from=b02BaselineIdentity(X=eb7ebcf9…字面量钉定，同 S1.to 纪律)⇒CurrentSchemaID（符号，registry 末步）；语句＝§2.1 audit_events DDL＋三触发器。**S2.to 同步钉字面量 X**（B02 纪律延伸：历史步骤端点不随 release 漂移）。
### 2.7 B01 migrate step 的 Class A 映射行落地
applyMigrationStep 事务内追加 audit_events 链行（event_type="schema.migration.step"，payload_family="migration"——declared-pending 族同 §2.4 处理，payload={step_name, from_schema_id, to_schema_id, version}）⇒DC §3.2 映射表首行建成（schema 当前值↔迁移事件同事务原子）。

## 3. 正清单
| 文件 | 动作 |
|---|---|
| mc/substrate/schema.sql | ＋audit_events DDL＋三触发器（§2.1） |
| mc/substrate/auchain.go | 新建：序列化器＋AppendAuditEvent＋VerifyAuditChain＋ComputeRoot＋OpenAggregate |
| mc/substrate/auchain_vectors_test.go | 新建：vectors.json 30 例＋root_vectors.json 11 例符合性（嵌 go:embed 读 docs/design/vectors/） |
| mc/substrate/auchain_test.go | 新建：W 族链机制测试 |
| mc/substrate/migrate.go/migrate_queries.go | ＋b02BaselineIdentity 常量；S2.to 钉字面量；＋S3 步骤＋S3 语句常量 |
| mc/substrate/migrate_apply.go | applyMigrationStep 事务内＋audit_events 链行（§2.7） |
| mc/verbs/verifychain.go | 新建：VerifyChain verb |
| mc/cmd/mc/main.go | ＋verify-chain 顶层命令（withSpineRead） |
| mc/verbs/verifychain_test.go | 新建：verb 层测试 |
| evidence/B07/diff-plan-001.md | 本设计件 |

## 4. 验收证据计划（F1＝链测试全套）
- **W1** 序列化符合性：vectors.json 30 例逐条（expect_error 例断言 NOT_CONFIRMED 上抛；正常例断言 canonical hex＋event digest 与向量逐字相等）。
- **W2** 根算法符合性：root_vectors.json 11 例（R7/R8 错误例断言显式拒绝；R1-R6/R6b/R9/R9b 断言 root hex 逐字相等）。
- **W3** genesis 域分离：不同 aggregate_type⇒不同 GENESIS⇒跨类型移链被 verify 检出（G-genesis-run 向量同型）。
- **W4** 链递推：同 aggregate_id 三事件⇒chain_digest 逐跳复算闭合＋prev_hash 双关系成立。
- **W5** seq 分配确定性：并发事务下 MAX+1（BEGIN IMMEDIATE 串行化）无跳号无重复（seq_prefix 触发器负控：直接 INSERT 空洞 seq⇒ABORT）。
- **W6** append-only 触发器负控：UPDATE audit_events⇒ABORT；DELETE⇒ABORT。
- **W7** 断链检出：篡改一行 chain_digest（绕过触发器＝直接写库文件后重开）⇒VerifyAuditChain 报断链 ERROR，其余聚合根仍出链头。
- **W8** 摘要不符检出：篡改 event_digest 存储值⇒自算≠存储⇒损坏信号 ERROR。
- **W9** verify-chain verb：干净 spine⇒EXIT 0＋JSON 形状断言（algorithm_id="shz-merkle-v1"、leaf_count、root_hex）；空表⇒EMPTY_ROOT 且 leaf_count=0（R1 同律）。
- **W10** aggregate_open：OpenAggregate 首行 prev_hash=GENESIS＋verify 过；层二声明面＝测试断言验证器输出**不含**整根消失检出字段（能力不宣称）。
- **W11** S3 迁移：B02 identity spine（S2 已记录）⇒Migrate⇒audit_events 表存在＋S3 ledger 行＋meta stamped new identity；全链 pre-ledger spine⇒S1+S2+S3 顺序应用。
- **W12** B01 映射行：迁移步骤同事务含 audit_events 行（迁移后 spine 的 schema.migration.step 链行存在且摘要复算闭合）。
- 全量 lane mc/check.sh B07_FINAL_MC_EXIT=0（终态树）。

## 5. 范围边界（诚实，D31① 同源）
- **declared-pending payload 族**（lifecycle/transition/lease/migration/aggregate_open/activity/signoffs）：本波只建 aggregate_open＋migration 两族的最小形态（字段表=本 plan §2.4/§2.7 登记，向量补齐属后续事件）⇒这两族链行**不标 verified**，只标 registered-pending-vectors；B03 已注册三族（usage_observation/usage_settle/admission）有向量但**本波无生产写入面**（usage 结算=BL/C 轨，admission=IG 轨）⇒序列化符合性由 W1 向量测试承担，链上首现属后续票。
- **整根消失检测（层二）＝不宣称具备**：无已锚定链头集合/独立保全登记在位（D22/IG 产物），aggregate_open 只登记生命周期（DC 原文义务）。
- **外部完整性＝不引入签名**（DC §3.4 外部完整性分工原文：锚定/撤销/存证=D22/IG）。
- **§7.5 链连续性**：机制建成并验证后由"目标验收要求"转为已建能力——本波 F1 达成即转换条件；恢复程序依赖链验证的路径属 §7.5 后续事件（本波不动恢复程序）。
- verify-chain 走只读入口（OpenSpineRead）⇒不创建 WAL sidecar（读 verb 纪律同 lease/wiki-state 族）。

## 6. 风险与回滚
- 序列化 Go 实现与 reference_encoder.py 口径漂移⇒W1 30 例逐字节断言先跑（失败即修，不以"近似"通过）。
- schema identity 变更（S3）⇒今日零已部署 spine⇒无兼容对象；回滚=revert 提交（identity 复原，registry S3 消失⇒B02 世代 release 对新 spine 报 unknown⇒拒绝，无数据丢失路径）。
- B01 migrate_apply.go 改动面＝既有已 EVIDENCED 机制⇒改动最小化（事务内单条 INSERT 追加），T1-T9 全族回归由全量 lane 承担。
- main.go dispatch 面新增命令⇒B05 gate 枚举器自动纳入（verify-chain 走 withSpineRead⇒不贡献写集⇒registry 无需条目；若误判则 G2 幻影检查会红出）。

# C03 diff-plan-001 — usage 家族解析＋observation 身份生成＋持久序（先列后改）

- 基线：shz/TC03 @ 4223f26（dev tip，B07 收口后）
- 契约锚：PS §4/§4.1（冻结语义）＋PS-F1①（AM §4c 崩溃窗）＋DC §3.1/§3.2（聚合模型＋Class A usage 事件）
- 依赖已证：C02✓（provider-transport 单点）／B01✓（迁移 runner）／B03✓（usage_observation 冻结 payload 表＋25 向量）／B07✓（audit chain＋AppendAuditEvent＋S3 步骤）

## 0. 现面事实（E1，逐条出自实读）

1. `resident/src/provider-proxy.ts:467` `requestId = randomUUID()`＝本地请求身份，请求发出前生成；随响应头 `x-culler-request-id` 与错误体返回。`:40-58` `ProviderRequestObservation`（request_id/attempt_id/provider_request_id/http_status/…）；`:101,:491` `observe?()` 注入缝；`resident/src/main.ts:293-294` `observeProviderAccess` → `runMc(["connection","proxy-observe"], JSON)`。
2. 响应体流（`:660-790`）：错误体经 `readBounded`（`:394`，maxErrorBytes）有界读取；**SSE**（text/event-stream）经 `ProviderEventEvidence` 逐块扫描（现有执行位，检出 provider 错误码）；**JSON 成功体＝纯流式透传**（pull 循环逐块 filter+normalize+enqueue，无完整体驻留点）⇒ usage 解析对 JSON 路径须新增有界缓冲。
3. `provider_request_id` 现取法（`:684`）＝上游头 `request-id`/`x-request-id` 经 `safeCode`（缺则 undefined）⇒ provability 三分的输入已在。
4. schema：三文件无 usage/observation 表。`substrate.go:37` `CurrentSchemaID = sha256(schema.sql + "\n" + delivery.sql + "\n" + environments.sql)`。
5. `migrate.go` registry＝S1(preLedger 5c56d313⇒362e88ea)／S2(362e88ea⇒eb7ebcf9…3bcd)／S3(add-audit-events-chain, eb7ebcf9…3bcd⇒CurrentSchemaID 符号)。B07 时代 identity **Y = 4a3159e3cef11986fd68eac67c0d53f7cadc6a992664b9f61b7c5a266150bf9a**（AM v0.2.15 登记，方法自校验复现 X 逐字节相等）。
6. B03 冻结 payload（`docs/design/vectors/reference_encoder.py:175-188`，声明序 output 先于 input，**故意非字典序**，冲突以 f0b-serializer.md 为准并报缺陷）：
   `model_id(str,req) / output_tokens(dec,req) / input_tokens(dec,req) / provider_request_id(str,opt) / local_request_id(str,req) / origin(enum provider|local, opt) / event_seq(dec_int, opt) / source_version(dec_int, opt) / report_kind(enum cumulative|incremental, opt) / finality(enum interim|final, opt) / identity_provability(enum native|cursor|none, opt) / usage_family(str, opt)`。`vectors.json` 含 25 例 usage_observation。
7. 向量符合性测试先例＝`mc/substrate/auchain_vectors_test.go`＋`check.sh` 的 `-count=1` 外部文件读取行（B07 落地形态）。
8. B07 DDL 风格先例（schema.sql:39-105）：hex64 双围栏（`length(x)=64 AND length(CAST(x AS BLOB))=64 AND NOT GLOB '*[^0-9a-f]*'`）＋time 27 字符 GLOB 围栏＋append-only 双 trigger（UPDATE/DELETE ABORT）＋`json_valid` CHECK。
9. B05 门禁（`mc/verbs/error_semantics_registry.json`＋gate 测试）：写路径枚举自 cmd/mc/main.go 源文本（withSpine 直达 case 段或经 command 函数可达段）⇒ **新写 verb 缺登记必红（G1）**。
10. 发布闭包不变量（AGENTS.md）：生产只跑 host_release.go/runner_release.go 列出的文件⇒ resident 新文件须登记。

## 1. 设计裁定（本票决策面，登记）

- **D1 两相持久**（PS §4.1⑥ 位点顺序＋PS-F1① 崩溃窗）：相 1＝原始事件落盘（独立 verb，自持事务）；相 2＝observation 派生（独立 verb，幂等）。崩溃窗=相 1 已落、相 2 未落⇒重启从 raw 重派生，去重键保证不双账。单一原子事务不产生"raw 已落 obs 未落"态，F1① 验收场景须两相才有意义。
- **D2 event_seq 分配=mc 侧相 1 首次落盘时**（`max(seq)+1` 于单写者事务内，DC §3.1 序列规则同族）；重复投递同一输入返回既有 seq，**不重新赋号**（PS §4.1⑥ 末句）。序号状态持久化边界=usage_events 表本身（PS §4.1③），不另建 cursor 表。
- **D3 审计链聚合**：aggregate_id=`usage`（aggregate_type=`usage`）。首个 observation 追加时若聚合未开，同一写事务内先 `OpenAggregate`（actor=resident proxy，reason=usage-observations）——免人工开聚合步骤；层二声明面照 B07 纪律（OpenAggregate 只登记生命周期）。
- **D4 解析执行位=resident**（PS P6：拒绝/判定必须落到 parse 执行位）：SSE 复用 `ProviderEventEvidence` 模式新增 usage 事件采集；JSON 成功体新增有界缓冲（新 limit 常数，超限时=该请求 usage 记 `unavailable` 事实，**不阻断响应透传、不改错误语义**——缓冲失败面≠provider 错误面）。
- **D5 origin 红线**（PS §4.1① 终审红线）：mc verb 拒绝 `origin=provider` 而 `provider_request_id` 为空的载荷（provider 原生身份必须来自 provider 请求 ID，本地生成身份永远 `origin=local`）。
- **D6 状态枚举（v1 三面）**：`reported`（provability∈{native,cursor} 的正常事实）／`unknown`（provability=none：增量事实直入，禁入 confirmed——PS §4.1⑤）／`reconciliation`（同身份异内容：UNKNOWN/RECONCILIATION，禁 LWW——PS §4）。BL 消费行为（④）归 BL 票面，本票只产状态与身份。
- **D7 表形=v1 积累形**（B09 迁移源）：usage_events=raw blob 积累（"usage 先以 blob 积累"，DEV-BASELINE §4 原句）；usage_observations=身份+事实结构化行。B09 的 v1→v2 回填在此形上做，**v2 形状不预建**。
- **D8 supersedes**：累计快照按 (request_identity, event_seq) 槽内 source_version 递增 supersedes（PK 含 source_version⇒旧版本行保留=append-only 历史，消费方取 max version）；partial→final 关系记 `supersedes_observation_id`（PS §4 生成侧保持）。
- **D9 同身份异内容**：相 2 载荷的 payload_digest 与既有 (identity, seq) 行冲突且版本不递增⇒拒写并记 `reconciliation` 态行（新 source_version=既有 max+1，state=reconciliation，内容不覆盖旧行）——禁 LWW 的机制形。

## 2. 正清单（只触这些文件）

Go（mc/）：
1. `mc/substrate/schema.sql` — 新增 DDL 块（草案见 §4）：`usage_events`（raw 积累，PK (request_identity, event_seq)，append-only 双 trigger）＋`usage_observations`（PK (request_identity, event_seq, source_version)，state 三值 CHECK，hex64 围栏 payload_digest，append-only 双 trigger）＋索引。
2. `mc/substrate/migrate.go` — 新常量 `b07BaselineIdentity`（Y 字面量 4a3159e3…50bf9a）＋S3.to 由 `CurrentSchemaID` 符号改钉 `b07BaselineIdentity` 字面量＋S4 `newMigrationStep(4, "add-usage-observations", b07BaselineIdentity, CurrentSchemaID, usageObservationsDDL)`（registry 末端唯一引用符号=既有约定）。
3. `mc/verbs/usage_observation.go`（新）— `UsageRecordEvent`（相 1）＋`UsageRecordObservation`（相 2）verb＋wire 结构；相 2 事务内：observation 行＋audit chain 行（payload 按 B03 冻结表 shz-canon-v1 序列化，AppendAuditEvent 并入本事务）＋必要时 OpenAggregate（D3）；R-03 原子审计（DC §3.2 映射表 usage 行）。
4. `mc/cmd/mc/main.go` — 新命令面 `usage`，子命令 `event`／`observation`（host scope，connection 族同类；resident 经 runMc 两参调用）。
5. `mc/verbs/error_semantics_registry.json` — 新增两条写路径六维登记（UsageRecordEvent／UsageRecordObservation；含 d 面=相 1/相 2 分裂态、对账义务＝重启重派生）。
6. `mc/substrate/usage_observation_vectors_test.go`（新）— B03 25 例 usage_observation 向量符合性（Go payload 结构→shz-canon-v1 canonical hex 逐字节 vs expected_event_digest；-count=1 外部读取）。
7. `mc/substrate/usage_observation_test.go`（新）— 机制测试族：seq 首落分配＋重复投递不重号（D2）／origin 红线拒（D5）／provability=none⇒unknown（D6）／同身份异内容⇒reconciliation 不 LWW（D9）／supersedes 版本递增（D8）／append-only trigger 负控／相 2 事务含 chain 行（R-03）／OpenAggregate 自动开聚合（D3）。
8. `mc/check.sh` — 新增 usage 向量符合性 `-count=1` 行（B07 先例形态）。
9. `mc/deployment/host_release.go` — 登记 `resident/src/usage-observation.ts`（闭包测试兜底）。

resident（TS）：
10. `resident/src/usage-observation.ts`（新）— 五家族解析器（openai-chat：缺最终 usage chunk⇒UNKNOWN 态非零值，final chunk=请求唯一末次累计快照；openai-responses：completed 缺失/重复/迟到按版本规则收敛；anthropic：message_start input＋message_delta output 组合⇒单请求一条累计快照，中间态 interim supersedes；google：usageMetadata 完整性，缺字段=partial 事实非估算；unknown 家族：不猜不估算，只产 interim 不可判观测——PS §4 逐家族终审验收面）＋身份装配（local_request_id=proxy requestId；provider_request_id=上游头；provability 三分判定）＋两相 verb 调用＋启动恢复（查 raw 无 obs 的事件⇒重派生，PS-F1①）。
11. `resident/src/provider-proxy.ts` — SSE 扫描面扩展（usage 事件采集，同 ProviderEventEvidence 模式）＋JSON 成功体有界缓冲（新 limit 常数）＋parse 执行位接线（响应仍逐字节透传不变，缓冲仅为旁路解析）。
12. `resident/src/main.ts` — usage-observation 模块接线（启动恢复＋逐响应持久化；复用 runMc 面，观察缝 observe 同族）。
13. `resident/src/usage-observation.test.ts`（新）— 五家族矩阵＋身份/去重/provability＋usage 内容零凭据泄漏断言＋缓冲超限=unavailable 事实不阻断透传。

## 3. 负清单（不触）

- `resident/src/provider-transport.ts`（C02 冻结执行面）／目的地策略／凭据面（§3）
- dispatch／Decide／prepare-attest-commit seam（wire 形状）
- `mc/substrate/auchain.go`（审计链机制只消费不改）
- B05 gate 测试与枚举器形状（只加 registry 条目）
- activity／dispatch／connections 既有表
- openwiki/（Scribe 域）／docs/contracts（治理域）
- usage_settle payload 族与结算（BL/C05 域）；admission 族（IG 域）
- release 面（publish.py／install.sh／release.yml 零改动）

## 4. DDL 草案（实施时在此形内细化列级）

```sql
-- usage_events — raw provider usage events, accumulated as blobs (C03 v1).
-- The dedupe key per PS §4.1②: (request_identity, event_seq); seq is
-- assigned by the writer on first persistence and never reassigned.
CREATE TABLE usage_events (
    request_identity    TEXT NOT NULL,           -- provider_request_id when native, else local_request_id
    identity_origin     TEXT NOT NULL CHECK (identity_origin IN ('provider','local')),
    local_request_id    TEXT NOT NULL,           -- proxy-generated, stable across restarts
    provider_request_id TEXT,
    native_event_id     TEXT,
    event_seq           INTEGER NOT NULL CHECK (event_seq >= 1),
    usage_family        TEXT NOT NULL,
    raw_json            TEXT NOT NULL CHECK (json_valid(raw_json)),
    occurred_at         TEXT NOT NULL /* 27-char RFC3339 UTC fence */,
    recorded_at         TEXT NOT NULL /* 27-char RFC3339 UTC fence */,
    PRIMARY KEY (request_identity, event_seq)
);
CREATE INDEX usage_events_local ON usage_events (local_request_id);
CREATE TRIGGER usage_events_no_update BEFORE UPDATE ON usage_events
BEGIN SELECT RAISE(ABORT, 'usage_events is append-only'); END;
CREATE TRIGGER usage_events_no_delete BEFORE DELETE ON usage_events
BEGIN SELECT RAISE(ABORT, 'usage_events is append-only'); END;

-- usage_observations — identity-carrying derived facts (PS §4.1).
-- One row per (request_identity, event_seq, source_version); version
-- increments supersede within the slot (D8), stale versions are kept.
CREATE TABLE usage_observations (
    observation_id            TEXT NOT NULL UNIQUE /* hex64 fence */,
    request_identity          TEXT NOT NULL,
    local_request_id          TEXT NOT NULL,
    provider_request_id       TEXT,
    event_seq                 INTEGER NOT NULL CHECK (event_seq >= 1),
    source_version            INTEGER NOT NULL CHECK (source_version >= 1),
    origin                    TEXT NOT NULL CHECK (origin IN ('provider','local')),
    usage_family              TEXT NOT NULL,
    model_id                  TEXT NOT NULL,
    output_tokens             TEXT NOT NULL,     -- decimal string, lexical value survives to the digest
    input_tokens              TEXT NOT NULL,
    report_kind               TEXT NOT NULL CHECK (report_kind IN ('cumulative','incremental')),
    finality                  TEXT NOT NULL CHECK (finality IN ('interim','final')),
    identity_provability      TEXT NOT NULL CHECK (identity_provability IN ('native','cursor','none')),
    state                     TEXT NOT NULL CHECK (state IN ('reported','unknown','reconciliation')),
    payload_digest            TEXT NOT NULL /* hex64 fence */,
    supersedes_observation_id TEXT /* hex64 fence, NULL when absent */,
    occurred_at               TEXT NOT NULL /* 27-char fence */,
    recorded_at               TEXT NOT NULL /* 27-char fence */,
    PRIMARY KEY (request_identity, event_seq, source_version)
);
CREATE INDEX usage_observations_local ON usage_observations (local_request_id);
CREATE TRIGGER usage_observations_no_update BEFORE UPDATE ON usage_observations
BEGIN SELECT RAISE(ABORT, 'usage_observations is append-only'); END;
CREATE TRIGGER usage_observations_no_delete BEFORE DELETE ON usage_observations
BEGIN SELECT RAISE(ABORT, 'usage_observations is append-only'); END;
```

## 5. 验证 lane

- L1 mc 单测族（§2.7）＋向量符合性（§2.6，25 例逐字节）。
- L2 `mc/check.sh` 全量（含新 -count=1 行）⇒ 最终干净 lane exit 0（shell 契约＋全 Go 包＋零 FAIL 零 UNVERIFIED）。
- L3 resident `bun test` 全量（新测试族＋provider-proxy 既有测试零回归＋main 接线面）。
- L4 host_release 闭包测试（mc/check.sh 内含）。
- L5 B05 gate：registry 两新条目齐备（G1 缺登记必红轨迹实证后转绿）。
- L6 blcheck：图零改动（C03 已在 DEV-BASELINE 图内）⇒ 标准门禁复跑 0 violations。
- 诚实边界（登记）：真实 provider 流端到端=F0a/CI 域（本票=合成流矩阵）；usage_settle 族=BL 域不产；B09 v2 形状不预建；218 真宿主面=候令。

## 6. STOP 条件（STOP-1 呈报，不猜）

- S1 家族解析须改 `provider-transport.ts`（C02 冻结面）⇒ 停，呈报触面。
- S2 25 向量与现行 Go 序列化器行为冲突⇒ 按 B03 纪律报 spec 缺陷（代码与 f0b-serializer.md 冲突以该文件为准），不改向量。
- S3 B05 gate 枚举器不覆盖 `usage` 命令形态⇒ 调整本票 verb 形态对齐既有 withSpine/command 模式，不改 gate。
- S4 有界缓冲引入 JSON 响应透传行为变化（时延/背压）超出"旁路解析"边界⇒ 停，呈报。

## 7. 登记与收尾

- 实施=单/少提交于 shz/TC03；全道绿后 ff 并 dev（合并前全道硬规则）；登记轮=AM＋status.json C03=EVIDENCED（或 CODED→EVIDENCED 依证据形）；blcheck 留痕入 evidence/C03/。
- 本票落地后解锁：B09（usage v1→v2 首迁移＋回填证明，审方指定首战）／C05／C08。

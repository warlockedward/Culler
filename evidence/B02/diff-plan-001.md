# B02 v1 基线冻结 · diff-plan-001（S5 两 token＋S2 registry 步骤＋S1.to 字面量钉定＋v1 基线 identity 登记）

日期 2026-11-25。shz/dev @ 61e3ae3（B01=EVIDENCED[域限定]，AM v0.2.12 已登记，blcheck 0 violations）。授权链＝所有者"继续剩余任务推进"＋A09 裁定 Q5 的明示后果（"B01 迁移器即刻解锁⇒B-chain 可开工"；B02 按票面定序 B01→B02 为链内下一票）＋DEV-BASELINE B02 票面（所有者设计件）＋DC §5＋AM §1.2 登记规约。

## 0. 票面分解（DEV-BASELINE:69）
| 要素 | 权威 | 本 plan 落点 |
|---|---|---|
| v1 基线冻结 | B02 票面 | S5 两 token 落地后 CurrentSchemaID＝v1 基线 identity X，实测登记（§2.4 AM 事件） |
| identity 清扫类改动全部前置完成 | B02 票面 | S5＝A09 R2 schema 文本面最后一面，于本窗执行；其余 identity 面已在 A09（EVIDENCED[域限定]）＋B01 闭合 |
| AM 哈希登记 | AM §1.2 登记规约 | F1："登记完成＝AM 事件"⇒AM v0.2.13 轮登记 X 为 v1 基线 hash |

## 1. 现状事实（E1，file:line，B01 终态树 @61e3ae3）
1. **S5 两 token**（A09 R2 表终值已裁，AM v0.2.11 域限定登记承诺"落地时补记入 A09 证据记录"）：
   - schema.sql:1 `-- The spine stores Culler state. Constraints and triggers enforce`＝文件头注释行。SQL 注释不进 sqlite_schema⇒**只改 identity 不改对象**。
   - schema.sql:2088（trigger `activity_health_alert_outbox` 体内）`'body', 'Culler health: ' || NEW.detail`⇒**identity 与对象双改**：sqlite_schema 的 trigger sql 列存原始语句文本（substrate.go:172-191 `schemaObjects` 哈希 `(type, name, tbl_name, sql)` 行）。
2. **identity 三文件其余 "culler" 4 处＝Git 分支名面，不在 S5 范围**：delivery.sql:118/:170（trigger WHEN 子句的 `'culler'` 分支值）、delivery.sql:356＋schema.sql:2819（分支名注释）、schema.sql:235（`destination_branch TEXT GENERATED ALWAYS AS ('culler') STORED`）＝类 A 功能语义非品牌⇒A09 裁定 Q①终值（R2 表"Git 分支名保留"）。
3. **S2 语句的字节纪律**：参考侧对象＝嵌入 Schema 文本在内存 DB 应用后的 sqlite_schema（substrate.go:155-165 `currentSchemaObjects`）⇒**S2 的 `CREATE TRIGGER` 语句必须与新 schema.sql 的 trigger 块逐字节一致（token 除外）**，否则迁移后 spine 的 trigger sql 文本与参考侧分叉⇒CheckSchema 红（migrateSpine 的 post-migration CheckSchema 会先拦下，但目标是一次过绿）。
4. **S1.to 符号面问题（B02 必改点）**：S1 现定义 `newMigrationStep(1, "add-schema-history-ledger", preLedgerBaselineIdentity, CurrentSchemaID, schemaHistoryDDL)`（migrate.go:45）⇒S1.ToSchemaID 引用符号。B01 构建时符号解析＝362e88ea…（B01 基线）⇒正确。B02 改变 schema.sql⇒CurrentSchemaID=X⇒S1.to 符号解析变成 X⇒`validateRegistry` 链验证（registry[0].ToSchemaID != registry[1].FromSchemaID⇒ErrMigrationSequenceAnomaly）⇒**整个迁移机制在 B02 release 上 fail-closed 停止**⇒B02 必须把 S1.to 钉为字面量 `b01BaselineIdentity = 362e88eaaed576fc0ef26d80403069c911f2c9b1017461d8988618d3d1cbd87a`（S1 实际产出的历史身份＝历史事实，钉定符合 registry 纪律本意：步骤定义在其 release 时冻结）。
5. **终值约定**：仅最后一步可引用 CurrentSchemaID 符号（构造性终止于当前，`TestProductionMigrationRegistryTerminatesAtCurrentIdentity` 使未来身份变化保持诚实）；所有历史步骤钉字面量端点。
6. **B01-migrated spine 的交叉兼容**：stamp＝362e88ea＋ledger 记录 S1（from=5c56d313 to=362e88ea checksum=B01 值）的 spine⇒B02 registry 的 S1'（同名同语句⇒同 checksum）匹配态 2⇒跳过 S1＋计划 S2⇒交叉至 X。无 S1 重执行、无数据动作。
7. **resident fixture 面**：resident/src/discord.test.ts:408/:425 `body: "Culler health: routing unavailable"`＝fixture 构造面（fake `runMc` 返回硬编码 JSON；resident 测试只验证 payload.body 的转发行为，从不执行 trigger）⇒S5 不使其红；fixture＝自由测试数据非生产文本 pin⇒**不改**（已考虑并拒绝：R2 表 S5 行恰列 schema.sql 两 token）。
8. **今日零已部署 spine**（B01 diff-plan §1.9 在案）⇒S2 迁移路径今日无活对象；机制为首个 prod-tier 部署跨越身份边界预建。

## 2. 设计
### 2.1 S5 两 token 落地（mc/substrate/schema.sql）
- schema.sql:1 `The spine stores Culler state`⇒`The spine stores S-Horizon state`
- schema.sql:2088 `'Culler health: '`⇒`'S-Horizon health: '`
⇒ identity 362e88eaaed576fc0ef26d80403069c911f2c9b1017461d8988618d3d1cbd87a ⇒ X（实施期双探针实测，登记回填）。

### 2.2 S2 registry 步骤（mc/substrate/migrate.go＋migrate_queries.go）
```
S2 = {
  Name:         "rename-health-alert-body",
  Version:      2,
  FromSchemaID: b01BaselineIdentity（字面量，§1.4）
  ToSchemaID:   CurrentSchemaID（符号，§1.5 终值约定）
  Statement:    DROP TRIGGER activity_health_alert_outbox;
                CREATE TRIGGER activity_health_alert_outbox …（新文本，逐字节匹配新 schema.sql trigger 块，§1.3）
}
```
- S2 语句语义＝identity 边界 362e88ea⇒X 的**最小对象 delta**：只改 trigger sql 文本；注释 token（#1）不产对象，随 identity 迁移本身记录（显式登记：S2 语句不含注释操作——账本行的 from/to identity 即其证据）。
- S1 定义变更：ToSchemaID 参数 CurrentSchemaID⇒b01BaselineIdentity 字面量（Name/Statement/Checksum 不变⇒B01 已记录行仍匹配态 2，§1.6）。
- checksum 纪律不变：step_checksum＝SHA-256(name+"\n"+statement)，registry 构建期推出。

### 2.3 wire 零变更
OnboardSpineRequest/OnboardSpineResult 帧不变；Status 词表已有 "migrated"（B01）；三选分支逻辑不变（S2 只是 registry 加一步，runner/OnboardSpine/inspect 面零改动）。

### 2.4 AM 登记（F1＝AM 事件）
AM v0.2.13 轮：X 登记为 v1 基线 hash（v1 基线＝账本表＋S5 完成态）＋B02=EVIDENCED＋S5 补记入 A09 证据记录（兑现 AM v0.2.11 域限定登记的承诺句"落地时补记"）。

## 3. 正清单（B02 变更文件）
| 文件 | 动作 |
|---|---|
| mc/substrate/schema.sql | S5 两 token（§2.1） |
| mc/substrate/migrate.go | ＋b01BaselineIdentity 常量；S1.to 钉字面量；＋S2 步骤 |
| mc/substrate/migrate_queries.go | ＋S2 语句常量 |
| mc/substrate/migrate_test.go | ＋S2 测试族（§5 U1-U6） |
| evidence/B02/diff-plan-001.md | 本设计件 |

## 4. 非目标（scope 边界）
- Git 分支名面（'culler' 4 处）不动＝R2 表终值已裁。
- resident fixture "Culler health:" 不动（§1.7 已考虑并拒绝）。
- 无数据回填（B09 域）、无新 wire 字段、无新 Status 词表值。
- openwiki（Scribe 域）不登记 schema 文本变化。

## 5. 验收证据计划（B02 F1＝"登记完成＝AM 事件"；机制证据＝库内测试）
- **U1** S2 态 1 执行：B01 identity（362e88ea，S1 已记录，trigger 体＝旧文本）spine⇒Migrate⇒trigger sql＝新文本＋ledger 行 S2＋activity 事件＋meta stamped X（四者同现＝单事务断言；迁移后 CheckSchema 绿本身＝语句字节匹配证明）。
- **U2** S1+S2 全链：pre-ledger spine（5c56d313，无 ledger 表）⇒Migrate⇒S1＋S2 顺序应用⇒meta=X＋ledger 两行＋trigger＝新文本。
- **U3** 态 2 幂等：已在 X（ledger S1+S2）的 spine⇒Migrate⇒no-op，零改动。
- **U4** OnboardSpine 交叉：B01 identity spine⇒OnboardSpine（MirrorState=present，UUID 匹配）⇒Status="migrated"＋UUID 保持＋host 侧接受＋交叉后 CheckSchema 绿。
- **U5** 负控（降级方向）：只带 S1 的 registry（模拟 B01 release 视角）面对 stamp=X 且 ledger 记有 S1+S2 的 spine⇒**实测 ErrMigrationSequenceAnomaly**（"ledger records unknown step rename-health-alert-body"＝B01 registry 不认识 B02 记录的步骤⇒序列异常停止）⇒OnboardSpine 以 database-migration-blocked 拒绝（防旧 release 静默接受/重置新 spine）。
- **U6** registry 链完整：S1 字面量＋S2 符号组合下 validateRegistry 过＋`TestProductionMigrationRegistryTerminatesAtCurrentIdentity`（registry 末步 to=X=CurrentSchemaID）绿。
- **U7** identity 实测：X＝SHA-256(三文件拼接文本)，变更前后双探针。
- 全量 lane mc/check.sh B02_FINAL_MC_EXIT=0（终态树，含 S2 测试族）。

## 6. 风险与回滚
- S2 语句格式漂移（与新 schema.sql trigger 块不逐字节一致）⇒迁移后 CheckSchema 红⇒migrateSpine post-migration CheckSchema 先失败⇒交叉 fail-closed（无半态）⇒修语句重跑。负控＝U1 断言。
- S1.to 钉错（typo）⇒validateRegistry 链验证失败⇒所有迁移 fail-closed 停止（不会静默带错前进）。
- identity churn：X 为新身份⇒B01 世代 release（若存在）对 stamp=X 的 spine 报 unknown⇒拒绝（无跨版本数据丢失路径；回滚＝revert 提交⇒identity 复原 362e88ea）。
- AM 登记漂移：v1 基线 hash X 于 AM v0.2.13 轮双面 cmp 登记。

# B01 迁移器 · diff-plan-001（schema_history 进基线、OnboardSpine 内嵌、verify 窗执行、四态检查点、Q10-A 旧二进制快照时序）

日期 2026-11-25。shz/dev @ d024e36（A09=EVIDENCED[域限定]，blcheck violations=0）。授权链＝所有者"继续剩余任务推进"（A09 门控满足后的 B-chain 第一票）＋DEV-BASELINE B01 票面（所有者设计件）＋MASTER D9 决议＋DC §4/§5 契约。

## 0. 票面分解（DEV-BASELINE:68）
| 要素 | 权威 | 本 plan 落点 |
|---|---|---|
| schema_history 进基线 | D9"账本表进 v1 基线" | §2.1 DDL 入 mc/substrate/schema.sql（B01 即进基线；B02＝S5 token＋v1 身份登记，票面顺序不变） |
| OnboardSpine 内嵌 | D9 | §2.8 迁移分支嵌入 OnboardSpine（helper 侧 runtime-local） |
| journal verify 窗执行 | D9 | §2.9 结构性论证：update 流程 verify 窗以新二进制重交叉 OnboardSpine⇒迁移自动发生在窗内 |
| 四态检查点 | DC §4 P1-2 | §2.2 语义映射表＋§2.5 原子性映射 |
| Q10-A 旧二进制快照时序 | MASTER:123 item6 | §2.9 j.Previous（record+image+schema）在 committed 前全程在位＋checkpoint 卷副本＝数据回滚点 |

## 1. 现状事实（E1，file:line）
1. schema identity＝三 .sql 文件 go:embed 拼接文本的 SHA-256（substrate.go:35-38 `CurrentSchemaID`）；`meta.schema_id` 于 provisioning 时戳写（onboard_spine.go:259-261）；`CheckSchema`＝规范化 sqlite_schema 对象哈希＋meta.schema_id==CurrentSchemaID＋dispatch 标量准入（substrate.go:195-235）。
2. OnboardSpine 现行为：tables>0 且 !compatible 且未授权 reset⇒拒绝 DomainError "database-schema-differs: redeploy.sh --reset-database replaces the database"（onboard_spine.go:210-213）＝AGENTS.md 不变量"no converter exists"现状。
3. `spineInspection.schemaID` 仅在 compatible 时置 CurrentSchemaID（onboard.go:395-397）⇒迁移路径选择所需的"部署侧 identity"今日不可见＝必改点。
4. update 相序机：`updatePhases = stop|prepare|checkpoint|replace|verify|committed|complete|restore|restored`（update_journal.go:19）；`beforeUpdate`（setup_update_transaction_prod.go:139-186）：stop⇒prepare⇒checkpoint（`CreateUpdateCheckpoint` 以 `j.Previous.Image` 复制 spine 卷，update_checkpoint.go:219-249）⇒replace⇒`stageInstalled`；verify 相于新二进制整跑 `__onboard` 的 supervision 段置位（setup_published_prod.go:322/:1029/:1050），`p.complete()`＝committed。
5. **update 流程的 home 段以新二进制重交叉 OnboardSpine**：host 侧 `docker exec <helper> mc __onboard-spine`（onboard_home_prod.go:273）⇒helper 侧 `OnboardSpine()`（onboard_spine.go:188）⇒这是 D9"OnboardSpine 内嵌＋verify 窗"的既有执行路径，无需新增调用点。
6. `activity` 表＝spine 事件总线（schema.sql:664，actor/kind/subject/detail）＝Class A 迁移事件载体。
7. `distribution.Record.Schema` 字段在案（record.go:137）⇒`j.Previous.Schema`＝旧 release 的 schema identity（Q10-A 快照身份已在 journal）。
8. 预 ledger 基线 identity 实测值＝`5c56d3132ea33fb85f9204462df94396c9dfb5c4801271b9b6019ff108ba0963`（F1 step1 走查 onboard home detail "schema 5c56d313…"，e9d6912 树＝d024e36 树 schema 文件同字节）。
9. 零已部署 spine（S5 裁定记录在案：本机零＋218 零）⇒B01 的 identity churn 今日无兼容对象，但机制为未来 release 边界预建。
10. AGENTS.md 不变量行"…makes each deployed spine incompatible, and no converter exists"＝B01 落地后的活文档更正面。

## 2. 设计
### 2.1 schema_history 账本表 DDL（入 schema.sql，紧随 meta 表）
```sql
-- schema_history records every applied migration step. The runner applies
-- steps in version order; a step is skipped only when its row exists with a
-- matching checksum (DC §4 four-state semantics).
CREATE TABLE schema_history (
    version        INTEGER PRIMARY KEY,
    step_name      TEXT NOT NULL UNIQUE,
    step_checksum  TEXT NOT NULL
                   CHECK (length(step_checksum) = 64 AND step_checksum NOT GLOB '*[^0-9a-f]*'),
    from_schema_id TEXT NOT NULL
                   CHECK (length(from_schema_id) = 64 AND from_schema_id NOT GLOB '*[^0-9a-f]*'),
    to_schema_id   TEXT NOT NULL
                   CHECK (length(to_schema_id) = 64 AND to_schema_id NOT GLOB '*[^0-9a-f]*'),
    applied_at     TEXT NOT NULL
);
```
（hex 双 fence＝activity.dispatch_key 同族纪律：length＋GLOB 双检，防 NUL 截断绕过。）

### 2.2 四态语义映射（DC §4 P1-2 → runner 行为）
| 态 | DC 原文 | runner 行为 |
|---|---|---|
| 1 | step 不存在→执行 | schema_history 无该 step_name 行⇒单事务执行（§2.5） |
| 2 | 存在且 checksum 相同→跳过 | 行存在且 step_checksum==registry 值⇒跳过（并验证 meta.schema_id 一致性，§2.4-5） |
| 3 | 存在但 checksum 不同→fail-closed 停止迁移 | 行存在但 checksum 冲突⇒返回 ErrMigrationChecksumConflict，**零数据库改动**（防历史迁移文件被静默改动） |
| 4 | step 序列缺失/异常→停止并人工诊断 | §2.4 五异常条件⇒返回 ErrMigrationSequenceAnomaly，零改动 |

### 2.3 checksum 纪律
`step_checksum = SHA-256(step_name + "\n" + canonicalSQL)`，canonicalSQL＝该步骤执行的规范化 SQL 文本常量（嵌二进制）。registry 值由构建期常量推出⇒同一 step_name 的 SQL 文本被后手改动⇒registry checksum 变⇒与已落盘行冲突⇒态 3 fail-closed（DC §4 防篡改语义的机制化）。

### 2.4 序列异常五条件（任一命中⇒态 4 停止＋人工诊断）
1. 行 step_name 不在 registry（未知步骤＝registry 收缩/重排）
2. 行 step_checksum ≠ registry 同名步骤值——**例外：此属态 3（fail-closed 停止迁移），单独错误类型，不并入异常**
3. version 空洞（存在 v=k 行但缺 v<k 行）
4. 行 from_schema_id/to_schema_id ≠ registry 同名步骤值（身份篡改）
5. meta.schema_id ≠（已记录最高 version 步骤的 to_schema_id｜无行时＝基线 identity）（状态/证据分裂＝DC §5 明禁）

### 2.5 DC §5 迁移原子性映射
每个步骤＝**单事务**：步骤 SQL＋schema_history 行＋activity Class A 事件＋`UPDATE meta SET schema_id=<step.to>`，同事务提交。SQLite 事务原子性⇒进程崩溃＝事务回滚⇒步骤未记录⇒下次运行重新执行（确定性恢复，DC §5"可确定性恢复的中间状态"的平凡满足）。"schema 身份已更新而对应迁移证据缺失"分裂态在单事务下不可构造（§2.4-5 对残留分裂 fail-closed）。v1 步骤全部单事务⇒无需多事务中间态定义（机制留接口：步骤 Apply 失败⇒事务回滚⇒无半态）。

### 2.6 Class A 迁移事件（activity 行）
`actor='mc'`，`kind='schema.migration.step'`，`subject=<step_name>`，`detail="from <from_schema_id> to <to_schema_id>"`。部署级事件 project=NULL（activity 表注释在案："Deployment events use NULL"）。

### 2.7 registry 与步骤 S1
```
S1 = {
  Name:         "add-schema-history-ledger",
  FromSchemaID: 5c56d3132ea33fb85f9204462df94396c9dfb5c4801271b9b6019ff108ba0963  // 预 ledger 基线（§1.8 实测）
  ToSchemaID:   <B01 提交后 substrate.CurrentSchemaID，实施期计算回填>
  CanonicalSQL: CREATE TABLE schema_history (…)   // ＝§2.1 DDL 文本
}
```
S1 内容＝创建账本表＋记录自身行＋戳写新 identity。预 ledger spine（无 schema_history 表）的入口态＝"pre-history baseline"：表缺失且 meta.schema_id==S1.FromSchemaID⇒计划＝全序列；表缺失但 identity 不符⇒ErrSchemaUnknown（无迁移路径）。B02 落地 S5 后追加 S2（alert trigger DROP+CREATE 新文本＋基线行更新），不在本票。

### 2.8 OnboardSpine 内嵌（mc/verbs/onboard_spine.go＋onboard.go）
- `inspectSpineReadOnly`：meta 存在时**恒读** meta.schema_id 入 inspection.schemaID（不再仅 compatible 时置值）＝部署侧 identity 成为一等等检事实。
- `OnboardSpine` 现 `!compatible` 分支（onboard_spine.go:210-213）改为三选：
  1. 迁移路径存在（registry 可规划）⇒执行 runner（同一 flock 内）⇒`Status="migrated"`，**deployment UUID 保持**（区别于 reset 的新/保身份语义：迁移保身份）；
  2. 无路径（unknown identity）⇒现行为不变（拒绝 "database-schema-differs"，reset 授权时 reset）；
  3. 迁移失败（checksum 冲突/序列异常）⇒拒绝并携带判别性错误（不落入 reset＝数据完整性存疑时不得以清空了事）。
- wire 面：`OnboardSpineResult.Status` 词表 ＋`"migrated"`；host 侧 `FinalizeOnboardHome.validStatus` ＋`"migrated"`＋detail 文案 "the database was migrated to schema <new>"。同文件两侧＝matched-pair 部署下无跨版本兼容问题；按 AGENTS.md"wire shape 变更＝设计决定"登记于本 plan（票面"OnboardSpine 内嵌"明文覆盖）。
- `OnboardSpineRequest` 帧**零变更**（SchemaID 字段语义不变＝helper 构建侧 CurrentSchemaID 校验）。

### 2.9 verify 窗执行与 Q10-A 时序（结构性论证，非新调用点）
update 流程以新二进制整跑 `__onboard`（setup_published_prod.go:1037 brokerWholeOnboard）⇒home 段交叉 OnboardSpine（onboard_home_prod.go:273）⇒§2.8 迁移分支在 **verify 相期间**（supervision 段置位 verify 相，setup_published_prod.go:1029；home 段先于 supervision 段执行⇒迁移发生于 verify 相置位前的最后一段既有交叉窗内，且严格早于 `p.complete()`=committed）执行。Q10-A 旧二进制快照时序：
- 数据回滚点＝checkpoint 相以 `j.Previous.Image` 复制的 spine 卷（update_checkpoint.go:245，迁移前已就位）；
- 身份回滚点＝`j.Previous`（record+image+schema，journal 在 committed 前全程在位；restore 相经 `recoverUpdate`⇒`restoreUpdate` 恢复旧 release，update_checkpoint.go:204-265）；
- 迁移失败⇒home 段失败⇒整跑非零⇒相不推进⇒`--restore` 路径可用⇒**旧二进制快照在迁移全程有效**。
⇒verify 窗证据＝§6 测试 T8/T9。

## 3. 正清单（B01 变更文件）
| 文件 | 动作 |
|---|---|
| mc/substrate/schema.sql | ＋schema_history DDL（§2.1，紧随 meta） |
| mc/substrate/migrate.go | 新建：MigrationStep/registry/四态 runner/错误类型/步骤 S1 定义 |
| mc/substrate/migrate_test.go | 新建：四态测试族＋原子性＋异常族（§6 T1-T7） |
| mc/verbs/onboard.go | inspectSpineReadOnly 恒读部署侧 schema_id |
| mc/verbs/onboard_spine.go | 迁移分支＋"migrated" status＋Finalize 接受面 |
| mc/verbs/onboard_spine_test.go（或既有 spine 测试文件） | T7 OnboardSpine 迁移交叉＋T8/T9 verify 窗/Q10-A 证据 |
| AGENTS.md | 不变量行更正（"no converter exists"⇒"conversion requires a registered migration step; an unregistered boundary refuses"） |

## 4. 非目标（scope 边界）
- S5 两 token（schema.sql:1/:2068）不动＝B02 预冻结窗（票面硬注顺序）。
- 无数据回填（B09 域）、无多宿主/滚动升级语义（DC §5 明出范围）、无新 wire 字段（仅 Status 词表＋1，§2.8）。
- 不实现 S2 步骤（B02 身份边界画定后定义）。

## 5. 失败模式与恢复
| 失败 | 行为 | 恢复 |
|---|---|---|
| 步骤 SQL 执行错误 | 事务回滚，零改动，DomainError 携带步骤名 | 修复后重跑 OnboardSpine（步骤未记录⇒重执行） |
| checksum 冲突（态 3） | fail-closed 停止，零改动 | 人工诊断（历史迁移文件被改？registry 版本错配？）⇒不得以 reset 掩盖 |
| 序列异常（态 4） | 停止＋人工诊断错误 | 同上 |
| 未知 identity | ErrSchemaUnknown⇒OnboardSpine 现拒绝行为 | reset（操作员显式授权）或旧 release 恢复 |
| update 流程中迁移失败 | home 段非零⇒相停 verify 前⇒journal Problem | `--restore`（checkpoint 卷副本＋j.Previous） |
| 进程崩溃于步骤事务中 | SQLite 回滚⇒步骤未记录 | 下次运行重执行（确定性恢复） |

## 6. 验收证据计划（B01 F1 面＝"四态检查点＋verify 窗证据"）
- **T1** 态 1：预 ledger spine（合成 DDL，无 schema_history）＋S1 计划⇒执行后表存在＋行存在＋activity 事件存在＋meta.schema_id==S1.To（单事务断言：四者同现或同缺）。
- **T2** 态 2：已记录 S1⇒重跑 runner⇒no-op（零新行、meta 不变、第二次幂等）。
- **T3** 态 3：篡改已落盘行 step_checksum⇒runner⇒ErrMigrationChecksumConflict＋数据库字节级无改动（前后 schema 对象哈希比对）。
- **T4** 态 4a：删除中间 version 行（空洞）⇒ErrMigrationSequenceAnomaly。
- **T5** 态 4b：植入未知 step_name 行⇒ErrMigrationSequenceAnomaly。
- **T6** 未知 identity：meta.schema_id＝registry 外值⇒ErrSchemaUnknown（OnboardSpine 回落现拒绝文案）。
- **T7** OnboardSpine 内嵌：以 S1.From identity 造 spine⇒新二进制 OnboardSpine⇒Status="migrated"＋UUID 保持＋host 侧 FinalizeOnboardHome 接受（含 detail 文案断言）。
- **T8** verify 窗证据：journal（phase=verify，j.Previous 在位含 Schema+Image）＋部署 spine＝S1.From identity⇒跑 OnboardSpine 交叉（update 流程 home 段的同一入口）⇒迁移成功且 journal 仍 restorable（phase 未 committed、Previous 未清）⇒断言 Q10-A 时序＝迁移发生于旧快照有效窗内。
- **T9** 迁移失败保持 restore 可用：态 3 spine＋journal（phase=verify）⇒OnboardSpine 交叉失败⇒journal phase 不推进（无 committed）⇒restore 路径入口仍开放（recoverUpdate 语义：!Committed()⇒restore 可选）。
- 合成 spine 的 identity 值＝测试内自算（DDL 文本 SHA-256），不依赖生产 identity 常量（防 pin 漂移）。

## 7. 风险与回滚
- identity churn：B01 提交后 CurrentSchemaID 变⇒已用预 ledger 树 provisioning 的 spine（今日零）将走 S1 迁移；回滚＝revert 提交（identity 复原，spine 无需动作）。
- OnboardSpine 拒绝面收窄（"differs"分支多一条迁移路径）：负控＝T6（unknown 仍拒）＋T9（fail-closed 不落入 reset）。
- AGENTS.md 不变量行更正＝活文档面，随 B01 提交同改。

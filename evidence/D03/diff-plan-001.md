# D03 DIFF-PLAN-001 · legacy:operator 迁移＋映射表＋封存

票面（DEV-BASELINE）：legacy:operator 迁移＋映射表＋封存｜deps D01｜契约锚 IG §7；IG-T5｜验收 P4.0：零改写比对＋封存拒写测试。
基线：dev tip bf7c6cf；schema 基线 identity X＝6be6f49b…c535（S7）——本票触 schema 面（S8⇒新基线 identity）。

## 0. 实施普查结论（先列后改的事实基础）

- **activity actor='operator' 生产写点＝21**：collie.go×7（collieEndTx 调用×4＋collie.resumed 字面×2＋collie.deleted 字面×1）、task.go×5（block/unblock/resume/cancel×2）、board.go×1（共享 actor 变量⇒同时流 board_operations/requirements_revisions）、packet.go×1、customization.go×2、environment.go×1、systemstatus.go×1、allowances.go×1（task.correction_allowance）、identity.go×1（**D01 自身 bootstrap user.created——一并迁**）。
- **控制谓词仅一处值同步**：domain/task.go:210 Cancel `actor == "operator"` ⇒ 随迁为 `"owner"`（`collie:<runID>` 前缀分支不动）；domain.BlockOperator="operator" 是 **block kind 分类学值**（非 actor，不动）；requirements.go:243 returned_kind='operator'（分类学，不动）；RequireOperatorTaskControl＝stage 判定（无 actor 值逻辑）；requireOperatorVerbTx＝provenance 判定（无 actor 值逻辑）。
- **非 activity 面（登记候后续票，非封存对象）**：Diagnostic.Source='operator'×4（connectionhealth×2/colliereplystate×2）、evalcase.Reviewer='operator'×1＝描述性标签非事件脊归属；'collie:<runID>' 复合 actor＝独立类别（agent-via-collie）不封存。
- **测试面**：4 文件含 'operator' 字面种子（onboard_spine/brief_history/dispatchverb/reads）；board_unblock_test×2 断言 actor='operator'（board_operations＋activity task.resumed）；dashboard TS 面 actor 不透明渲染（无谓词依赖；configs 的 "operator"＝配置层维度，异面）。
- **迁移簿记行 actor='mc'**（migrate_apply.go:141）⇒S8 不自我封存。
- **版本偏斜分析**：单二进制权威（mc 唯一开脊）＋更新前向原子（新二进制首次运行才应用 S8，旧二进制不回归）⇒无条件封存无偏斜锁窗。

## 1. 设计裁定

- **D1（值面裁定，呈报可覆）**：live 操作员控制面 actor 迁移目标＝**'owner'**（现行具名占位：部署所有者经 CLI 面的主体类别；与机器 actor 词汇 mc/dispatch/角色系区分；谓词稳定；detail prose 已承载"the operator ..."人语叙述）。**否决后继 user ID 直上（IG 纯形）＝当前面有决定性缺陷**：dashboard/Collie 身份面未立（D01/D02 具名稳态后续票）⇒无 user+mapping 登记路径⇒封存生效即锁死全部操作员控制动词（违反 IG §8"治理拒绝权不越界为无边界停摆权"）。**后继 user ID 接线＝具名稳态后续票**（身份面立票时：'owner' 写点解析 legacy_actor_mapping.mapped_user_id，映射表从登记性变为承载性；'owner' 占位退役）。
- **D2**：封存＝**无条件表级**（S8 落地即生效，含 fresh 脊）：`activity_legacy_actor_sealed` BEFORE INSERT WHEN NEW.actor='operator' ⇒ ABORT。"无默认兼容路径"（IG §7）⇒不采用数据驱动（映射行存在才封）的延迟封存。
- **D3**：legacy_actor_mapping＝**Class A 登记表**（PK legacy_actor；mapped_user_id FK users NOT NULL；basis/nature/strength NOT NULL；evidence 可空[独立证据⇒更强归因，IG §7]；created_at SpineTime；no_update/no_delete 双 trigger；mapped_user_id 索引[反向查]）。注册 verb＝IdentityLegacyMap（host scope，guard 绑定写入＋Class A 事件）；证据字段登记时填充，登记后追加面候后续票（Class A 不可变与 IG"按映射行的证据字段追加"的张力＝诚实边界，不扩面）。
- **D4**：guard 双点＝Go 层 recordActivity 拒 actor='operator'（identity/verbs 平面 fail-closed）＋SQL 层 trigger（兜底一切写者）——IG §7"guard 与 trigger 双点拒绝"逐点落实。
- **D5**：历史行**零改写**（IG §7 红线）：S8 只加对象不改 activity 存量行；T5 零改写证明＝B08 N2 先例（按 id 界 digest 比对——S8 自身簿记行 actor='mc' 合法新增，界外不计）。

## 2. 正清单（文件）

- `mc/substrate/schema.sql`：S8 身份块＝legacy_actor_mapping 表＋2 Class-A trigger＋1 索引＋activity 封存 trigger（5 对象）。
- `mc/substrate/migrate_queries.go`：＋s8 语句常量（逐字节等同 schema.sql 块）。
- `mc/substrate/migrate.go`：＋b08…即 xD02Identity 常量（X 钉定字面量）＋S7.to 钉 X＋S8 add-legacy-actor-sealing（X⇒CurrentSchemaID 符号）。
- `mc/substrate/legacy_seal_test.go`（新）：S8 步骤族（I1 落地/I2 逐字节）＋strip helper（供既有测试族复用）。
- `mc/verbs/identity.go`：＋IdentityLegacyMap verb（host scope，guard 绑定 INSERT，重复注册⇒PK ABORT⇒域错）＋recordActivity 增 guard 点（actor='operator'⇒Domainf 拒）。
- `mc/cmd/mc/identity_command.go`＋`main.go`＋`systemstatus.go`：＋legacy-map 子命令路由（withSpine）。
- live 面迁移（'operator'⇒'owner'）：verbs/collie.go×7、verbs/task.go×5、verbs/board.go×1、verbs/packet.go×1、verbs/customization.go×2、verbs/environment.go×1、verbs/systemstatus.go×1、domain/allowances.go×2（activity＋RequestTaskRevision）、domain/task.go:210 谓词、verbs/identity.go:61。
- 测试面随迁：board_unblock_test×2 断言⇒'owner'；brief_history/reads/dispatchverb 种子⇒'owner'（incidental fixture 非 legacy 模拟）；onboard_spine 4 种子＝**真 legacy 模拟**⇒保持 'operator'，脊改 pre-S8 供给后迁移（顺带实证 onboard 路径携带 legacy 行过 S8 零改写）。
- registry 8 步全族更新：migrate_test（preLedger/b01/no-op）、usage_observation_test U13、activity_chain_test b08Spine+N1、auchain_test W11+W12、onboard_migration_test（strip×2＋ledger 7⇒8＋注释）。
- `mc/verbs/identity_ig_t5_test.go`（新）：T5 测试族（§4）。

## 3. 负清单

- 不改 block kind／returned_kind 分类学值（'operator' 作为**原因类别**存续）；不动 'collie:<runID>' 复合 actor；不迁 Diagnostic.Source／evalcase.Reviewer（登记候票）；不建 owner 标记列／不做 user 解析（后继接线＝后续票）；无 S9；TS 面零触碰（actor 不透明渲染）；不扩 grants 语义（封存例外"显式 grant＋限期"＝IG 保留条款，当前面无人需要⇒不建机制，登记）。

## 4. T5 测试族（验收句＝"零改写比对＋封存拒写测试"）

- **T5-1 零改写比对**：pre-S8 供给脊＋种子（'operator' 行×N＋混合 actor＋mapping 无关数据）⇒活动行全列 digest（id 界）⇒Migrate 应用 S8⇒digest 相等＋'operator' 行逐字在位＋簿记行（'mc' schema.migration.step）界外合法。
- **T5-2 封存拒写**：post-S8 脊⇒raw INSERT actor='operator'⇒ABORT（"sealed"）；recordActivity('operator',…)⇒Go guard 先拒（零 SQL）；'owner'/'mc' 写放行。
- **T5-3 证据字段核验（IG-T5 点名句）**：IdentityLegacyMap 注册 legacy:operator⇒alice（basis/nature/strength/evidence 四值）⇒行逐字 round-trip；重复注册⇒域拒；UPDATE/DELETE⇒Class A trigger ABORT。
- **T5-4 live 面迁移行为**：board unblock（actor='owner' 断言×2 面）＋task cancel（谓词面：'owner' 触发 RequireOperatorTaskControl 门控，'mc' 不触发——谓词同步的行为证据）＋collie resume（'owner'）＋allowance（'owner'）＋bootstrap user.created（'owner'）。
- **T5-5 onboard 携带 legacy 行**：pre-S8 脊种子 'operator' 行⇒OnboardSpine 迁移（S1..S8 全链）⇒'operator' 行逐字在位＋新写被封存（legacy 模拟脊走完整迁移路径的端到端证明）。
- registry 8 步既有族（B08 模式）随更。

## 5. 验证 lane

- L1 定向（substrate S8 族＋verbs T5 族＋既有 identity/dispatch/board/collie 回归）。
- L2 mc/check.sh 全量⇒干净 lane EXIT=0（27 包）。
- L3 登记轮⇒blcheck（D03 入册后 deps D01✓⇒无 ready-unmet）。

## 6. STOP 条件

- S1 谓词同步发现 actor 值逻辑超出 task.go:210 一处（如 dashboard/TS 门控）⇒停，呈报。
- S2 任一测试以 'operator' 为**期望 live 行为**（非 legacy 模拟 fixture）⇒停，逐例呈报。
- S3 S8 应用需改既有步骤语句/端点（S7.to 钉定面之外的 registry 面）⇒停（链完整性红线）。
- S4 封存咬住非 'operator' 字面形态的 legacy 写（如参数化拼出 'operator'）⇒停（trigger 只能匹配存储值；若存在拼写路径需 Go guard 面扩，呈报）。

## 7. 登记与收尾

实施＝少提交于 shz/TD03；全道绿后 ff dev；登记轮＝AM v0.2.21＋status.json D03=EVIDENCED＋blcheck；新基线 identity 入册；诚实边界（'owner' 占位后继接线候票／Diagnostic·evalcase 词汇候票／映射证据追加候票／封存例外 grant 机制未建）入登记。

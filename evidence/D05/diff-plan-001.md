# D05 DIFF-PLAN-001 · degraded_grants 五要件表（per-operation 拒语义）

票面（DEV-BASELINE）：degraded_grants 五要件表（per-operation 拒语义）｜deps D01｜契约锚 IG §3；TM §8.1；MASTER D31②｜验收 P4.0：五要件缺项=拒 测试。
基线：dev tip 6a67043；schema 基线 identity＝bd9ebc9c…049a8（S8）——本票触 schema 面（S9⇒新基线 identity）。

## 0. 契约形态 recon（先列后改的事实基础）

- **TM §8.1 五要件**＝具体风险 × 操作范围（项目/任务/Agent/能力清单）× 补偿控制 × 有效期 × 撤销条件；"不得扩大到未列明项"；**自动失效**＝补偿失效/有效期届满/撤销条件命中⇒作废并 fail-closed；审查意见、AI 判断、历史运行事实**均不构成授权**；签发主体＝部署所有者指定的具名授权主体（主体机制归 IG＝D01 users+guard）。
- **IG §3 范围修正**：判定作用于"该项降级操作"——无匹配当前五要件的有效授权⇒**拒该降级操作**；**正常运行路径不因表空被拒**（空表＝无降级授权存在，非全局停机）；D-IG-3 决策面唯一（判定只在 mc guard Go 代码）。
- 决策/签发/撤销形态＝D01/D02 既有惯用法（guard 绑定＋trigger 归属复验＋SpineTime＋实时读判定＋issuer 自撤销）。

## 1. 设计裁定

- **D0＝schema 面（S9 add-degraded-grants，3 对象）**：degraded_grants 表＋索引＋guard INSERT trigger（D01 grants 模板：trusted_ctx 非空＋issued_by∈ctx.actors 归属复验）。五要件＝risk/scope/compensation/revocation_conditions NOT NULL（1-1024 围栏）＋expires_at NOT NULL——**存储层结构性缺项=拒**，guard 层（verb 校验）双点。
- **D1（表形）**：grant_id 32hex PK；resource 可空≤256（NULL＝不限对象）；issued_by_user_id FK users NOT NULL（具名授权主体）；created_at SpineTime default；revoked_at 可空（撤销通道，单向——无 unrevoke 面，与 grants 先例一致）。
- **D2（签发）**：IdentityDegradedGrantIssue（host scope，会话认证）：五要件全非空＋TTL>0⇒guard 绑定后 INSERT；expires_at 经 datetime('now','+N hours') SQL 内计算（SpineTime 纪律）；"不得扩大"＝签发时值原样入行，判定侧精确匹配。
- **D3（判定，per-operation）**：identity.DegradedAllows(ctx, q, risk, scope, resource, now) Decision——**实时 spine 读 only**（D02 零缓存先例）；allow 当且仅当存在行：risk 精确 ∧ scope 精确 ∧（resource NULL 通配或精确）∧ revoked_at IS NULL ∧ expires_at≥now；**任何要件缺位/失配/过期/已撤⇒deny**；审查意见/AI 判断/历史事实不入判定输入（判定只读该行集——结构性）。
- **D4（撤销通道）**：IdentityDegradedGrantRevoke（issuer 自撤销；跨方＝sign-off 路径，D01 grant-revoke 先例）：UPDATE revoked_at＝事务内 SpineTime。
- **D5（诚实边界，入册）**：①补偿控制**实效**验证＝治理义务（系统不能机器验证补偿在实时态成立⇒撤销通道是机制，"自动失效"由签发主体在证据出现时执行）；②具名授权主体的**选择**＝治理行为（机制＝users+guard，不建 Authority 表）；③SoD（申请人不得自批/审批人不得兼任执行人）＝D04 break-glass 分级双批＋D08 signoffs 平面，不在本票；④risk 值＝自由文本标识（RR-3 形），脊内无 RR 登记表⇒交叉引用在审计层，不建 FK；⑤**降级操作业务接线**（TM §8.0"实装点在 IG/PS/DT 各自的准入处"＝各准入点调 DegradedAllows）＝具名后续票——本票提供表＋判定＋签发/撤销，验收句不要求业务接线（同 D02 GrantAllows 先例）。

## 2. 正清单（文件）

- `mc/substrate/schema.sql`：S9 块（表＋索引 degraded_grants_risk_scope＋trigger degraded_grants_guarded_insert，3 对象）。
- `mc/substrate/migrate_queries.go`：＋degradedGrantsDDL（逐字节等同）。
- `mc/substrate/migrate.go`：＋d03SealIdentity（bd9ebc9c… 字面量钉定）＋S8.to 钉 d03SealIdentity＋S9 add-degraded-grants（⇒CurrentSchemaID 符号）。
- `mc/substrate/degraded_grants_test.go`（新）：stripDegradedGrants＋d03Spine＋M1（S9 单步落地：对象/ledger 9/stamp/CheckSchema/trigger 咬合）＋M2（3 对象逐字节 fresh==migrated）。
- `mc/identity/identity.go`：＋DegradedAllows（决策平面）。
- `mc/verbs/identity.go`：＋IdentityDegradedGrantIssue＋IdentityDegradedGrantRevoke（＋recordActivity 复用；legacy-map 之后）。
- `mc/cmd/mc/identity_command.go`＋`systemstatus.go`：degraded-grant-issue/degraded-grant-revoke 子命令路由（withSpine）。
- registry 9 步全族更新：migrate_test（T1/U1/U3）、usage_observation_test U13、activity_chain_test（b08Spine+N1）、auchain_test（W11/W12/closed-chain）、identity_foundation_test（b08IdentitySpine strip＋I1 applied 2⇒3）、legacy_seal_test（d01IdentitySpine strip）、onboard_migration_test（strip×2＋ledger 8⇒9＋注释）。
- `mc/verbs/identity_degraded_t5_test.go`（新，验收族）：§4 G 族。

## 3. 负清单

- 无降级操作业务接线（准入点调用面＝具名后续票）；无 RR 登记表/FK；无 Authority/SoD/signoff 面（D04/D08 域）；无 unrevoke 面；不扩 grants/user_sessions 语义；TS 面零触碰；B05 gate 源枚举面不动（新 verb 经 commandSystemSpine＝proxy-observe 先例既有边界，不登记）；main.go runLocal 清单不动（identity 族已在列）。

## 4. G 测试族（验收句＝"五要件缺项=拒 测试"）

- **G1 五要件缺项=拒（签发侧，5 子例）**：risk 空／scope 空／compensation 空／revocation_conditions 空／TTL≤0（有效期缺）——各⇒拒（Usage/Domain），零行落库。
- **G2 判定矩阵（决策侧）**：有效行⇒allow；risk 失配⇒deny；scope 失配⇒deny；resource 失配⇒deny（NULL 通配行覆盖任意 resource⇒allow 子例）；过期（假时钟）⇒deny；已撤⇒deny；**空表⇒deny**（"无匹配当前五要件的有效授权⇒拒该降级操作"）。
- **G3 空表不伤正常路径**：degraded 表空时，D01/D02 既有面（session create、grant issue、GrantAllows）照常——"正常运行路径不因表空被拒"的行为证据。
- **G4 归属与不可伪造**：issuer 自签⇒行 issued_by＝绑定 actor；无 guard 裸 INSERT⇒trigger ABORT（双围栏：无上下文／归属不符）；跨方签发（会话 A 签给声称 B 的 issued_by）⇒ABORT。
- **G5 撤销通道**：issuer 撤销⇒revoked_at 置位⇒G2 判定转 deny；重复撤销⇒域拒；跨方撤销⇒sign-off 拒（同 D01 先例文案）。
- 假时钟：DegradedAllows 的 now 参数注入（verbs 面用事务内 SpineTime，纪律不变）。

## 5. 验证 lane

- L1 定向（substrate M 族＋verbs G 族＋既有 identity/registry 回归）。
- L2 mc/check.sh 全量⇒干净 lane EXIT=0（27 包）。
- L3 登记轮⇒blcheck（D05 入册后 deps D01✓⇒无 ready-unmet）。

## 6. STOP 条件

- S1 五要件表需第 6 列或通配/前缀匹配语义⇒停（TM §8.1"不得扩大到未列明项"精确性红线）。
- S2 验收句需要任一降级操作业务接线方可实证⇒停（票面＝表＋判定＋签发/撤销，接线=后续票）。
- S3 判定平面出现 require-signoff 档⇒停（D-IG-3 单平面；signoff=D04/D08）。
- S4 S9 应用咬住既有生产路径（非测试的降级写点）⇒停（普查未见 degraded 写点；若咬住＝语义冲突，呈报）。

## 7. 登记与收尾

实施＝少提交于 shz/TD05；全道绿后 ff dev；登记轮＝AM v0.2.22＋status.json D05=EVIDENCED＋blcheck；新基线 identity 入册；诚实边界（D5 五项）入登记。

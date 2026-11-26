# D01 diff-plan-001 — 身份基础：user/session/grant/team 表＋guard 单平面＋可信上下文注入＋不可伪造性（先列后改）

- 基线：shz/TD01 @ 088e2b8（dev tip，B08 收口后，identity V）
- 契约锚：IG §1（主体模型＋关系约束）／§2（认证与会话：bcrypt＋服务端可撤销会话表＋token CSPRNG≥256bit 仅存哈希＋非秘密 session ID＋撤销三档＋双 TTL＋状态枚举＋Class A 事件）／§3（Capability 授权：D-IG-3 决策面唯一＝mc guard；trigger＝执行约束面；可信上下文注入不可伪造性＝P4.0 验收前置）＋AM §4c IG-T1（四组攻击：①直接 SQL 旁路 ②跨连接复用串扰 ③异常/事务失败清理 ④guard 与 trigger 作用域一致性）
- 依赖已证：B02✓（v1 基线冻结，S7 在其后）
- 依赖裁定：IG §2 明文 bcrypt⇒新增直接依赖 `golang.org/x/crypto`（v0.59.0 实测可达；传递升级 x/sys v0.50.0/x/text v0.44.0——供应链变更登记，TM 域候审计）；session ID 用 crypto/rand 16B hex（stdlib，不引 uuid）。

## 0. 现面事实（E1）

1. 无通用 guard 判定平面：现有 `attachment_guard.go`/`operator_guard.go` 是单面守卫（附件/操作者），非 IG §3 的 allow/deny/require-signoff 决策平面⇒D01 首建。
2. 现有身份面＝RunIdentity/loadIdentity（run.json tier：operator/agent/role）——执行身份平面；D01 建 P4 数据访问身份平面（user/session/grant/team），**I3 身份≠权柄、两平面正交**：guard 用 P4 身份做数据决策，不消费 run.json tier；现有 verb 面零触碰。
3. `inTx`（verbs/verbs.go:68）：BEGIN IMMEDIATE 于单 conn＋`authorizeOperatorTransaction`＋fn(ctx, conn-as-Q)＋COMMIT/ROLLBACK——conn 生命周期受控⇒可信上下文的绑定/清除挂载点在此。
4. B08 后：activity 已是 Class A 证据账本（迁移时点入链；稳态入链＝具名后续票）⇒D01 的会话/grant 生命周期事件落 activity 行（kind=session.*/grant.*），链上首现候稳态票（诚实边界，登记）。
5. SQLite temp 表＝连接局部（per-connection in-memory）：跨连接不可见（②结构性免疫）；未建表⇒引用即错（①fail-closed 拒绝）；DDL 不受写事务回滚影响⇒清除须显式（③挂载 inTx 终结路径）。
6. 不可伪造性范围裁定（诚实入 plan）：防"不经 guard 的代码路径注入上下文"（无 temp 表⇒trigger 拒）；**不防**持任意 SQL 写权限的攻击者自建 temp 表——该攻击面＝"mc 是唯一开脊程序"访问控制平面之外的既有权限，非本机制声称的边界（IG §3 原文指向＝"防重复信任同一不可信输入式假第二道"，即 context 不得源自调用方自报值）。

## 1. 设计裁定

- **D1 S7 步骤（纯 DDL）**：`users`（id 16hex PK、username 唯一、password_hash bcrypt、created_at）＋`user_sessions`（session_id 16hex 非秘密 PK、token_hash sha256 64hex UNIQUE、user_id FK、state∈active/revoked/expired、created_at/absolute_expires_at/idle_expires_at/last_seen_at）＋`teams`（id、name 唯一）＋`team_members`（team_id+user_id 联合 PK、joined_at；IG §1 关系约束＝成员资格给发现与进入，对象级读权仍需 grant——策略面在 guard，本表不自动授权）＋`grants`（grant_id 16hex PK、grantee_user_id、scope、resource、granted_by_user_id、created_at/effective_at/expires_at/revoked_at；I2 有界限时可撤销留事件）。**trigger 面（执行约束面，D-IG-3）**：`grants` INSERT／`user_sessions` INSERT／`user_sessions` UPDATE(state) 三 trigger 读 `main.trusted_ctx`（temp）：表不存在或无行⇒ABORT fail-closed；行的 actor 与所写行归属必须一致（grants.granted_by_user_id=ctx.actor；sessions.user_id=ctx.actor 或 ctx 携带 admin 范围标记——对象范围一致性＝"所写行的归属必须与注入上下文相符"，非自报列可比）。
- **D2 guard 单平面**（新包 `mc/identity/`）：`Guard.Decide(actorID, sessionID, action, scope)→{allow|deny|require-signoff, reason}`＝**唯一** allow/deny/require-signoff 决策点（Go 内；策略源＝会话有效性＋grant 表＋自操作集；无默认数据读权 I2）。trigger 不做授权判定（只验对象范围/状态合法性＝执行约束面）——D-IG-3 的字面兑现。
- **D3 可信上下文注入**：`identity.BindTrustedContext(conn, actorID, adminScope bool)`＝CREATE TEMP TABLE trusted_ctx(actor TEXT PRIMARY KEY, admin INTEGER)＋INSERT 单行（在 inTx 已开的写事务 conn 上，与写同事务提交/回滚）；`inTx` 终结路径（COMMIT 后或 ROLLBACK 后）显式 `DROP TABLE IF EXISTS trusted_ctx`（③异常/复用即清除——temp DDL 不受回滚管，必须显式）。注入值来源＝guard 决策所用的已验证 actor（非请求自报）⇒⑥不可伪造性范围裁定。
- **D4 会话语义（IG §2 逐条）**：token＝crypto/rand 32B（256bit）hex64，服务端只存 sha256(token)；session_id＝crypto/rand 16B hex（非秘密，API 定位用）；绝对 TTL＋闲置 TTL 常量（D01 缺省：绝对 24h／闲置 30min——值入缺省常量，策略面候 D02 撤销矩阵票扩展）；状态枚举 active/revoked/expired；撤销三档（单会话/用户全部/全局紧急）＝同一 verb 的 scope 参数；创建/撤销/过期＝activity 行（Class A；过期＝读侧惰性判定＋状态行更新，非定时器——D01 不建 cron 面，登记）。
- **D5 verb 面**（新命令族 `identity`，host scope，connection 族同形）：`session create`（username＋password⇒bcrypt 校验⇒会话行＋activity）／`session revoke`（scope=single|user|global）／`grant issue`（guard.Decide 先判＋上下文绑定＋grant 行＋activity）／`grant revoke`（同形）。四写 verb＋B05 面：经 main.go 直连 withSpine 路由（非 commandSystemSpine 族⇒**在 B05 枚举面内**⇒registry 需四条目——与 C03 的边界情形不同，本票写面被 gate 捕获，随票登记）。
- **D6 guard 最小策略（D01 内闭环，不预建 D02-D09 语义）**：self-actions（own session create/revoke）恒 allow（会话有效时）；grant issue/revoke＝allow 仅当 actor 会话有效且（D01 内：granted_by=actor 自证＋无跨 Team 继承——IG §1 关系约束的最小面：跨 Team grant⇒require-signoff，签核面候 D04 break-glass 票）；deny 默认（无 grant 无自操作⇒拒，I2 无默认读权）。
- **D7 表与迁移**：S7 语句＝上述 DDL＋trigger＋索引（byte-identical 纪律同 S4/S5/S6）；`b08MapIdentity` 常量＝V 钉定；S6.to 钉 V；identity 新基线 X。
- **D8 两平面隔离证明**：现有 run.json/RunIdentity 面零改动；guard 不读 run.json tier；`identity` 命令不在 runLocal 委托族（host scope 直连，操作者面）。

## 2. 正清单

1. `mc/go.mod`/`go.sum` — x/crypto（bcrypt）直接依赖＋x/sys、x/text 传递升级。
2. `mc/substrate/schema.sql` — S7 DDL 块（5 表＋3 trigger＋索引）。
3. `mc/substrate/migrate_queries.go` — `identityFoundationDDL` 常量。
4. `mc/substrate/migrate.go` — `b08MapIdentity`（V）＋S6.to 钉 V＋S7 `add-identity-foundation`（X⇒CurrentSchemaID）。
5. `mc/identity/identity.go`（新包）— Guard.Decide（D6 最小策略）＋BindTrustedContext/ClearTrustedContext（D3）＋会话 token/ID 生成（D4）＋bcrypt 包装。
6. `mc/identity/identity_test.go`（新）— IG-T1 四组攻击族（N1-N4）＋guard 决策矩阵＋TTL/状态机。
7. `mc/verbs/identity.go`（新）— 四写 verb（D5）＋Class A activity 行。
8. `mc/verbs/verbs.go` — inTx 终结路径 trusted_ctx 清除（D3③；对无上下文的既有调用者＝no-op）。
9. `mc/cmd/mc/main.go` — `identity` 命令路由（直连 withSpine 形态，非 runLocal 委托族）。
10. `mc/verbs/error_semantics_registry.json` — 四新写 verb 六维条目（B05 gate 捕获面，D5）。
11. 既有 registry 测试随 7 步更新（migrate/usage/auchain/onboard 族，B08 先例同形）。

## 3. 负清单

- RunIdentity/run.json tier／现有 verb 面／dispatch/Decide/seam
- resident/TS 面（P4 身份平面为脊内部＋mc verb；dashboard 面候 D01 后续票）
- 撤销传播矩阵（D02）／break-glass（D04）／degraded_grants（D05）／legacy:operator 迁移（D03——本票不建 legacy_actor_mapping）
- OIDC（P4.1 尾，IG §2 明文后置）／pepper/KMS（§2 澄清：可选不建）
- openwiki/／docs/contracts／release 面（Go 全部编译入 mc 二进制，零新生产文件⇒host_release 不动）

## 4. IG-T1 四组攻击测试形态（N 族）

- **N1 直接 SQL 旁路**：无 guard 绑定的裸 `INSERT INTO grants`／`user_sessions`⇒trigger ABORT（"no such table: trusted_ctx" 或显式 ABORT 句）；guard 决策 bypass（有绑定但 Decide=deny）⇒verb 层拒（写不发生）。
- **N2 跨连接复用串扰**：conn A 绑定 actor 上下文（未提交态）⇒conn B 上同形写⇒拒（temp 表连接局部，结构性）；提交后 A 的上下文已被 inTx 清除⇒A 复用再写无绑定⇒拒。
- **N3 异常/事务失败清理**：inTx 内绑定后业务写失败⇒ROLLBACK⇒inTx 终结清除⇒同 conn 下一写无上下文⇒拒（防"上一事务的 actor 泄漏进下一事务"）。
- **N4 guard 与 trigger 作用域一致**：矩阵逐格——guard allow 的写⇒trigger 放行；guard deny⇒写不达 trigger；trigger 接受面⊆guard 决策面（无"trigger 单独放行而 guard 未判"的写路径）＋对象范围不一致（ctx.actor≠所写行归属）⇒ABORT。
- 附：会话状态机（active⇒revoked/expired 单向）＋撤销三档＋双 TTL 惰性过期＋token 仅哈希存储（明文不出内存）＋bcrypt 强度。

## 5. 验证 lane

- L1 identity 包（N 族）＋substrate（S7 迁移族）＋verbs（registry 条目＋inTx 清除面）定向绿。
- L2 `mc/check.sh` 全量⇒干净 lane EXIT=0。
- L3 blcheck：D01 入册后 deps B02✓⇒无 ready-unmet。
- 诚实边界：bcrypt 依赖升级面候供应链审计（Mimosa 完整审计重跑已候令）；Class A 事件稳态入链候 B08 稳态后续票；闲置 TTL 的读侧刷新（last_seen_at 更新）随 D02 撤销矩阵票细化；dashboard 身份面候后续票。

## 6. STOP 条件

- S1 inTx 终结清除触既有动词行为（任何非 identity 写面变化）⇒停（负清单红线）。
- S2 trigger 需 guard 之外的第二决策语义（如 trigger 内判 grant 有效性）⇒停（D-IG-3 单平面红线，呈报）。
- S3 x/crypto 传递升级影响既有行为（go.sum 冲突/编译面）⇒停，呈报依赖裁定。

## 7. 登记与收尾

实施＝少提交于 shz/TD01；全道绿后 ff dev；登记轮＝AM v0.2.19＋status.json D01=EVIDENCED＋blcheck；D02-D09 解锁呈报。

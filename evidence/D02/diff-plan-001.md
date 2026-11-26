# D02 DIFF-PLAN-001 · 撤销矩阵＋决策点零缓存证明＋派生即失效（epoch fence 预备）

票面（DEV-BASELINE）：撤销矩阵＋零缓存证明＋派生即失效（epoch fence 预备）｜deps D01｜契约锚 IG §6；IG-T2/T8｜验收 P4.0：矩阵逐行实测＋零缓存证明。
基线：dev tip 05c06ee；schema 基线 identity X＝6be6f49b…c535（S7）；D01 身份基础面（6 表＋4 trigger＋guard 单平面＋inTx 生命周期）已 EVIDENCED。

## 0. 范围裁定

**D0 ＝ 零 schema 变更**。grants 表既有列（granted_by_user_id/effective_at/expires_at/revoked_at）足以承载全部三块语义；不预建 epoch 列（无缓存⇒无 fence，IG §6 原文"进程模型若变，epoch fence 先于缓存启用"——fence 是缓存的前置义务，不是先行建设项；预建即 identity churn＋第二权威风险）。基线 identity X 不变，无 S8。

**D02 三块**：
1. **grant 消费决策点**（新，IG-T2 矩阵第 2 行与零缓存证明的载体）：`identity.GrantAllows` ＝ 实时 spine 读的单平面授权判定（allow/deny，无 require-signoff 档——signoff 是签发平面概念，D-IG-3）。D01 只有签发/撤销决策，消费判定缺位⇒矩阵"下一次授权判定即拒"不可实测。
2. **派生 grant 随 granter 失效即失效**（IG §6 v0.2.1〔新增〕条）：触发点＝`IdentitySessionRevoke` scope 'user'（granter 活动会话全部清除＝granter 失效）；同一 inTx 内级联翻转该 granter 签发的全部未撤销 grant（比较基准＝撤销事件 effective 时点＝事务内 SpineTime，不等原 TTL；DC §3.2 同一写事务提交）。
3. **决策点零缓存证明＋矩阵逐行实测**（IG-T8 P4.0 验证面＋IG-T2）：见 §4 测试族。

## 1. 矩阵逐行实测裁定（IG §6 五行，逐行定性）

| 行 | 裁定 |
|---|---|
| user session | **可实测**：建会话⇒会话下受保护请求过⇒single 撤销⇒同一 token 下一受保护请求即拒（R1）。 |
| capability grant | **可实测**：签发⇒GrantAllows 实时判定 allow⇒撤销⇒deny；时间界三态（未生效/窗口内/过期，假时钟）（R2）。 |
| agent_run | **[固化]既有面引用，不重实现**：run fence＝既有 lease fencing（DC-FACTS-001 逐行 ✅）＋`domain.Cancel`（tasks.decision='cancelled'/archived，board.go:262）；"G-B 补构建维"＝构建身份维度，属 A12/A13 域（IMPL-AUTH-DRAFT-002：G-B 域待 F1 覆盖声明定稿后立项）⇒登记不主张新实测。 |
| 在飞长任务 | **检查点集合逐点定性**：每次 spine 写 verb＝inTx guard＋trigger 实时复验面（D01 固化，Z4 负控引用）；每次签名/交付请求＝D06/E-track 未实施⇒登记候票；provider per-request liveness＝[固化]既有先例（resident 代理面，TS 域）⇒引用登记。 |
| service_principal | **不可实测（对象不存在）**：现行 schema 无 SP（D01 users 表无 subtype，SP 创建面未入任何票）⇒行登记候 SP 创建面具名后续票，不在本票制造 SP。 |

## 2. 正清单（文件）

- `mc/identity/identity.go`：**＋`GrantAllows(ctx, q, actorID, scope, resource *string, now time.Time) Decision`**——实时读 grants 单行判定（exists ∧ revoked_at IS NULL ∧ effective_at ≤ now ≤ expires_at ∧ scope 精确匹配 ∧ resource 匹配或 NULL 通配）；判定输入只允许实时 spine 读（§6 零缓存义务冻结面）；时间比较＝SpineTime 定长格式字典序等价时序。
- `mc/verbs/identity.go`：`IdentitySessionRevoke` scope 'user' 分支同事务级联（inline UPDATE grants SET revoked_at＝事务内 SpineTime WHERE granted_by_user_id＝actor AND revoked_at IS NULL）＋activity detail 增级联计数（既有 recordActivity 位）。
- `mc/verbs/identity_ig_t2_t8_test.go`（新）：§4 测试族。
- 其余：**零变更**（schema.sql／migrate*／main.go／systemstatus.go／go.mod 不动⇒B05 gate 源枚举面与路由面不变）。

## 3. 负清单

- 无 S8／无 identity 变化；无新 verb／CLI 子命令（GrantAllows 是决策平面函数，消费接线候操作面后续票——D02 票面是撤销与证明，不是把 grant 检查焊进某个既有业务 verb，避免跨域扩面）；无 epoch 列／无缓存实现；无 SP；无 dispatch/run-fence/lease 代码变更（G-B 与[固化]域）；无 admin 跨用户撤销面（D01 verb 未暴露，候后续票；级联届时同样适用于目标活动会话清零）；无 dashboard／TS 面。

## 4. 测试族（IG-T2 矩阵＋IG-T8 零缓存，R/Z 命名）

- **R1 会话行**：session create⇒token 下 grant-issue（受保护请求）成功⇒session-revoke single⇒同一 token 再 grant-issue⇒域拒（"下一次受保护请求即拒"）。
- **R2 grant 行**：A 对 B 签发 24h grant⇒GrantAllows(B,scope,now)＝allow⇒grant-revoke⇒GrantAllows＝deny（"下一次授权判定即拒"）；时间界三态子例（假时钟：now＜effective_at⇒deny；窗口内⇒allow；now＞expires_at⇒deny）＋resource 精确/通配两态。
- **R3 派生即死**：A 对 B 签发 24h grant（expires_at 远在未来）⇒A session-revoke scope 'user'⇒同一 verb 返回后：A 会话全 revoked ∧ B 的 grant revoked_at 非 NULL（两态翻转同事务提交——DC §3.2）⇒GrantAllows(B, 真实 now)＝deny 且原 TTL 未至（断言 expires_at＞now）⇒级联 revoked_at 与撤销事件同时点（同秒界断言）。
- **R4 single 不级联**：A 两会话＋A 对 B 的 grant⇒撤销其一（single）⇒grant revoked_at 仍 NULL，GrantAllows＝allow（granter 未失效）。
- **Z1 跨调用不复用判定**：两次独立 inTx 判定之间发生撤销⇒第一次 allow、第二次 deny（判定每次实时重读，无跨调用 memo；负控＝若判定读"上次结果"必红）。
- **Z2 同调用内不旧替新**：同一 inTx 内：判定 allow⇒撤销该 grant⇒再判定⇒deny（判定输入＝实时读，非快照变量）。
- **Z3 连接池跨 Actor 无污染**：SetMaxOpenConns(1) 强制同一物理连接复用⇒A 绑定+guard 写+commit（连接回池）⇒B 的下一 inTx：入口清除先行的实证＝无绑定裸写被 fail-closed 拒＋声称 A 归属的写被 "disagrees" ABORT。
- **Z4 执行点复验负控（引用 D01[固化]面）**：绕过 inTx 的裸 `db.Exec` 写 grants（无绑定）⇒trigger 拒（写点不依赖任何决策缓存，trigger 在写入瞬间复验）。
- 假时钟：GrantAllows 的 now 参数注入（verbs 面级联用事务内 SpineTime，SpineTime 纪律不变）。

## 5. 验证 lane

- L1 `go build ./...`＋`gofmt`＋定向（./identity/ 随 verbs 包内调用；mc/verbs -run 'TestRevocationMatrix|TestDerivedGrant|TestZeroCache|TestPoolContamination' 等 R/Z 族＋既有 IG-T1 族回归）。
- L2 `mc/check.sh` 全量⇒干净 lane EXIT=0（27 包）。
- L3 登记轮⇒blcheck（D02 入册后 deps D01✓⇒无 ready-unmet）。

## 6. STOP 条件

- S1 级联或消费判定需要新 schema 对象（列/表）⇒停（D0 零 schema 裁定）。
- S2 grant 消费点出现第二决策档（require-signoff 进入消费判定）⇒停（D-IG-3 单平面红线，呈报）。
- S3 矩阵实测触达 dispatch/lease/run-fence 代码面⇒停（[固化]域与 G-B 域，越界）。
- S4 零缓存证明需要改 mc per-invocation 进程模型⇒停（进程模型[固化]，任何缓存引入须 epoch fence 先行——本票无缓存）。

## 7. 登记与收尾

实施＝少提交于 shz/TD02；全道绿后 ff dev；登记轮＝AM v0.2.20＋status.json D02=EVIDENCED＋blcheck；诚实边界入册（矩阵第 3/5 行与在飞长任务签名/交付检查点＝具名候票面；epoch fence＝契约登记零代码）。

## 8. 实施补记（先列后改的追加登记）

- **测试 fixture 形态裁定**：T2/T8 族不走密码认证（该面 D01 族已实证）；会话 fixture 改经 guard 绑定路径直接播种（NewSessionToken＋inTx BindTrustedContext＋user_sessions guarded INSERT）——候选代码零凭据形值，且播种本身顺带实证对象范围 insert trigger 放行面。
- **GrantAllows 决策形态**：包级无状态函数（不进 Guard 实例——判定输入为纯实时 spine 读，判定内无时钟依赖；now 由调用方注入并渲染为 SpineTime 定长形与存储值同格式字典序比较）。verbs 侧级联用事务内 datetime('now')＝SpineTime 纪律不变。
- **级联 UPDATE 无 trigger 争用**：grants_guarded_insert 是 INSERT trigger；级联是 UPDATE grants（归属列不参与 UPDATE 约束），且与撤销同 inTx 同提交——DC §3.2 同事务原子性由 inTx 单一提交点承载，无需第二机制。

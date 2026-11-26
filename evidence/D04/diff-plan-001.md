# D04 DIFF-PLAN-001 · break-glass 机制＋紧急单人操作清单冻结成文

票面（DEV-BASELINE）：break-glass 机制＋**紧急单人操作清单冻结成文**｜deps D01｜契约锚 IG §3；D23；IG-T7｜验收 P4.0：清单成文＋单人紧急例演练。
基线：dev tip c208a92；schema 基线 identity＝f9d1090e…e615（S9）——**本票零 schema 变更**（D01 admin 缝已备齐：trusted_ctx.admin 位＋user_sessions_guarded_revoke 的 admin=1 豁免＋DecideSessionLifecycle 的 global 拒位占本票）。

## 0. 契约形态 recon（先列后改的事实基础）

- **IG-T7（AM §4c）**：break-glass 单人紧急例外的**精确操作清单**——"恢复/止损类"由 P4.0 验收**冻结成文**，**禁实施方自行扩类**（二审三·2）。
- **IG §3〔补严·首审三.3〕**：双人标志是**分级规则**非可选字段——对象级读权类必须双批准；申请人不得自批、审批人不得兼任执行人；**单人紧急例外仅限"恢复/止损类"操作**且**事后追认+Authority 复审留痕**；D23 默认矩阵不因标志可配而弱化。
- **MASTER D23 Break-glass 协议**：目的申明、最小数据范围、限时、强制审计、可配双人批准、到期自动撤销。
- **D01 既有缝**：`user_sessions_guarded_revoke` trigger（active⇒revoked 需 trusted_ctx；NEW.user_id∈ctx 或 admin=1）＋`DecideSessionLifecycle("global")` 拒位（"no D01 authorization path"＝本票占位）＋`BindTrustedContext(actor, admin)`（admin 位现无授权路径置 1＝D04 开）。
- **blcheck -docs 只哈希 bind 白名单**：新清单文档不入 anchors.bind.json＝不触门禁（绑定＝独立治理动作，登记）。

## 1. 设计裁定

- **D0＝零 schema 变更**（无 S10；identity 不变）。
- **D1（清单冻结，IG-T7）**：单人紧急例外类（恢复/止损类）＝**恰含一项：`session-revoke:global`**（全局会话紧急撤销＝止损：一切活动会话即时失效）。成文落点＝`docs/contracts/BREAK-GLASS.md`（新文档：冻结清单＋D23 协议逐项映射＋变更规则[仅所有者裁定＋重冻结]＋演练记录引用）。**扩类/缩类＝所有者裁定**，实施方禁自行扩类（guard 对清单外操作一律 fail-closed deny＝结构性）。
- **D2（guard 单平面，D-IG-3）**：`Guard.DecideBreakGlass(operation string) Decision`——冻结清单内⇒Allow（purpose 要件由 verb 面强制）；**清单外⇒Deny**（含未来双人级操作——signoff 平面＝D08，本平面不建双批机制，也不预置双人档；无 signoff 面之前双人级操作无路径＝fail-closed 保守方向）。
- **D3（verb 接线）**：`IdentitySessionRevoke` scope 'global'＝break-glass 路径：①purpose 必填（D23 目的申明；缺失⇒Usage 拒）②DecideBreakGlass("session-revoke:global") 必 Allow（⇒Domain 拒）③**bind trusted_ctx admin=true**——全系统唯一 admin 位路径，由 guard 裁定授权（"绑定不是授权：guard 决策在 Go，先于绑定"原则的 D04 兑现）④全局 UPDATE user_sessions（trigger admin 豁免实证）⑤**专属 activity kind `session.breakglass_revoked`**（高审计密度：purpose＋撤销计数入 detail；D23 强制审计）。scope single/user 行为零变。
- **D4（演练，验收句"单人紧急例演练"）**：B 族测试＝单人具名主体端到端演练（跨用户撤销实证＝admin 豁免路径）＋目的缺失拒＋扩类禁（guard 清单外 deny＋trigger 无绑定/低权限归属 ABORT 负控）＋single/user 回归。
- **D5（诚实边界，入册）**：①D23"限时/到期自动撤销"＝n/a（全局撤销为即时操作，不签发委托凭据；到期机制候未来 break-glass 委托凭据面）；②"事后追认＋Authority 复审"＝signoffs 平面 D08（本票落审计行＝追认的待办证据载体，追认机制不建）；③双人级（对象级读权类）＝D08 signoff 面（本票 guard 对之 fail-closed deny）；④BREAK-GLASS.md 未入 anchors.bind 哈希绑定（独立治理动作候所有者）；⑤清单当前恰一项＝合同面事实（"恢复/止损类"在现行 identity 平面的唯一可实现操作），非能力上限声明。

## 2. 正清单（文件）

- `docs/contracts/BREAK-GLASS.md`（新）：冻结清单＋D23 逐项映射＋变更规则＋演练引用。
- `mc/identity/identity.go`：＋DecideBreakGlass（冻结清单常量＋判定）；DecideSessionLifecycle 注释随 global 路径开启更正（"later admin plane"⇒break-glass 平面已立）。
- `mc/verbs/identity.go`：IdentitySessionRevokeRequest ＋Purpose 字段；scope 'global' 分支＝D3 五步路径；recordActivity 复用。
- `mc/verbs/identity_breakglass_test.go`（新）：B1-B4 演练族。
- `mc/cmd/mc`：零变更（session-revoke 子命令既有，Purpose 字段经既有 JSON 解码面进入——wire 形状增字段＝additive，登记）。

## 3. 负清单

- 无 schema/S10/identity 变化；无 signoffs 表/双批机制（D08）；无新 verb/CLI 子命令（global＝既有 session-revoke 的 scope 值）；不扩 single/user 语义；不建委托凭据/到期机制；TS/dashboard 零触碰；BREAK-GLASS.md 不入 anchors.bind。

## 4. B 演练族（验收句＝"清单成文＋单人紧急例演练"）

- **B1 单人紧急例演练（端到端）**：种子 alice＋bob 双用户各活动会话⇒alice（单人具名，会话认证）scope 'global'＋purpose⇒**全部**活动会话 revoked（含 bob 的＝跨用户＝admin 豁免路径实证）＋revoked_by＝alice＋审计行 kind=session.breakglass_revoked（detail 含 purpose＋计数）＋result 报告计数。
- **B2 目的申明强制**：global 无 purpose⇒Usage 拒＋零状态变化（D23 目的申明）。
- **B3 扩类禁（双负控）**：guard 面——DecideBreakGlass("project:delete")/任意清单外操作⇒Deny（fail-closed，结构性扩类不可能）；trigger 面——无 trusted_ctx 裸全局 UPDATE⇒ABORT；仅 admin=0 绑定他用户归属的全局 UPDATE⇒ABORT（admin 位不可自授＝只有 guard 裁定路径能置 1，而该路径只认冻结清单）。
- **B4 回归**：single/user scope 行为零变（purpose 非必填；仅本人会话；既有 D01/D02 测试族全绿）。

## 5. 验证 lane

- L1 定向（identity guard 面＋verbs B 族＋既有 IG-T1/T2/T5/D02 回归）。
- L2 mc/check.sh 全量⇒干净 lane EXIT=0（27 包）。
- L3 登记轮⇒blcheck（D04 入册后 deps D01✓⇒无 ready-unmet；新文档不触 bind）。

## 6. STOP 条件

- S1 清单冻结面需第二项操作才有合同意义（即"恰一项"不成立）⇒停，呈报（清单内容＝所有者裁定域）。
- S2 global 路径需要 schema 面（如 break-glass 专属审计表）⇒停（D0 零 schema 裁定；审计走 activity 既有面）。
- S3 guard 需为双人级操作预置 signoff 联动⇒停（D-IG-3＋D08 域；本平面 fail-closed）。

## 7. 登记与收尾

实施＝少提交于 shz/TD04；全道绿后 ff dev；登记轮＝AM v0.2.23＋status.json D04=EVIDENCED＋blcheck；诚实边界（D5 五项）入登记；BREAK-GLASS.md 冻结成文＝验收句"清单成文"的交付物。

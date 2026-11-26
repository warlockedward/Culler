# BREAK-GLASS · 紧急单人操作清单（冻结）

- 状态：**P4.0 冻结（IG-T7；轮四十九 D04 验收成文）**。本文件是 break-glass 单人紧急例外操作的**唯一定义载体**：清单外的一切操作都不存在单人紧急路径（fail-closed）。
- 契约锚：IG §3〔补严·首审三.3〕（双人标志为分级规则；单人紧急例外仅限"恢复/止损类"且事后追认+Authority 复审留痕）；MASTER D23（Break-glass 协议六要素）；IG-T7（精确操作清单冻结成文，**禁实施方自行扩类**）。
- 机制面：mc guard 单平面（`DecideBreakGlass`）对下述清单之外的操作一律拒绝；执行约束面（session state trigger 的 admin 豁免）只认 guard 在同一写事务内绑定的 admin 上下文——**admin 位不可自授**。

## 1. 冻结清单（恢复/止损类，单人紧急例外）

| # | 操作 | 语义 | 止损理由 |
|---|---|---|---|
| 1 | `session-revoke:global` | 全局会话紧急撤销：一切用户的一切活动会话即时失效 | 凭据/会话暴露事件下的即时止损：不等待双人流程，单人具名主体可执行；跨用户撤销经 trigger admin 豁免路径，该路径仅由本清单的 guard 裁定打开 |

**变更规则**：新增/移除清单项＝所有者裁定＋本文件重冻结（登记轮随裁）；实施方（代码/文档/测试）不得自行扩类——guard 对清单外操作的拒绝是结构性的（无白名单第二源）。

## 2. D23 协议逐项映射（操作 1）

| D23 要素 | 本操作落点 |
|---|---|
| 目的申明 | **强制**：全局撤销请求必带非空 purpose；缺失⇒拒绝（零状态变化）。purpose 原文入审计行。 |
| 最小数据范围 | 只触 `user_sessions.state/revoked_by`（全局 UPDATE）＋被失效主体的派生 grant（`grants.revoked_at`，IG §6 派生即失效随全局失效同事务级联）＋一条审计行；不读不写其他表，不返回任何会话内容。 |
| 限时 | n/a：即时操作，不签发委托凭据（到期自动撤销机制候未来 break-glass 委托凭据面，不在此预建）。 |
| 强制审计 | 专属 activity kind `session.breakglass_revoked`（高审计密度）：detail 含 purpose 原文＋撤销计数＋执行主体；Class A 事件。 |
| 可配双人批准 | 不适用本清单：双人级（对象级读权类）操作**不在**单人紧急清单内，其批准机制＝signoffs 平面（D08），signoff 面落地前该级操作 fail-closed 无路径。 |
| 到期自动撤销 | n/a：同"限时"（无委托物可到期）。 |

## 3. 事后追认与复审（IG §3）

执行后**必须**：追认（signoff 平面，D08 落地后）＋Acceptance Authority 复审。本票交付的审计行（`session.breakglass_revoked`）是追认的**待办证据载体**——追认机制本身不在本票（负清单）。

## 4. 演练记录

单人紧急例演练＝`mc/verbs/identity_breakglass_test.go` 的 B 族（B1 端到端演练：单人具名主体跨用户全局撤销＋审计行落点；B2 目的申明强制；B3 扩类禁双负控：guard 清单外 deny＋trigger 无绑定/低权限归属 ABORT；B4 single/user 回归）。演练通过随 D04 登记轮入册；清单任何变更须重跑并追加演练记录。

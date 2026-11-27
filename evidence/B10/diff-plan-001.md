# B10 activity 稳态写入链 · diff-plan-001

日期 2026-11-27。shz/dev @ 5ff6347（D01–D08 全 EVIDENCED，AM v0.2.27=59a9bf52…4359，blcheck 73 票 0 violations）。授权链＝所有者"继续执行"（B-chain 链内下一票）＋DEV-BASELINE:77 B10 票面（轮四十八 Q4 批准票面）＋B08 D0/D4/D5/D12 具名后续裁定（evidence/B08/diff-plan-001.md）＋DC §3.2 稳态半。

## 0. 票面分解

| 要素 | 权威 | 本 plan 落点 |
|---|---|---|
| 稳态新 activity 行同事务入链 | DC §3.2（activity 入链义务的稳态半）＋R-03 原子审计 | §2 D1 集中 `AppendActivity` |
| 分散调用点中央化 | B08 D4（"集中 AppendActivity＋35 点机械改接"） | §2 D2/D3＋§1.1 位点表（实数 36） |
| 源码级覆盖门（新写点不走中央 API 必红） | B08 D5＋票面＋B05 两阶段先例（evidence/B05/diff-plan-001.md §1.5/§2.3） | §2 D5 |
| dispatchseam 设计决策面处置 | 票面第三件＋B08 负清单红线 S2 | §2 D0（本票裁决件） |
| 零改写回归 | 票面 | §2 D4/D6＋§4 测试族 |

## 1. 现状事实（E1，file:line，@5ff6347 树）

1. **位点全量＝36 处（非测试）**，25 文件：substrate/migrate_apply.go:140；verbs/identity.go:599（共享 helper `recordActivity`）；其余 34 处为分散裸 INSERT。形制分类（列集＋值语义）：
   - **A 纯四列**（actor,kind,subject,detail 全非 NULL 值）：28 处——collieinventory/roles/colliepreflight/operator_requests/board×2/artifactretention/task/complete/environment/collie×5/customization(+project)/collietitle/collieselect/infrastructurefailure/dispatchverb×2(+project)/wikiround(+project+三 dispatch 列，值全给)/refusalroute:227(+dispatch_key)/refusalroute:236(+三列)/migrate_apply(actor='mc')/identity.go:599。
   - **B NULL 字面量**：dispatchseam:1266（subject、detail 存 NULL）。
   - **C NULLIF(?, '')**：dispatchseam:1282 的 detail。
   - **D json_object() SQL 函数**：domain/allowances.go:69、:86 的 detail。
   - **E SELECT 派生行**：domain/wikiupdate.go:337（subject=run_id、project 读自 wiki_updates）。
2. **链机制单一权威已在位**（B08/B07 产物，substrate 内）：`AppendAuditEvent`（auchain.go:374）＝事务面 join（*sql.Tx/*sql.Conn 加入调用方事务；*sql.DB 自开 BEGIN IMMEDIATE）＋seq=max+1＋prev 校验 fail-closed＋规范编码＋双摘要＋INSERT audit_events；`activityPayload`（activity_chain.go:46）＝B08 字段表渲染器（NULL 列⇒键缺席）；`activityChainEventID(id)`＝行 id 派生幂等事件 id；`GenesisHash(activityAggregateID)`；usage 稳态先例 `appendUsageChainEvent`（head 查询＋同事务追加）。
3. **B08 回填验证器只在 S5 事务内跑**（activity_chain.go:166 VerifyActivityBackfill：映射/链/表三计数相等＋可达＋序）⇒与后续稳态行无冲突；'activity' 聚合无 aggregate_open（D12：首行即 genesis）。
4. **activity 触发器面**（schema.sql）：append-only update/delete、receipt-keys-are-text（TEXT  typeof 闸）、**health-alert-outbox AFTER INSERT WHEN kind='dispatch.health'**（读 NEW.id 写 outbox，未配路由⇒ABORT）。同值 INSERT⇒同触发行为＝改接零漂移面；UNIQUE(activity.dispatch_request_id)＝seam 重放围栏列。
5. **接口面**：`verbs.Q = domain.Q` 类型别名（verbs/verbs.go:63），方法集 {ExecContext, QueryContext, QueryRowContext}；substrate.auditor={ExecContext, QueryContext}⊂之⇒verbs/domain 的任意事务面可直接传入；**substrate 现不 import domain（无环）**；domain 现不 import substrate（本票新增该边，方向合法：substrate 为底座）。
6. **B08 具名裁定**：D4＝本票面（集中 API＋机械改接＋源级闸）；D5＝闸随本票；D12＝步骤 activity 行"候稳态票统一"⇒本票改接 migrate_apply 位点兑现；D0 稳态半残余＝本票闭合对象。

## 2. 设计

- **D0（票面第三件：dispatchseam 设计决策面处置＝裁决件）**：seam 的冻结面＝prepare/attest/commit 的**决策 JSON wire shape**（dispatch.Decide 纯函数域）；两个 activity 写点（writeDispatchReceipt/writeAttestedReceipt）是 **commit 侧脊写**，不在冻结 wire 上。处置＝改接经中央 API（receipt 的 dispatch_result 字节**原值透传**，storableReceipt 输出不动），链行＋映射行并入**同一 commit 事务**＝DC R-03 强化而非削弱。负清单红线不变：任何使冻结 JSON 形状变化的实现需求⇒停（S2）。
- **D1（集中 `AppendActivity`）**：落 `substrate/activity_chain.go`（与 activityPayload/activityChainEventID 同文件＝单一权威物理同址）。签名 `func AppendActivity(ctx context.Context, q auditor, ev ActivityEvent) error`；`ActivityEvent{Actor, Kind string; Subject, Detail, Project, DispatchKey, DispatchRequestID, DispatchResult *string}`（nil⇒列 NULL）。序：①INSERT activity（同列集）→②LastInsertId→③行回读（id, actor, kind, created_at, 全 8 列）→④`activityPayload`（同回填渲染器，created_at 经 spineTimeToCanonical）→⑤event_id=`activityChainEventID(id)`（幂等映射语义与存量同构）→⑥head 查询（MAX(seq)+chain_digest，'activity' 聚合）→⑦`AppendAuditEvent`（prev=head 摘要或 genesis；event_type=kind；payload_family='activity'）→⑧INSERT activity_chain_map。全部跑在调用方事务面⇒原子审计（行、链、映射三者同生同灭）。
- **D2（36 点机械改接，值语义逐点保真）**：A 类＝字段取址传入；B 类＝nil 指针（列 NULL 不变）；C 类＝Go 侧 nil-if-empty（NULLIF(x,'') 的逐字等价）；D 类＝**事务内预读 `SELECT json_object(...)`** 后传串（同一 SQLite JSON 渲染器＝存储字节逐字不变；禁 Go 侧重编码——键序/空白会漂）；E 类＝预读派生列后传入。每点错误处理原样（返回/包装/continue 语义不动——非机械即停 S1）。
- **D3（recordActivity）**：保留封存守卫（actor=='operator' 拒）＋委托 AppendActivity；subject 空串**仍存 ''**（今日语义，零改写——不"顺手"改成 NULL）。其全部 identity/D05/D06/D07/D08 调用方自动入链。
- **D4（迁移步骤位点）**：migrate_apply:140 改接⇒新步骤（S13+）的步骤 activity 行入 'activity' 聚合链（D12 候票裁定在本票闭合）。B10 自身**不新增迁移步骤、零 schema 变化**（identity 不变，仍是 4786047e…8be1）。
- **D5（源级覆盖门，B05 两阶段形制）**：`substrate/activity_coverage_test.go`。枚举器＝读全模块非测试 .go 源（runtime.Caller 定位模块根，filepath.WalkDir），匹配 `INSERT INTO activity` 且后随字符非词字符（排除 activity_chain_map）⇒位点集。G1 覆盖方向：位点集 ⊆ {substrate/activity_chain.go}（旁路必红）。G2 幻影方向：activity_chain.go 内该标记恰含 1 处（中央 API 自身被掏空⇒红）。fail-closed：任何读/解析异常⇒直接 FAIL，不以空集通过。门的负控＝扫描器函数表驱动测试（注入旁路样本⇒检出；B05 N1/N2 形制）。
- **D6（零改写回归的载体）**：①存储列值逐点不变（§2 D2 构造性＋D 类预读保渲染器同源）；②全量既有断言（各 verb 测试对 activity 行内容的数百条断言）保持绿＝回归证明；③新增断言族＝行+链+映射同事务三元组（回滚⇒三者皆无）。
- **D7（诚实边界，入登记）**：**中段窗口残余**＝以旧版发布迁移至 S5 之后、B10 代码上线之前写入的 activity 行（旧版 S6–S12 步骤行＋当时稳态 verb 行）不在 'activity' 链上——无回填、无 retrofit（B08 D0 最小变更面裁定延续；当前无任何部署脊处于该窗口：fork 未发布、owner 未部署）。登记为有界可见例外；覆盖断言限定"B10 后新行"。其余：闸是源级非行为级（运行时旁路若不走 INSERT INTO activity 字面量仍会被链验证器/触发器面拒）；usage/audit 聚合不受影响；'activity' 聚合 head 自此随稳态移动（锚定面 D07 的 heads_json 自动涵盖，零协议变化）。

## 3. 文件清单

- 改：`substrate/activity_chain.go`（+AppendActivity/ActivityEvent/head 助手）；`substrate/migrate_apply.go`（D4）；`verbs/identity.go`（D3）；§1.1 全部位点文件（verbs 19 文件、domain 4 文件——domain 四文件新增 mc/substrate import）；`evidence/B10/`（本件＋测试证据）。
- 新：`substrate/activity_coverage_test.go`（D5）；`substrate/activity_steady_test.go`（T1–T6 行为族）。
- 零新包、零 schema 变化、零 wire shape 变化、零部署清单条目变化（无新常驻文件；测试文件不入清单）。

## 4. 验证（测试族）

- T1 原子三元组：AppendActivity 于显式事务⇒行+链+映射三者立；回滚⇒三者皆无（R-03 结构证明）。
- T2 链衔接：两行连发⇒seq 相邻、prev 绑定、event_id 派生式与回填同构；payload 字节形态与 activityPayload 一致；`VerifyAuditChain` 全链绿。
- T3 新生脊 genesis：空链首行⇒seq 1、prev=GenesisHash('activity')（D12 语义）。
- T4 覆盖门双向＋负控（旁路样本检出；幻影臂）。
- T5 seam 零改写：经 writeAttestedReceipt 路径⇒dispatch_result 存储字节＝storableReceipt 输出逐字；UNIQUE 重放语义不变；链行在位。
- T6 步骤行入链：对 d08 脊 Migrate 一步⇒其步骤 activity 行有链行+映射（D4/D12 闭合）。
- T7 json_object 保真：allowances 路径⇒detail 字节＝SQLite 渲染（D 类预读策略）。
- lane：L1 定向（substrate/verbs/domain 族）；L2 全量 mc/check.sh⇒EXIT=0 **29 包**（零新包）零 FAIL 零 UNVERIFIED；L3 登记轮 blcheck。

## 5. STOP 条件

- S1 任一改接点需改错误处理/控制流语义（非机械）⇒停，呈报该点。
- S2 seam prepare/attest 冻结 JSON 面被波及（wire shape 漂移）⇒停（B08 负清单红线延续）。
- S3 触发器行为漂移（health-outbox/keys-are-text/UNIQUE 围栏任一发红且非我引入缺陷）⇒停。
- S4 覆盖门枚举异常（解析不了某位点形制）⇒门 fail-closed 红，不以豁免绕过；先修门再放道。
- S5 D/E 类预读改变 SQL 求值时序语义（json_object/wiki_updates 读在改接前后取到不同值）⇒停（预读与原 INSERT 同事务同点，理论零漂；出现即语义非机械）。

## 6. 登记与收尾

实施＝少提交于 shz/TB10；全道绿后 ff dev；登记轮＝AM v0.2.28＋status.json B10=EVIDENCED＋blcheck；identity 不变（无迁移步骤）；B08 具名残余（稳态半）闭合随登记说明；D7 边界入册。

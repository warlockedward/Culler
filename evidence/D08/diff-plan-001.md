# D08 DIFF-PLAN-001 · signoffs 表＋SoD 可配＋审批防伪造＋批准永不创建需求

票面（DEV-BASELINE）：signoffs 表+SoD 可配+审批防伪造+批准永不创建需求（结构红线测试）｜deps D06✓,B02✓｜契约锚 D18；IG；MASTER §2 生命周期 5｜验收 P4.0/4.1：结构红线测试。
基线：dev tip 6353655；schema 基线 identity＝9e322e35…01a0（S10）→本票触 schema 面（S12⇒新基线 identity）。

## 0. 契约形态 recon（先列后改的事实基础）

- **D18**：签署层骨架＝所有权分层：VISION/产线配置归 owner；签署权可委派；**SoD 可配置非默认**；超时升级；**批准永不产生新需求**。签署证据模型见 D-G3。
- **IG §3〔补严·首审三.3〕**（break-glass 双人分级）：**申请人不得自批、审批人不得兼任执行人**——此为 IG 硬规则（分级规则非可选字段）；单人紧急例外的事后追认须留痕（D04 已登记：审计行＝追认待办证据载体，追认机制＝本票）。
- **MASTER §2 生命周期 5**：Acceptance＝Governance 授权、Control 记录；**治理反噬三形态**（为审批改任务状态/为配额改调度语义/为审计把行为工单化）防线＝不变量 3＋D18。
- **D-IG-3**：allow/deny/require-signoff 判定只在 mc guard；**决策点零缓存**（实时 spine 读）。
- **悬挂消费面**：D04 break-glass 事后追认（审计行 kind=session.breakglass_revoked 待追认）＋D05 双人级（对象级读权类，签发平面 fail-closed 候本票）。

## 1. 设计裁定

- **D0＝schema S12 signoffs（5 对象）**：signoff_id 32hex PK UNIQUE；subject_class CHECK ∈('breakglass.ratification','generic')；subject_ref TEXT 1-256（被批对象的定位符）；requested_by TEXT 可空（原操作申请人，任意 actor 形态，无 FK——追认场景被追认者未必是 users 行）；approved_by FK users NOT NULL（审批人）；decision CHECK ∈('approved','rejected')；reason 1-1024；created_at SpineTime。3 trigger：guarded_insert（approved_by 归属复验）＋**no_update/no_delete（签署记录 append-only＝防伪造结构承载）**＋signoffs_subject 索引。
- **D1（签署绑定，D06 同构：对象即授权对象）**：signoff 绑定**具体主体**（subject_class+subject_ref），不存在通用批准权——签一张 signoff 只对 subject_ref 生效。审批人由 **session token 派生**（sessionActor，从不采信请求自述——防伪造的决策面半边）。
- **D2（SoD 分级，D18 可配置非默认＋IG 硬规则并存）**：guard `DecideSignoff(subjectClass)`——**SoD 强制类冻结集＝{breakglass.ratification}**（IG §3 硬规则：执行人[审计行 actor]不得自批，requested_by 从审计行取值不采信请求）；generic 类**不强制 SoD**（D18：可配置非默认——类注册表即配置点，扩类/改判＝所有者裁定，guard 对未识别类 fail-closed 拒）。审批人不得兼任执行人＝R3 行为断言（breakglass 类：签追认的 approver ≠ 被追认审计行的 executor）。
- **D3（审批防伪造，双层）**：执行约束面＝guarded_insert（无上下文/归属不符⇒ABORT）＋append-only lattice（改删⇒ABORT）；决策面＝审批人 sessionActor 派生＋subject_class 白名单 fail-closed＋known 类的主体存在性验证（breakglass.ratification ⇒ subject_ref 必须是 activity 表中 kind='session.breakglass_revoked' 的行 id；主体不存在⇒拒）。
- **D4（D18 结构红线，验收句主体）**：`SignoffRecord` 事务的写入面＝signoffs 行＋activity Class A 事件，**仅此两表**；红线测试＝签署前后 tasks/proposals/projects/degraded_grants 等需求表计数不变＋代码路径无任何需求表写（结构承载：verb 实现不触需求表 INSERT）。
- **D5（诚实边界，入册）**：①超时升级（D18）未建（无定时器面，候后续票）；②签署权委派（D18"签署权可委派"）＝经 D01/D02 grant 面既有承载，本票不新增委派机制；③消费接线＝breakglass.ratification 类可用（D04 追认可落），degraded.grant 双人级签发接线＝候后续票（本票提供 plane＋generic 类）；④层二/审批展示面（dashboard）不在票面；⑤subject_ref 对 generic 类不做存在性验证（自由定位符，登记）。

## 2. 正清单（文件）

- `mc/substrate/schema.sql`：S12 块（表＋3 trigger＋1 索引，5 对象）。
- `mc/substrate/migrate_queries.go`：＋signoffsDDL（逐字节等同）。
- `mc/substrate/migrate.go`：＋d07AnchorIdentity（f8620d7e…87c3 钉定）＋S11.to 钉＋S12 add-signoffs（⇒CurrentSchemaID 符号）。
- `mc/identity/identity.go`：＋DecideSignoff（guard 单平面：类白名单＋SoD 冻结集判定）。
- `mc/verbs/signoff.go`：＋SignoffRecord verb（host scope，session 派生审批人，类验证，SoD 判定，Class A 事件）。
- `mc/cmd/mc/signoff_command.go`＋main.go（family＋switch）＋systemstatus.go（signoff-record case）。
- `mc/substrate/signoffs_test.go`（新）：stripSignoffs＋d07Spine＋S12 族。
- `mc/verbs/signoff_t18_test.go`（新）：§4 G 族（D18 红线＋防伪造＋SoD 分级）。
- registry 12 步全族更新（机械模式）。

## 3. 负清单

- 无超时升级机制；无签署权委派新机制（grant 面既有）；无 dashboard 审批展示面；无 degraded.grant 双人级签发接线（plane 就绪，候后续票）；无第二决策体系（trigger 只管 append-only＋归属，不判定）；无 TS 面；subject_ref generic 类无存在性验证（登记）。

## 4. G 测试族（验收句＝"结构红线测试"）

- **G1 D18 红线（批准永不创建需求）**：签署前后 tasks/proposals/projects/degraded_grants/sign_requests 计数逐一相等＋签署事务写入面断言（signoffs＋activity 之外零写入——行为证据）。
- **G2 审批防伪造**：无上下文裸 INSERT⇒ABORT；归属不符（绑 bob 签 approved_by=alice）⇒ABORT；no_update/no_delete⇒ABORT；**审批人由 session 派生**：bob 的 token 签出的 signoff.approved_by＝bob 派生 id（不可能声称 alice）。
- **G3 SoD 分级（IG 硬规则）**：breakglass.ratification 类——先落一条 break-glass 审计行（actor=alice＝执行人），alice 自批⇒拒（**申请人/执行人不得自批**）；bob 审批⇒过，requested_by 自动绑定审计行 actor=alice（不采信请求）；**审批人不得兼任执行人**断言＝同 G3 前半。
- **G4 可配置非默认＋fail-closed**：generic 类自签⇒过（SoD 非默认）；未识别类⇒拒（类白名单 fail-closed，结构性禁扩类）；breakglass 类 subject_ref 指向不存在的审计行⇒拒。
- **G5 决策枚举＋append-only**：rejected 决策落库；update/delete⇒ABORT。
- S12 步骤族：单步落地＋lattice＋DDL 逐字节。

## 5. 验证 lane

- L1 定向（substrate S12 族＋verbs G 族＋既有回归）。
- L2 mc/check.sh 全量⇒干净 lane EXIT=0（30 包[新零包——signoff 在 verbs 内，无新包]）。
- L3 登记轮⇒blcheck。

## 6. STOP 条件

- S1 SoD 需按调用方可变切换（非类注册表）⇒停（D18 可配置指类级配置；per-call 切换＝绕过面）。
- S2 需求表写入进入签署事务⇒停（D18 红线，结构不可能——verb 无此代码路径；若实现中发现需要⇒呈报）。
- S3 审批需觸發锚定/签名⇒停（签署（signoff）与锚定签名（sign）两平面分离，IG §5.2 语义）。
- S4 breakglass 类需接受非 break-glass 审计行⇒停（类验证白名单 fail-closed）。

## 7. 登记与收尾

实施＝少提交于 shz/TD08；全道绿后 ff dev；登记轮＝AM v0.2.27＋status.json D08=EVIDENCED＋blcheck；新基线 identity 入册；诚实边界（D5 五项）入登记；D04/D05 悬挂面闭合情况随登记说明。

# D06 DIFF-PLAN-001 · 签名请求状态机＋节点＋五证（O11 裁决包）

票面（DEV-BASELINE）：签名请求状态机+节点+五证（REQUESTED/SIGNED/RECORDED+fencing）｜deps A02✓,D01✓｜契约锚 IG §5.2/§5.3；IG-T3/T9｜验收 P4.1：IG-T9 五证含取消×签名并发+重启迟到｜门:O11 裁决。
基线：dev tip bb84aea；schema 基线 identity f9d1090e…e615（S9）。

## 0. O11 裁决对象声明（设计先行路径，轮四十八 Q3 裁定）

本 diff-plan §1 呈出的**具体协议形态**即 O11 决定件全文：wire 形状（P1）／节点持久账本与 fencing 形态（P2）／receipt 验证形式（P3）／节点载体（P4）／节点进程形态（P5）＋IG §5.4 载体裁决（B 优先/A 受限/C 过渡）批准。所有者对 §1 一言裁决＝O11 完成；未裁决前**不进码**（批准设计范围≠代码授权纪律）。

## 1. 协议形态（O11 裁决件）

- **P1 wire 形态（spine 面 verb ⇄ 节点，本地进程间）**：
  - `sign.request {request_id, root_digest(64hex), algorithm_id, serializer_id, scope}`——request_id 由 spine Class A 请求事件同事务生成（32hex 不透明 ID，与 identity 平面同形）；
  - `sign.cancel {request_id, reason}`⇒确认 `{request_id, request_digest, state: FENCED}`（绑定请求摘要——P2/P3）；
  - `sign.query {request_id}`⇒`{status: NOT_ACCEPTED|PROCESSING|SIGNED|FENCED, receipt?}`；
  - `sign.receipt {request_id, request_digest, result_digest, attestation}`——attestation＝节点私钥对 canonical(request_id, request_digest, result_digest) 的签名（P3）。
  - request_digest＝P1 请求元组的规范形摘要（shz-canon-v1 复用 B07 序列化器——同一条审计链规范，不新建序列化权威）。
- **P2 节点持久账本与 fencing 形态**：节点账本＝**MC_HOME/sign-node/ledger.jsonl**（append-only 日志＋flock 串行化；"禁仅进程内缓存"IG §5.3① 由落盘结构承载）；每 request_id 状态单向 `ACCEPTED⇒SIGNED` 或 `ACCEPTED⇒FENCED`（日志重放取末态）；**FENCED 单调永久**——重启后迟到请求必拒（IG-T9②）；请求到达时账本末态 FENCED⇒拒并回 `fenced` 确认（IG-T9① 确定竞争序：flock 下 cancel×sign 竞争必有一方赢，"已确认取消但仍签成"结构不可能）；请求摘要与账本记录不符⇒拒＋记 `tampered`（IG-T9③ 取消确认绑定 (request_id, 请求摘要)）；**账本无该 request_id ⇒ NOT_ACCEPTED（IG-T9④：当下受理状态，≠FENCED 的未来执行资格否定）**；节点不可达/查询失败⇒调用方不得解释为 NOT_ACCEPTED（IG-T9⑤）。
- **P3 receipt 验证形式**：attestation 原语＝**ed25519**（Go stdlib，节点 bootstrap 生成密钥对；私钥经 A02 hostSecretStore 面存放，公钥环留 MC_HOME——历史 root 永可验语义与 §5.1 锚定私钥行一致）。spine 在 RECORDED 前**必须**以公钥验证 attestation 绑定 (request_id, request_digest, result_digest)；验证失败＝拒，状态不前（不得仅凭结果文件认定已签，IG §5.3③）。登记：C 形载体的合同文面为"独立用户态进程+age"——本形态以 ed25519（age 同族原语）兑现，载体等价登记；若所有者指定字面 age 二进制，＝节点载体替换，P1-P3 协议形态不变。
- **P4 节点载体与生产锚定门**：按 IG §5.4 O11 记录批准——**B（组织 KMS/HSM）＝生产锚定优先；A（同机隔离容器）＝受限（IG-T4 六项验证=未来验收项，当前 NOT_FOUND）；C（独立用户态进程）＝仅过渡，禁入生产锚定**。D06 交付的参考节点＝C 形过渡体：**其签名不得用作生产锚定**（锚定发布 D07 消费签名前须过 §5.4 载体门——该门控落 D07 面，本票只在节点输出上打 `transitional` 标记）。
- **P5 节点进程形态**：节点＝**按次调用的子进程**（mc verb 以 `sign-node` 命令形态 spawn，per-invocation 模型与 mc 同构；全部状态在 P2 盘上账本⇒进程无内存态⇒"重启"=每次调用天然成立）；flock 承载并发序（P2）。不新增受监督进程/launchd 单元（负清单）。

## 2. spine 状态机与对账（IG §5.3 冻结语义，非裁决面）

- S10 schema：`sign_requests`（request_id 32hex PK／root_digest 64hex／algorithm_id／serializer_id／source_actor FK users／scope／created_at／expires_at SpineTime／state CHECK∈requested|signed|recorded|closed／result_digest 可空／receipt_attestation 可空／supersedes_request_id 可空 FK 自指）＋Class A 事件（sign.requested／sign.result／sign.recorded／sign.closed）＋guarded insert（source_actor 归属复验，D01 模板）。**状态单向**：requested⇒signed⇒recorded 或 requested⇒closed（superseded）；trigger 禁回退（同 user_sessions_state_machine 先例）。
- 对账四分支（`sign-reconcile` verb，host scope，实时读节点 query）：SIGNED⇒取回 receipt⇒验 P3⇒RECORDED（**不重签**，IG-T3 关键例）；PROCESSING⇒保持原 request_id 继续对账；NOT_ACCEPTED⇒close 旧＋supersedes 新请求（新 request_id+原因，DC §3.3）；**UNKNOWN（不可达/查询失败/状态不定）⇒保持待对账，不签发替代**（替代前提＝P2 的 FENCED 可验证确认，IG §5.3⑤）。
- 禁"先记成功再签名"（RECORDED 必含已验证 attestation）与"签完再记请求"（节点只执行 spine 已 REQUESTED 的 request_id——P2 账本无记录⇒拒，孤儿签名结构不可能）。
- **RECORDED≠锚定发布**：锚定发布＝D07 独立 Class A 事件流（(root_digest, algorithm_id) 幂等重试）；两恢复路径不混（本票不建锚定面）。

## 3. 五证测试族（IG-T9，P4.1 实测形态）

- F1 确定竞争序：cancel 确认后迟到 request⇒节点必拒（回 fenced 确认）；**并发竞**：sign.request×sign.cancel 同时到（flock 两侧压力）⇒恰一方赢，输方状态确定，"已确认取消但仍签成"零例。
- F2 持久化：FENCED 后节点"重启"（新进程调用）⇒迟到 request 仍拒。
- F3 确认绑定：query FENCED 返回 request_digest；篡改摘要的迟到请求⇒拒＋tampered 记录。
- F4 语义分离：从未受理的 request_id⇒NOT_ACCEPTED（非 FENCED）⇒对账走 close+supersedes 路径而非"已 fencing"路径。
- F5 UNKNOWN 保守：节点不可达（断管道模拟）⇒对账保持待对账，零替代请求。
- C1-C3（IG-T3 三崩溃点）：C1 崩于 REQUESTED 后（节点无账本⇒NOT_ACCEPTED⇒close+supersedes）；C2 **崩于 SIGNED 未 RECORDED**（节点账本有结果⇒对账取回⇒RECORDED，**断言节点恰签一次**——账本 SIGNED 条目唯一）；C3 RECORDED 后重放（幂等零副作用）。
- S10 族：状态单向 trigger 负控＋guarded insert 归属＋四态迁移。

## 4. 正/负清单

正：schema.sql S10 块＋migrate_queries/migrate.go（S10，identity⇒新基线）＋mc/identity 或 verbs 的 sign_* 决策/对账面＋`sign-node` 子进程命令（mc/cmd/mc 内新命令族，age/ed25519 面 stdlib only 零新依赖）＋F/C/S 测试族＋registry 10 步全族更新。
负：无 launchd/受监督进程（P5 per-invocation）；无锚定发布面（D07）；无 KMS/HSM 集成（B 形＝所有者基础设施侧，本票协议形态与其兼容：P1 wire 对载体无假设）；无 TS/dashboard 面；attestation 验证失败不产生任何"降级接受"路径。

## 5. STOP 条件

- S1 O11 未裁决⇒不进码（本票设计面即裁决包，裁决前止步于此）。
- S2 协议形态需引入进程内节点守护（常驻）⇒停（P5 per-invocation 裁定；常驻＝新监督面，需另行呈报）。
- S3 节点账本需第二 SQLite 权威⇒停（P2 JSONL+flock；第二 spine=红线）。
- S4 ed25519 载体等价被否且字面 age 二进制不可得⇒停（载体面呈报，协议形态 P1-P3 保留）。

## 6. 登记与收尾

设计面（本件）⇒O11 裁决（所有者一言）⇒进码⇒全道⇒登记轮 AM v0.2.25（或随 O11 裁决轮）＋status.json D06=EVIDENCED＋blcheck；新基线 identity 入册；诚实边界（参考节点 transitional 标记／生产锚定门在 D07 消费侧）入登记。

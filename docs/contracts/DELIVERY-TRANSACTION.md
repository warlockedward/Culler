# DELIVERY TRANSACTION（DT · v0.2 草案）

- 状态：**v0.2 DRAFT — 提交复审（SUBMITTED_FOR_REVIEW，登记事件=AM 变更史；本文件不自授 RECOMMENDED）**。v0.2=对 adversarial-findings-001 与 ADV-1..9 九卡（adjudication-recommendations-001，owner 裁「全部按推荐」）的一轮有界修订：P1×4（ADV-1/2/3/9）＋P2×3（ADV-4/5/8）＋P3×2（ADV-6/7）全采纳，逐格落点见 §8；复审限定于改动格（BL/DC 先例的有界复审纪律）。七契约之第七（D21：Backup&Recovery 为章非册）。评审封闭前 Track E 五票维持 BLOCKED（DEV-BASELINE §E）。
- 证据记法沿 TM/DC：〔固化〕=b22df40 已有且经审计（DC-FACTS/ANALYSIS E1）；〔P3〕=MASTER D11/路线图已决义务待实现；〔新增〕=本契约规范新设；〔Planned〕=设计位未供环境事实，不承诺可用。

## §0 权威边界

- DT=交付事务（批准消费、push、PR、远端观察与对账）的**规范语义源**。状态权威唯一=AM；工单图唯一=DEV-BASELINE；存储/幂等/链原语归 DC（DT 只消费与声明消费点，不重定义）；BL/PS 域与交付事务无涉，其幂等模式（settle supersedes、compare 直查键）为**形状参考**非语义授权。
- `GIT-DELIVERY.md` 管本地 merge lane 的操作描述层；与本契约在**远端事务语义**上不一致时 DT 为准，在本地合并/checkout/恢复细节上 GIT-DELIVERY 为准（其 journal/receipt/partial-checkout 规则 DT 以引用方式纳入，见 INV-D3）。
- 本文档不携带实施许可（G-d 声明沿备料原文）：§5/§6 的修复规格经工单授权后方可进生产码。

## §1 域与不变量

三段域：**批准消费 → push（宿主直跑）→ PR 创建与远端回读观察**。

- **INV-D1 批准即消耗**：一次批准只授权一个不可变四元组 `(packet, submitted_version, candidate_sha, destination)` 的一次交付。四元组任一要素变化→批准失效须重批；唯一例外=§4 supersedes 记录链（展示性重开不消耗新批准，命令性重开必须新批准）。依据形态：〔固化〕批准失效规则（GIT-DELIVERY:100-106）。**界定（v0.2，ADV-1）**：四元组界定**一次交付执行**的授权。同一 (packet, submitted_version) 在目的推进后的自动重算（candidate 更新）属**同一执行的内部尝试**，不消耗批准；消耗点=首次进入 push 相。四元组中 candidate_sha 记录为「该次尝试的实际候选」，随尝试更新并事件化。
- **INV-D2 本地权威**：交付状态的一切合法转移先落 spine Class A 事件；远端输出（push stdout、PR 状态、API 观察）**永不**直接进入调度。〔固化 D11-R4 泛化：B8-E 格"PR 状态只进展示层"〕
- **INV-D3 原子审计**：状态变更与其 Class A 事件同写事务（DC §3.2 同型）；宿主 git 副作用窗口=**journal 先于 branch update、receipt 后于 branch update** 三段式，恢复语义引用 GIT-DELIVERY:226-231（按届时正文，漂移由 §0 权威条款吸收〔ADV-7，v0.2〕；含 partial-checkout 只接受 destination∪candidate 两态）。链入按 DC §3.4，事件摘要用已批准件 shz-canon-v1（F0b-1/F0b-3 批准在 AM v0.1.29）。
- **INV-D4 UNKNOWN 优先**：不可证明的远端效果一律记 UNKNOWN，经观察对账收敛；**禁止**以本地信号（超时、连接关闭、tick 停摆）推断"远端未发生"。双向对称：亦禁止以"push exit 0"直接断言远端已达——exit 0 须经 §3 观察路径背书。

## §2 D11 四规则形式化

状态机合法转移集（delivery 态，v0.2 二段命名〔ADV-3〕：`approved→pushing→pushed(self-attested, sha)→confirmed-remote(sha')→lane-clear`；`pushing→unknown→confirmed-remote|lane-hold`；`unknown→pushing` 仅在 §3 观察判定「未到达」时合法〔ADV-9〕）：

- **R1（本地权威）** = INV-D2。非法转移拒绝例：`confirmed-remote→调度动作`（无 Class A 记录）=不存在此边。〔固化〕
- **R2（自证 pushed_sha，二段态〔ADV-3〕）**：`pushed` 态成立当且仅当记录 `destination_commit` 且通过 `ValidCommit` 形态检查〔固化：观察记录已强制 exact commit〕；事件字段=四元组+destination_commit+时间+run 身份，经 INV-D3 链入。**二段分离**：`pushed(self-attested)` ≠ `confirmed-remote(观察背书)`——凡风险类结论（S1/S5、§5 全表与矩阵的安全侧）断言基准=confirmed-remote；展示/调度可消费 pushed，但必须带「未经观察背书」标注。
- **R3（只快进+lease）**：push 只允许 fast-forward；非 FF→`lane-hold`+漂移观察（不重试、不强推、不回退）。串行权威=spine 单打开者（mc 不变量），仓库操作不领 Project lease〔固化 GIT-DELIVERY:44-46〕——跨进程 git 目标锁由 journal/reservation 既有面承接，DT 不另设锁权威。
- **R4（PR 先只进展示）** = INV-D2 的 PR 面；PR 号/URL/state 全部是观察数据。

## §3 push 自证与对账

- 三相：journal（意图+四元组）→ 宿主 git push（keychain 身份，token 不出宿主〔固化 B8-I〕，配置环境=§6 白名单）→ receipt（destination_commit 自证）。
- **对账观察**：`pushed/unknown → confirmed-remote` 唯由宿主直跑 `ls-remote` 类读回，断言 destination_commit 与自证值**语义全等**（比较规则=§6 E03 规格：canonical 摘要非字节串）。
- **UNKNOWN 处置**：push 超时/断连→`unknown`；对账前该 delivery 的 lane 挂起（可解释、可见、可恢复）；恢复动作只读（观察+记录），**不得**二次 push"试一次"——FF 语义使同 commit 重复 push 在远端为 no-op 的假设**不作为无防御前提**。**判定后合法路径（v0.2，ADV-9）**：观察判定**未到达**（目的地与批准前态全等）→同批准、同尝试计数语义下允许再次 push（仍 R3 只快进）；观察判定**已到且≠自证** → S5 路径；未判定 → 维持挂起+人工面。重试次数/间隔=F0b 参数，不新设批准消耗。**人工面（ADV-8）**=操作者经既有控制面（dashboard/Collie）决策，DT 不新设命令通道。
- 目的地一致性：push 目的地=批准目标，URL host+path 精确一致，正反皆测（V-3 强制项，B8-T）。

## §4 PR 幂等（直查键）

- 创建前置=**确定性直查**：以 `(destination repo, base head sha, proposed head sha, 分支命名规范)` 为键查既有 PR；命中→复用（展示更新+supersedes 记录）；命中既有 PR **无论何人创建**→复用+`provenance=external` 标注，不删除不归因，supersedes 链保留该外部事实为节点〔ADV-5，v0.2〕。未命中→创建。禁止模糊匹配（标题/时间窗启发）。键编码复用 shz-canon-v1〔消费声明〕。
- supersedes：同 task 新提交→新 PR 记录并指向被替代记录（BL settle 四元组同型），被替代记录不删除、不再授权任何动作；LWW 禁止。
- PR 创建=Class A 事件（意图先于远端动作，同 §3 三相结构）；创建超时的 UNKNOWN 对账=直查键读回（幂等查询不产生副作用）。
- GitHub API 观察面的认证形态=〔Planned〕（O1/O2 事实未供；G-b 登记于此，不预设 PAT/OAuth/App 任一）。

## §5 交付故障矩阵（八场景 · 规范源=本节；E04 验收主体）

| # | 场景 | 强制结果 | 恢复注记（挂起界/事件链完备性）〔ADV-6，v0.2〕 | 锚 |
|---|---|---|---|---|
| S1 | push 超时/断连（远端效果未知） | 态=unknown，lane 挂起，禁自动重试 push | 只读对账收敛；事件链无缺录（合法重试路径=§3/ADV-9） | §3/INV-D4 |
| S2 | 非 FF 拒绝（destination 已被推进） | 漂移记录+quiet 重算候选；同 submitted_version 按 INV-D1 界定段（v0.2）处理批准归属 | 新 candidate 走 R3 | GIT-DELIVERY:102-104 |
| S3 | journal 后、receipt 前进程崩溃 | 恢复完成原记录（不产生第二候选/第二批准消耗） | INV-D1/D3 | GIT-DELIVERY:226-231 |
| S4 | branch update 中崩溃（checkout 半更新） | partial-checkout 只接受 destination∪candidate 两态，否则 block 人工 | block+Resolve 决策 | GIT-DELIVERY:230-231 |
| S5 | 不可解释外部推进〔ADV-4，v0.2〕 | ls-remote≠记录：该推进可由批准记录/已知自动化主体解释→走 S2；不可解释→hold+人工。不强推不回退 | 人工裁定 | R3/§3 |
| S6 | PR 被第三方 close/merge | 观察进展示，不自动重开不自动重推；如需重走=supersedes 或新批准 | 展示+决策面 | R4/§4 |
| S7 | keychain/credential helper 不可用 | fail-closed；无回退明文、无 token 落日志（脱敏〔固化〕） | 修复环境后重入受 R3 闸 | B8-I/§6 |
| S8 | 观察重录/自激环（b22df40 :1059 缺陷族） | 重录按语义摘要判定：等义不等字节→通过；异义→显式 mismatch；**禁止**原始 JSON 字节相等驱动状态清除 | §6 修复规格+E03 | ANALYSIS/DC-FACTS |

每场景验收=§7.1 记录 + 与 V-list 无重叠声明（V 系测容器/边界，本矩阵测交付事务；交点=宿主 git 环境，归 V-3/E05）。

## §6 强制配置清单（V-3 之"落 DT"）+ 观察器隔离修复规格

**push/观察进程配置白名单（E05 验收=逐条正反实测）**：
1. `GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM` → 部署生成的空模板（不继承用户/系统配置）；
2. `GIT_CONFIG_COUNT=0` 且不继承调用方 `GIT_CONFIG_*`；
3. `core.hooksPath` → 部署持有的空目录；仓库级 hooks **不执行**（V-3 必测：恶意仓库配置+恶意 hook 在场，宿主 push 不受其执行）；
4. credential helper 显式列于部署配置（不遍历系统 helper 链）；
5. `safe.directory` 仅部署仓库路径；
6. 模板目录置空；
7. push 用配置与操作者日常仓库配置完全隔离（正反例皆测）；
8. 目的地=批准目标一致项（§3 末行）；
9. **取源约束（v0.2，ADV-2）**：push/观察的目的地一律来自批准记录/宿主受管配置（destination 复用面），**仓库级 remote 配置视为不可信输入**；正反例：仓库 remote 被改时，push 仍打批准目标或显式拒绝——两形态择一由 ADR 定，测试随 E05。

**E03 修复规格**（契约级；实施另需工单授权）：远端观察结果的复录比较**必须**基于 canonical 摘要（shz-canon-v1 对结果字段全表），禁止原始 JSON 字节相等判断驱动状态转换（现缺陷=重录 JSON 键序/空白漂移即触发"differs from its recorded result"或反之掩盖语义差异；`observed_destination` 的相等清空语义保留，判定输入换摘要）。**前置义务**：观察结果 `result` 字段全表须先入 B03/F0b 家族注册（当前 declared-pending 未覆盖该结果形）——E03 工单的 READY 条件含此注册完成。

## §7 验收映射与残余

- E01←§1(INV-D1/D3)+§2(R2/R3)+§3；E02←§4；E03←§6 规格+§5-S8；E04←§5 全表；E05←§6 清单（=V-3 强制配置正文，本册即"落"）。P3 Exit（AM §2"交付故障矩阵全绿"）判定主体=评审封闭后的 §5 执行记录，本稿提交不产生该判定。
- **残余风险候选（只列不接；接受=Acceptance Authority 专属，TM §8）**：RC-1 GitHub API 可用率与限速非产品可控（观察面降级形态）；RC-2 非 FF 拒绝后的人工介入时延无产品上界；RC-3 keychain 在 headless Linux 的形态未定（O1 事实，影响 S7 实装）；RC-4 ls-remote 读回与真实目的地之间仍存在毫秒级外部推进窗（S5 覆盖其存在性，不承诺消除）；RC-5（v0.2，ADV-8）对端长期不可达时挂起无上界；RC-6（v0.2，ADV-8）PR 直查面同构问题——对端不可达时 §4 幂等读回无上界。
- 冻结路径：评审（审方出分）→ 正文封闭 → CONTRACT_FROZEN 批准+哈希登记归所有者与 Integrity 流程，与本稿无关。

## §8 修订映射

v0.1：首稿。输入=D11 台账行逐字〔master 57〕、TM B8 六格/V-3/§7.2、AM §2/§4b、DC §3.2/§3.4、GIT-DELIVERY 全文、ANALYSIS 缺陷族、DT-PREP 缺口 G-a…d（G-a：八场景规范源=本册 §5 确立；G-b：§4 Planned 格；G-c：§7 P3 判定主体声明；G-d：§0 末行）。

v0.2：九卡有界修订（owner 裁「全部按推荐」；采纳稿=evidence/DT-PREP/adjudication-recommendations-001 §1）。逐格落点：ADV-1→§1 INV-D1 界定段＋§5-S2 改引；ADV-2→§6 第 9 条；ADV-3→§2 状态机二段命名＋R1 例/R2 二段分离＋§3 对账行；ADV-4→§5-S5 更名＋判据行；ADV-5→§4 provenance=external；ADV-6→§5 表头与 S1 措辞；ADV-7→INV-D3 引用注；ADV-8→§3 人工面＋§7 RC-5/RC-6；ADV-9→§3 判定后合法路径＋状态机 `unknown→pushing` 条件边。前置事件=轮六十二 218 netns 迁移案落（owner 挂起条件满足）。E01–E05 维持 BLOCKED（封闭前不解锁，独立转换原则）；评分与 RECOMMENDED 授予归所有者，本稿不自授。

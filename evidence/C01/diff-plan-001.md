# C01 diff-plan #1（设计件 + 字段闭集草案；**未进码**——见 §5 前置门）

状态=DESIGN（route-A 已解 C01 治理门，但 C01 触 **P1 不变量**上的净新增配置层，字段闭集需 Authority 确认后才起 P1 码）。侦察结论全部实读（file:line 本回合亲验）。

## 0.5 ⚑ 进码前二次侦察：**落点从"新命名空间"改为"扩现有 CredentialSource"**（轮二十六·据代码实况，纠正本件 §1 草案；**不改所有者 P1 批的字段闭集，只改放置**）
深读 config.go 后本件原拟"新建 `runtimes.registry` 命名空间"**是错的**——会与既有抽象成平行第二权威（违复用优先律）。实况：
- `CredentialSource`（config.go:73-80）**已有** `InferenceURL`(`inference_url`) 字段，且**目的地流已经通**（agentprofiles.go:181→ResidentRuntimeBinding；:508 integrationAddress；deployment/agentprofile_models.go:149/175/199）。
- 但 `validateCredentialSource`（:1222-1247）把它**锁死为目录值**：`:1243 (source.InferenceURL != "" && source.InferenceURL != reference.InferenceURL)` ⇒ 非空时**必须等于发布目录固定值**；`base_url` 全禁（:1226-1228）；OpenCode kind 要求"no address"（:1230）。⇒ **当前无任何自定义端点入口**，D4（vLLM/SGLang/Ollama 私有推理）今天进不来。
- **正解**=在 `CredentialSource`+`validateCredentialSource` 上**增一条受控 custom kind 分支**（如 `CredentialKindCustomInference`），携 §1 批过的字段闭集（`inference_url` 必填且过 scheme=https / canonical_host / 拒 loopback·私有·保留 / `port∈port_allowlist{443}` 默认 / `pooling∈{off,bound}` / 禁 `insecure*`）；目的地解析复用既有 :181/:508 路径，不另建。
- **收益**：单一权威、既有测试面延续、改动集中在 `validateCredentialSource`（P1 校验咽喉，正是该改的地方）；blast radius 远小于新命名空间。
- **仍守**：`base_url` 继续禁（不放宽，P1）；新分支只开**受控登记**，目的地仍不来自 agent/请求侧。
- ⇒ **进码专窗据本节实施，勿再照 §1 旧"新命名空间"文**。

## 0.6 ⚑ §0.5 被自己问倒 + 本件现已自相矛盾（轮二十六·读全 validateCredentialSource 后；**新的进码前置门**）
读全 `validateCredentialSource`（config.go:1222-1248）后：§0.5"复用 inference_url"与 §1/§2/§3/§6"建 `runtimes.registry` 命名空间"**在本件内直接冲突**——且不是小事，牵 P1：
- **读法 N（票面"命名空间隔离+三闸条目化"字面）= L2 Provider Registry 是独立目的地命名空间**：新增 `runtimes.registry.<id>`{base_url/port_allowlist/pooling}，credential/binding **引用 registry id**。⇒ 目的地与凭据解耦、每条独立过闸、可审计集合；不触碰 :1242-1243 的"发布集成"约束。
- **读法 R（§0.5）= 目的地挂在 `CredentialSource.inference_url`**：现有 :1242-1243 强制 API-key 类 `inference_url` **必须等于发布集成固定值**；R 要为 custom 类**放宽这条**。⇒ 省面，但**可能松动 PS §1 刻意维持的"目的地独立于凭据"边界 + 削弱发布集成保证**（把目的地与 per-credential 耦合）。
- **反转自我纠正**：§0.5 我断"复用=免第二权威"；反过来，**把目的地折进凭据字段才是 PS 想避免的耦合**。票面"**命名空间隔离**"四字更像指名读法 N。**我上一步的纠正可能本身错了** ⇒ 不拿未定解读写 P1 码（AGENTS：改变关键决策才问；不为显推进而半做/赌）。
- **需 Authority 一句定读法**：`N`（独立 registry 命名空间）/ `R`（扩 inference_url，接受耦合权衡，我在 plan 显记弱化了 :1243）/ 或**给 v0.1 PS 原文**（本仓缺，最硬）。
- **执行侧倾向（非裁）= 读法 N**：更贴票面字样、且不弱化既有"发布集成"不变量（P1 面"不放宽现有约束"优先）。**候此一句再进码**；§1 字段闭集（本轮 P1 已批）在 N/R 下都适用，变的只是放置与是否触 :1243。
> 本件 §1–§6 现按读法 N 书写；若裁 R，则 §0.5 生效、§1/§2/§3/§6 需按 inference_url 方案改写。**矛盾已在此标出，不留给读者踩。**

## 0. C01 到底要什么（厘清，避免误读 routing）
- `routing.Registry`= `map[string]string`（binding→harness 族）是**模型路由表**，**不是** PS §1 的 L2 Provider Registry。
- PS §1 L2 = **受控 provider 端点登记**（自托管 vLLM/Ollama 等 D4 需求）：目的地**仍不得来自 agent/请求侧（P1）**，只能来自操作者登记的 registry 条目，agent 侧只引用条目 id。
- 现状障碍：v3+ 配置里 `credentials.*.base_url` 与 `bindings.*.base_url` **被双闸禁用**（config.go:563-571，`carries a forbidden base_url`）——即当前"目的地完全硬编码于 harness 定义"。C01 = 在**不破 P1** 下开一条**操作者受控端点**入口。

## 1. 字段闭集草案（**据 §1 增量+§2 约束+config 现状重建；v0.1 原文不在库⇒需 Authority 逐字段核**）
新增命名空间 `runtimes.registry.<id>`（独立于 credentials/bindings=票面"命名空间隔离"），每条**字段闭集**（超出即缺省拒）：
| 字段 | 类型/域 | 依据 |
|---|---|---|
| `base_url` | 必填；scheme **仅 https**；无 userinfo；canonical_host 规范化（§2.1） | §1 删 insecure_ok 后唯一端点入口 |
| `port_allowlist` | 默认 `{443}`；条目级扩展须评审（不静默放宽） | §1 明文 |
| `pooling` | `off`(默认) / `bound` | §1 + §2.4 |
| **禁止存在** | `insecure` / `insecure_ok`（任何形） | §1 删除、v0.1.1 明文 |
| 待定（**不臆造**） | TLS pin/ca（§2.3 提"证书校验+SNI 绑 canonical_host"但未定字段名） | 标 UNKNOWN，待 v0.1 或 Authority |
> ⚑ **真实信息缺口**：以上闭集是**重建草案**，非 PS v0.1 原文（原文不在本仓，git 历史 PS 仅一提交 902e528）。字段名/是否含 TLS 字段/`protocol` 维度需 Authority 拍板——我不在 P1 面上替 Authority 定安全字段集。

## 2. 双闸改造（"改 custom 路径"的确切含义）
- 保留：`credentials.*.base_url` / `bindings.*.base_url` **继续禁**（现状语义不变，P1 不松）。
- 新增：`runtimes.registry.<id>` 命名空间=**唯一**合法 custom 端点登记处；binding 通过**引用 registry id**（非内联 base_url）使用之。
- 校验链（每条 registry 条目，**默认拒**）：scheme==https → canonical_host 解析 → 拒 loopback/私有/保留（§0-P2/§2.1 完整对象判定）→ port∈port_allowlist → 字段闭集（未知键拒，复用现 `meta.Undecoded()` 机制）。

## 3. 三 https 闸"条目化"
现双闸是"presence 检查"；C01 要的是 registry 每条**独立过**三闸（scheme/地址类/端口），非全局一次——落点=config.go 校验段新增 `validateRegistryEntries()`。

## 4. 测试（票面"配置链闭集测试（缺省拒）"）
- 缺 `base_url`→拒；非 https（http/含 userinfo）→拒；loopback/私有 IP 字面量→拒；未知字段→拒（闭集）；port 不在 allowlist→拒；`insecure_ok` 出现→拒；`pooling` 非 {off,bound}→拒；合法条目→过且 binding 能引用；**正反各一**（植入必红负控）。

## 5. 前置门（为何现在不进码）
C01 改的是 **P1/安全不变量**上的配置面。§H 铁律 + 记忆红线："AI 不能是风险 Acceptance Authority""文档/码不得制造机制"。⇒ **进码前置=Authority 认可 §1 字段闭集**（含 TLS 字段待定项）。草案已尽可达约束之所能；差您一句"字段闭集照 §1 / 增删 X"。
（另：C01 端到端 provider 实际连接=D2 执行面，非本配置票；本票只管配置链闭集校验。）

## 6. 禁触区
`routing/`（harness 表，正交，零动）｜ credentials/bindings 双闸的**禁令本身**（只在其旁开 registry 命名空间）｜ P1/P2 不变量语义（只新增受控入口，不放宽目的地来源纪律）｜ 执行面 Destination 判定（§2 是 C02/C04 域）。

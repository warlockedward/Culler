# Mimosa 完整审计记录（Q7 push 门兑现）· 2026-11-27

轮五十三收口窗，owner 裁定「全按推荐」（F2 先行＝完整审计，绿后进 F1 push 确认）。

## 1. 扫描事实

- 工具＝Mimosa `security_scan_start` depth=deep，目标＝shz/dev @ 6ac1c0d 全树。
- jobId＝scan-job-mwyc1bx6-6fad435564d41204，attempt 1 完成（无 enobufs——scanner_enobufs 兼容期欠账就此清偿）。
- **scanId＝scan-2026-11-27T01-59-14.124Z-2e2724cedeb1**
- **封印＝sha256:6957518e8066bdab1d95d08914341b8f875b97d21625e26b9a1605ccc7949056**
- 历史目录＝/private/tmp/mimosa-shorizon-history/<scanId>（仓外；findings.json/report.md/seal.json/coverage.json）。
- 覆盖：1621 源文件全解析（read/parse failures=0，未截断）；依赖面 572 包扫描，advisory 匹配 4 包 40 条。
- **诚实核心**：coverage.completeness＝`partial`，runStatus＝**`inconclusive`**，evidenceBoundary＝`static_only_no_runtime_execution`，全部 559 finding 的 verdictEffect＝`none`、evidenceState＝`static-finding`（validation 阶段 investigated=0）。缺口声明＝调用图部分不完整（动态派发/规模）。**＝审计已完整执行，但扫描器自身未产生任何通过/失败判定；UNKNOWN≠PASS，本记录不构成安全背书。**

## 2. 发现分布（559＝high 435 / medium 120 / low 4）

| 域 | 条数 | 归属 |
|---|---|---|
| dashboard | 161 | 上游 Culler 既有面（campaign 未改） |
| mc（Go） | 157 | 其中 **11 条触达 campaign 文件**，146 条上游既有 |
| runner | 144 | 上游既有 |
| resident | 61 | 上游既有 |
| evals/performance/release/tools | 36 | 上游既有＋工具面 |

## 3. campaign 文件 11 条逐条真值分诊（agent E1：静态推理＋读码，非独立审查）

1. **main.go:492（command-injection）**：exec.CommandContext 程序名恒字面 "docker"，argv 数组直传不经 shell；args 转递＝helper 委托既有设计面（run.json 动词域闸）。**误报**。
2. **main.go:42（污点链 run→dispatchArgs→delegate）**：同一委托面的图可达表述，落点即第 1 条。**误报（同因）**。
3. **systemstatus.go:83（command-injection）**：action 参数全部调用点＝编译期字符串字面量（status/probe/diagnostics/…/signoff-record）；argv 数组直传，容器名过 ensureSystemStateHelper 当前版本镜像闸。**误报**。
4-10. **sign.go:168/245/260/525/541/566（污点链 env→路径/验证）**：污点源＝MC_HOME 等宿主环境变量＝操作者自己的部署配置＝平台信任根（spine 单写语义、锚定目录、签名节点目录皆建于其上）；流向＝路径操作与收据验证入参，非命令注入面。**误报（信任域外：部署配置在本平台模型中是权威输入，不可信源是容器内/网络侧输入）**。
11. **sign.go:541/566 medium 半**：NodeDir/CanonicalHome 文件路径操作＝上述部署根内寻址，设计本体。**同上**。

依赖 advisory 4 包 40 条＋非 campaign 的 146 条 mc＋全域上游 552 条：**上游 Culler 既有代码与依赖面，不在本 campaign 改动集内**；其处置（修/不修/上游）属 owner 面，本票不扩。

## 4. 门状态（登记后）

- Q7 义务「Mimosa 完整审计重跑」：**执行完成（sealed）**。扫描器判定：**inconclusive（其设计如此：静态发现需人工验证才产生 verdict）**。
- agent 分诊：**campaign 新增面 11 条全部真值否定（逐条有据）**；其余＝上游既有面登记。
- Acceptance 归属：本记录＝agent E1 证据；**push 门的最终接受权在 owner**（AI 永不做风险接受权威）。
- 账：dev tip 随本登记前进／identity 不变／main 仍 PRISTINE／无外发（push 待 owner 确认，形态＝shz/dev→origin/main 纯 fast-forward，113 commits ahead，origin＝warlockedward/Culler 唯一远端）。

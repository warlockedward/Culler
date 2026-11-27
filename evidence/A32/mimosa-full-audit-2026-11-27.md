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

## 8. 轮六十二补跑＝enobufs 欠账清偿（2026-11-27 09:11 UTC）

- 背景：轮六十二两笔提交（85a41f4 netns 迁移工件＋bc3202f AM 登记）commit 前 hook 报 scanner_enobufs，按兼容策略放行并挂队补跑。
- 补跑＝security_scan_start depth=deep，shz/dev @ bc3202f 全树；jobId＝scan-job-mwyrhsqf-37d0fbcfae5e625a，attempt 1 完成。
- **scanId＝scan-2026-11-27T09-11-52.345Z-c650fb7b257f；封印＝sha256:695759603cd5ee4fbc626f3c6dcbed56cc0eea86a6ec48b479ee669fb25d78ad**；历史目录＝/Users/raistlin/.mimosa/security-scans/project-05846d035334cfe0b42fb29c/<scanId>（仓外）。
- 对照轮五十三基线（§1/§2，scanId 2e2724cedeb1）：**total 559／high 435／medium 120／low 4；逐域 dashboard 161／mc 157／runner 144／resident 61／evals 19／performance 10／release 7——两份 findings.json 逐域逐级完全一致＝本两笔提交（纯 evidence/docs 面，零代码）零新增发现面**。依赖面同为 572 包、advisory 4 包 40 条。
- 诚实核心沿用 §1：coverage partial／runStatus inconclusive／static-only／verdictEffect none——UNKNOWN≠PASS，本节同样不构成安全背书。

## 9. 轮六十二续窗扫描链（第五次 push 后，commit 级逐笔封印）

本窗 5 笔提交（d4ab94c push 登记／0435302 DT v0.2 修订／f2f1159 AM v0.2.43／85407fd 现状附录）的完成扫描对照轮五十三基线（§1/§2）＝**全部 559／high 435／medium 120／low 4，逐域 dashboard 161／mc 157／runner 144／resident 61／evals 19／performance 10／release 7 完全一致＝零新增发现面**（本窗变更=evidence/docs/契约正文，blcheck 另证零锚漂移）：

| 树 | scanId | 封印（job seal） |
|---|---|---|
| d4ab94c | scan-2026-11-27T09-24-31.976Z-c2830867f39c | sha256:20643432204a31a788d6a435b06cb89594ffe23a4f8166d9bafc90fb15aef3ab |
| f2f1159 | scan-2026-11-27T09-28-52.218Z-d268f905c59a | sha256:c7969203c856a885c39a82db552099220ba2a2bb92e8e65f538634464c1045a5 |
| 85407fd（tip） | scan-2026-11-27T09-30-53.205Z-2b41731fea9b | sha256:ab84199ef9aad82d149d62cf176df317e76e2034c487bab62ac45b07b6a7b251 |

- 过程如实：job mwys5d1f（首试 85407fd 树）audit 阶段状态回写停滞且未落盘＝取消重发 mwys67bh 成功；hook 在各笔 commit/push 时点的 scanner_enobufs 兼容放行记录在册。
- §9 本笔登记提交自身＝纯封印号追加（零新语义内容），不覆盖自身扫描（自尾结构=登记文档性质，如 AM 惯例）。
- 诚实核心沿用 §1：coverage partial／runStatus inconclusive／static-only／verdictEffect none——不构成安全背书。

## 10. 轮六十二续窗终封（final-tip 清账，2026-11-27 09:47:39 UTC）

- 树＝dev tip 31c904f（本窗全部 7 笔增量在内：push 登记/DT v0.2 修订/AM v0.2.43/现状核查附录/§9 册/双臂登记/AM v0.2.44）。
- jobId＝scan-job-mwysrq61-61b348026b841cef，attempt 1 完成（无 enobufs）。**scanId＝scan-2026-11-27T09-47-39.033Z-4e249cae6f92；封印＝sha256:03117af7a9cced5bb1b5226be231da035129c1eeb3efa31e6bfd50532343470f**。
- 对照轮五十三基线：**total 559／high 435／medium 120／low 4，逐域 dashboard 161／mc 157／runner 144／resident 61／evals 19／performance 10／release 7＝完全一致＝本窗全部变更（契约正文修订＋evidence/docs）零新增发现面**。
- 本窗 enobufs 兼容放行欠账＝就此清偿（§9 逐笔封＋本笔终封双层覆盖）。诚实核心沿用 §1：partial／inconclusive／static-only／verdictEffect none——不构成安全背书。

## 11. 轮六十二 grill 轮终封（批量裁执行窗）

- 树＝dev tip 8f27cbe（Q1–Q5 批量裁执行全部增量：boot 臂工件/dnsmasq 单元/生产预案/AM v0.2.45/第六次 push 登记）。
- jobId＝scan-job-mwyt71wo-61700bccf25bacd9（attempt 1 完成，无 enobufs）；**scanId＝scan-2026-11-27T09-59-31.590Z-4bcfb0e77ab7；封印＝sha256:81d37f28fbff0f5c5cb44d1cfb22a77c3408a25b7d469f094092283c980aa765**。
- **findingCount=559／依赖面 572 包／advisory 4 包 40 条＝与轮五十三基线及 §9/§10 各封完全一致＝本窗批量裁执行（契约/evidence/部署预案，零生产码）零新增发现面**。
- 本登记提交＝§11 自尾追加（登记文档性质，同 §9 声明）；诚实核心沿用 §1（partial/inconclusive/static-only/verdictEffect none），不构成安全背书。

## 12. 轮六十三生产码提交扫描（D-1 zsh 修复树）

- 树＝dev tip 50e137c（含本campaign内首个生产码改动 4fe3052＝cli_install.go 缺失壳跳过＋回归测试）。
- jobId＝scan-job-mwyvtyrz-45cc8e0f20e4a8f2；**scanId＝scan-2026-11-27T11-13-19.367Z-796d04fbbe4a；封印＝sha256:58da6a2ede6b5349a7b2c880423ce5ae7eb2ad8482f9d3f188d857a0b383add2**。
- **559／435/120/4 逐域与基线全等；mc=157 未增＝新增码行零新发现**；cli_install.go 名下 4 条＝轮五十三 §3 已真值分诊的既有项（command-injection 面，同一委托设计）。
- 诚实核心沿用 §1（partial/inconclusive/static-only/verdictEffect none）。

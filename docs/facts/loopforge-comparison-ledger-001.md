# LoopForge 对照追踪台账 · loopforge-comparison-ledger-001

日期 2026-11-27（轮五十七）。所有者问「把 LoopForge 克隆进本项目做随时对照是否更好」⇒**裁定形＝不克隆进仓（三件硬事实反对），改用"仓外追踪镜像＋仓内台账"双层制**；本文件＝仓内台账权威位。

## 1. 为什么不进仓（研判记录）

1. **许可证＝NOASSERTION**（gh api 实测 2026-11-27：Tencent/LoopForge 无标准许可声明）。无许可证＝默认全权保留；把其代码树放进**已公开推送的 MIT 仓**（origin=warlockedward/Culler）＝合规风险不可确认的代码引入，按任务书 §16.4 属必须拒走的面。
2. **体量**：仓 78 MB（GitHub size 字段）＋持续活跃（pushed_at 当日）⇒进仓即永久污染历史与克隆成本。
3. **漂移权威**：vendored 副本会静止在钉入的 commit 而无人知——对照研究要的恰是"知道自己在跟哪个版本比"；外置镜像＋本台账 SHA 钉记＝同一能力，零第二权威（Reuse Governance：Research Record 属于我们，Repository 属于他们）。

## 2. 追踪机制（可执行协议）

- **镜像位（仓外，持久）**＝`/Users/raistlin/Downloads/git/Ai/loopforge-track/LoopForge`——blobless 克隆（`--filter=blob:none --no-checkout`）：全历史 commit 图可 diff，不拉文件体，成本≈数 MB。
- **刷新**＝`git fetch origin`（不 checkout）；**触发＝按需**（设计某域前/迭代规划前/季度盘点），无自动任务。
- **对照义务**＝动向前必做三件事：记 `git log -1 --format=%H` 新钉；`git rev-list --count <旧钉>..HEAD` 增量数；粗扫 `git log --oneline <旧钉>..HEAD` 主题⇒有值则立研究记录（evidence/ 或 docs/design/ 新件，含文件路径级证据），借用/反模式各归其账。
- **进仓的只有我们的分析件**（下节史册），从不进他们的代码。

## 3. SHA 钉记史

| 钉 | 日期 | 增量 | 产出 |
|---|---|---|---|
| `451d38c` | 2026-11-19 | 首研 | `ANALYSIS-2026-11-19-LoopForge-vs-Culler.md`（仓根，未跟踪——注：属操作者工作面文件，未入 campaign 账）＋PS §2 gitegress 传输模式（已进 PROVIDER-SECURITY 正文）＋借用/反模式清单（会话记忆） |
| `be6918d`（v0.4.3 预备，2026-11-26） | 2026-11-27 | **+101 commits** | 本台账＋下表动向摘要（未逐文件深研——深研按域触发） |

## 4. 451d38c→be6918d 动向摘要（主题级，E1=log 实扫）

- **词汇面大改**：SOP→LOOP 全仓更名（#1286）——对照文档若引用其概念须换词。
- **与我们在册面直接相关**：①**资产进化与 RSI 工作台**（#1314）＝我们 D-track/受控进化域的活体对照物，**建议下轮迭代规划前按域深研**；②privileged Docker sandbox 配置面（#1450）＝我们沙箱票（OpenSandbox 系）同域；③workspace 只读目录回收/checkpoint 写削减（#1410/#1412）＝我们 runtime 卷语义可对照；④MCP 审批等待期保持工具响应（#1351）＋审批卡 UI（#1250）＝HITL 面板对照；⑤authz 对齐（#1328）＝D-IG 三域矩阵同题。
- 运营形态观察：fix 密度高、UI 驱动、issue 导入/审批流产品化——与 S-Horizon 的"编排内核＋治理"侧重不同，反模式清单继续有效。

## 5. 边界

台账＝指针与摘要，不复制其代码文本；深研结论一律以我们自己的研究记录/ADR 形态入账（带 SHA＋日期＋文件路径证据）。此机制可平移到任何其他对照仓。

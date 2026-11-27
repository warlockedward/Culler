# 上游 Culler 追踪台账 · culler-upstream-tracking-ledger-001

日期 2026-11-27（轮五十七续）。所有者指示：本项目＝fork 自 Culler 官方仓库的二次开发，上游更新可作参考设计。机制沿用 LoopForge 双层制（其 §5 预告的可平移首例），**但血统关系多一条专属红线**（§4）。

## 1. 事实

- 上游＝`github.com/vinchenkov/Culler`（gh api 实读：MIT、非归档、非 fork、58 stars、pushed_at 2026-11-21、全仓 16 MB）。
- **分裂点＝`b22df40`（"Release Culler 0.1.0"）＝本地 main PRISTINE 锚**；b22df40 经镜像实证仍为上游 HEAD 之祖先（谱系无重写）。
- 上游自分裂点＝**+9 commits（至 `9e29e98`＝0.1.1 merge）**，40 files +870/−85，主题：
  1. **版本语义政策**：`478fa64 "Reserve minor versions for data conversions and incompatible changes"`——**治理级信号**：上游开始为"数据转换/不兼容变更"预留 minor 版本 ⇒ 其原生形态可能正走向 converter 支持；**我方教义相反**（schema 变更⇒脊不兼容、无转换器、identity 字面钉史 S1-S12＋AM 登记制）。此分叉是**有意识的架构决定**（AGENTS Invariants＋D20 无兼容层裁定在前），非落后——任何借鉴需过 ADR，不随上游默认走。
  2. agent profile 删除面（dashboard＋Collie，#89a4df3）；③操作者控制 UX×2（清 infrastructure failure／dismiss 失败回复＋allowance 重试，#ebabb02/#5d2be3c）；④README/VERSION/CHANGELOG/发布件。
- 镜像位（仓外持久）＝`/Users/raistlin/Downloads/git/Ai/culler-upstream-track/Culler`——blobless（`--filter=blob:none --no-checkout`），实测 204 KB 全历史可 diff。

## 2. 刷新协议（与 LoopForge 台账同形）

按需 `git fetch origin`（不 checkout）；动向前三步＝记新钉 SHA→`git rev-list --count <旧钉>..HEAD` 增量→`git log --oneline` 主题扫⇒有值者按域立研究记录。**本台账 §3 记钉史。**

## 3. SHA 钉记史

| 钉 | 日期 | 增量 | 摘要 |
|---|---|---|---|
| `b22df40`（fork base） | 2026-11-19 | — | 分裂点，本地 main 永久锚 |
| `9e29e98`（0.1.1 merge） | 2026-11-27 | **+9** | 版本语义政策转向信号（§1.1）＋profile 删除＋操作者 UX×2 |

## 4. 血统专属红线（LoopForge 制之上追加）

1. **绝不向 vinchenkov/Culler 外发**（推送域记忆在案）⇒**工作仓内永不添加名为 upstream 的 git remote**——把"不可能误推"做成结构事实而非纪律自觉；镜像是独立 clone（自带 origin＝上游），与工作仓零 git 关联。
2. **永不 bulk-merge 上游**：分裂后我方 74 票量级的 schema/ verb/ wire 演化与上游 40 文件改动直接对撞无意义；上游变更只允许两种入仓形态＝(a) 研究记录/设计参考（本文类）；(b) **逐笔有票有 ADR 的选择性借鉴**（owner 批准，附版本证据＝本台账钉记）。
3. 借鉴评审时先查本台账 §1.1 的**分叉点清单**（当前唯一在册：版本/converter 语义）——分叉面上的上游改动默认不采，除非 owner 重裁。

## 5. 与 LoopForge 台账的分工

LoopForge＝同业竞品（设计模式对照）；本台账＝血统父仓（行为差异/缺陷修复/惯例演化对照，兼"如果我们不 fork 会怎样"的反事实基线）。两份台账同放 `docs/facts/`，刷新义务同形，机制已可平移第三仓。

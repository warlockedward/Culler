# VISION.md 草案 · 候签署（轮六十三，P6 起草先例：agent 起草，owner 签署）

放置位置＝218 部署 workspace（`/data-shunhong/shorizon-workspace/culler/VISION.md`），**不入 dev 主干**；签署与落位归 owner。Strategist 指令以本文件为方向输入（`mc/verbs/directives/strategist.md:7`）。

---

# Vision

S-Horizon is the autonomous software factory for this repository and the products
we build with it. It runs on our own hardware, answers to one operator, and
develops software the way a strong small team would — with the operator as the
decision authority at every point where judgment, money, or risk changes hands.

Culler itself is the first project the factory develops. The pipeline proposes,
builds, verifies, and packages the work; the operator reviews packets and rules.
Nothing merges, ships, or spends without a named human decision.

What we are building toward:

- A Culler deployment that a single operator can run headless on a Linux host,
  where agents do the routine development work and a human keeps authority over
  direction, delivery, and safety.
- Repository work that arrives as review packets with real evidence: checks that
  ran, results that were read, and limits that were honestly recorded — never a
  claim that stands on hope.
- An environment that respects the machine it lives on: shared hosts stay
  untouched outside the deployment's own declared footprint.

We are not building a multi-tenant cloud service, and we are not building remote
runner infrastructure. Scale beyond one host and one team is out of scope until
running at this scale stops being the interesting problem.

The measure of success is simple: over months, the factory turns the operator's
judgment into shipped, verified software with less of their time per unit of
work — and no surprise, at any point, about who is allowed to do what.

---

*草案注记（不随签署落位）：措辞锚定既有事实——单组织自托管定位（DEV-BASELINE 共识）、"packet+人工批准"形态（DT/GIT-DELIVERY）、共享宿主零伤害纪律（§2 (a)-(e)）、非目标清单（多租户 SaaS/remote runners，2026-11-19 共识）。owner 可整段采纳、改写或另撰。*

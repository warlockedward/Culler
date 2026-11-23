# A09 R2 改名波 · 侦察与执行分解 #1（爆炸半径=实测数字；**未进码——本票经侦察判定不可单回合安全原子完成，见 §4/§5**）

侦察日期 2026-11-23；基线 dev `4afb251`。**本件推翻执行侧上一轮"A09 能自主推到底"的乐观**——那是未测就下的判断，数字证明它错。守 never-merge-red：不硬冲一张半做必破 trunk 全绿的 687-文件破坏性票。

## 1. 实测爆炸半径（grep 直读，非估算）
| 维度 | 命中 | 含义 |
|---|---|---|
| 全库含 culler 文件（go/ts/sh/sql/md） | **687** | 改名面 |
| `com.culler` 命名空间文件 | 32 | systemd/launchd 单元名 + 掩码正则 + 其测试 |
| `.culler` home 引用文件 | 80 | `~/.culler`→`~/.shz` 及派生路径 |
| 不同 `CULLER_*` 环境变量 | 78 | 全须与代码/测试/文档同步 |
| **测试文件**含 culler | **191 go + 42 ts = 233** | 改名必带动测试同步（占绝对多数连锁源） |
| schema.sql / delivery.sql 含 culler | 4 / 3 | **任一字节改动⇒SHA-256 变⇒所有已部署 spine 不兼容、无转换器**（AGENTS 铁律） |
| host_release.go 产清单 | 15 | 改路径⇒闭包测试（缺项/多项即红）连锁 |

## 2. 其中一处**语义歧义，非品牌、须澄清**（不能盲改）
- `mc/substrate/schema.sql:215 destination_branch TEXT GENERATED ... AS ('culler') STORED` 与 `delivery.sql:118/170 ... NEW.destination_branch <> 'culler'` —— 这里的 `'culler'` 是**Git 交付目标分支名**（功能语义：packet 交付到哪条分支），**与产品品牌改名可能无关**。
- 改它 ⇒ ① 每条 review packet 行的生成列值变；② 触发器比较逻辑变；③ **spine hash 变**（不兼容）。
- `schema.sql:1` "The spine stores Culler state"、`:2068 'Culler health: '` 是注释/健康通知正文（用户可见文案），`delivery.sql:356`/`schema:2799` 是注释。
- **歧义问**：A09 的改名目标是否**包含把 Git 分支名 `culler`→`shz`**（=改交付域功能约定），还是仅品牌/路径/单元/env、Git 分支名 `culler` 保持？二者代价与风险不同。执行侧不猜。

## 3. 一个可拆出的**独立语义矛盾**（须用户裁）
票面含 `shz migrate-home`（迁移工具）↔ **D20"不建兼容层，旧 Culler 部署一次性卸载重装"**。若 D20 生效，`migrate-home` 与"一次性重装"冲突——migrate 是可选开发者数据搬迁？还是 D20 需微调？执行侧不擅调和。

## 4. 建议的可独立全绿分步 WBS（每步=一个原子提交，各自全道绿再并；**非单回合**）
- **S1（最低风险，先做）**：78 `CULLER_*` env 前缀→`SHZ_*`，含其读取点+测试+文档；不动 home/单元/schema。可独立回归。
- **S2**：`.culler` home + `~/.culler` 路径 + 80 文件 + 其测试（**不碰 schema**）。生产 home 破坏性但 dev 期 D20 允许。
- **S3**：`com.culler.*` 单元命名空间（32 文件：supervision 生成 + 掩码正则 + 测试）——触 systemd/launchd 激活面。
- **S4**：`mc`→`shz` 二进制/CLI 名 + `host_release.go` 产清单 15 项 + 闭包测试 + 所有 spawn/路径引用（**最大连锁**，含 233 测试同步）。
- **S5（触 spine 不兼容，最后、且需 §2 分支名歧义先定 + D20 重装窗口）**：schema.sql/delivery.sql 内 culler（注释 + 若含则 Git 分支语义值）⇒**必须在 B02 v1 基线冻结之前**（票面硬注），且需明确重装/迁移策略。
每步都：改→`mc/check.sh`+resident+dashboard 全道→扫描（Go/TS 在覆盖集；shell 按分域声明）→ff 并 dev。**任一不全绿则该步不并**。

## 5. 为何本回合不进码
(a) S1–S5 任一步都不是"改一处"，S4 一牵 233+测试；在单回合内塞进整波=大概率破 trunk 绿（违背 never-merge-red/全道绿才并）。(b) §2/§3 两个语义点**会改变改什么**（分支名是否动、migrate vs D20），属产品/安全决策，AI 不代裁。⇒ 执行 A09 的正确第一步=本侦察+分解（已完成），真正动手需：**用户对 §2/§3 各一句 + 批准按 S1..S5 分步开专窗**。

## 6. 若用户给方向，执行侧可即刻自主连做的
一旦"分支名不动/migrate 保留或删"落定，**S1 起我可分步自主做**（每步全绿再并，不需逐步请示，符合"连续推进"）；schema 那步（S5）单独等 B02 窗口协调。

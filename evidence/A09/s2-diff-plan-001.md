# A09-S2 · 部署 home 面 `.culler` → `.shorizon`（R2 表终值）— diff-plan #1

## 0. 授权与终值依据
- P4（轮二十五"全按推荐"）分步授权 S1→S5 每步全绿再并；Q5（轮三十三）两默认追认清了票面歧义。
- **终值依据（交叉核对）**：ITERATION-ROADMAP §2R R2 表"部署 home：默认路径改 `~/.shorizon`（env 变量名 `MC_HOME`/`MC_*` 前缀保留=D15）"＋BRAND-WHITELIST B 行"部署 home `~/.culler`→`~/.shorizon`"。⇒ **home 面终值=`.shorizon`（不是 `.shz`；`.shz` 只给 CLI 二进制与 token 前缀）**。

## 1. 替换面（token 级精确边界，机械可验）
| Form | 正则 | 终值 | 实测 token 数 |
|---|---|---|---|
| H（home 基名族） | `(?<![a-zA-Z0-9])\.culler\b` | `.shorizon` | 185（64 文件） |
| C（config 目录） | `\.config/culler\b` | `.config/shorizon` | 19（8 文件） |

H 族覆盖：生产 home 默认值（ops.go:87／canonical_home.go:21／culler_prod.go:198／dispatchverb.go:909／cli_install.go:120／uninstall 候选表）、Project 环境目录约定 `.culler/Dockerfile|.culler/tools.sh`（白名单 B 行"`.culler/` 目录随 A09"）、`Dockerfile.culler-project` 文件名、staging 临时名（`.culler-restore-`／`.culler-command-`）、测试 fixture 路径、掩码正则 `\.culler`、AGENTS.md/INSTALL.md/README 活文档。
**token 级边界天然排除** `com.culler.*`（S3 面，前缀=字母 `m`⇒lookbehind 不命中，143 token 零触碰）。

## 2. 排除面（守界，逐面登记理由）
| 面 | 理由 |
|---|---|
| `com.culler.*` 标签/单元名（S3 面） | R2 表 launchd/systemd 行：darwin 终值=`com.<域名根>.shorizon.*`，**域名根=O9 所有者供给事实（DEV-BASELINE §2"工单不得自行推测"）⇒ S3 BLOCKED on O9**；token 边界已天然排除 |
| keychain 文件名/service 前缀（Form K，`culler.keychain-db`/`Culler-<sha>`） | R2 表 Keychain 行="P0 凭据后端 ADR-0 一并设计"⇒ **A02 凭据后端面**（A02 已 EVIDENCED 机制的命名约定），不并 S2 |
| skill 映射链（`~/.agents/skills/culler`＋清单逻辑名＋仓库源目录＋frontmatter name） | BRAND-WHITELIST **C 类=agent 语义面，R2 末尾单独 commit＋角色指令检查**（AGENTS.md 三件套义务）；R2 表只点名用户机目录终值，映射链整体改名属 C 类 |
| `openwiki/.culler-run.json`（wiki/completion.go:170） | mc wiki 导出↔openwiki（Scribe 专属域）的**跨域文件约定**；产品侧单方面改名=约定断裂（openwiki 侧归 Scribe）⇒ 具名残留候 Scribe 协同 |
| `culler-worktrees` 路径约定（Form W） | AGENTS.md 明文"更名本身是一行工单"⇒ 收口批（S5 后）与 worktree 物理迁移一起做 |
| `culler_evals` Python 包名 | R2 表未列；目录/包名面=更接近 S4 面，具名残留 |
| `docs/contracts/`、`docs/facts/`、`docs/design/`、`evidence/`、`openwiki/` | 治理面/时点记录/Scribe 域（S1 已立先例：治理文档历史事实条目不随机械改名动） |
| `ITERATION-ROADMAP-2026-11-19.md`、`performance/results/`、`evals/verification/*.json` | 时点文档/实验记录=事实面，不改写历史 |
| `.mission-control`（uninstall 候选 legacy home） | 前代产品名，历史清理候选，不属本次迁移 |

## 3. 验证
1. 范围内 Form H/C 残留=0（token 级 regex 复核）；排除面 token 数不变（com.culler=143、skills/culler=31、keychain=9、worktrees 活面）——前后差证明"只动了该动的"。
2. 生产 home 默认值单测（names_test.go/ops 面）随 token 同步后必跑必过。
3. `mc/check.sh` 全链＋resident＋dashboard 三道全绿才 ff dev（full-lanes-before-merge 硬规则）。

## 4. 已知影响（如实登记）
- **生产 home 破坏性变更**：改后产品默认找 `~/.shorizon`，旧 `~/.culler` 部署=不存在（D20 不建兼容层，用户基数=自己；R2 表"老部署一次性搬迁步骤=文档化，非代码兼容层"）。本机如有 `~/.culler` 生产部署面，S2 落地后其默认解析消失⇒需要时由 `MC_HOME` 显式指回（不属产品行为）。
- AGENTS.md 的 "S-Horizon uses `~/.culler`, the production home" 等活描述随 S2 更新为 `~/.shorizon`（AGENTS.md=活文档面，不在排除表）。

## 6. 执行期自抓缺陷（如实登记，均已修复）
1. **Form C 纯件被预过滤漏掉**：替换脚本以 Form H 的字面子串 `.culler`（点紧跟 culler）做文件级快筛，但 Form C token `.config/culler` 的点在 config 前 ⇒ 纯 Form C 文件（tests/live-model 3 件×3 token）整体被跳过 ⇒ 补跑 Form C 替换。教训：多形态改名不得用单一形态的字面子串做文件级预过滤，执行后必须**按形态逐一复核残留**。
2. **跨面污染（lookbehind 盲区）**：Form H 正则 `(?<![a-zA-Z0-9])\.culler\b` 挡字母数字前缀但**不挡反斜杠前缀** ⇒ shell/awk 模式里的转义形态（S3 单元面！`com`+反斜杠+`.culler`+反斜杠+`.`）被命中改写成 shorizon 对应形态 ⇒ 单反斜杠形态 3 文件 8 行（redeploy.sh / redeploy-dashboard.sh / runner/image/build-prod.sh 的 launchd/systemd 单元谓词）⇒ `units_loaded()` 恒假 ⇒ **redeploy_test.sh 的调用序断言红**（mc/check.sh 第 3 跑 MC_CHECK_EXIT=1，零测试 FAIL=shell 门早死）。教训：改名正则的"边界"必须把**转义形态**（反斜杠点、`%s` 插值形态）列入探测面；§2 排除表写的是语义面（`com.culler.*`），执行面正则没实现该语义=计划与实现的裂缝，由门（redeploy_test）抓住。
3. **pin 测试输入被改、输出未随迁**：`TestRuntimeNamesPinDomainAndGrammar` 的 fixture home 路径（输入）被 S2 改名，但 pin 的哈希常量（输出，`676ccb230491`）不含 `.culler` 字面 ⇒ 非改名目标 ⇒ 输入输出失配红。修复=对**新 fixture** 独立重算 pin（python 复算 `sha256("mc-deployment-v1\0"+home)[:12]`：旧值复算吻合=hash 函数与 domain 常量零改动的旁证；新值 `2e262ab57590`=测试函数自身输出，双源一致后写入）。转写≠放宽：断言仍是严格常量 pin，仅 fixture 语义更新。
4. **恢复通道自身的形态盲区（第 2 条的二次残留）**：第一次恢复搜索只用**单反斜杠**形态做字面匹配，漏掉 dev-reset.sh 里的**双反斜杠** awk 源字节形态（同一污染、不同转义层）⇒ 第 4 跑 check.sh 死在 `dev-reset_test.sh: the systemd stop was not issued`（单元谓词仍指向 shorizon 形态，测试 stub 报 culler 形态单位 ⇒ 找不到 ⇒ 不发 stop）。按 `com+反斜杠×N+点+shorizon` 全转义层重扫恢复（dev-reset.sh×1＋本件引用处×1）⇒ dev-reset_test 单门 RC=0。教训：**恢复探测面必须枚举全部转义层**（1x/2x/4x 反斜杠），不能只搜已见形态。

# A09 尾面 · Q-S skill 语义面 ＋ Q-K Keychain darwin 名面 ＋ Q-W worktree 路径 — diff-plan #1

## 0. 授权
所有者 2026-11-25 二轮"全按推荐"：Q-S 执行（F1 分类表 §4 推荐）／Q-K 直接终值（零部署⇒无迁移代码）／Q-W 一行工单＋物理迁移／F1 走查＝本机 dev tier scratch 域。

## 1. Q-S 面（skill 逻辑名＋目录＋三件套同步，R2 表明文"R2 末尾单独 commit"）
**标识符面（culler→shorizon）**
| 位置 | 改 |
|---|---|
| `mc/verbs/skills/collie/culler/` | `git mv` → `mc/verbs/skills/collie/shorizon/` |
| `SKILL.md:2` frontmatter | `name: culler`→`name: shorizon`（shipped-skill 不变量：folder≡frontmatter name） |
| `mc/verbs/roleskills.go:194` | `if name == "culler"`→`"shorizon"`（shared references 注入＋installation.json 装配的条件键） |
| `mc/verbs/roleskills.go:203` | `"skill": "~/.agents/skills/culler"`→`shorizon` |
| `mc/cmd/mc/agent_skill_prod.go:17` | 部署树 skill 目录 `release/host/skills/culler`→`shorizon`（用户机安装源目录） |
| `mc/verbs/directives/collie.md:10` | `~/.agents/skills/culler/SKILL.md`→shorizon |
| `runner/collie-runner/main.ts:384` | 同上路径 |
| 测试 fixture（产品安装目录断言） | `agent_skill_external_test.go:67`、`agent_skill_prod_test.go:13/:25/:37`、`collie-runner/main.test.ts:1092` |

**prose 面（角色输入三件套＝AGENTS.md 义务；产品名 S-Horizon 替换）**
- `mc/verbs/brief.go`×8：spawnBriefHeader "Culler immutable run brief"／"## Standard Culler directive"（brief_test.go×2＋customization_test.go×1 pin 随改）／Culler supplies keeps published 散文×5／注释×2。
- `mc/verbs/directives/{arbiter,collie,packager,scribe,setup,strategist,verifier,worker}.md` 全量 "Culler contract/Culler supplies/Culler tokens/Culler state..." 散文。
- `mc/verbs/skills/**/SKILL.md` prose（含 collie/shorizon/SKILL.md 自身＋references/*.md prose）。

**保留面（逐面理由）**
| token | 位置 | 理由 |
|---|---|---|
| `destination_branch='culler'` | brief.go:555 SQL | Git 分支面（Q5 轮三十三裁定保留） |
| `/opt/culler-runtime/...`×2 | mc-setup/SKILL.md:137/:149 | 容器内运行时路径常量＝内部代号面（D15；镜像布局不改＝A04 面） |
| `FROM culler-project-base:` | project-environment-setup/SKILL.md:107 | 容器/refs 行 R2 表明文保留（mc 内部代号族） |
| `references/culler-model.md` 文件名 | skill 目录内 | R2 表未列的内部参考文件名（内容 prose 改，文件名留＝具名残留登记） |
| `installEnvelopeSkills(..., ["culler"])` | agent-runner/skills.test.ts:203 | 通用 skill 安装器的任意名数据实例（测安装器行为非产品语义） |

**角色指令检查（AGENTS.md 义务）**：mc lane 的 skill 装载测试（roleskills 测试族＋agent_skill 测试族）即检查器；跑全道＝履行。

## 2. Q-K 面（Keychain darwin 名面直接终值）
| 位置 | 现值 | 终值 |
|---|---|---|
| `mc/deployment/runtimeauth_keychain_darwin.go:19` | `claudeIsolatedKeychainName = "culler.keychain-db"` | `"shorizon.keychain-db"` |
| `mc/deployment/host_secret.go:37` | `"Culler-" + hex`（keychain service 引用/blob 目录共享逻辑） | `"S-Horizon-" + hex` |
| `mc/deployment/runtimeauth_host_sources.go:84-86` | claude 探测源 `Library/Keychains/culler.keychain-db{-wal,-shm}` | `shorizon.keychain-db...` |
| 测试 pin 面 | host_secret 测试族／runtimeauth 测试族 | 随改 |
**Linux state tree 命名不动**（`culler-host-credentials/`＋`Culler-<hash>/`＝A02 证据 E1 在案的 ADR-0 SIGNED 定义域；host_secret_linux.go:57 属该域）。**注意** host_secret.go:37 的 `"Culler-"` 前缀是 darwin keychain service 引用与 Linux blob 目录**共享**的函数⇒若只改 darwin 面则需拆分该逻辑——执行时 E1 核 :37 的实际调用面后定"拆函数"vs"整体改"（整体改会动 Linux 面＝越 ADR-0 域⇒**判定：拆**。darwin keychain service 引用名与 Linux blob 目录名是两个面，函数拆分使各自终值独立）。

## 3. Q-W 面（worktree 路径一行工单＋物理迁移）
| 位置 | 改 |
|---|---|
| `AGENTS.md:65` | `/private/tmp/culler-worktrees/<task>`→`/private/tmp/shorizon-worktrees/<task>` |
| `evals/culler_evals/storage.py`／`continuation.py` worktree 路径面 | 随改（执行时 E1 核） |
| `docs/BRAND-WHITELIST.md` worktree 行 | 现值列保留（时点记录），不动 |
| **物理迁移** | `git worktree remove`+`add` dev 工作树到 `/private/tmp/shorizon-worktrees/dev`（dev 自指面：从目录外执行；AGENTS.md 规则先落地再迁移） |
| evals/tests/... worktree 路径引用 | 执行时 E1 核 |

## 4. 验证
1. Q-S：`./mc/check.sh` 全链（roleskills/agent_skill/brief/customization 测试族）＋resident（无涉）＋runner agent-runner/collie-runner lane（skills.test.ts／main.test.ts 面）。
2. Q-K：mc lane（host_secret/runtimeauth 测试族）＋darwin keychain 面在本机可实跑（macOS host）⇒若有 keychain 集成测试则本机绿即实证。
3. Q-W：evals pytest 所触文件＋AGENTS.md 无测试面。
4. 三面同提交或分提交：按"R2 末尾单独 commit"＝**Q-S 单独提交**；Q-K/Q-W 同面小改可并一提交（或各自一提交，执行时按最小原子面定）。
5. 残留 grep：角色输入面 `grep -ri culler mc/verbs/{directives,brief.go,skills}` ＝仅保留面 5 类。

## 5. 守界
不动 Git 分支面／容器路径面／Docker 标签面（前三已裁定）；Linux state tree 命名不越 ADR-0 SIGNED 域；main 不推进；完成后＝F1 走查（dev tier scratch）⇒A09 票面 F1 判定呈报。

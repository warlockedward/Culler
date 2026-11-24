# A09 尾面 · Q-S skill 语义面＋Q-K Keychain darwin 名面＋Q-W worktree 路径 · 执行记录 001

日期 2026-11-25。工作树 `/private/tmp/culler-worktrees/a09tails`（物理迁移至 `/private/tmp/shorizon-worktrees/` 为 Q-W 尾步，见 §5）。

## 0. 授权
所有者 2026-11-25 二轮"全按推荐"：Q-S 执行（skill 逻辑名+目录+三件套同步+角色指令检查）／Q-K 直接终值（零部署⇒无迁移代码）／Q-W 一行工单＋物理迁移／F1 走查＝本机 dev tier scratch 域。

## 1. Q-S 改动面（skill 语义尾面＝R2 表"角色输入文本"行＋AGENTS.md 三件套义务）
**标识符面（culler→shorizon，12 点）**
- `mc/verbs/skills/collie/culler/`→`mc/verbs/skills/collie/shorizon/`（git mv；shipped-skill 不变量：folder name≡frontmatter name）
- SKILL.md frontmatter `name: shorizon`
- `roleskills.go:106` skillByPath 调用点＋`:194` 条件键＋`:203` installation.json skill 路径（**diff-plan 外新发现×1**：:106 调用点）
- `agent_skill_prod.go:17`＋注释
- `agent_skill_integration.go`×6（**diff-plan 外新发现的生产文件**：部署树 target×2＋用户机 symlink 名×2＋记录校验常量 Base=="shorizon"×2＋prose×3）
- `agent_skill.go:20/:27/:62`（**diff-plan 外新发现的源映射枢纽**：HostReleaseSourcePath 的 `skills/culler/` prefix＋仓库源目录段 `collie/culler`⇒`collie/shorizon`（git mv 后必随，否则安装断裂）＋installation.json host face skill 路径）
- `host_release.go` manifest×15（**diff-plan 外新发现的生产发布清单**：11 文件条目＋2 目录条目＋2 installation.json 特殊路径；AGENTS.md 铁律面：新目录名入清单，闭包测试失败面）
- `host_release_closure_test.go:201` 豁免前缀（**diff-plan 外**：不改则闭包测试把 skills/shorizon/ 当"未服务文件"红）
- `directives/collie.md:10` 加载路径
- `runner/collie-runner/main.ts:384`＋`:919` collie 必需 skill 名 `["culler"]`→`["shorizon"]`（**diff-plan 外**：产品语义名）
- 测试 fixture×8（agent_skill_prod_test.go×3／agent_skill_external_test.go×1／agent_skill_test.go×3／collie main.test.ts×1）

**prose 面（角色输入三件套，101 token 全词替换 capitalized Culler→S-Horizon）**
brief.go×8（含 spawnBriefHeader "S-Horizon immutable run brief"＋"## Standard S-Horizon directive"）＋directives 8 文件×52＋skills 16 文件×41（含 SKILL.md 自身＋references prose）。测试 pin×3 随改（brief_test.go×2 "Standard S-Horizon directive"＋customization_test.go×1）。

**保留面（逐面理由）**
| token | 理由 |
|---|---|
| brief.go:555 `destination_branch='culler'` | Git 分支面（轮三十三 Q5 裁定保留） |
| mc-setup/SKILL.md:137/:149 `/opt/culler-runtime/...`×2 | 容器内运行时路径常量＝D15 内部代号面 |
| project-environment-setup/SKILL.md:107 `FROM culler-project-base:` | 容器/refs 行 R2 表明文保留 |
| `references/culler-model.md` 文件名 | R2 表未列的 skill 内部参考文件名（内容 prose 已改；文件名＝具名残留登记） |
| agent-runner/skills.test.ts:203 `["culler"]`×4 | 通用 skill 安装器的任意名数据实例 |
| seedingprojection_test.go:49 注释 "Culler-owned state" | Go 注释（prose 批量面只覆盖 mc/verbs 角色输入面；此处为测试文件注释，登记不改＝零行为影响）※实为已改面外的登记项 |
| `X-Culler-Service`（agentsview_supervision.go:304）／`X-Culler-Release`（测试 header） | 内部组件 HTTP header＝wire 相邻面，非 R2 表行，不动 |

## 2. Q-K 改动面（Keychain darwin 名面直接终值）
**决定性架构事实（E1）**：`host_secret.go:37` 生成的 `"Culler-<sha256(home)>"` service 名在 Linux 侧**参与 HKDF key derivation（info 分量）＋GCM AAD**＝加密身份组件（ADR-0 SIGNED 设计值），在 darwin 侧＝钥匙串 app 可见 service 引用（R2 表终值面）。⇒ 实现＝**平台前缀常量分派**（推翻 diff-plan 原"拆函数"预案，常量分派更原子）：
- `host_secret.go:37`：`hostSecretNamePrefix + hex...`
- `host_secret_darwin.go`：`const hostSecretNamePrefix = "S-Horizon-"`（操作者钥匙串可见面＝R2 表 Keychain 行终值）
- `host_secret_linux.go`：`const hostSecretNamePrefix = "Culler-"`（加密身份面不变；hostKeyServicePattern `^Culler-[0-9a-f]{64}$` 同值保持）
- `host_secret_other.go`：同值常量（编译面完备）
- `runtimeauth_keychain_darwin.go:19`：`claudeIsolatedKeychainName = "shorizon.keychain-db"`（R2 表 Keychain 文件行终值）
- `runtimeauth_host_sources.go` removeCredentialFilesAt：**删除边界面认两代**（shorizon.keychain-db{-wal,-shm}×3＋legacy culler.keychain-db{-wal,-shm}×3）＝S3 防御面"认前代"设计语言同形（旧 release 创建的钥匙串文件在新 release 终止时仍被清掉）
- 测试：runtimeauth_host_sources_test.go fixture 8 文件（双代）＋`len(files) != 8`；host_secret_linux_test.go 的 `"Culler-..."` fixture＝Linux 面未变⇒不动（正确性证明）
- **Linux state tree 命名（`culler-host-credentials/`＋blob 目录）不动**＝Q-K 裁定明文（ADR-0 SIGNED 域）

## 3. Q-W 改动面（worktree 路径一行工单）
- `AGENTS.md:65`：`/private/tmp/culler-worktrees/<task>`→`/private/tmp/shorizon-worktrees/<task>`
- `evals/culler_evals/storage.py:17` WORKTREE_ROOT
- `evals/culler_evals/continuation.py:107/:169`（恢复源校验谓词＋resume checkout 目标）
- `evals/README.md:395` 活文档
- **保留**：`evals/verification/*.json`（历史执行记录路径＝时点事实，S2 先例）＋docs/BRAND-WHITELIST.md:24（R2 表现值列）
- **物理迁移**（尾步，提交后）：dev 工作树从 `/private/tmp/culler-worktrees/dev` 迁至 `/private/tmp/shorizon-worktrees/dev`（git worktree remove+add，内容全在提交链＝零风险；a09tails 自指面同样迁移）

## 4. 验证
- [x] evals pytest（test_continuation＋test_storage）20/20 passed（Q-W 路径面无回归）
- [x] `./resident/check.sh` 第 2 跑 **774 pass / 0 fail EXIT=0**（第 1 跑 751/23＝自抓缺陷③单根因：directives.go:39 硬编码 `skills/collie/culler/SKILL.md` 两层路径形态漏于枚举⇒包级 init() panic⇒全部 real-binary 测试族（split-brain/dispatch crossing/wiki helper/proxy renew）连带红；修复后基线恢复）
- [ ] `./mc/check.sh` 第 2 跑（进行中；第 1 跑 EXIT=1＝同一 panic 源，directives.go 修复后重跑）
- [x] runner agent-runner 第 2 跑 **160 pass / 0 fail EXIT=0**（第 1 跑 105/3＋3 errors＝`Cannot find module '@anthropic-ai/claude-agent-sdk'`＝新 worktree 缺 npm 依赖环境性，AGENTS.md 工具链程序安装后复绿）；collie-runner **55 pass / 0 fail EXIT=0**（main.ts 标识符面测试通过）
- [x] 角色指令检查（AGENTS.md 义务）＝mc lane 的 skill 装载测试族（roleskills/agent_skill/host_release closure/brief/customization）＋本记录保留面逐条核对
- [x] 残留 grep 终验：角色输入面 capitalized Culler=0；`collie/culler` 两层路径形态=0；`skills/culler` 生产面=0（除 legacy 删除边界 5 行）；`culler-worktrees` 活面=0（除时点记录 JSON）

## 5. 自抓缺陷（登记）
① **diff-plan 枚举缺口×4 生产文件**（agent_skill_integration.go／agent_skill.go／host_release.go manifest／host_release_closure_test.go 豁免前缀＋roleskills.go:106 调用点＋collie main.ts:919 必需名）：均非字面 `skills/culler` 普查能命中（是 `skills", "culler"` 拼接形态／目录名段／manifest 条目）⇒ 由**闭包测试面逻辑推演**（"部署树名改了⇒安装源映射必须随改"）发现。教训同 S4a 缺陷①：枚举面必须含"拼接形态"，普查 regex 要覆盖 `"<面名>", "culler"` 与 `Join(..., "culler")` 形态。
② **Q-K 原 diff-plan 方案（拆函数）被架构事实取代**：读 Linux 后端发现 service 名参与 HKDF/AAD⇒拆函数改 darwin 单侧＝Linux 侧生成不变但……实为"共享生成器平台分派常量"才是原子解。登记＝plan 的预案在执行期被更优实现取代（非错误，预案本身已声明"执行时 E1 核后定"）。
③ **两层 skill 路径形态漏于枚举**（directives.go:39 `roleSkillFS.ReadFile("skills/collie/culler/SKILL.md")`）：git mv 目录后该包级 init() 的 fs 读取 panic（编译期无错＝go:embed FS 路径在运行时解析）⇒ resident real-binary 测试族 23 fail＋mc lane cmd/mc 全族 FAIL 同源单根因。教训：**目录改名后的普查必须含"父目录段+改名段"的两层路径形态**（`collie/culler` 不匹配 `skills/culler` 模式）；包级 init() 的文件路径引用是运行时 panic 点，编译与 vet 均不可见。
④ **prose 批量替换面的生产逻辑标记漏改**（mc lane 第 2 跑抓出）：`directives.go:156` 的 contract 段提取标记 `strings.Cut(prose, "## Culler contract\n")`＝生产逻辑读取 directives 标题的解析标记，prose 批量面覆盖了 directives/*.md 的标题本身但没覆盖这个**消费侧标记**⇒`ProjectRoleShow` contract 段恒空⇒TestCollieShowsTheVerifierCustomizationContract 红。同族第二点：`customization.go:227` 定制冲突诊断消息 "...conflict with the Culler contract."（用户可见散文）＋其 4 个测试 pin（customization_test/wikiround_test/wikistate_test/wikiupdate_test）。教训：标题类 prose 是**生产逻辑的解析锚点**，批量改名标题必须同提交改消费侧锚点（=与 S4b scrub 防御点同族的"生产配对点"纪律）。

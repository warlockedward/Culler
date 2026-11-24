# A09-F1 · R2 验收残留分类表（"干净机端到端无 Culler 残留"差距分析）

日期 2026-11-25。shz/dev @ 2468f9a（S1/S2/S3/S4a/S4b 全部落地后）。验收判据来源＝ROADMAP §2R 末尾"**验收**：R2——干净服务器（amd64）与干净 Mac 各完成一次端到端安装→跑通 fake-harness 流水线→批准→合并，**全程无 'Culler' 用户可见残留**"＋BRAND-WHITELIST 验收钩"A09 收口时以'仅剩 A 类'为其验收判据之一"。

## 1. R2 表 B 行逐面终态（全部 E1）
| R2 表行 | 终值 | 终态 | 证据 |
|---|---|---|---|
| CLI 二进制＋安装脚本/PATH | `shz` | **已落地**（S4a 59d141d） | argv[0] 枢纽＋产物名＋记录名＋探针＋操作者文本；CI fast-suite-macos success |
| 部署 home | `~/.shorizon` | **已落地**（S2 6ae57e3） | 生产默认值＋候选表＋文档 |
| env 前缀 | `SHZ_*` | **已落地**（S1 65ee65a） | MC_HOME 名不变（D15） |
| launchd/systemd 单元 | darwin `com.s.shorizon.*`／linux `shorizon-*` | **已落地**（S3 2468f9a，O9=`s` 裁定） | UnitLabel 平台分派＋防御面两代＋uninstall 四身份 |
| token 前缀 | `shz_proxy_*`＋掩码 | **已落地**（S4b b93b953，方案 A 裁定） | provider-token.ts 权威＋scrub 防御同提交＋全道绿 |
| Keychain 文件/service | `shorizon.keychain-db`／`S-Horizon-<sha256(home)>` | **未落地（余面 K）** | `runtimeauth_keychain_darwin.go:19 claudeIsolatedKeychainName="culler.keychain-db"`＋`host_secret.go:37 "Culler-"+hex`（darwin keychain service 名＋Linux blob 目录名共享逻辑＝**darwin/linux 混合面**）；R2 表注"一次性迁移（旧读→新写→删旧，P0 凭据后端 ADR-0 一并设计，Linux 后端新起即无历史包袱）"⇒ Linux 侧 state tree 命名（`culler-host-credentials/`＋`Culler-<hash>/`，A02 证据 E1 在案）＝ADR-0 SIGNED 定义域⇒不动；darwin keychain 名面＝用户机钥匙串 app 可见⇒属"用户可见残留" |
| skill 目录 `~/.agents/skills/culler`→`shorizon` | 见 R2 表 CLI 行＋C 类行 | **未落地（余面 S）＝C 类 agent 语义面** | `roleskills.go:194 if name == "culler"`（skill 逻辑名）＋`:203 installation.json skill 路径`＋`agent_skill_prod.go`（用户机安装目的目录）＋`collie-runner/main.ts:384`＋`directives/collie.md:10`。逻辑名与目录名耦合＝AGENTS.md shipped-skill 不变量（folder name 与 frontmatter name 必须匹配）⇒**不可拆改**（只改目录不改逻辑名＝违不变量；只改逻辑名不改目录＝R2 表终值未达）。R2 表"角色输入文本"行明文：这是唯一**故意触及 agent 语义**的改名，**放 R2 末尾单独 commit＋跑角色指令检查（三件套同步义务）** |
| worktree 路径 `culler-worktrees`→`shorizon-worktrees` | R2 表 worktree 行 | **未落地（余面 W）＝收口批一行工单** | AGENTS.md:65（现行 host agent 路径约定）＋`evals/culler_evals/{storage,continuation}.py`＋**物理迁移**现存 worktree（dev 自身在 `/private/tmp/culler-worktrees/dev`＝自指面：迁移需先退出该目录） |
| 交付分支 `culler` | 保留（Q5 裁定＝功能语义） | 已裁定不动 | schema.sql:215 生成列＋触发器×2＋注释×2 |
| schema 文本面 | 2 token（S5 裁定） | **候 B02 v1 基线预冻结窗**（票面硬注顺序） | s5-scope-ruling-001.md |
| 发布通道/资产名/仓库 URL | 所有者操作域 | 不动（R2 表明文） | culler.go:14 installer URL 等 |
| 容器名 `mc-*`/refs/mc-land/mc-real | **保留**（R2 表明文＝内部代号零用户价值改动） | 已裁定不动 | — |
| Docker 镜像身份标签 `com.culler.environment.*` | Q5=(iii) 裁定保留（镜像内嵌数据非用户可见品牌） | 已裁定不动 | s3-diff-plan-002 §1 |

## 2. "用户可见残留"判定的逐面结论
干净 Mac 上操作者可见的 Culler 字样（S1–S4b 落地后）：
1. **skill 链接目录** `~/.agents/skills/culler/`（Finder/终端可见）＋skill 内容里的 "Culler token/Culler Project" 表述（agent 面）⇒余面 S。
2. **claude 隔离钥匙串名** `culler.keychain-db`（macOS 钥匙串访问 app 可见）＋keychain service 引用名 `Culler-<sha>`（host_secret.go:37）⇒余面 K。
3. **worktree 路径** `/private/tmp/culler-worktrees/`（终端可见，仅 host agent 工作面，操作者日常不可见）⇒余面 W。
4. `culler_evals` Python 包名＋`culler-worktrees` evals 存储（evals 工具面，操作者日常不可见）⇒A 类/具名残留。
5. LICENSE/README derived-from/CHANGELOG 0.1.0 上游历史/`mc` 内部代号⇒**A 类永久保留**（合规义务，非残留）。

⇒ **F1 严格验收（"全程无 Culler 用户可见残留"）要求余面 S＋K 落地**；余面 W 属操作者不可见（agent 工作约定）不阻塞 F1 但属 R2 表行（收口批）。

## 3. 执行顺序约束（票面明文，非执行侧裁量）
- 余面 S＝"**R2 末尾**单独 commit＋角色指令检查"⇒顺序上在 R2 其余行之后＝当前时点（S1–S4b 已落地，S5 候窗不阻塞）。三件套同步义务（AGENTS.md"When you change these things"）：角色指令 `mc/verbs/directives/<role>.md`＋run brief `mc/verbs/brief.go`＋角色 skills `mc/verbs/skills/<role>/`——skill 改名触及"角色接收什么"⇒全套同步＋跑角色指令检查。
- 余面 K＝R2 表注"一次性迁移（旧读→新写→删旧…ADR-0 一并设计）"⇒迁移机制面与 ADR-0 域纠缠；但**零部署**⇒旧读路径无对象⇒可直接终值（无迁移代码）或候所有者裁定。darwin keychain 名＋service 引用名两点的改动牵 `runtimeauth_keychain_darwin.go`／`host_secret.go`／`runtimeauth_host_sources.go`（claude keychain 源探测面）＋boundary/blocked.go glob（`*.keychain-db` 通配不受影响）＋相关测试。
- 余面 W＝一行工单（AGENTS.md 明文）：AGENTS.md:65＋evals 两处＋物理迁移（先 `git worktree move` 或 remove+add 到 `/private/tmp/shorizon-worktrees/`，dev 自指面需从目录外执行）。

## 4. 呈报（候所有者一言，执行侧不扩权）
R2 表验收钩的字面要求与票面排序约束均已在案，但**余面 S 的"R2 末尾单独 commit"与余面 K 的迁移机制注记＝两票面各自携带的执行义务，不在本轮"全按推荐"六裁范围内**⇒按 decision-unlock 纪律候点名：
- **Q-S**：执行 skill 语义尾面（逻辑名＋目录＋三件套同步＋角色指令检查，单独 commit）？推荐＝执行（R2 表终值在案＋F1 严格验收要求；三件套义务按 AGENTS.md 执行）。
- **Q-K**：Keychain darwin 名面（`shorizon.keychain-db`＋`S-Horizon-<sha>` service 引用）直接终值（零部署⇒无迁移代码）？推荐＝直接终值（R2 表 Keychain 行终值在案；Linux state tree 命名不在此面＝ADR-0 域不动）。
- **Q-W**：worktree 路径收口批一行工单（AGENTS.md:65＋evals×2＋物理迁移）？推荐＝执行（票面明文一行工单；物理迁移在 boot 风险下建议趁工作树内容全在提交链时做）。
- **F1 走查**：干净机端到端（dev tier scratch install→fake harness 流水线→批准→合并）＝F1 验收的实证步，需所有者在"本机跑一次 dev 部署"上点头（AGENTS.md：dev tier 许可、不指 ~/.shorizon、无 launchd）或指定 218（218＝source production 面，重）。

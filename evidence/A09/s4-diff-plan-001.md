# A09-S4 · CLI 命令名面 `culler` → `shz` — diff-plan #1

## 0. 授权与终值依据
- P4（轮二十五）分步授权 S1→S5 每步全绿再并；Q5（轮三十三"全按推进"）两默认追认清除票面歧义。
- R2 表（ITERATION-ROADMAP §2R）B 行第 1 条："CLI 二进制 `culler` → **`shz`**（D20）｜安装脚本、PATH、`culler_services`、skill 目录…"＋BRAND-WHITELIST B 行"CLI 命令与文档命令例…client name `culler` → `shz`"。

## 1. 机制事实（全部 E1 实读，2026-11-24）
- **argv[0] 双表面**：`mc/cmd/mc/main.go:39` `if filepath.Base(os.Args[0]) == "culler" { runCuller(...) }`——同一二进制按文件名切换操作者面（culler 生命周期动词）/内部面（mc spine 动词）。**改名枢纽点**：漏改此处=操作者 CLI 静默退化为内部面。
- **安装产物**：`mc/cmd/mc/setup_install_prod.go:92` `copyPublishedFile(source, filepath.Join(directory, "culler"), 0755)`=发布包 `bin/mc` 被**拷贝为** `<bin-dir>/culler`（0755）。包内文件名单元为 `bin/mc`（culler-package packageFiles :319，D15 内部代号不改）。
- **安装记录**：`mc/deployment/cli_install.go:20` `CLIInstallRecord = "state/culler-command.json"`；:76/:89 回读校验 `filepath.Base(r.Path) != "culler"` 即拒；:186 记录 `Path=<dir>/culler`；:279 shell 探针 `command -v culler`；:122/:124 生成操作者命令串 `"culler " + command`。
- **用法/帮助/错误文案**：`mc/cmd/mc/culler.go` :12（lifecycle guidance）、:29-33（命令表描述"Start Culler services."等品牌散文）、:151/:154/:157/:161（option 归属错误文案）、:182/:194/:244（usage literal `culler <command>`）；`culler_uninstall_prod.go:22/:64`、`uninstall_prod.go:171`（"culler uninstall" 文案）；`resident/src/install-maintenance.ts:108`（"Use culler update to recover."）；`scripts/source-update-guard.sh:9`＋其测试 :45（成对文案）；`install.sh:159`（"Run culler update."）。

## 2. 改动面（S4a=本次执行；全部 culler→shz 为**命令名**语义，非品牌散文）
| 文件 | token | 改动 |
|---|---|---|
| mc/cmd/mc/main.go:39 | argv[0] 检查 | `"culler"`→`"shz"` |
| mc/cmd/mc/culler.go | :12/:151/:154/:157/:161/:182/:194/:244 | 命令名文案→shz；:29-33/:13 品牌散文 "Culler services"/"Configure Culler"→"S-Horizon"（用户可见 help 面，R1 D 规则同面随改） |
| mc/deployment/cli_install.go | :20/:47/:69/:70/:76/:77/:89/:90/:122/:124/:129/:186/:225/:279/:283/:295/:297 | 记录文件名 `culler-command.json`→`shz-command.json`；校验常量→shz；命令前缀串→shz；PATH 探针→`command -v shz`；shell 块注释 "# Culler command"→"# shz command"；操作者文案→shz |
| mc/cmd/mc/setup_install_prod.go:92 | 安装目标文件名 | `"culler"`→`"shz"` |
| mc/cmd/mc/culler_uninstall_prod.go:22/:64 | 文案 | →`shz uninstall` |
| mc/cmd/mc/uninstall_prod.go:171 | 文案 | →`shz uninstall` |
| resident/src/install-maintenance.ts:108 | 文案 | →`shz update` |
| scripts/source-update-guard.sh:9 ＋ scripts/source-update-guard_test.sh:45 | 成对文案 | →`shz update`（生产与门同改） |
| install.sh:159 | 操作者文案 | →`shz update` |
| **活文档命令例**（描述当前真实行为，随行为同改=R1 D 规则） | INSTALL.md（`culler update/status/version/start/stop/restart/uninstall/logs` 例≈20 行）、SECURITY.md:9、AGENTS.md:124/150、release/README.md:51、release/UPDATES.md:43/60/61/169/172/173 | `culler <cmd>`→`shz <cmd>` |
| **断言上述串的测试**（同一提交内随改，否则 lane 必红） | mc/deployment/cli_install_test.go（9 token）、mc/cmd/mc/culler_prod_test.go、cli_test.go、helper_manager_test.go、agent_skill_host_prod_test.go、setup_published_preview_prod_test.go、private_release_test.go、production_image_testfake*.go、uninstall_modes_prod_test.go、onboard_home_prod_test.go、resident install-maintenance 相关测试、source-update-guard_test.sh | **逐 token 判面**：CLI 命令名面→shz；Git 分支面（destination_branch/"culler branch"）→**不动**；skill 链面（skills/culler）→**不动** |

## 3. 明确不做（分面裁定，逐面登记理由）
| 面 | 判定 | 理由 |
|---|---|---|
| Git 交付分支名 `culler`（≈500 token：schema.sql 生成列、resident repository/effects、GIT-DELIVERY.md、smoke 验证 `culler:SMOKE_RESULT.md`、dashboard destination 显示） | **保留** | 所有者轮三十三 Q5 裁定（P2 原推荐）：功能语义非品牌；改它牵 spine 不兼容（S5 面） |
| skill 映射链（`skills/culler/*` 清单名、`~/.agents/skills/culler`、runSkillFrom({name:"culler"})、frontmatter name） | **保留** | BRAND-WHITELIST C 类=agent 语义面，R2 末尾单独 commit＋角色指令三件套同步义务；终值=shorizon（R2 表） |
| 发布资产名 `culler-$platform.tar.gz`＋ghcr 镜像名＋`SHZ_REPOSITORY` 默认 `vinchenkov/Culler`＋culler.go:14 installer URL | **保留** | R2 表发布通道行="资产名与源指向按 A04/O8；GitHub 仓库更名是所有者操作"＝所有者操作域 |
| `model_provider="culler"`（runner/agent-runner/adapters/provider-access.ts:71＋runtime-source 副本） | **具名残留** | opencode provider 配置标识符=wire 相邻面（AGENTS.md：wire shape 变更=设计决定），需单独裁定 |
| trace 目录名（trace-lane.ts:132 `projects/culler`、agentsview copies/culler、trace-copy.test.ts） | **具名残留** | 派生自工作区命名的目录面，非 R2 表列名 |
| Discord presence（discord.ts:717 browser/device="culler"）＋测试 fixture 频道名 "culler" | **具名残留** | R2 表未列；品牌面非 CLI 面 |
| docs/hero-{dark,light}.svg:112 代码块里的 `culler` | **具名残留** | R1 资产替换="需设计输入"（BRAND-WHITELIST B 行明示挂 A09 或后续资产单） |
| Go 标识符（runCuller/cullerOptions/cullerCommands/cullerLifecycleGuidance…、cmd/culler-package 目录） | **保留** | 代码标识符非用户可见；D15 最小改动原则；目录名改动牵 host_release/closure 面=收口批 |
| `culler_evals` Python 包名、`culler-worktrees` 路径约定、`/srv/culler` 测试 fixture home、install_test.sh release 记录 fixture（`ghcr.io/owner/culler` 镜像名） | **保留/具名** | R2 表未列前者两个（worktrees=收口批一行工单）；后两者=fixture 外部身份建模 |
| `culler_proxy_*` token 前缀（A09 票面"shz_proxy_*+掩码正则"） | **S4b 候裁** | R2 表="（R2，与 P2b 交集）"——P2b=预算治理面交集，需先核掩码正则（trace-mask）的实际 token 面再动，不并 S4a |

## 4. 行为语义变化（如实登记）
- 操作者命令 `culler update`→**`shz update`**（D20 终态；本机无已安装 CLI，无存量迁移面）。
- 状态记录文件 `state/culler-command.json`→`state/shz-command.json`（D20 不建兼容层；本机无部署=零影响）。
- `state/culler-command.json` 的**文件名**是宿主状态布局的一部分：uninstall/doctor 若引用旧名需同改（本提交内全改，不产生半状态）。
- argv[0] 检查改为 `== "shz"`：**不再接受文件名 `culler` 的操作者面**（旧名文件=内部面 mc 行为）。D20 一次性语义，与 home 面同族。

## 5. 验证
1. `go build ./...`＋vet 五 tag 组（含 test_fake_routing=fixture 域编译不破）。
2. 断言面测试随改后：`mc/check.sh` 全链（含 shell 门 source-update-guard_test.sh 成对文案）＋resident（install-maintenance 文案测）＋dashboard（984 基线）。
3. 范围内 CLI 面残留 grep=0（`command -v culler`/"culler <command>"/`state/culler-command.json`/`copyPublishedFile(..., "culler")` 形态全查）。
4. 排除面计数不变（Git 分支面≈500、skill 链 31、com.culler 143）——前后差证明只动了该动的。

## 6. 守界
不推远端；不触发布资产名/仓库 URL/ghcr 名（所有者操作域）；不改 schema（S5/B02 窗）；不动 com.culler.*（S3/O9）；A09 status 保持 IN_PROGRESS（S4a≠票完成，S3/S5/S4b 未收口）。

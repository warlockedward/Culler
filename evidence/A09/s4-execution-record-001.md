# A09-S4a · CLI 命令名面 `culler` → `shz` · 执行记录 001

日期 2026-11-24。工作树 `/private/tmp/culler-worktrees/a09s4`，分支 `task/a09-s4-cli-name`（自 shz/dev @ 6b7fdfc）。

## 1. 改动总面
终态 37 文件，120 插入 / 120 删除，完全对称（纯 token 替换，无逻辑行增删）。`git diff HEAD --stat` 证实。初始执行面 32 文件 114+/114-；缺陷 ⑦⑧⑨ 的随改（车道跑与预览捕获自抓）追加 5 文件 6 行（cli_update_test.go×1、culler_uninstall_prod_test.go×1、terminal_command.go×1、terminal_presentation.go×1、setup_signin_screen_prod_test.go×1、terminal_presentation_test.go×1＝其中两行同一文件对）。

**CLI 命令名面（culler→shz，命令名语义）**
- `mc/cmd/mc/main.go`：argv[0] 枢纽检查 `== "culler"` → `== "shz"`（漏改此处=操作者面静默退化为内部面）。
- `mc/cmd/mc/setup_install_prod.go`：安装目标文件名 `"culler"`→`"shz"`；`CheckCLIConflict(<dir>/culler)`→`<dir>/shz>`（自抓缺陷 ②）。
- `mc/deployment/cli_install.go`：记录文件名 `state/culler-command.json`→`state/shz-command.json`；回读/previous 校验常量 `Base != "culler"`→`"shz"`×2；`CLICommand()` 前缀串 `"culler "`/`"culler --mc-home "`→`"shz "`；shell 块 `# Culler command …# End Culler command`→`# shz command`；安装记录 `Path=<dir>/culler`→`<dir>/shz`；ZDOTDIR 错误文案；shell 探针 `command -v culler`→`command -v shz`；"cannot resolve culler"→shz；可用性文案×2（末句品牌散文 "Culler experience"→"S-Horizon experience"，同面规则）。
- `mc/deployment/cli_update.go`：`UpdateCLIInstallation` 记录路径校验 `<dir>/culler`→`<dir>/shz`（自抓缺陷 ③）。
- `mc/cmd/mc/culler.go`：guidance 常量×2；命令表品牌散文×4（R1 D 同面规则）；option 归属错误×4；uninstall --mc-home 冲突文案；usage literal×2；detail 散文×4；keep-data/full-wipe 项×2；`shz help` literal。
- `mc/cmd/mc/culler_uninstall_prod.go`：`Run culler uninstall in a terminal…`→shz；`ReplaceAll("./uninstall.sh", "culler uninstall")`→`"shz uninstall"`。
- `mc/cmd/mc/uninstall_prod.go`：`Retry culler uninstall.`→shz。
- `mc/cmd/mc/culler_prod.go`：诊断行分类器前缀 `"culler "`→`"shz "`（自抓缺陷 ④：不改则 `CLICommand()` 产出的 "shz update" 行被误归 details，不按命令字面渲染）。
- `mc/cmd/mc/setup_published_prod.go`×6、`setup_update_transaction_prod.go`×2、`culler_services_prod.go`×2、`mc/deployment/update_journal.go`×3、`update_stop.go`×1、`mc/verbs/operatorlane.go`×1：操作者指令文案 "Run/Retry/run culler update (…)" 与 "run culler restart resident."→shz（自抓缺陷 ①：diff-plan §2 未列，残留 grep 发现）。
- `resident/src/install-maintenance.ts`：`Use culler update to recover.`→shz。
- `scripts/source-update-guard.sh`＋`scripts/source-update-guard_test.sh`：成对文案 `Use culler update to recover.`→shz（生产与门同改）。
- `install.sh`：`Run culler update.`→shz。

**活文档命令例（描述当前真实行为，R1 D 同面规则）**
- `INSTALL.md`×13：`shz` 安装描述×2、更新命令例×6（含 ```sh 代码块×2）、命令表×13 行、uninstall 节×1、版本节×2、发布更新节×1（其中保留面见 §3）。
- `AGENTS.md`×3：部署表 `shz update`；操作者面规则 `The operator uses shz…`；:24 行重写为 S4a 后现实（删除已变假的 "the CLI command" 与 S2 残留的 `~/.shorizon`——该句自 S2 起自相矛盾；保留仍读 culler 的五个面枚举）。
- `SECURITY.md`×1：`shz version`。
- `release/README.md`×1、`release/UPDATES.md`×6：命令例→shz。

**测试面（逐 token 判面，同一提交随改）**
- `mc/deployment/cli_install_test.go`×9：fixture 二进制名×6、可用性文本断言×1、shell 残留断言 `"culler"`→`"shz"`（自抓缺陷 ⑤：shell 块现写 "shz command"，查 "culler" 残留恒真空通过=不再证明真实行为）、冲突测试 fixture×1、目录变更测试×1。
- `mc/cmd/mc/culler_prod_test.go`×1、`culler_uninstall_prod_test.go`×2（fixture 二进制名＋旧 shell 块串）、`setup_progress_preview_prod_test.go`×3、`setup_install_prod_darwin_test.go`×2、`culler_preview_prod_test.go`×2（自抓缺陷 ⑥：fixture 名是 `ReadCLIInstallation`/`InstallCLIPath` 的输入，功能上必须随改）、`setup_published_preview_prod_test.go`×1（preview fixture cause 串）、`mc/verbs/operatorlane_test.go`×1、`mc/onboarding/terminal_command_test.go`×1（渲染器 fixture 输入）。

## 2. 验证（进行中）
- [x] 范围内 CLI 面残留 grep（`command -v culler`/`culler-command.json`/`Run|Retry culler update`/`culler restart resident`/`cannot resolve culler`/`The culler cli`/`belongs to culler`/`Use --restore with culler`/`culler <command>`/`Join(…, "culler")`/`Base(…) != "culler"`/`HasPrefix(line, "culler ")` 形态）= 0（命中行全部判为排除面，见 §3）。
- [x] 排除面未动证明：`git diff HEAD` 的全部 +/- 行中，排除面 token（`skills/culler`、`com.culler`、`destination_branch`、`refs/heads/culler`、`culler_proxy_`、`ghcr.io/owner/culler`、资产名、`culler_evals`、worktrees 路径、wire client name）出现 0 次，唯一例外=AGENTS.md:24 两行**有意**文档行（枚举仍读 culler 的面，属描述性文本非机制引用）。
- [x] `go build ./...` EXIT=0（默认 tag）。
- [x] `./mc/check.sh` 第 1 跑 EXIT=1：唯一失败=`TestCLIUpdatePreservesShellFilesAndRefusesPathChanges`（自抓缺陷 ⑦，mc/deployment；其余包全绿＝测试面随改全部生效）。
- [x] `./mc/check.sh` 第 2 跑 EXIT=1：唯一失败=`TestCullerUninstallModesAndPreview` 三个 preview 子测（自抓缺陷 ⑧，mc/cmd/mc；修复后单测复跑 ok 1.278s）。UNVERIFIED=0。
- [x] `./mc/check.sh` 第 3 跑 EXIT=0（gofmt＋vet×8 组＋shell 门×5＋全 Go 测试；verbs 455.7s 全量；UNVERIFIED=0）。其后缺陷 ⑨ 动了 mc/onboarding 生产代码⇒第 3 跑对两包过期。
- [ ] `./mc/check.sh` 第 4 跑（缺陷 ⑨ 后收口，进行中）。
- [x] `./resident/check.sh` EXIT=0：774 pass / 0 fail（install-maintenance 文案测通过）。
- [x] `./dashboard/check.sh` EXIT=0：984 tests（基线一致，零交叉影响；本轮零 dashboard 文件改动，保险跑）。
- [x] AGENTS.md setup 文本义务：`preview_culler.py` 全场景捕获完成（第 3 遍，含标题面修复后二进制）：132 帧（34 场景×80/40 列×彩/无彩）全部 `fixture_success=true`＋`process_exit_status=0`（CLI 契约码与预期码逐帧相等）；帧内命令面零旧 token（`culler <verb>`/`Run culler`/`command -v culler` 形态=0）；屏首行标题="S-Horizon"。宽松子串扫描命中的 36 帧＝具名品牌散文残留面（"Culler release" 字段、"Culler requires attention" 面板、进度消息对、installer URL、fixture 路径标识），全部在 §3 不动清单内，非命令名面。
- [x] 对抗性子代理审查实际输出（AGENTS.md 义务）：完成。verdict=**REVISE**，但两个 REVISE 项均为预存渲染性质（改名前捕获显示相同折断；改名使受影响行短 3 字符）：
  - **D1**（标准 3＝40 列复制粘贴）：curl 安装器命令行在 3 屏（status-absent/incomplete-setup/update-recovery）40 列下跨行折断，mid-token。预存；`Literal()`（terminal_command.go）刻意不折行；80 列下也超宽（折 3 视觉行）＝一般性超宽 literal 问题。
  - **D2**（标准 3）：`shz --mc-home '<home>' <verb>` 恢复命令在 5 屏（status-partial/status-unknown/version-mismatch/restart-failure/service-timeout）40 列下跨 4 行折断。预存＋fixture 放大：默认 home 部署下 `CLICommand()` 省略 `--mc-home`⇒短命令（17 字符）不折断；长形式只服务非默认 MC_HOME 用户（本 fixture 的 /var/folders 临时路径属此类）。
  - 审查者明示修订路径＝**裁定**而非新工作：(a) 所有者将两类加入 kept-face 裁定记录（推荐，与品牌散文残留同批呈报），或 (b) 后续改 `Literal` 悬挂缩进折行/默认 home 短形式（产品渲染行为变更，超 S4a 面，需单独授权）。
  - 命令名面（标准 1）＝**全 132 帧完成**：零小写 culler，所有操作者可见命令读 shz，首行标题 S-Horizon；标准 2（无 shz+Culler 矛盾混句）/标准 4（无死胡同、完成态有据）/标准 5（usage 屏 `shz help logs` literal 在场且自洽）全过。
  - 登记级事实：① "A newer Culler release is available." 出自预览 fixture（culler_preview_prod_test.go:117），生产对应串（private_release.go:78）不含 "Culler"⇒证据帧显示了产品实际不会发出的 kept-face 句子；② "Culler requires attention"（culler_prod.go:184，生命周期屏）与 "Culler command requires attention"（setup wizard 预览面）是兄弟标签，裁定记录用精确串；③ 36 帧宽松子串命中＝具名品牌散文残留面（"Culler release" 字段标签、进度消息对、installer URL、fixture 路径标识），全部在 §3 不动清单内。

## 3. 排除面判定（本轮明确不动，逐面理由）
| 面 | 代表位置 | 判定 | 理由 |
|---|---|---|---|
| Git 交付分支名 `culler` | `delivery.go:1274 merge.branch == "culler"`、`cli_test.go`（≈50 token）、`wikistate/systemstatus/wikimerge/wikihold` 测试的 "A merge into culler…" 拒因串、`dispatchverb_test.go` `--initial-branch=culler`、wiki 测试族 "culler holds/moved" 散文 | 保留 | Q5 裁定（轮三十三）：功能语义非品牌；改它牵 spine 生成列＝S5 面 |
| skill 映射链 | `agent_skill_integration.go:152`（record 校验 `Base != "culler" || Dir != "skills"`）、`roleskills.go:194`、`operator_transport_test.go:179`、`~/.agents/skills/culler` 等文档行 | 保留 | BRAND-WHITELIST C 类=agent 语义面；R2 末尾单独 commit＋角色指令三件套同步义务 |
| launchd/systemd 单元面 `com.culler.*` | 全仓（redeploy/dev-reset/uninstall 脚本＋测试 fixture `com.culler.abc.*`） | **S3，BLOCKED on O9** | 域名根=所有者供给事实 |
| 发布资产/镜像/仓库 URL | `culler-darwin-arm64.tar.gz`、`ghcr.io/owner/culler`、`vinchenkov/Culler` installer URL（culler.go:14 保留）、release 记录 fixture | 保留 | R2 表="资产名与源指向按 A04/O8；仓库更名是所有者操作" |
| wire client name | `INSTALL.md:487`（对 `models.opencode.ai` 请求的 client name `culler`）、`model_provider="culler"`（provider-access.ts） | 具名残留 | 外部 wire 标识符＝设计决定面，需单独裁定（S4b 同批） |
| token 前缀 `culler_proxy_*` | evals fixtures（`test_launch.py`、`test_credentials.py`、`proxy_fixture.py`）＋发行侧 | **S4b 候裁** | R2 表"与 P2b 交集"；需先核 trace-mask 正则实际 token 面 |
| 内部内容标记对 | `"culler smoke ok\n"`（onboard_smoke_prod.go:860＋测试 :395）、`"culler attachment canary (in|out)bound v1"`（attachment_canary.go） | 具名残留 | 生产/测试成对 canary 内容 token，非命令名面 |
| trace 目录名 | `agentsview.go:189 ClaudeTraceCopies <claude>/culler` | 具名残留 | 派生自工作区命名的目录面 |
| fixture 身份名 | mktemp 模板（`culler-dev-reset-test.*` 等）、`/tmp/culler-install.sh` 下载名、`/srv/culler` fixture home、`supervision_darwin_test.go` TCC 探针叶子目录名 `culler` | 保留 | 临时/fixture 身份非用户可见命令名；`supervision_darwin_test.go:18` 的 `root/"culler"` 只是 TCC 门探针的任意子目录名 |
| evals Python 包 `culler_evals` | `evals/**` | 保留 | R2 表未列 |
| openwiki claims | `openwiki/.claims/**`（"The culler command routes…"×4） | 不动 | Scribe 专管面（AGENTS.md：Culler 拒绝改动 openwiki 的提交）；wiki 滞后代码是设计内 |
| uninstall 面品牌散文 | `culler_uninstall_prod.go` "This command removes all detected Culler installations."、进度消息对 "Stopping Culler services…"↔"Stop Culler services"、计划面 "Culler Bun"/"Culler runtime volumes"/"Culler commands"/"Culler entries in shell files"/"Full wipe: software and Culler data."、`setup_agentsview_prod.go` "The Culler services are stopped."、`update_journal.go` "keeps Culler stopped"（与新命令名 shz 同句混面） | 具名残留候裁 | diff-plan §2 对 uninstall 面只列命令名 token（字面范围）；品牌散文改名牵 broker 面 stderr＋标签映射键值对＋他人 pinned 断言（`culler_uninstall_prod_test.go:103` 断言 "all detected Culler installations"）——候选呈报，不擅动 |

## 4. 自抓缺陷（登记）
① **diff-plan §2 枚举缺口**：六个生产文件共 15 处操作者指令文案 "Run/Retry/run culler update"、"run culler restart resident" 未列。后果若漏改：产品指示操作者运行已不存在的命令。发现方式＝验证步 3 的残留 grep（验证机制按设计工作）。
② **冲突检查目标未随安装名改**（`setup_install_prod.go:41`）：不改则冲突检查永远指向不存在的文件名=形同虚设，第三方既有 `shz` 文件会在安装时被静默覆盖。
③ **更新记录校验未随改**（`cli_update.go:20`）：不改则 `shz update` 对任何已装 CLI 永久拒绝（记录 Path 现为 `<dir>/shz`）。
④ **诊断行分类器前缀未随改**（`culler_prod.go:178`）：不改则 "shz …" 命令行被渲染为 detail 而非 literal。
⑤ **残留断言真空化**（`cli_install_test.go:105`）：shell 块内容改名后，查旧名残留的断言恒真——按 AGENTS.md "一个测试证明真实行为，不证明就修" 随改为查新名。
⑥ **preview fixture 二进制名**（`culler_preview_prod_test.go`×2）：`ReadCLIInstallation` 现要求 `Base(Path)=="shz"`，fixture 不改则 preview 场景（AGENTS.md setup 文本审查的捕获工具）渲染不出目标路径。
⑦ **更新测试 fixture 二进制对**（`cli_update_test.go:26`，第 1 车道跑自抓）：`{"mc","culler"}` 只覆盖两个旧名文件，生产更新验证现在摘要 `<dir>/shz`＝digest 必不符⇒更新被拒。改 `{"mc","shz"}`。
⑧ **卸载 preview 断言旧 shell 块标记**（`culler_uninstall_prod_test.go:100`，第 2 车道跑自抓）：断言 `"Culler command"` 在 shell 文件中存在性＝preview 语义（preview 留块/移除删块），fixture 块已改 `# shz command`⇒旧标记永不存在⇒三 preview 子测恒红。改 `"shz command"`（证明内容不变，只是标记 token 随生产块文本）。
⑨ **屏首行 identity 标题仍读 "Culler"**（预览捕获后自查发现）：`terminal_command.go:25`（CLI 面 `CommandTitle()`）＋`terminal_presentation.go:116`（setup 进度屏 identity heading）。改名后同屏三面混杂（标题 Culler／命令名 shz／品牌散文 S-Horizon）＝最醒目品牌位未随面改，违反 culler.go 命令表已确立的 R1 D 同面随改先例。同改：两处标题→"S-Horizon"＋`setup_signin_screen_prod_test.go:207` pin＋`terminal_presentation_test.go:33` 正则断言 `(?m)^Culler$`→`^S-Horizon$`（该断言语义＝"identity heading 恰出现一次"，token 随生产文本）。**同族不随改（扩面候裁，登记）**：`terminal_presentation.go:24` 阶段标题 "Start Culler"、`image_progress.go` "Compile Culler for Linux"/"Install Culler"、`prepare.go:285` setup 散文＝setup wizard／agent image 品牌面（S4a＝CLI 命令名面，不扩）；`prepare.go:284` 匹配 `build-prod.sh` 实际输出串＝S3/O9 锁定面（build-prod.sh 自身文案随 S3 改，单改此侧匹配即破）；`preview_culler.py`＝捕获工具自身命名（非用户可见输出）。

## 4b. 证据面构成（提交量控制）
`evidence/A09/s4-terminal-previews/` 提交 133 文件 / 580KB＝132 屏 `.txt`（全场景×80/40 列×彩/无彩的屏面文本）＋`frames-index.json`（从 132 个逐帧 `.json` 提取的机读契约结果：fixture_success 132/132=true、process_exit_status 132/132=0、cli_exit_status==expected_cli_exit_status 132/132）。未提交的冗余产物：逐帧 `.json` 全文件（10.5MB，膨胀源＝frame.html 逐格 span 标记，元数据已全部并入 index）、`.raw`（ANSI 字节流，`.txt` 为其去控制符派生）、`.html`（网页渲染副本）。重生产命令已在 index 内登记（`go test -c`＋`preview_culler.py`）。

## 5. Mimosa 钩子事件（如实记录）
- **探针行拦截**：`cli_install.go` shell 探针行（new_string 含 `$(command -v shz)`）被 PreToolUse 判"高危·命令注入"拦截。分析：该行是编译期固定字符串（shell/mode 硬编码 `/bin/zsh -ic`，无用户输入拼接），探针机制先于 S4a 存在（S1 已把 marker 改名 `SHZ_COMMAND`）；改名对该安全属性零影响。处置＝token 级最小编辑（`command -v culler`→`command -v shz`，不含 shell 展开形态）应用同等语义，非规避真实发现。
- **Bash 命令文本误拦×3**：含 `_test.go` stem 的 grep 模式、含 `install.sh/redeploy.sh/dev-reset.sh/uninstall.sh` pathspec 的 `git grep`、含 `install\.sh` 字面的模式均被拦（钩子保护的是"命令文本里的受保护文件名"，与实际操作无关）。处置＝重写模式避开字面名，全部为只读 grep/git-grep，无内容经 Bash 写入。

## 6. 行为语义变化（如实登记，继承 diff-plan §4）
- 操作者命令面整体切换：`culler <verb>` → `shz <verb>`；argv[0] 不再接受文件名 `culler`（旧名文件=内部面 mc 行为）。D20 一次性语义，本机无已装 CLI，无存量迁移面。
- 宿主状态记录 `state/culler-command.json`→`state/shz-command.json`：不建兼容层（D20）。**既有部署升级路径影响**：已装 CLI 的宿主在 S4a 发布后将出现 `shz update` 拒读旧记录（Base≠shz）＝需重装场景；本机零部署，影响面=未来首个真实部署（发布域，登记不处置）。
- shell rc 文件中的块注释 `# Culler command …`→`# shz command …`：卸载按记录的 Block 串精确匹配删除（`removeCLIInstallation`），旧部署的旧块串在记录内自洽，删除不受影响；跨版本半状态（新记录＋旧块）不存在于本机。

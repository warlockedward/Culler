# A09-F1 · 干净机端到端走查记录 001（dev tier scratch 域）

日期 2026-11-25。被测树＝shz/dev @ e9d6912（缺陷⑥⑦⑧落地后最终态；lane #3 FINAL3_MC_EXIT=0 权威门绿）。域＝所有者二轮"全按推荐"批准的"本机跑一次 dev 部署"＝本机 dev tier scratch（AGENTS.md：fake harness＋scratch MC_HOME＋无 launchd，不指 ~/.shorizon）。

## 0. 判据拆解
R2 验收句（ROADMAP §2R）："干净 Mac 完成一次端到端安装→跑通 fake-harness 流水线→批准→合并，全程无 'Culler' 用户可见残留"。
- **安装体验面**（步骤 1）：dev 安装全程操作者可见输出＋落盘用户可见文件。
- **管线机制面**（步骤 2）：tick→dispatch→lease→fake-harness Worker→complete→packet→approve→apply(merge)＝仓库标准证明 `TestWalkingSkeleton`（docker_e2e lane，AGENTS.md 在案 opt-in lane）。
- **清理面**（步骤 3）：scratch 域完全移除，零部署残留。

## 1. 步骤 1＝安装体验面（EXIT=0，双跑）
命令（非交互 --yes 全显式选项）：
```
sh install.sh --dev --mc-home /private/tmp/f1-scratch/home --project f1walk \
  --workspace-root /private/tmp/f1-scratch/workspace --accept-generated-sandbox \
  --project-vision later --runtime-bindings fake --accept-default-routing \
  --accept-default-tunables --accept-default-git-identity \
  --accept-default-dashboard-bind --discord-mode disabled
```
首跑（571ffa1 时代）发现缺陷⑧＝routing.md 标题面（`# Culler routing`，routing.go:110 Markdown() 渲染；Parse 契约明文"# heading/comment lines are allowed"⇒标题非解析锚点⇒纯 prose 面）⇒修复 e9d6912⇒清空 scratch 终树重跑（step1b）。

**step1b 审计（e9d6912 终树）**：
| 面 | 结果 |
|---|---|
| installer stdout 23 行操作者可见文本 | **Culler 零命中**（全 S-Horizon：sign-in 说明／git 陈述×2／VISION.md 说明／sandbox profile／__onboard JSON detail 全段） |
| stderr | 空 |
| 退出码 | 0 |
| home/routing.md 标题行 | `# S-Horizon routing`（缺陷⑧生效） |
| home/config.toml 头行 | `# S-Horizon operator configuration.`（缺陷⑦ Marshal 头生效） |
| home/config.toml author_name | `S-Horizon`（缺陷⑥值面生效） |
| home .md/.toml/.json 文本面全域 grep | Culler 零命中 |
| home/release/{host,runner} 文件拷贝 | 程序源码（.ts/.js）＝非用户可见文本面；命中抽样＝已登记候点名面（collie main.ts:508/:920，AM v0.2.9 入册）＋prod-tier Discord 测试消息面（走查 discord-mode=disabled⇒未执行）＋内部标识符 |
| workspace 初始提交署名 | `Culler <culler@localhost>`＝**V1 已知项**（onboardprojectgit.go:339 workspace bootstrap git 身份；值面未列 R2 表⇒候点名；与部署身份 S-Horizon<mc@localhost> 两条独立线） |
| spine.db | schema 文本面＝S5 域（候 B02 v1 基线预冻结窗，票面硬注顺序）⇒非步骤 1 验收项 |
| supervision | section status=deferred⇒零 launchd（dev tier 规则符合） |

**步骤 1 结论＝GREEN（限定：V1 已知项候点名＋S5 域候窗）**。

## 2. 步骤 2＝管线机制面（GREEN）
`mise exec -- go test -tags docker_e2e -v -timeout 30m ./e2e/... -run '^TestWalkingSkeleton$'` @ dev e9d6912：
- **PASS: TestWalkingSkeleton (304.04s)**；F1_STEP2_EXIT=0；ok mc/e2e 304.597s
- 内容＝真实 host mc（test_fake_routing）＋真实 resident＋真实容器：tick→dispatch→lease→fake-harness Worker→`mc complete`→capture→packet→**approve**（纯状态写，culler 分支未动）→**apply（merge 入 culler）**＋Scribe wiki update 半边（wiki commit merged_commit=main 前基线）。零 provider token（fake harness/binding 全路由）。
- 该测试在 e9d6912 树上执行⇒缺陷⑦改动的 `operatorconfig.Ensure`（config.toml 生成）＋vision 面在机制链路上无回归。

## 3. 步骤 3＝清理面（GREEN）
- `rm -rf /private/tmp/f1-scratch`：scratch home（spine.db／config.toml／bin/mc／release 拷贝）＋workspace（含 V1 提交的 git repo）全移除。该域＝走查自身产物，零用户数据。
- Docker 残留核对：`mc-e2e-*` 容器 0／卷 0（e2e fixture 自清理彻底）；镜像 `mc-fake-e2e:latest`＋`mc-fake-e2e-project:latest` 保留＝lane 自身资产（`mc`＝A 类内部代号面，R2 表保留；复用免重建）。
- 任务 worktree a09f6/a09f7（缺陷⑥⑦工作树）已 `git worktree remove` 回收（工作在分支链上＝零风险）；worktree list＝main＋dev。

## 4. 已知项登记（候点名，不擅动）
- **V1**（值面）：workspace bootstrap git 身份 `user.name=Culler user.email=culler@localhost`（onboardprojectgit.go:339）＝走查域内唯一 Culler 产物（workspace git log 可见）。
- **P1**（prose，未执行分支）：onboardconfig.go:32 Discord onboarding 测试消息内容（走查 discord-mode=disabled⇒未触发）。
- **P2**（prose，未触发分支）：onboardprojectgit.go:360 缺 main 分支诊断（走查新 workspace 走 git init 路径⇒未触发）。
- **P3**（prose，未触发状态）：vision.go:83 vision Unknown 状态 Reason（走查 vision=absent⇒未触发）。
- **P4**（prose，未触发路径）：vision.go:147 generatedVisionPurpose（agent 生成 vision 时写入文件；走查 project-vision=later⇒未触发）。
- **R1/R2'（AM v0.2.9 已入册）**：runner/collie-runner/main.ts:508（agent-input 信封面正常路径）＋:920（错误路径日志）。
- **prod-tier 面（候走查域指定）**：setup_*_prod.go 全家（`!test_fake_routing`⇒dev 走查不经过）："Start Culler"／"Install Culler"／"Update Culler"／"Culler saves the key in the Mac Keychain"等＋image_progress.go:73/:141 进度文案＋prepare.go:285 诊断文案（其 matcher 两代识别已随缺陷⑦修复，文案本身＝生产路径用户可见面）。
- **openwiki/**：Scribe 维护域（AGENTS.md 规则，人工不改；R2 表"openwiki/ 由 Scribe 重写"明文）＝wiki 重写候 dogfooding 触发。

## 5. 走查结论
**F1（dev tier scratch 域）＝GREEN（限定结论）**：
1. 安装体验面＝操作者可见输出与落盘用户可见文件**零 Culler 残留**（缺陷⑥⑦⑧修复后终树实证）。
2. 管线机制面＝fake-harness 流水线→批准→合并标准证明 PASS（304s，timer-driven 全管线）。
3. 清理面＝scratch 域完全移除，零容器零卷残留。
4. 限定项：V1（值面候点名，走查域唯一 Culler 产物）＋P1-P4（未执行分支 prose）＋prod-tier 面（候走查域指定）＋S5（schema 文本面候 B02 v1 基线预冻结窗，票面硬注顺序不变）。R2 验收句的"干净服务器（amd64）"半边＝218/CI 域，不在本走查授权范围（所有者批准域＝本机 dev tier scratch）。

**纪律**：走查全程未指 ~/.shorizon、未加载 launchd、未产生 provider token 消耗、未 push。Mimosa scanner_enobufs 于走查期间 commit 继续按兼容策略（不构成安全声明）。

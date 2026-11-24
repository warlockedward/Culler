# A09-S3 · 单元身份面迁移（darwin `com.s.shorizon.*` / linux `shorizon-*`）— diff-plan #1

## 0. 授权与终值依据
- 所有者 2026-11-25 grill-me 轮"全按推荐"：Q2=**O9 域名根=`s`**⇒darwin 终值 `com.s.shorizon.<suffix>.<unit>`；Q3=**linux 文法=dash 三段**⇒`shorizon-<suffix>-<unit>.service`；Q4=**uninstall 识别面保留 legacy `com.culler.` 前缀**（mission-control 先例同形）；Q5=(iii)=**Docker 镜像身份标签面（`com.culler.environment.operation/inputs`、`com.culler.project/home`）保留不动**（镜像内嵌数据非用户可见品牌）。
- R2 表（BRAND-WHITELIST B 行 launchd/systemd 行）在案终值文法。
- 设计意图在案：`supervision_systemd.go:126-128` 注释"the rename to S-Horizon lands with A09 as one sweep"。

## 1. 面分裁定（本提交的语义决定，逐面登记）
| 面 | 终态 | 理由 |
|---|---|---|
| **产出面** `UnitLabel()` | 仅新文法（darwin `com.s.shorizon.<suffix>.<unit>`；linux `shorizon-<suffix>-<unit>`） | D20 一次性语义家族（argv[0]/home 面同族）；零部署⇒无旧产出对象 |
| **防御/操纵面**（build-prod 守卫、redeploy.sh 停循环＋wait、dev-reset.sh 停＋quiesce、redeploy-dashboard.sh 查找） | 新文法＋legacy `com.culler.` **两代** | 这些面语义="检测到受监督部署即处理"；旧文法部署若在场（跨 S3 发布边界的升级世界），识别盲区＝守卫/停循环静默放行＝其要防的风险本身；"防御面认前代"设计语言由 Q4 裁定确立 |
| **uninstall 盘点** `CullerLaunchLabel()` | 新 darwin＋新 linux＋legacy `com.culler.`＋legacy `com.mission-control.` **四识别** | Q4 明示 |
| Docker 镜像身份标签面 | **不动** | Q5=(iii) |
| `mc-managed=true` label、Go 标识符、治理/时点/openwiki 面 | **不动** | D15＋S1/S2 先例 |

## 2. 改动面（全部 E1 已读）
### 生产机制面
| 文件 | 改动 |
|---|---|
| mc/deployment/agentsview_supervision.go:25-28 | 删 `UnitLabel`（迁至平台文件） |
| **新** mc/deployment/supervision_unitlabel_darwin.go | `UnitLabel = "com.s.shorizon." + names.Suffix + "." + unit`（文件名后缀＝隐式 GOOS 约束，与 supervision_activate_darwin.go 同形） |
| **新** mc/deployment/supervision_unitlabel_linux.go | `UnitLabel = "shorizon-" + names.Suffix + "-" + unit` |
| mc/verbs/uninstallinventory.go:70-80 | `CullerLaunchLabel` 前缀表加 `com.s.shorizon.`（agentsview 识别扩至新前缀）＋新增 linux dash 文法分支（`shorizon-<12hex>-<unit>`） |
| mc/deployment/supervision_systemd.go:126-128 | 注释更新（rename 已落地） |
| mc/deployment/supervision_systemd.go:137/:184 | Description "Culler <unit> (<label>)"→"S-Horizon <unit> (<label>)"（用户可见单元描述面，R1 D 同面随改先例） |
| runner/image/build-prod.sh | Linux 守卫＝全枚举＋本地两代过滤（systemd 单 pattern 参数无法并两代；249 空 pattern exit-1 陷阱已在注释在案⇒弃 pattern 改全枚举）；Darwin grep 两代；awk bootout/disable 指导行两代 |
| redeploy.sh | `read_units()` Linux 全枚举；`units_loaded()` grep 两代；停循环 awk（linux `^com\.culler\..*\.service$\|^shorizon-[0-9a-f]{12}-(resident\|dashboard\|agentsview)\.service$`，darwin 第 3 列两代前缀）；注释更新 |
| dev-reset.sh | linux 停 awk 两代（com.culler 点文法＋shorizon dash 文法）；darwin 停 awk 两代（com.culler.*.unit＋com.s.shorizon.*.unit）；`culler_units_loaded()` quiesce case 两代；注释更新 |
| redeploy-dashboard.sh | linux 查找 awk 两代；darwin 查找 grep 两代 |
| AGENTS.md:142 | "refuses while a `com.culler.*` launchd unit is loaded"→描述新现实（current-or-legacy supervised unit） |
| AGENTS.md:24 | 枚举行删 `com.culler.*` units（已迁移） |

### 测试 fixture 面（随生产输出面改）
| 文件 | 改动 |
|---|---|
| mc/cmd/mc/setup_existing_prod_test.go:277 | `labels`＝UnitLabel 产出 pin⇒新 darwin 文法（`com.s.shorizon.`+Suffix+…）；:278 `com.culler.other.resident` fixture 保留（非本部署 label＝foreign 数据实例） |
| mc/deployment/supervision_test.go:121/:123 | wantLabels⇒新 darwin 文法 |
| mc/deployment/agentsview_supervision_test.go:95 | plist Label golden⇒新 darwin 文法 |
| mc/deployment/supervision_systemd_test.go:34/:50 | golden Description "Culler resident (…)→"S-Horizon resident (…)"（label fixture 输入保留＝渲染器透传验证） |
| mc/verbs/uninstallinventory_test.go | 新增正例：新 darwin 文法三单元＋agentsview；新 linux dash 文法三单元（`.service` 剥离后形态）；负例保留（notes.txt/personal/agentsview.extra）；legacy com.culler.0287f755f1b7.* 三例保留＝Q4 legacy 识别正例 |
| runner/image/build-prod.test.sh:62/:110 | stub launchctl/systemctl 输出⇒新文法 label＋追加一行 legacy 文法（证明守卫两代识别） |
| dev-reset_test.sh / redeploy_test.sh / redeploy-dashboard_test.sh | stub 输出改新文法（"已部署单元"建模跟随产出面）；断言序面随同改 |

### 明确不动（数据实例面）
- setup_existing_preview_prod_test.go:177＋setup_existing_prod_test.go:197-198＝`spine_transfer_prod.go:117` "%s requires %s to be unloaded" label 透传消息的 fixture 实例（生产零硬编码前缀⇒label 内容无关断言）。
- doctorviewers_test.go:35/:50/:62＝InspectAgentsView mock 返回值实例＋Detail 透传断言＋拒绝文本实例（同上）。
- uninstall_prod_test.go:30＝uninstallReport.LaunchAgents 数据字段实例。
- Docker 标签面 45 行/11 文件（Q5=(iii)）。
- docs/BRAND-WHITELIST.md:22（R2 表现值列＝时点记录）、docs/contracts/**、docs/facts/**、evidence/**、openwiki/**、ITERATION-ROADMAP*。

## 3. 行为语义变化
- 新部署 darwin：launchd label＝`com.s.shorizon.<12hex>.{resident,dashboard,agentsview}`；安装文件 `~/Library/LaunchAgents/com.s.shorizon.<12hex>.<unit>.plist`（destination=label+".plist"，activationOrder_darwin:18）。
- 新部署 linux：单元文件 `~/.config/systemd/user/shorizon-<12hex>-<unit>.service`；systemctl 命令同形。
- 零部署⇒无任何存量单元文件迁移面。
- 防御面双代＝行为增强（识别集扩大），非行为改变（无部署时两代皆空集）。

## 4. 验证
1. `go build ./...`＋vet×8 组（两 GOOS 平台 UnitLabel 分派编译面）。
2. `./mc/check.sh` 全链（含 shell 门 dev-reset_test/redeploy_test/redeploy-dashboard_test/source-update-guard/uninstall_test 的 stub 面成对改）。
3. resident lane（CullerLaunchLabel 调用面若在 resident＝无，保险跑）。
4. dashboard lane（984 基线）。
5. runner/image lane（build-prod.test.sh 在覆盖集）。
6. 残留 grep：`com.culler.` 生产树＝**仅**防御/盘点两代识别行＋Q5 Docker 标签面＋不动数据实例面；`com.s.shorizon.` 在场于产出/防御/盘点/测试；`shorizon-[0-9a-f]{12}-` 在场于 linux 产出/防御/盘点/测试。
7. systemd 单元名合法性：`shorizon-<12hex>-<unit>.service` 字符集∈`[A-Za-z0-9._:-]`＋长度 152 限内（≈30 字符）。

## 5. 守界
不推远端；不触 Docker 标签面（Q5）；不触 schema（S5/B02 窗）；A09 status 维持 IN_PROGRESS（S3 后余量＝A09 F1 净机验证＋S5 候窗）。

# A09-S3 · 单元身份面 `com.culler.*` 迁移 — diff-plan #1（裁定包骨架）

## 0. 授权与终值依据
- R2 表（BRAND-WHITELIST B 行 launchd/systemd 行）：现值 `com.culler.*` ⇒ 终值 **`com.<域名根>.shorizon.*`（域名根＝O9 所有者供给事实）；Linux 直接 `shorizon-*`**。
- 设计意图登记在案：`supervision_systemd.go:126-128` 注释＝"Descriptions follow the CURRENT runtime identity (com.culler labels): the rename to S-Horizon lands with A09 as one sweep, and new artifacts must not run ahead of it with a mixed brand."
- 本件＝面枚举骨架（零代码）。执行前置＝三个所有者输入（§6），其中 O9 域值＝DEV-BASELINE §2"工单不得自行推测"锁定面。

## 1. 机制事实（E1 实读，2026-11-24）
### 1.1 构造枢纽（单点）
`mc/deployment/agentsview_supervision.go:26-28`：
```go
func UnitLabel(names Names, unit string) string {
    return "com.culler." + names.Suffix + "." + unit
}
```
`names.Suffix`＝12-hex 部署派生标识（RuntimeNames(home)）；unit∈{resident,dashboard,agentsview}（SupervisedUnits）。该函数产出：darwin launchd plist `<key>Label</key>`（supervision_darwin.go:91/:95）＋linux systemd 单元名（`installedUnitFileName(label)=label+".service"`，supervision_platform_linux.go:33）。**A03 218 证据实证**：`com.culler.92a163b06548.resident.service`。

### 1.2 回读/识别面
- `mc/verbs/uninstallinventory.go:70-80` `CullerLaunchLabel(label)`：前缀表 `{"com.culler.", "com.mission-control."}`＋unit 名判定（resident/dashboard＋agentsview 仅 culler 前缀）＝**卸载盘点识别谓词**（含 legacy 前代产品名 Mission Control＝有意保留识别面）。调用点×6（uninstall_prod.go:555/:577 两平台 uninstallServiceUnits＋doctor 等）。
- uninstall 计划面 `uninstallReport.LaunchAgents` 直接回显 label 给操作者（culler_uninstall_prod.go "Startup units" 面板）。

### 1.3 shell 前门守卫面（S2 跨面污染热点，转义形态）
- `runner/image/build-prod.sh:74`：`systemctl --user list-units --type=service --no-legend 'com.culler.*'`＝生产构建拒绝守卫（AGENTS.md 不变量段引用）。
- `scripts/redeploy.sh`、`redeploy-dashboard.sh`、`dev-reset.sh`：awk/grep 单元谓词（含 `com\.culler\.` 单/双反斜杠转义形态＝S2 缺陷 b/d 两次自抓的同一面）。
- `AGENTS.md` 不变量段："build-prod.sh refuses while a `com.culler.*` launchd unit is loaded"（活文档，随行为同改）。

### 1.4 Description 字符串面
`supervision_systemd.go:137` `"Culler " + item.unit + " (" + item.label + ")"`、`:184` `"Culler AgentsView (" + UnitLabel(...) + ")"`；darwin plist 对应面。

### 1.5 Docker 身份标签面（**R2 表未命名＝新发现面**）
`com.culler.environment.operation`＋`com.culler.environment.inputs`（28 处：environment-lane.ts 镜像构建 label 写入 :482-483/:586/:788/:820-821＋**信任判定回读** :454-455 `trusted()`＝镜像身份验证输入、:578-579/:718-719/:955/:970 容器标签比对）；`com.culler.project`＋`com.culler.home`（17 处：effects.ts:703-704 容器创建 label、project_delete.go 过滤/匹配、project-containers.ts:15 回读、uninstall_prod.go:694/:705 镜像盘点 filter＋operation identity 验证）。**语义＝镜像/容器的身份 wire**（写入 docker 数据，跨进程回读），与 S4b token 同族＝wire-adjacent 面。

### 1.6 全量 token 普查（生产树，本件 §1 各面之和）
mc＋resident＋dashboard＋runner＋scripts＋tests 内 `com.culler.` token＝约 90 行（含测试 fixture：supervision_test/supervision_systemd_test/setup_existing_prod_test/uninstall_prod_test/doctorviewers_test/uninstallinventory_test/project_delete_docker_test/project_delete_integration_test/uninstall_modes_prod_test/effects.test/project-containers.test/build-prod.test.sh 的 `com.culler.deadbeef.resident`/`com.culler.fixture.*`/`com.culler.0123456789ab.agentsview` 等钉值）。evidence/openwiki/治理面 token 不随改（S1/S2 先例）。

## 2. 行为语义变化（如实登记）
- 新部署的单元身份＝新前缀；**无任何现存部署**（本机零、218 零）⇒零兼容对象（与 S4a argv[0] 一次性语义同族）。
- **uninstall 识别面的设计问题**（候裁）：改名后 `CullerLaunchLabel` 前缀表是否**保留 `com.culler.` 识别**？保留＝未来若存在旧身份部署可被卸载清点（同 mission-control legacy 先例）；不保留＝识别面收缩为仅新前缀。mission-control 先例＝"前代产品识别保留"，推论 `com.culler.` 应同为 legacy 保留（S3 推荐，候确认）。
- Docker 标签面若改名：已构建镜像（218 runner stack 的 project images）持旧标签＝产品回读不再识别⇒视为 foreign image 跳过（uninstall 语义＝不误删，安全方向）。零部署⇒今日零影响。

## 3. 明确不做（排除面）
| 面 | 判定 | 理由 |
|---|---|---|
| `mc-managed=true` docker label（uninstall filter） | 保留 | 内部代号面（mc 不改，D15） |
| evidence/ 原始件里的 `com.culler.*`（A03 218 证据等） | 保留 | 时点记录 |
| openwiki/.claims、docs/contracts 时点条目 | 保留 | Scribe 域＋治理先例 |
| `com.mission-control.*` legacy 识别 | 保留 | 前代产品卸载语义，不在 A09 范围 |

## 4. 执行计划（三输入齐后，单提交一次清扫，同 S4a 模式）
1. `UnitLabel()` 构造枢纽按平台分派终值文法（darwin=`com.<O9>.shorizon.<suffix>.<unit>`；linux=`shorizon-<suffix>-<unit>` 或 `shorizon-<suffix>.<unit>`＝**形状候裁 Q-S3-2**）。
2. `CullerLaunchLabel()` 前缀表＝新前缀（双平台文法）＋legacy `com.culler.`（若 Q-S3-3=保留）。
3. shell 守卫三脚本＋build-prod.sh 谓词同改（**含全部转义层形态**＝S2 缺陷 b/d 教训：改后按 `com`＋反斜杠×N＋`.shorizon`/新前缀 全形态复扫防跨面残留）。
4. Description 串＋AGENTS.md 不变量段活文档同改。
5. Docker 标签面（若 Q-S3-4 裁定改）：写入点＋回读点＋filter 谓词同提交成对改；测试 fixture 钉值随改。
6. 测试面 fixture 钉值（deadbeef/fixture/0123456789ab 家族）逐 token 判面随改。

## 5. 验证计划
1. `go build ./...`＋vet tag 组（darwin/linux 双平台编译面）。
2. `./mc/check.sh` 全链（supervision_test/supervision_systemd_test/shell 门 redeploy_test＋dev-reset_test＋source-update-guard＋uninstall_test 全含单元谓词断言）。
3. resident lane（effects/environment-lane/project-containers 测试族，若标签面改）。
4. **218 再渲染验证**：新文法单元文件 `systemd-analyze verify`（A03 C2.verify 同形＝unit 文法改变后旧验证记录过期，需新跑一轮；218 往返＝所有者点名轮内顺带，非独立往返）。
5. 排除面计数不变（evidence token 数前后差＝0）。

## 6. 所有者三输入（执行前置）
| ID | 输入 | 说明 |
|---|---|---|
| **O9**（已有登记） | darwin 域名根值（如 `s.horizon`/`shorizon`/…） | DEV-BASELINE §2 锁定面；本件不推测 |
| **Q-S3-2** | linux systemd 单元名文法 | R2 表"Linux 直接 shorizon-*"未定细节：`shorizon-<suffix>-<unit>.service` vs `shorizon-<suffix>.<unit>.service` vs 其他；systemd 单元名合法字符集允许 `-`/`.`/`_` |
| **Q-S3-3** | uninstall 识别面是否保留 legacy `com.culler.` 前缀 | 推荐＝保留（mission-control legacy 先例同形） |
| **Q-S3-4** | Docker 标签命名空间终值 | R2 表**未命名此面**＝新发现；候选：(i) 随单元面 `com.<O9>.shorizon.*` (ii) `shz.*` (iii) 保留 `com.culler.*`（标签＝机器间 wire 非用户可见品牌，保留成本最低且零行为变化）。推荐 (iii) 保留：理由＝该标签面是**镜像身份数据**（docker registry 内持久），非运行标识符；改它无用户可见收益，只引入"旧镜像变 foreign"的语义扰动；A09 票面 F1"净机端到端无 Culler 残留"若含此面则改（**此条＝票面解释问题，请所有者一言**：F1 的"无 Culler 残留"是否覆盖 docker 镜像内嵌 label 数据？） |

## 7. 守界
本件＝零代码。O9 值到达前 S3 维持 BLOCKED（不变）。Docker 标签面若裁定 (iii) 保留⇒S3 面收缩为纯单元身份面，A09 票面 F1 按保留裁定重释。

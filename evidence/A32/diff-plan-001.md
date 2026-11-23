# A32 diff-plan #1（实施轮：C-A P-1 = release.yml 新增 `opt-in-lanes`，两车道起步+一车道呈裁）

状态=CODED（本地义务毕；**runner 实跑证据=推送后首条 CI**，EVIDENCED 转写挂波门，不提前）。授权=轮十六"批准整条议程"（D2 P-1；唯一必改文件、零 secrets、发布枚举三处零触碰——C-A 准备件 §5 不改清单逐项遵守）。

## 1. 改动（恰一文件）
`.github/workflows/release.yml`：`fast-suite-macos` 与 `application-package` 之间新增 job `opt-in-lanes`（ubuntu-24.04，`timeout-minutes: 60`）：
- setup 配方逐字仿 fast-suite（同 pinned action SHA）；依赖装 npm openwiki+patch+agent-runner bun（**不装 dashboard 依赖**——两道均不需，减面）。
- 车道 1=`docker_boundary`；车道 2=`docker_e2e`（`-timeout 45m`=D4 数值，AGENTS 文本同步随 D4 步）。
- **不入任何 needs 链**（`release-checks` 九项逐字不动、`go-cache-cleanup.needs` 逐字不动——新 job 不存 go-build 缓存以保此零触碰）；零新 secrets。

## 2. 三车道→两车道的票面条款执行记录（波门呈裁件）
票面 F1① 原列三道。`update_docker` **本 job 不纳入**，依据=其前置资产唯一来源 `runner/image/build-prod.sh` 在 runner 环境**存在功能性阻断证据**：该脚本 `:74-79` 令 `systemctl --user list-units` 非零退出即 abort（exit 3），GitHub hosted runner 无用户 systemd 总线（该命令典型 `Failed to connect to bus` 失败）⇒ 依票面 F1③"runner 未实测=先试跑再定去留"条款，**证据先于试跑到达**，采"缓纳"。证据级别=**E2 推断（未实测 runner，无日志）**；如波门判应赌实测，恢复成本=job 加 2 步（build-prod + env 两行）。
boundary/e2e 不受此影响：其镜像由测试自身经 `runner/image/build.sh` 构建（`mc/e2e/e2e_test.go:61-65` imageCommand 亲验，不触 build-prod 守卫）。

## 3. 本地证据
- 三守卫 `GUARDS_EXIT=0`（release-workflow/security-gate/smoke-gate，16 pass/0 fail——新 job YAML 解析成功且既有断言零转写，守卫形态为存在性检查之预判经实测成立）。
- 待补：合并前 `dashboard` 全道跑一次（本 job 文件属 dashboard 测试读取面）。

## 5. 返工段 #1（轮三十：自托管 runner 上线后，据测量恢复票面原范围）
**背景**：§2 曾把 `update_docker` 缓纳，依据=「`build-prod.sh:74-79` 的 `systemctl --user` 守卫在 GitHub **托管** runner 上必 `exit 3`」，当时标 **E2 推断未实测**。现两台机器分别取得测量结果，据此更正执行范围：
| 机器 | 实测 | 结论 |
|---|---|---|
| 托管 runner（run 45097992173） | `deployment-linux` 的 13 条监督测 `activation rolled back … job is not loaded`；`opt-in-lanes` 早跑 | **E2→E1 证实**：托管 runner 无 user manager，守卫与依赖 systemd 的测都不可用 |
| **218 自托管 runner（身份 `shzci`）** | `systemctl --user is-system-running=running`；守卫原命令 **`rc=0` 且空输出**（="无部署在跑"，语义为真）；`python3`/`node`/`docker 29.4.0` 皆可用 | **`build-prod.sh` 可在此合法运行** ⇒ 缓纳理由消失 |

**因此本返工做三件，均在原批准范围内（P-1 点名授权覆盖"新增 job opt-in-lanes 含 docker_e2e+docker_boundary+update_docker（+nightly 属性道）"）**：
1. **在 job 内构建 `mc-prod`**（调用票面唯一 sanctioned 路径 `runner/image/build-prod.sh`，不另写第二套构建逻辑=不建平行权威）。**这修掉一个我先前未拦住的假绿风险**：218 上原有一枚 `mc-prod:latest`（实测 `created=2026-11-21T12:37 rev=9e0c54bd…`）**与被测提交无关**——若不构建，即使跑绿也不是对被推代码的证据（且 release 身份不一致会 fail-closed）。现强制现场重建。
2. **恢复 `update_docker`**（原票面第三道）+ `nightly` 属性道（秒级）——非扩面，是补齐我上一步自行削减的范围。
3. **加 `workflow_dispatch` 触发器（W3，已获准）用于在**非 main** 的探针分支上迭代本 job**：避免拿可能写错的构建步骤反复推 `main`。安全阀已先验：**所有发布/凭据步骤均门在 `event_name=='push' && ref=='refs/heads/main'`**（:136/249/329/376/423/494/565/652/773/888 实读），dispatch 不满足 ⇒ **不可能触发发布**；且三守卫实测不断言触发器集合（`smoke-gate` 那处 `on:` 系注释用词）。
4. `timeout-minutes` 60→**150**（现场建双镜像 + browser-smoke + e2e 45m 预算 + 其它；60 分必然不够，属参数校正非扩面）。
**围栏不变**：`runs-on: [self-hosted, shz-ci]`（私有标签；本 workflow 仅本 job 请求它 ⇒ 其余 job 结构上落不到该宿主）。**arm 事实不变**：218=x86_64，托管 arm job 仍不在其上跑（§findings 已登记为覆盖缺口，非本票可解）。

## 6. 本轮仍不做的（边界）
不改发布 job 权限/环境/凭据注入；不改 `security-nightly.yml`；不改三处发布检查枚举与 prune 正则（P-1 承诺）；不动 `build-prod.sh` 本体（敏感发布件，A27 有"无 override"在册）；不删他人钉测。

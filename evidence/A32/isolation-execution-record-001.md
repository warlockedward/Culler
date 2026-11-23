# CI Runner 隔离实施记录 #1（2026-11-23，执行 `runner-isolation-plan-001.md` S0–S6 全链）

## 0. 一句话结论
专用 Docker 栈（独立 containerd + 独立 dockerd + `shz-br0@10.199/16` + socket 组权限 + 摘除共享组）在 218 落地，job 全程改走专用栈，主 daemon 与其他项目容器**前后差零扰动**（§3）。计划字面形的三处 flag 冲突被实跑证伪并已在计划件 §2a 更正（fd5bb32）。

## 1. S0–S5 逐步实况（全部现场实读）
| 步 | 动作 | 实测证据 | 判据达成 |
|---|---|---|---|
| S0 | 暂停接单＋取基线 | `systemctl --machine shzci@.host --user stop actions-runner`：active→inactive，`Runner.Listener` 进程数=0；GitHub API `status=offline busy=false`。基线（218 `/tmp/shz-s0-baseline-*.txt`，已拉回本地 `/tmp/shz-s0-baseline-containers.txt` 4537B / `-volumes.txt` 8967B）：**容器 63（含已退）/ running 60 / 卷 169 / healthy 32 / unhealthy 2**（unhealthy=他人 `nf-pve-runner`/`nf-testbox`，Up 3–4 weeks，在册不动） | ✅ |
| S1 | 独立 containerd unit `shz-ci-containerd.service` | `containerd config default > /etc/shz-ci/containerd.toml`（273 行独立生成）；ExecStart 独立 `--root /var/lib/shz-ci/containerd --state /run/shz-ci-containerd --address /run/shz-ci-containerd/containerd.sock`；`systemctl is-active`=active；socket 存在；`ctr -a <sock> version` 应答 v2.2.2。**主 containerd 全程未动**（is-active=active） | ✅ |
| S2 | 独立 dockerd unit `shz-ci-docker.service` | 三次失败-修复迭代（dockerd 自身报错实证，非推断）：① `--data-root` flag 与宿主 `/etc/docker/daemon.json` 同指令冲突⇒改独立 `--config-file /etc/shz-ci/daemon.json`（仅 data-root）；② `--bridge`×`--bip` 互斥⇒`ExecStartPre` 幂等预建 `shz-br0@10.199.0.1/16`＋仅 `--bridge shz-br0`；③ 默认 pidfile 撞主 daemon⇒`--pidfile=/run/shz-ci-docker/docker.pid`。终态：`is-active`=active；`info`：**DockerRootDir=/var/lib/shz-ci/docker、ServerVersion=29.4.0、Containers=0**；`shz-br0` inet 10.199.0.1/16；`images`=空表（证非共享）；socket `srw-rw---- root shzci`。**主 daemon DockerRootDir=/data-shunhong/docker_data 未变** | ✅ |
| S3 | job 指向专用 socket＋触发验证 | `release.yml` `opt-in-lanes` job 级 `env: DOCKER_HOST: unix:///run/shz-ci-docker.sock`（392e10f；job 级=仅本 job，不动宿主全局）；三守卫 16/0（release-workflow/security-gate/smoke-gate，零转写）；YAML 解析核过。验证 run **45165214478**（ci/routing-base=392e10f）：job conclusion=**success**（OPT_IN_LANES_EXIT=completed/success，本地 20:21）。**专用栈活证据**：`mc-e2e-helper-*` 容器运行于专用 daemon；6 镜像（mc-prod/mc-project/mc-fake-e2e/mc-fake-e2e-project/mc-real-e2e/mc-real-e2e-project）全部仅存在于专用栈（创建时间=run 窗口内）。**红集差值**：boundary 20 FAIL 行与 run 45153594551 **逐字同集**（机器 diff）；其中 `TestAttachmentCanaryRecoversExactInterruptedAttempt` 签名由"container name already in use"（0.27s fail-fast）**转为** `warm uid-10002 attachment read failed: exit status 41`（6.92s）=N3 残留雷结构性消失（新栈零容器起步），该测试本身仍红=T2 族共同根因（预期，findings-003 §3.1）。e2e 25 FAIL 行同集、根因签名 exit-41×22＋timed-out×20 同值。**硬门在专用栈上真绿**：`ok mc/deployment 29.108s`＋`ok mc/property 2.742s`。job 后专用 daemon 无残留容器（running=0） | ✅ |
| S4 | 摘除 `shzci` 的 docker 组 | `gpasswd -d shzci docker`：`id shzci` groups 由 `1001(shzci),120(docker)`→`1001(shzci)`。判据实测：`su - shzci -c "docker ps"`（默认 socket）=**`permission denied while trying to connect to the docker API at unix:///var/run/docker.sock`**（隔离成立）；`su - shzci -c "docker -H unix:///run/shz-ci-docker.sock ps"`=**正常输出**（socket 组权限生效）。再触发验证 run **45173724893**（证 job 无任何默认 socket 依赖路径）——已收口（§4）：job success、红集与 S3 同 commit 同集、硬门二次真绿、专用栈 run 后自净 | 组摘＋socket 双面判据✅＋摘组后 run success✅ |
| S5 | 主 daemon 无扰动复核（前后差） | S3 run 结束后实读：容器清单 diff（S0 基线 vs S3 后）=**仅 Status/uptime 时序漂移**（同一 63 个容器 ID、同一镜像，零增减零改名）；卷清单 diff=**零**（VOLUMES-IDENTICAL）；计数 main_total=63 / running=60 / volumes=169 / healthy=32 / unhealthy=2 = 基线逐项相等 | ✅ |

## 2. 218 上的新增物（全清单，供回退/审计）
- `/etc/systemd/system/shz-ci-containerd.service`（enable+active）
- `/etc/systemd/system/shz-ci-docker.service`（enable+active；含 ExecStartPre 预建桥）
- `/etc/shz-ci/containerd.toml`、`/etc/shz-ci/daemon.json`（各仅必要项）
- `/var/lib/shz-ci/containerd/`、`/var/lib/shz-ci/docker/`（独立状态目录；S3 run 后专用栈 data-root ≈2G+，含 6 镜像层）
- `/run/shz-ci-containerd/`、`/run/shz-ci-docker/`（运行时 socket/pidfile；tmpfs，重启即清）
- 网桥 `shz-br0@10.199.0.1/16`（与 172.17–172.31 全部既有网段无交集）
- 账户面：`shzci` 已移出 `docker` 组（S4）
- **未动**：主 dockerd/containerd 任何配置、主 daemon 任何容器与卷（除 S3 前清理的自家 fixture 残留 1 件，前后差 61→60 在 findings-003 §7）、`build-prod.sh`、他人项目

## 3. "未影响其他项目"的证明形态（在册纪律：要证不能假设）
前后差=§1 S5 行的三份比对（容器清单/卷清单/五计数），S0 基线件与 S3 后态件双端留存（218 `/tmp/shz-s0-baseline-*.txt`、`/tmp/shz-s3-after-*.txt`；本地副本同路径）。S4 验证 run（45173724893）结束后再取一次终态件收口 §5。

## 4. S4 验证 run 实况（2026-11-23 收口）
run **45173724893**（ci/routing-base=392e10f，摘组后派发）：job opt-in-lanes conclusion=**success**（`OPT_IN_LANES_EXIT=completed/success`，本地 21:14）。判据全达：
- **摘组后 job 仍全走专用 socket**：build 双镜像、boundary、e2e、update_docker、property 全完成，无一回退默认 socket（默认 socket 已 permission denied，见 §1 S4 行）⇒ "job 无任何默认 socket 依赖路径"成立。
- **红集=同 commit 同引擎语义的稳定性实证**：boundary 20 FAIL 行与 S3 run（45165214478）**逐字同集**（机器 diff）；e2e 25 FAIL 行同集（exit-41 共同根因签名同值）；两道软门 `##[error]exit code 1` 被吞×2。
- **硬门在摘组+专用栈上二次真绿**：`ok mc/deployment 28.862s`＋`ok mc/property 2.659s`。
- **专用栈 run 后自净**：`docker -H <shzsock> volume ls`=0（测试 t.Cleanup 卷全清）、running 容器=0。

## 5. S6 终态与收口（2026-11-23）
- runner `online`/`busy=false`（GitHub API 实读）＝恢复常驻接单。
- 终态五计数=**S0 基线逐项相等**：main_total=63 / main_running=60 / main_volumes=169 / healthy=32 / unhealthy=2（他人 nf-* 既有）。容器清单 vs 基线 diff=18 行**纯 Status/uptime 文本漂移**（总量不变⇒零增减零改名）；卷总量=基线。
- 主 daemon `mc-*` 前缀容器=**0**（S3 前清的 fixture 残留未再出现——N3 雷在专用栈上无共享命名空间可落）。
- shzci 组=仅 `shzci`（docker 组摘除终态确认）。
- 本件收口=隔离 S0–S6 全链实证完成；专用栈成为 CI 唯一 docker 面。

## 6. 风险残留（不粉饰，承计划 §4）
- 两 root dockerd 并存的 iptables 复杂度：S3/S5 实测**未见**对其他容器网络扰动（nexusflow 等 healthy 计数不变、卷不变），但"未见"非"证无"；若日后现网 NAT 规则异常，回退路线=计划 §4 末（runner 不常驻接单，按需 dispatch）。
- 专用栈 data-root 每轮重复占镜像层（S3 后 ≈2G+）；`/var/lib` 余量充足（44G 级），GC 策略（每轮 `docker -H <sock> system prune` 或 cron）为后续小项，未实施。
- 仍属共享内核的资源/状态/权限隔离，**不宣称 VM 级**（计划 §4 原话有效）。
- arm 覆盖缺口不变（218=x86_64）。

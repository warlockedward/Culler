# CI Runner 隔离方案 #1（所有者裁"专用比较合理——218 上还跑着很多其他开发项目"；本件=设计，未实施）

## 0. 现状实测（本会话只读探测，全部实跑）
| 事实 | 值 | 含义 |
|---|---|---|
| `shzci` 组 | `shzci docker` | **在 docker 组 ⇒ 对宿主 root dockerd 有全权**（可启停/删任意容器与卷） |
| 主 dockerd | root 服务，`data-root=/data-shunhong/docker_data`，sock=`/var/run/docker.sock` | **与被测项目无关的生产/开发栈共用** |
| 同宿主共存 Docker 网络 | `bridge 172.17` / `data_default 172.18` / `fastgpt_opensandbox 172.19` / `newapi 172.20` / `data_fastgpt 172.21` / `data_vector 172.22` / `lsm-main 172.23` / `nexusflow-frontend 172.26` / `neo4j 172.25` / `nexusflow-*-observability 172.28/29/31`，`172.30` 被路由占用 | **≥8 套其他项目在跑**（nexusflow、fastgpt、vector、neo4j、newapi、lsm-main、opensandbox…）⇒ 所有者判断成立且比预期更要紧 |
| 空闲网段 | `10.199/16` 未被占用 | 专用网桥可用 |
| 资源 | `/var/lib` 44G 空闲；内存 avail **350G** | 独立栈有空间 |
| `shzci` sudo | 不可 | 第二 daemon 须由 root systemd 起，shzci 仅取 socket 权限（正合隔离需要） |
| containerd | `containerd.io v2.2.2`，`/etc/containerd/config.toml` | 可用独立 root/state/socket/config 起**第二实例** |
| `/etc/subuid` | 已有 1 行 shzci；**uidmap 未装、fuse-overlayfs 无** | rootless 需先补依赖 |

## 1. 为什么**不**用最省事的 rootless docker（关键取舍，写死留档）
本仓库这一族车道测的是**真实安全边界语义**：
- DEFECT-004/A29：容器内 git 以 uid 10002 访问**宿主 uid 拥有**的仓库 ⇒ `dubious ownership`；
- A03/A16/A21：spine 挂载的属主/WAL sidecar 共享语义、容器 uid 与宿主 uid 的差；
- 这些断言的含义依赖"容器 uid ≠ 宿主 uid"的**具体映射关系**。
rootless 下容器 root/uid 会被 **subuid 偏移**（如映射到宿主 100000+），且默认 overlay 行为随 fuse-overlayfs 改变 ⇒ 测试**可能仍绿但验的不再是生产那条边界**＝**用证据有效性换隔离**。所有者诉求是"别影响其他项目"，不是"改测试语义"。⇒ 否决 rootless（若将来必须，需另立票重估各 uid 断言并复核，不得顺手换）。

## 2. 采纳方案：独立 Docker 栈（语义无损 + 真隔离）
四个改动，缺一不可（**只做前三 = 白做**）：
1. **独立 containerd 实例** `shz-ci-containerd`：独立 `--root /var/lib/shz-ci/containerd --state /run/shz-ci-containerd --address /run/shz-ci-containerd/containerd.sock` + 独立生成的 config。
2. **独立 dockerd** `shz-ci-docker`：root 运行，`--containerd=/run/shz-ci-containerd/containerd.sock`、data-root=`/var/lib/shz-ci/docker`（经 `--config-file` 传入，见 §2a）、`-H unix:///run/shz-ci-docker.sock`、独立 `--pidfile`（见 §2a）、`--group shzci`（socket 组权限给 CI 用户）、**`--bridge shz-br0`**（`shz-br0@10.199.0.1/16` 由 unit `ExecStartPre` 幂等预建；`--bip` 与其互斥，见 §2a）＋ `--default-address-pool base=10.199.64.0/18,size=24`（避开 172.17–172.31 全部既有网段）。**不复用主 containerd、不共用 data-root** ⇒ 与 nexusflow 等的容器/卷/镜像状态目录完全无交集。
3. **job 环境指向专用 socket**：仅在本 job `env: DOCKER_HOST=unix:///run/shz-ci-docker.sock`（不动宿主全局、不影响其他项目）。
4. **把 `shzci` 移出 `docker` 组** ⇒ 宿主 root dockerd 对 CI **彻底不可达**（隔离闭环；若漏这步，runner 仍能用默认 socket 碰到所有项目＝本方案的全部意义归零）。

### 2a. S1–S2 实跑实施偏差（2026-11-23，§2 字面形被运行证伪，实现语义等价）
下列三处冲突均于 S2 实跑由 dockerd 报错实证（§2 字面照抄不可运行）：
1. **`--data-root` flag 与宿主配置文件冲突**：`/etc/docker/daemon.json` 含同名指令（`data-root=/data-shunhong/docker_data`）⇒ dockerd "specified both as a flag and in the configuration file" 即 exit 1 ⇒ 实现=专用 daemon 用独立 `--config-file /etc/shz-ci/daemon.json`（仅 data-root 一项），flags 不再带 data-root。
2. **`--bridge` 与 `--bip` 互斥**：dockerd 自报 "You specified -b & --bip, mutually exclusive options" ⇒ 实现=unit `ExecStartPre` 幂等预建 `shz-br0@10.199.0.1/16`（存在则跳过），dockerd 仅持 `--bridge shz-br0` 附着既有桥。
3. **第二 dockerd 须独立 pidfile**：默认 `/var/run/docker.pid` 与主 daemon 互斥（"process with PID … is still running"）⇒ 实现=`--pidfile=/run/shz-ci-docker/docker.pid`。
S1 的 containerd 实例无冲突（独立 root/state/address＋独立生成 config，实跑绿）。S2 判据全过：独立 `DockerRootDir=/var/lib/shz-ci/docker`、`shz-br0` 10.199.0.1/16、`images` 空表证非共享、socket `root:shzci`、主 daemon 数据根与容器数未动。

## 3. 分步实施 + 各自验证（每步可独立回退；**顺序不可颠倒**）
| 步 | 动作 | 通过判据 | 回退 |
|---|---|---|---|
| S0 | **暂停 runner 接单**（当前 job 跑完后） | GitHub 侧 runner `status=offline` 或不再派活 | 重新 start |
| S1 | 建独立 containerd unit，`systemctl start` | socket 存在；`ctr -a <sock> version` 可答 | `disable --now` + 删 unit（主 containerd 全程未动） |
| S2 | 建独立 dockerd unit，`systemctl start` | `docker -H <sock> info` 显示独立 data-root + `shz-br0`；`images` 列表**为空**（证非共享）| 同上；主 dockerd 未动 |
| S3 | **先不摘组**，把 job 的 DOCKER_HOST 指向新 socket，触发一次 dispatch | `opt-in-lanes` 在新栈上真跑（且 nexusflow 容器数不变——前后对比） | 改回 workflow 或停 dispatch |
| S4 | **摘除 `shzci` 的 docker 组**，再触发一次 | `su - shzci -c "docker ps"` 对**默认** socket **报错**（=隔离成立）；对新 socket 正常；job 仍绿 | `usermod -aG docker shzci`（不建议，除非新栈故障需应急） |
| S5 | 复核其他项目无扰动 | `docker ps`（主 daemon）计数与 S0 基线一致；nexusflow/fastgpt 等容器均 `healthy` | — |
| S6 | 恢复 runner 接单 + 清理 | runner online、busy=false | — |

基线快照在 S0 取（主 daemon 容器/卷清单 + 健康态），S5 逐字比对——**"未影响其他项目"要证，不能假设**（在册纪律：感知≠不存在，须前后差）。

## 4. 风险与残留（不粉饰）
- **iptables/网桥**：两个 root dockerd 并存会各自写 FORWARD/`DOCKER-*` 链。既有项目已有一批 bridge，新链以 `shz-` 前缀独立；理论风险=nat 规则复杂度上升。缓解=独立 bridge 名与网段、S5 复核其他容器网络仍通。**这是本方案唯一不能在设计阶段完全排除的风险，须以 S3/S5 实测收敛**；若实测发现对现网有扰动，回退到"runner 不常驻接单"（=保留注册、按需 dispatch）并向所有者报告再议 VM 路线。
- **磁盘**：独立 data-root 会重复占镜像层（约 +数 GB/轮），`/var/lib` 44G 空闲 ⇒ 需配 GC（每轮后 `docker system prune` 限本 daemon，或 cron；属后续小项）。
- **buildx cache**：build-prod 用 BuildKit 走本 daemon；缓存留在专用 data-root，不污染主栈。
- **仍属"共享内核"隔离**：这是 root 服务跑在同一内核上，非 VM/微内核级隔离。若日后要求"CI 与宿主彻底不同内核"，则需 `/dev/kvm` + libvirt（当前未装）另立运维项；本方案满足的是**资源/状态/权限隔离**（不再能触碰其他项目容器与卷），与所有者诉求一致，但**不宣称更强隔离**。
- **arm 覆盖仍缺**：218=x86_64 ⇒ 托管 arm job 无处可跑（既有登记，与本方案无关）。

## 5. 未做的（守边界）
不擅自 `lxd init`（曾评估为容器路线，未 init 且会新建存储/网桥，改动面更大）；不改主 dockerd/containerd 任何配置；不给 shzci sudo；不动 `build-prod.sh` 本体（A27 在册"无 override"）；不删他人钉测。

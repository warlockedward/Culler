# A31 验证记录 #1（TM §7.1 · Linux 生产 launch 的宿主网关映射 · 真 Linux Docker 引擎端到端）

- verification_id: **A31-V-001**
- target_component: `resident/src/effects.ts` 两处 provider-access 挂载点新增 `--add-host host.docker.internal:host-gateway`（A31 交付）——解决"Linux 引擎下容器不解析 host.docker.internal ⇒ 容器到宿主代理断链"（Mac 天然可解析，Linux 需此映射）
- tested_version: dev tip `a66322b`（含 A31 提交 fe90b9d）
- 运行域：**218.84.111.20 Docker 29.4.0（真 Linux 引擎，x86_64）**——route A 所有者扩权内；用本地既有 `mc-prod:latest` 镜像，`--rm` 无挂载无端口
- 方法：marker-needs-negative-proof 双侧——同镜像同命令，仅 `--add-host` 有无之差

## 结果（实读）
1. **带旗（正控）**：`docker run --rm --add-host host.docker.internal:host-gateway mc-prod:latest getent hosts host.docker.internal` → **`172.17.0.1 host.docker.internal`** ⇒ Linux 容器解析到 docker 网桥宿主地址=A31 断言在真 Linux 引擎坐实（此即 effects 交付的那枚旗的效果）。
2. **不带旗（负控）**：同命令去 `--add-host` → **无任何解析输出行** ⇒ 无映射则 Linux 容器不解析该名，反证旗有效（非环境天然可解析）。
3. 残留：`docker ps -a --filter ancestor=mc-prod` = **0**（`--rm` 全清）；未触生产容器/卷/网络配置。

## 诚实标注
- 命令尾 `without_rc=0` 测的是 `| head` 管道退出码非 getent；有效证据=**解析行"有 vs 无"** 之差（本记录①②），非退出码。不以此充数。
- `172.17.0.1`=218 的 docker0 网桥网关（宿主侧地址，非机密）；本次仅容器内 `getent` 解析，**未发起任何对宿主端口的实连**（宿主代理端口连通=effects 真实 launch 场景，其容器侧由 A02 proxy-tls 记录 + docker_boundary 本地绿覆盖；此记录专证"名字解析得到"这一环）。
- 覆盖边界：effects 两处挂载点产参已由 fe90b9d 的 `effects.test.ts` 正/负断言（旗值成对 + fake 无旗）在案；本记录补的是**真 Linux 引擎上该旗确实产生解析效果**（单测证"参数发出"，此证"引擎兑现"）。

- reviewed_by: host-agent（agent E1；218 真宿主经所有者 route-A 扩权；非独立复验）
- verdict: **A31 保持 IN_PROGRESS，不虚标 EVIDENCED**——票面 F1③="Linux 真容器→宿主代理端到端"，本记录仅坐实"解析"半环（真 Linux Docker 双侧）；"→宿主代理端口实连"半环未证，须 route B/CI 的 docker_e2e 于 Linux 引擎跑通才达全验。此前延付的"解析"环节已从"本机 Mac 容器"升级到"真 Linux 引擎"（fe80b9d 单测证参数发出 + 本记录证引擎兑现），是实质推进但未达票面全句，据实不升。

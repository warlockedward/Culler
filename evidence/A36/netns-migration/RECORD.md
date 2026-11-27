# A36 netns 迁移实施记录（218 专用 CI dockerd 隔离）

日期：2026-11-27。窗口：轮六十二。批准：owner「CI dockerd 迁入独立 netns，不要对宿主机的其他租户转发的动作产生影响。包括 systemctl disable shz-ci-docker 的问题」。

## 结论

迁移完成并通过全链路验证。CI dockerd 与 CI containerd 运行于 netns `shz-ci` 内，其 netfilter 生命周期与宿主三表完全隔离。宿主 CI 面 footprint：1 个 netns、1 对 veth（transit `10.200.0.0/30`）、1 条 scoped 回程路由、3 条 `shz-ci-gw` 注释防火墙规则。

## 背景与问题

- 事故（轮六十一取证）：双 dockerd 同处宿主 netns，CI dockerd 启动清理（`Deleting nftables rules`，dockerd[4016261]）冲掉主 daemon nat DOCKER 链 → 全宿转发失效。
- Docker 权限模型：socket 即全权（prune/网络操作无 per-client 限制）；netfilter 每 netns 单租户设计；CI 网络 churn 对共享链无只读模式。独立 daemon＋独立 netns 是该共享宿主上的最小安全隔离。

## 架构

- netns `shz-ci`：`lo`、`shz-br0`（10.199.0.1/16，CI 默认桥）、`veth-ci-c`（10.200.0.2/30）、`ip_forward=1`、default via 10.200.0.1。
- 宿主：`veth-ci-h`（10.200.0.1/30）；route `10.199.0.0/16 via 10.200.0.2`（调试/回程面）；DOCKER-USER ×2（`-s`/`-d 10.200.0.0/30`，`shz-ci-gw`）；nat POSTROUTING MASQUERADE（`-s 10.200.0.0/30 -o <wan>`，`shz-ci-gw`）。
- 关键事实（tcpdump 实证）：CI dockerd 在 netns 内自管 MASQUERADE，全部 CI 流量出境源恒为 `10.200.0.2`——宿主规则集一律以 /30 transit 为面，10.199.0.0/16 只作路由面。
- 引导链：`shz-ci-netns`（Wants/After network-online）→ `shz-ci-containerd` → `shz-ci-docker`（Requires 链）。三单元 enabled——disable 问题解除。

## 迁移中发现并修复的缺陷（真机抓错）

1. 宿主残留 `shz-br0`（迁移前 ExecStartPre 遗留，10.199.0.1/16，linkdown 无端口）劫持 10.199.0.0/16 回程 → 全部 CI 回包黑洞（T1/T3 前置失败根因）。修复：删除残留桥＋宿主加 scoped 回程路由。
2. nat 残留无标注规则 `-s 10.199.0.0/16 ! -o shz-br0 -j MASQUERADE`（迁移前年代）。修复：精确删除——终局归一化 diff 的唯一删除项。
3. DOCKER-USER/nat 各存一对 /16＋/18 重复规则（旧脚本两代叠加）。修复：归并为 /16（后随第 5 条再归并为 /30）。
4. 远端已装 setup 脚本仍为 /16 transit 设计（重跑会 clobber netns default route、重插 /18）。修复：重写为 /30 终版，`sh -n`＋幂等重跑收敛实证（IDEMPOTENT-RUN-OK）。
5. 首版宿主规则集以 10.199.0.0/16 为面——tcpdump 实证出境源恒为 10.200.0.2，规则不命中 → 容器/netns→WAN 100% 丢包（T2/W2 根因）。修正为 /30 面（单变量定位：宿主→WAN 通 / netns→WAN 不通 / 双向 tcpdump 见未改写源地址出网）。

## 验证矩阵（218 实机）

| 测试 | 结果 |
|---|---|
| netns→宿主 10.200.0.1 | 0% loss，0.116ms |
| 容器→宿主 10.200.0.1（规则切换前后两次） | 0% loss，0.19–0.29ms |
| 容器→WAN 223.5.5.5 | 0% loss，89.5ms（宿主基线 89.25ms） |
| netns→WAN 223.5.5.5 | 0% loss，89.3ms |
| 容器 TCP 53→223.5.5.5 | TCP53-OK |
| 主 daemon 容器 | 全部 Up/healthy，零触碰 |
| 三表 before vs final（去时间戳/计数器＋排序归一化） | 恰一条差异＝本迁移清除的 CI 自家残留规则；其余零增零删零改 |
| 窗口中段规则移动 | 租户容器启停循环（events：fastgpt-app/milvusStandalone die+start）的顺序重排＋临时增删，窗末自愈归零 |
| 脚本幂等 | setup 重跑收敛，footprint 不变 |

## 工件

- 远端：`/usr/local/sbin/shz-ci-netns-setup|teardown`、`/etc/systemd/system/shz-ci-netns.service`、`/etc/systemd/system/shz-ci-{docker,containerd}.service.d/netns.conf`（NetworkNamespacePath）。
- 本目录：setup/teardown 终版、unit、两个 drop-in、本记录。
- 快照：218 `/root/shz-ci-migration/tables-{before,after,after2,final}.txt`（＋v6 对应文件）。

## 回滚

`/usr/local/sbin/shz-ci-netns-teardown` ＋ `systemctl disable shz-ci-{docker,containerd,netns}` → 宿主零 CI 网络残留。

## 边界与未验证（如实）

- INPUT 方向（宿主本机服务暴露给 CI 容器）未开放，也无需开放（R-a 约束；CI 仅需 egress）。
- 现状 ufw routed=allow；DOCKER-USER 规则是显式意图声明，routed 策略若被管理员改变，CI 规则自持。
- 断电重启全链（netns→containerd→dockerd→容器恢复）未实测——脚本语法、幂等运行、单元依赖链、enable 状态均单独验证；全链 reboot 验证留待下次维护窗口。

## 附录 · push 后只读现状核查（2026-11-27，轮六十二续窗）

- 主 daemon nat DOCKER 链＝**28 条规则完整重建**（DNAT 映射在位，nat MASQUERADE 15 条）——v0.2.41 登记的「链空＝他租户转发预期不通」快照已过期：租户容器（fastgpt/milvus/nexusflow 族）窗口内重启驱动主 dockerd 自愈重建。**owner 无需再专门重启主 docker.service**（该处置项自动消解）。
- 本核查为纯只读（iptables -S / docker ps / systemctl is-active），零变更，符合共享宿主零伤害约束 (b)。
- CI 栈终态复核＝shz-ci-docker/shz-ci-containerd/shz-ci-netns 三 active＋容器 egress 回归 0% 丢包。

## 附录 B · 全链停起臂（Q3 (b) 案，R-a 点名批准＝owner 2026-11-27 批量裁，A31-V-002 外第二例）

`systemctl stop shz-ci-docker shz-ci-containerd shz-ci-dnsmasq shz-ci-netns` → 三 inactive＋netns/veth/规则/路由宿主侧全归零（shz 规则计数=0，route-gone，netns 目录空）＋主 daemon 64 容器持续服务＝**teardown 即宿主零 CI 网络残留实证**。`systemctl start` 链反向起回＝四 active，footprint 精确恢复（DU 2＋nat 1＋route 1），egress/DNS/run 回归全绿。**金标准＝宿主三表归一化 diff 在整轮（停→起→重启→DNS 修复→停→起）后与 pre-dnsfix 基线逐字相等（HOST-UNCHANGED）**。boot 链等价性＝单元依赖边/脚本/传播旗全验；剩余未测面收窄为 PID1 开机时序一项（reboot 维护窗口补）。dnsmasq 单元 Requires netns＝netns 停⇒转发器随停，无僵尸重试。

## 附录 C · 缺陷 #6＝netns 内 DNS 失联（真机抓错，根因＋三案对比）

根因：dockerd/containerd 经 NetworkNamespacePath 只换 netns，读到的仍是宿主 /etc/resolv.conf=systemd-resolved stub 127.0.0.53——该地址在 netns 回环上无监听 ⇒ 一切 registry 解析失败（09:48 `fetch failed: dial tcp lookup ... 127.0.0.53:53 read: connection refused`；netns 内 getent rc=2 判别实证）。三案实测对比（全部真机）：
- **(a) unit BindPaths/BindReadOnlyPaths 挂 resolv.conf**：DNS 通（pull OK）但**给 dockerd/containerd 制造私有 mount ns＝打断 overlay 挂载传播**→容器 rootfs 空（`exec /hello: stat no such file`/`cat not found`），fresh pull 亦复现；`:rshared` 传播旗亦不能补救（两轮实测）⇒ 弃（docker 不能进单元私有 mount ns）。
- **(b) RESOLV_HOST_CONF glibc 环境变量**：netns 内判别＝rc=2 无效（glibc 2.35 不支持）⇒ 弃（负结果在册）。
- **(c) netns 内 dnsmasq 转发器（采纳）**：dnsmasq-base 已在场（零新装软件）；shz-ci-dnsmasq 单元在 netns lo 的 127.0.0.53:53 转发 223.5.5.5/114.114.114.114——**不碰任何 mount ns、不碰宿主配置、不碰宿主防火墙**；netns 默认解析路径原样复活。验证：registry 解析 OK、pull busybox/hello OK、ROOTFS/EGRESS/CONTAINER-DNS 全绿、宿主三表逐字不变。边界：upstream 硬编码两枚公共 DNS 于 unit（配置面性质，改=单元文件一笔）；宿主 resolv.conf 属租户共享面全程未触（零伤害 (b) 保持）。

## 附录 D · 终态单元账（5 shz 单元全 enabled）

shz-ci-netns（oneshot，Wants network-online）← shz-ci-dnsmasq（Requires netns；Before containerd/docker）← shz-ci-containerd ← shz-ci-docker。工件＝本目录 5 文件＋netns.conf×2＋本记录。

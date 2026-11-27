# 迁移目的验收臂 · CI dockerd 重启零扰动（218，2026-11-27 09:44:20 UTC）

## 命题

v0.2.41 事故类＝CI dockerd 起停的生命周期清理例程冲刷宿主共享 netfilter 链（`Deleting nftables rules` 时代，nat DOCKER 链被清 → 他租户转发失效）。迁移后的验收命题＝**CI dockerd 重启对宿主三表零扰动**。

## 执行与读数

- `systemctl restart shz-ci-docker`（09:44:20，Stopping/Started 同秒）；前后宿主 iptables-save 全量快照＋nat DOCKER 链单独快照（`/root/shz-ci-migration/restart-*`）。
- **金标准（nat DOCKER 链）**：27 条 → 28 条，**删除项＝零**；唯一新增 `-A DOCKER ... --dport 38006 -j DNAT`（＝主 daemon 对 fastgpt-app 容器重启的自家登记，docker-proxy pid 959247 实证，事件流 1795772659/2661 die/start 与我的重启同秒＝租户 crash-loop 周期碰撞，非我窗口所致）。
- **三表归一化 diff（去时间戳/计数器＋排序）**：新增 5 条全部＝主 daemon 对租户容器（fastgpt/milvus/nexusflow 族）重启的自管规则（DNAT/ACCEPT/endpoint-DROP ×3）；**删除项＝零；改动项＝零；CI 面规则＝逐字未动（DOCKER-USER×2＋nat MASQ×1 计数复核）**。
- **机制证据（journalctl）**：CI dockerd 启动清理例程照常执行，但目标是 netns 内 nft 表——`Deleting nftables IPv4/IPv6 rules error="exit status 1" output="... No such file or directory / delete table ip docker-bridges"`＝其视野内已不存在宿主链，事故路径结构性封死。
- 回归：CI daemon 29.4.0 active；容器 egress 223.5.5.5 0% 丢包 91.7ms；主 daemon 全程不受影响（events 流只含租户自身）。

## 判定（执行侧 E1，等级如实）

CI dockerd 起停的宿主 netfilter 扰动＝**机制级不可能**（netns 视野隔离）＋**实测级零删除**（金标准窗口对读）。事故类关闭。边界：本臂未覆盖整机 reboot 引导链（候维护窗口，AM v0.2.42 已登记）；宿主三表新增项的归因基于事件时间线＋规则归属（主 daemon 桥网面）＝强推断（E2），非逐字节进程归因。

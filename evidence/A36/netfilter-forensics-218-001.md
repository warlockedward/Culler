# 218 netfilter 归属取证纪要（轮六十一·owner 质疑→定案）

日期 2026-11-27。触发＝owner 贴出 `iptables -t nat -L`（PREROUTING 4 条 DNAT：38001/38022/38023/38040→192.168.110.64 系＋大排 MASQUERADE），问是否我所为、并令"不得弄坏 218 上其他 docker 应用"。全部只读取证，零变更。

## 1. 定案：那些 DNAT/MASQUERADE 不是本 campaign 产生的

- **命令域**：本会话（含登记件）在 218 执行过的防火墙面动作全集＝轮五十六 filter-INPUT 一条临时 5 元组 ACCEPT＋同文删除（前后 `iptables -S INPUT` diff 空，AM v0.2.32/V-002 在册）＋轮六十经产品码 `ufw allow`/`ufw delete` 各一（窗始窗终 `ufw status numbered` 逐字等，evidence/A36/218-acceptance/ufw-before.txt 在册）。**`-t nat` 命令＝零条**。
- **归属实证**：38001/38022/38040 命中本机他服务配置（`/root/.warp/remote-server/…`、`/root/ycj/core-api/…`）；我窗始 ufw 基线中 38000:38040/38023/7434 放行**先于我一切动作存在**。MASQUERADE 排＝两个 dockerd 对 docker0/br-*/shz-br0 的自管标准规则；`systemctl cat shz-ci-docker.service` 全文无 nat 操作。
- **时间线**：主 docker.service 重启＝今日 **05:38:38（systemd/journal 记录）**，与本会话无涉（我方仅启停过专用栈：03:37 停／03:44-03:47／07:06-07:21，与主 daemon 重启互错；主 dockerd 05:38 起连续存活）。

## 2. owner 症状（"docker 重启后一会规则就没了、转发失效"）的机制画像

- **主嫌**：主 daemon 上他租户三容器 **crash-loop**（journal：restartCount 115→118，exit 1/134，3-7 秒/轮）——带 publish 端口的容器每死一次 dockerd 即删其 DNAT，转发呈"闪现"。此为他服务自身故障，呈报处置权 owner。
- **次因（经典形）**：手工 PREROUTING DNAT 在 dockerd 重启后条目存活，但 dockerd 重建 FORWARD 默认 DROP 且不为手工转发补 ACCEPT ⇒"规则在、流量死"。持久化建议（iptables-persistent 或 oneshot 单元 After=docker.service＋DOCKER-USER 链内 FORWARD ACCEPT——该链设计即为用户规则，重启存活）＝呈报，不代执行。

## 3. 纪律吸收（已常设化 DEV-BASELINE §2）

共享宿主零伤害约束 (a)-(d)：动作域显式化／**三表全快照窗口纪律**（本会话此前=计数级对读，规则级为升级项，自下个窗口起执行）／他租户故障止于呈报。既有零触碰实证（计数窗始窗终等）如实保留为下限证据。

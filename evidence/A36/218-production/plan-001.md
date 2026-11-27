# 218 生产部署轮预案（Q4 批准案的执行面 · 候 owner 逐条批准后分步执行）

依据＝owner 2026-11-27 批量裁 Q4：(a) `culler gateway allow`＝Linux 生产路径答案确认；218 生产部署轮批准发起。本件＝预案，不含授权自扩：每步执行前对照本文件对应条目＋零伤害快照纪律。

## §1 该轮承载的三组验收（为何此轮是共同验收场）

1. **A36-F1 字面**：装后＋代理本体在场——provider proxy 绑 127.0.0.1×`culler gateway allow` scoped 许可×真实 agent 容器 dial `host.docker.internal:PORT`。
2. **A31-F1③ 组合命题**：生产形态（部署面 provision）下容器→宿主代理可达＝此前 UNKNOWN 的组合半环，本轮实测。
3. **A09/A04 amd64 半面**：mc-prod 镜像 GOARCH=arm64 硬编码＋install.sh Apple-silicon 闸＝UNVERIFIED 残留，Linux x86_64 生产部署即其真机验收。

## §2 前置闸（执行日逐条核对，任一不满足＝停在该条呈报）

| # | 前置 | 现状 | 处置 |
|---|---|---|---|
| P1 | **磁盘余量** | `/` 96% 满，仅 **18G 可用**（218 实测 09:51） | 部署需 mc-prod amd64（~6G）＋project/e2e 镜像＋spine 增长；**执行前 `df` 闸 ≥25G**，不足则先呈报清理窗（租户卷不代碰） |
| P2 | **A04 构建面** | GOARCH=arm64 硬编码＋install.sh Apple 闸＝Linux amd64 产物不存在 | 部署轮第一步＝A04 票域实施（交叉/本机 amd64 构建 mc+镜像）；无授权不自开＝**本预案批准即含 P2 的 A04 最小实施授权候您点名** |
| P3 | schema identity | 三文件 SHA 与 dev 码匹配 | 部署前 `mc` 自报 identity 对读；不匹配＝停（无转换器不变量） |
| P4 | 主 daemon 只读 | 租户 64 容器活跃；crash-loop 族在册 | 产品容器跑**主 docker daemon**（Culler 标准用法，无双 daemon 问题）；netfilter 三表窗始/窗末快照纪律照旧；`culler gateway allow`＝产品码 ufw 路径（非手写 iptables） |
| P5 | R-a | INPUT 链变更＝`gateway allow` 的 ufw 规则 | 本预案批准即预授权该**产品码生成**的 scoped 许可（规则带注释可点名撤销）；撤销窗同轮内跑 F-2 形负控 |

## §3 步骤序列（每步：动作→读数→入册）

1. **构建**：amd64 host `mc`＋`mc-prod` 镜像（A04 域最小改动＝去 GOARCH 硬编码/Apple 闸改能力探测——实施按 A04 票面与 dev 基线）。
2. **安装**：`sh scripts/install-source.sh` 于 218（source-production tier；launchd 不适用 Linux＝systemd 路径，A03 EVIDENCED 件覆盖）。
3. **初始化**：spine 新建（空脊）；onboarding/setup agent 跑 Project 环境；配置 harness 凭据（O1/A02 已闭面复用）。
4. **网关**：provider proxy 默认绑 127.0.0.1；`culler gateway allow --port <实测临时口>`（R-a/P5）；窗始三表快照→ufw 链增量仅 1 条注释规则实证。
5. **验收臂**：真实 agent 容器（生产同形旗 `--add-host host.docker.internal:host-gateway`）dial→OK；`gateway revoke`→timeout 负控；ufw diff＝窗始逐字等＝零残留。
6. **收口**：TM §7.1 记录×3（A36-V-003/A31-F1③/A09-amd64）；三表终快照；dev tip/账登记；**保留部署轮运行**（218 成为常设生产环境＝轮五十五 o3 指定方向）或 `dev-reset` 回滚＝二选一候裁。
7. 全程：任何他租户异常＝呈报不代修（零伤害 (d)）。

## §4 回滚

- 安装层：`./uninstall.sh --dry-run`→确认后执行（只碰 Culler 自有面）；spine 数据＝保留候裁。
- 网关层：`culler gateway revoke`（产品码，秒级）；ufw 链 diff 复原实证。
- 镜像/构建层：amd64 产物不入 arm64 发布线（A04 改动走 blcheck＋全闸）。

## §5 边界（如实）

- 本预案＝Linux 生产形态**首次真机部署**：A03 持久监管面（systemd install/enable/开机）与 A25 生命周期的 amd64 半面将一并暴露为验收对象，非预先承诺通过。
- crash-loop 租户族（fastgpt/milvus）与部署轮共享磁盘与 DNS 出口：P1 闸不满足则本窗口不开。
- 执行节奏＝逐条批准对应条目后动；预案批准≠P2 自扩。

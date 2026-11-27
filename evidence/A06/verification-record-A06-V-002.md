# 验证记录 A06-V-002（218 真 docker 臂复测——版本/拓扑变更触发，TM §7.1 作用域条款）

日期 2026-11-27（轮五十六）。执行者＝host agent（ZCode）；授权域＝轮三十三 Q4 有界扩权（同 V-001：scratch MC_HOME＋XDG 隔离、专用 docker 栈、零残留前后差；禁区=他人项目状态/主 daemon/账户面——全程未触）。

## 0. 复测理由（不冒充首证）

V-001（2026-11-23，drill 7cbcb71b，release 41ce8b0，schema 5c56d313＝B 线旧基线）之 PASS 按 TM §7.1 P1-2 仅对其版本×拓扑×配置组合有效。此后 spine 走过 S5-S12（现行 identity 4786047e…8be1）、二进制走过 B10/A25b 系代码面、聚合面出现真链行（B10 稳态入链）——组合已变⇒**复测义务成立**（A06 行原注"218 真 docker 臂演练＝Q4 授权域内待执行"即指对本形态的重验）。本件＝该复测的执行记录；V-001 原件不覆盖、不改写。

## 1. 执行面（真路径＝runRuntimeDocker，非 fake run）

- 硬件/OS/拓扑＝O3 218（host=F1104，Linux 5.15.0-94 x86_64，docker server 29.4.0；专用栈 systemd 单元 `shz-ci-docker.service` 经 systemd 启停（V-001 时代为手工形态），socket /run/shz-ci-docker.sock；主 daemon 全程未触）。
- 被测 mc＝linux/amd64 `5ca5dcae6326d501448381c0b688ec37258ba95c9eed31363d181b6c50899c7e`（构建自 dev tip 树 c41b3b2＝含 B10 中央写链之代码面）；播种驱动＝`37bebce9…bfbd7`（B06 lane 件复用＝真 OnboardSpine/真 AppendActivity 建脊）。
- 镜像＝专用栈既有 `mc-prod:latest`；scratch 域＝/root/a06-drill（home/state/lane 全 0700）；卷名＝`mc-spine-419755090692`（deployment.RuntimeNames 确定性派生，非手造）。
- 脊＝真 spine（schema **4786047e…8be1＝S12 现行 identity**，uuid cb9a147f…）＋**22 行真实 activity**（2 身份事件＋20 行经 B10 `AppendActivity` 链写入＝演练聚合抽样首次踩在稳态链上，V-001 时代此路不存在）。

## 2. 两次演练（同域连发，密钥环幂等探针）

- **run-1（bf935019…）**：空聚合脊⇒verdict PASS／integrity ok／tables 92／tasks 0／activity 0。副证＝**生产秘密边界真实开火**：home 0755 时 hostSecretStore owner-only 0700 闸以 exit 1 domain-rejection 拒绝（修 0700 后过）＝A02 边界在 A06 消费路径上的直接证据（非演练脚本模拟）。
- **run-2（43b6ccbf…，record.json 原件＝本目录 218-drill-43b6ccbf/）**：播种后脊⇒**PASS**／integrity ok／schema+uuid 相符／**tasks 0／activity 22（聚合根抽样非平凡）**；SHB1 备份 1 306 721B sha256 fac93ba9…；**key_fingerprint 67b4a99e…＋reference 6aad6c82… 跨两次演练逐字稳定＝密钥环幂等真机证**（同 home 重入不重签盐、不漂移指纹）。
- 完整链条＝StageSpineCopy（sidecars 随行＋settled 门）→AES-256-GCM→`backups/encrypted/`→解密→快照只读门→quick_check→恒等（schema/uuid/tables=92）→聚合抽样→record 0600 落盘。

## 3. 零残留前后差（授权域判据，实读）

- 专用栈：容器 0（全 --rm）；`mc-spine-419755090692` 删除后卷表回到既有 2（culler-e2e-runtime-2733731／mc-e2e-spine-1811380908＝演练前既有，未动）；`shz-ci-docker.service` 复原 **inactive**（侦察态＝dead，同值归还）。
- **主 daemon 三项计数前后逐字相等：containers 62／volumes 173／images 185**（基线取于启动专用栈之前）。仪器小误如实登记：首次读主面时 `unset DOCKER_HOST` 遗漏致三行 connect 报错（＝读错栈非写错物），显式重跑取真值。
- scratch `/root/a06-drill` 整体删除＋远端 ls 复核无 a06 痕迹；备份件/密钥环/record 全随域消亡（演练域语义；密钥材料不留存他处）。

## 4. 字段核对（防"不删列"口惠）

record.json 实测 **16 字段＝TM §7.1 全 15 列＋票面具名 `z5_replication`=NOT_APPLICABLE 预留，零缺列**。A06 票面文本"17 字段"系轮三十三登记的计数偏差——权威口径＝TM §7.1 的 15 列清单逐列在位（本件按内容与权威文档闭合"全字段"义务，偏差如实登记，票面历史字节不改）。reviewed_by 按 §7.1 显式记录兼任（AI 执行＋AI 记录＋人审候办＝不制造双人假象；AI 非 Acceptance Authority）。

## 5. 结论与边界（不跨级）

- **A06 票面 F1（"三层演练证据 DC §7.7"）判据闭合**：三层结构（record＋evidence_digest＋execution 批次）＋真 docker 臂（本件 V-002 于现行形态）⇒A06 由 IN_PROGRESS→**EVIDENCED**（按票面 F1 字面义务判定，§1 纪律：EVIDENCED≠IMPLEMENTATION_VERIFIED）。
- **具名残留原样保持、不闭合不冒充**：DC §7.4 双事件轮换协议／resident 周期自动演练集成／Z5 异地通道实现＝各自候裁面；V-001 的"篡改既有备份件无真机回放路径"边界在本动词面依旧成立（机制级证据仍归本机单测 TestSHB1RoundTripAndTamperRefusal）。
- 本件＝agent E1 自记，非独立审查；PASS 作用域＝tested_version(5ca5dcae/c41b3b2)×environment(F1104 专用栈 mc-prod:latest)×attack_scenario(本记录 §2) 组合，再变更再复测（钩子与 §6 复测同族）。
- 与 B06 钩子关系：两 lane 各钉各的复测义务（B06＝G 闭合后包络 diff；本件＝版本/拓扑变更即复测），互不替代。
- 证据件差异（对 V-001 如实注明）：V-001 曾把加密备份件原件（1 MB .enc）拷出留档；本件**未导出备份原件**——scratch 域按零残留判据整体删除，密文件随之消亡（域内无留存价值：密钥环同域，导出反增密钥邻接面）。有效证据＝record.json 原件＋本文件所载全部 sha256（备份件 fac93ba9…/解密脊 06ae13ab…）＝内容摘要级复核链在位，原件级留档按 V-001 先例为可选项而非义务（§7.7 字段无"保留密文原件"要求）。

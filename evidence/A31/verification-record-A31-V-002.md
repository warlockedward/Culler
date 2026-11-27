# 验证记录 A31-V-002（218 真引擎端口实连半环——受控双向证明＋生产形态缺口具名）

日期 2026-11-27（轮五十六续）。执行者＝host agent；授权域＝O3 定向机既有域（专用栈/systemd 启停/scratch 探针件，全程零生产代码改动、零持久配置变更）。TM §7.1 记录制；本文件＝验证叙事，机器事实以下列三份原始件为凭（本目录）。

## 1. 三段证据（单变量链）

- **S-1 解析半环**（前证不重做）＝V-001：`--add-host host.docker.internal:host-gateway` ⇒ `/etc/hosts` 注入 `10.199.0.1 host.docker.internal`；本轮复用同读法再证一次（POSITIVE 运行容器内 hosts 行在案）。
- **S-2 端口实连＝成立，但暴露前置条件**：宿主监听 10.199.0.1:17999（桥内地址，无外溢）＋iptables **临时精确 5 元组** ACCEPT（`-s 10.199.0.0/16 -d 10.199.0.1 -p tcp --dport 17999`，非持久、测后删）⇒容器经**生产同款旗标**拨号 `dial=OK rtt=1.08ms echo=pong`；删规则后同拨号 `i/o timeout`＝**负控复现**。`iptables -S INPUT` 前后 diff 为空（`INPUT_CHAIN_RESTORED_EXACT` 原样打印为证）。⇒ 代码＋映射＋连接通路本身完全成立；**唯一缺项＝宿主对桥子网→宿主端口的 INPUT 许可**。
- **S-3 失败定位全程**（仪器诚实）：首跑 set -e 在 dial FAIL 处中止泄漏监听进程⇒二跑 0.0.0.0 bind 撞 EADDRINUSE（"address already in use"）而误读为防火墙证据——已纠正：route_localnet 开关试验（前后=0 复原，非因）＋宿主本机自连 dial=OK（监听存活证）＋ufw 规则清单零桥条目（阻断面=INPUT -P DROP 默认）。教训：仪器自误不掩真因，逐步单变量。

## 2. 具名生产缺口（G-A31-FW，呈报不私修）

生产 `provider-proxy.ts:435` 默认 bind=127.0.0.1（配置可显式 IP），而 `config.go:403-406` 注释承诺"显式 IP bind 让 agent 容器经 host gateway 可达（Linux）"。218 实证：**该承诺在 ufw INPUT-DROP 策略的宿主上不成立**——即便容器解析正确、代理绑桥地址，宿主 INPUT 仍拦桥→host 流量；部署面（install/setup/launchd-systemd 域）**无任何为代理端口开桥侧许可的 provision**。Linux 真机全栈 provider 往返在现行形态下不可达。候选处置（各有权域，全部候 owner 裁）：
- (a) 代理 bind 选桥网关地址＋安装面 provision 一条 scoped ufw/iptables 许可（暴露面前移，需威胁评审：桥子网=自家 agent 容器，近似 loopback 信任域）；
- (b) Linux 运行声明降级（宿主代理仅 Mac/无硬防火墙宿主支持＝文档面，零代码）；
- (c) 容器侧代理（每 agent 容器内起 proxy，绕开 host 往返＝架构变体，触 D-IG/资源面，最重）。
先例形制＝A03-G4→A26（发现→立票归 owner，执行侧不私修）。**A06/B06 教训延续：修复不在 A31 票面正清单，A31 维持 IN_PROGRESS 不虚标。**

## 3. §7.1 字段（本验证的机器事实在案）

- verification_id: A31-V-002 ｜ execution_id: 218-2026-11-27-a31e ｜ supersedes: 空（V-001 解析半环仍有效，本件补连接半环前置条件面）
- tested_version: mc 面零改动；探针 `95d1c04e019ad33b8a8e574dfbec6c68fe11383d25d73e3764dc8b5bb22f2e8f`（源码＝dev 4376333 树 mc/hostgateway/main.go，本提交件）；注入码＝effects.ts:734/:1543（轮十六已并主干，行为测 effects.test.ts:936/:956/:2304）
- environment: host=F1104 Linux 5.15 x86_64；专用栈 shz-ci-docker.service（systemd 单元，测后复原 inactive）；shz-br0@10.199.0.1；alpine:3.22；ufw=active INPUT-DROP 默认（既有宿主策略，未持久改动）
- evidence_digest: 本文件＋三段 stdout 原始记录（POSITIVE dial=OK 1.08ms／NEGATIVE timeout 复现／INPUT_CHAIN_RESTORED_EXACT）；原始转录入本目录 a31-connect-proof-transcript.txt
- verdict: **PARTIAL-PROVEN**（半环 1 解析＝PASS；半环 2 实连＝**PASS-under-precondition**（宿主 INPUT 许可）；生产形态端到端＝**NOT-PROVEN→具名缺口 G-A31-FW**）——verdict 词汇表内 PARTIAL 无此枚举：按 TM 语义拆分记录＝各子命题各自 PASS/UNKNOWN，组合命题 UNKNOWN；UNKNOWN≠PASS，A31 状态不升。
- reviewed_by: 执行与记录＝实施 agent（兼任显式）；人审＝候
- z5_replication: NOT_APPLICABLE

## 4. 拆除与残留差（授权域判据，实读）

监听进程 kill（`pgrep -x a31-probe` 空）；探针与临时文件 `/tmp/a31*` 全删（`ls` 空）；iptables 链＝前后 diff 空；服务复原 inactive；**主 daemon containers/volumes/images＝62/173/185 与基线逐字等**（unset DOCKER_HOST 后实读）。一处首报假警如实登记：`pgrep -f a31-probe` 命中 ssh 自身命令行＝模式误报，`pgrep -x` 复核为空。

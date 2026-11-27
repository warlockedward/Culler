# 验证记录 A36-V-001（218 验收臂·R-a 新规首个适用轮，TM §7.1）

日期 2026-11-27（轮六十）。执行者＝host agent；批准链＝轮五十九 owner「按顺序执行」→本轮请示包→owner「**批准 F-1/F-2**」＝**R-a 常设新规首个完整适用例**（事前逐条请示→批准→执行→全痕）。

## §7.1 字段

- verification_id: A36-V-001
- target_component: provider gateway provision surface (allow→path→scoped→revoke→inert→zero-residue)
- tested_version: mc=`b6b12515d56b36dc…48b95`（dev f90e2ec 树交叉编译；双端 sha 对读在册）；probe=`95d1c04e…f2e8f`（网络路径监听替身）
- environment: O3 218（F1104, Linux 5.15 x86_64）；专用栈 systemd 起停（窗终回 inactive＝窗前态）；桥 shz-br0@10.199.0.0/16；ufw active；alpine:3.22
- attack_scenario: 本文件＋同目录原始件（allow-result/rule-after/ufw-diff/ufw-before/listen.log）；可复现＝同载荷按序跑 §2
- expected_result: allow 仅创建 1 条带标 scoped 规则；容器经生产同形旗达 bind 端口；revoke 仅删带标；复拨必超时；链回字节等前态
- actual_result: **PASS 全链**——F-1 产品码建规则 `[16] 10.199.0.1/tcp ALLOW IN 10.199.0.0/16/tcp # shz-provider-gateway`（diff 仅此 1 生，其余为编号位移）；容器（10.199.0.2，`--add-host host.docker.internal:host-gateway` 生产同形）dial=OK rtt=1.12ms pong；F-2 revoke `removed 1 tagged rule(s); no untagged rule was touched`；复拨 i/o timeout＝惰化复证；**`diff ufw-before 现态` 空＝UFW 链字节级回前态**；监听器清（pgrep -x 空）；主 daemon 53/173 与窗始逐字等（零触碰实证）
- evidence_ref: evidence/A36/218-acceptance/（本目录）
- evidence_digest: 原始件 sha256 随登记轮件
- test_script_digest: 产品动词两枚（gateway-allow/gateway-revoke，零手写 iptables）＋probe dial/listen
- execution_id: 2026-11-27T04-3xZ-r60
- supersedes_verification_id: （空；A31-V-002 系手工 iptables 单变量实验，本件＝产品码全臂，两案并存）
- verdict: **PASS**（作用域＝tested_version×environment 组合）
- reviewed_by: 执行与记录＝实施 agent（兼任显式）；F-1/F-2 批准与最终接受＝owner（本会话在册）
- verified_at: 2026-11-27（窗口尾）
- z5_replication: NOT_APPLICABLE

## 1. R-a 新规执行史（首案）

请示包＝规则文本/作用域/存续期/撤销路径逐条事前呈报；owner 批准后才触链；全程产品码执行（对比轮五十六的手工 iptables——新规把"临时实验"与"产品功能"正确分离）。真机顺带抓出两枚本地不可见缺陷（PATH LookPath／桥名启发式）＝§9 登记＋`f90e2ec` 修复——"验收臂必须真机"票面理由的正面实证。

## 2. 臂界（诚实声明，不跨称）

本臂证明的是**包路径层端到端**：规则对到达 bind 地址的包生杀（开→通、关→断），监听器＝bind 地址上的 TCP 替身。规则作用于（地址,包）非（进程），故真 provider-proxy 进程占用同一 bind 地址时路径语义等价；代理应用层行为（401/路由/SecretFilter）归 resident/PS 测试面，**本臂不为其背书**。容器→真代理进程的完整往返（含 resident 全栈在跑）随 218 部署轮合流（A24/A14 同窗），A31 F1③ 之"A32 CI 合流"句维持在册。

## 3. 判定输入（供 owner 验收）

A36 票面 F1 各件：装后真往返 PASS（按 §2 臂界读法）✓／scoped-only 断言 ✓（产品码推导＋物理网卡拒回归钉）／撤销零残留 ✓（字节等前态）／全道绿＝登记轮时终验。执行侧建议：A36 CODED→EVIDENCED（臂界在册）；A31 维持 IN_PROGRESS（其 F1③ 完整句含 A32 CI 合流）。

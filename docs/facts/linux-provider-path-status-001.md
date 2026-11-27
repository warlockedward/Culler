# Linux provider 路径现状 · linux-provider-path-status-001（Q2-(b) 过渡事实卡）

日期 2026-11-27（轮五十七）。所有者裁定 G-A31-FW 处置＝(a) 修复为主（立票 A36）＋(b) 过渡登记（本卡）。**A36 落地前，本卡＝Linux 形态 provider 路径的权威陈述；落地后本卡按 A36 F1 更新收口。**

## 事实（全部 E1 实读/实测）

1. 产品代码已备好映射与消费链：容器 birth 注入 `--add-host host.docker.internal:host-gateway`（effects.ts:737/:1546，行为测 effects.test.ts:936/:956/:2304）；容器侧消费 URL 恒 `https://host.docker.internal:<port>`（provider-proxy.ts:785，与 bind 无关）。
2. 宿主监听默认＝**127.0.0.1**（provider-proxy bind 选择面；A05 提供配置项可显式 IP）。
3. 硬化 Linux 宿主（ufw INPUT-DROP 形态，O3 218＝活体样本）：**桥子网→宿主端口的入站流量默认被拦**——容器解析正确、代理即便绑桥地址，连接仍超时（A31-V-002 单变量三段：临时精确许可下 dial=OK 1.08ms／撤许可 timeout 复现／许可撤除后 INPUT 链 diff 空）。
4. 部署面（install.sh/setup/监督单元）**不存在任何防火墙许可 provision**（检索无载，A31 recon 在册）。
5. **结论（运行声明）**：在默认 bind＋默认宿主防火墙策略的 Linux 主机上，**agent 容器→宿主 provider 代理路径不可用**；当前 Linux 形态的 provider 访问需操作者自备两项：代理显式 bind 到桥网关地址（或宿主可达地址）**且** 授予桥子网→该端口的 INPUT 许可。macOS 形态不受本卡影响（Docker Desktop/VPNKit 使 host.docker.internal 对 loopback 绑定可达，真证史＝A14 系各轮）。
6. 该结论**不降级任何安全机制**：代理 per-launch token 闸、egress 亲验、C02 目的地面约束全部在位——本卡陈述的是"到不了"，不是"不设防"。

## 与在册面的关系

- A31 F1③（Linux 端到端独立成立）：本卡＝其过渡真值；A36（bind 形态＋provision＋验收臂）闭合后 A31 随 218 窗口合流取证。
- A14 V-1b/c/4/5/6/7 真采样格、A24 post 矩阵：依赖 218 专窗——若这些轮次含 provider 往返面，需先读本卡（Linux 臂在 A36 前的预期结果＝本卡第 5 条）。

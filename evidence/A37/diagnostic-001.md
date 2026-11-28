# A37 诊断记录 · provider 路径最后一公里（轮六十四，2026-11-28）

授权＝owner「Q1(a)＋Q2 各立票」。范围＝只读诊断（curl/一次性容器，零监听、零防火墙、零常驻）。结论先行：

## 根因（E1 全部实测）

**产品 acceptance 失败 ≠ codex↔new-api 协议不兼容（该假设被证伪），＝ provider-proxy 默认绑 host loopback，agent 容器按构造不可达。**

| 探针 | 结果 |
|---|---|
| P1 `/v1/models`（key 经 credential-read→0600 临时头文件，用毕 shred） | 200，Qwen3.8-27B 在列 |
| P2 `/v1/chat/completions` | 200 正常补全 |
| P3 `/v1/responses`（产品强制 `wire_api="responses"`，provider-access.ts:74） | **200 正常 response 对象**（vllm-0.34 全支持） |
| A2 完整 adapter 旗标 codex exec（--network host 容器，直连 new-api） | **`turn.completed`＋agent_message "PONG"＋usage{in:8729,out:28}**＝真实 agent 回合成立 |
| B1 桥网容器 `--add-host host.docker.internal:host-gateway` → 已发布端口 38011 | 200（DNAT 路径活） |
| B2 同容器 → host **仅 loopback** 端口 | `B2-DEAD-AS-EXPECTED`＝**产品代理形状对 agent 不可达的直接复现** |

## 链条闭合

r15 "adapter produced no output" ＝ codex 起来→拨 provider-proxy（127.0.0.1 绑定）→容器视角不可达→无事件流退出。此前 A35/A36/A31 全部真机证据与此一致（netns/宿主路由/INPUT 语义互相印证）。A2 证明代理以外的每一环（二进制/协议/模型/凭据）都通。

## 解法＝A36 票面既有设计（不新建能力）

① proxy bind 支持桥网关地址（A36 票面①，`surfaces.provider_proxy.bind` 既有配置项）＋② scoped gateway allow（产品码 `shz gateway allow`，R-a：逐字呈报后执行）。生产 provision 完成后，acceptance/双臂（A36-F1/A31-F1③/C11-e2e 端到端格）**预计同一路径全闭**——这是推断（E2），待 provision+激活一次实测升 E1。

## 操作卫生

key 只经 0600 临时文件→curl 头文件/codex env，全程 sed 脱敏输出，用毕 shred；两轮探针均自证（"secrets swept"）。新-api 401/200 行为正常，租户无感知（探针仅 loopback/自有桥网内）。

## 状态判定

- **C11**＝三切片代码在产＋配置链/录入/tick 真机证；端到端真实回合经 A2 证实"代理外全通"，代理内一格＝待 A36 provision 后 e2e（票面 §5-③ 明写挂部署轮，现部署就绪只差 provision 授权）。
- **A35**＝① ②（双向）真机证实；③ wiki export 未触及（部署未跑到 wiki 导出路径）＝票保持 IN_PROGRESS 直至③有真机判决或降级声明。
- 下一步候 owner＝**批准 A36 provision 执行**（bind 桥网＋gateway allow 逐字呈报＋重跑 acceptance/激活/双臂）。

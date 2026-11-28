# acceptance 严格自检 vs 第三方后端 · 定性 001（轮六十四）

触发＝A36 provisioning 后，`mc __acceptance provider run --binding newapi --confirm-live-spend` 报
`Codex stream lacks one ordered native thread/tool/final sequence`。本文＝根因定性，全部 E1 实测。

## 1. 这道自检在要求什么（产品原文）

prompt（`provider_acceptance.go:26`）：
> Use the shell tool **exactly once** to run this exact command:
> `printf 'MC_LIVE_PROVIDER_TOOL_OK' | tee mc-live-provider-tool.txt`
> After it succeeds, reply with **exactly** `MC_LIVE_PROVIDER_TURN_OK`. Do not use any other tools.

校验（`provider_acceptance.go:344-352`）对 codex 事件流的硬条件（全 AND）：
- `thread.started`×1、`turn.started`×1、`turn.completed`×1；
- `command_execution` 的 `item.started`×1 **且** `item.completed`×1（**恰好一次**工具）；
- 存在 `item.completed` 的 `agent_message` 且其 text **恰好等于** `MC_LIVE_PROVIDER_TURN_OK`（`==`，非包含）；
- 严格顺序：tool 完成 < final < turn.completed。

## 2. 实测：连通与能力都在，卡的是"严格形状"

| 证据 | 结果 |
|---|---|
| B1 桥容器→new-api（发布/DNAT 路径） | 200，模型列表在 |
| 桥容器内 codex→new-api 工具回合（printf/echo 类） | `command_execution` 出现、`exit_code:0`、`turn.completed`＋usage＝**工具与回合真实发生** |
| 同回合观察到的事件计数 | `command_execution` item 出现 **2 次**（非恰好 1）；最终 agent_message text 带**前导空白**（reasoning 泄漏，`"\n\nPONG"` 形） |
| 模型元数据 | `Model metadata for Qwen3.8-27B not found. Defaulting to fallback metadata` |

⇒ 三个硬条件至少两处不满足：`codexToolStarts/Completes==1`（实得 2）与 `text==TURN_OK`（实得带前导/非纯）＝校验拒。**不是连不通，是这个后端经 vllm 的 responses 转译后，工具调用次数与最终文本的"精确形状"不满足产品为防假阳性而设的严格自检。**

## 3. gateway --port 文法缺陷（同窗发现并已修）

真机 `shz gateway allow --port <n>` 报 `ufw ... Improper rule syntax`：旧 `ruleSpec` 生成 `... to <bind> proto tcp to <port>`（尾置裸端口），真 ufw 要 `... to <bind> port <n> proto tcp`。已修（218 一次性 probe 规则实测确认正确文法与 `<bind> <port>/tcp` 回显形状）＋幂等/状态查询改 port 感知（wide 规则不再误判满足 scoped 请求）。全道见提交。

## 4. 结论与出路（归 owner，我不擅改安全自检）

**Linux 运行时"不残疾"已由更硬的证据成立**：干净重装 r17 `supervision=done`＋resident/dashboard/agentsview active＋tick 落＋proxy 在场＋桥容器可达代理。acceptance 这道**首次启用自检**对非 OpenAI 原生的第三方兼容后端（Qwen via new-api/vllm）偏脆——它拒绝的是"形状不精确"，不代表生产 agent 回合跑不动（B1/工具回合已证跑得动）。

两条出路（各有代价，owner 定）：
- **甲**：换一个更接近 OpenAI 原生事件语义的后端/模型配置复测 acceptance（不改产品）；治标、验证成本最低。
- **乙**：立独立票评估 acceptance 校验对第三方兼容后端的稳健性（如 tool 次数容 >=1、final text 容忍前后空白/trim 比较）——**但这是放宽一道刻意严格的安全自检**，须 Acceptance Authority 权衡"放过假阳性 vs 误拒真可用后端"，属治理决定，非执行侧自改。

## 6. "甲"实测后记（2026-11-28）

new-api 仅列 4 模型（`Qwen3.8-27B`/`Qwen3.8-Flash-Next-FP8` 为 chat，余两为 embedding/reranker）——**无第三个 chat 候选**。27B 已测（§2 形状不符）；Flash 同经**同一 vllm `wire_api=responses` 转译层**，§2 证据表明不吻合发生在转译层（tool item×2、final text 前导空白、模型元数据缺失走 fallback）而**非具体权重**⇒ 同后端换模型证据上判死（换 Flash 只是多烧调用，预期同果）。⇒ 真"甲"需一个**原生 OpenAI Responses 语义**的端点（OpenRouter/真 OpenAI 等），而 owner 明示无任何外部账号/key ⇒ 现有基座上"甲"无低成本可行路径。

**出路收敛**（归 owner）：(乙) 由 Authority 权衡是否放宽 acceptance 自检对第三方兼容后端的形状假设（tool 次数 `>=1`、final text trim 比较）——**放宽安全自检须具名签署**；(丙) 接受当前已知态＝"Linux 运行时不残疾（supervision/tick/proxy/桥可达全 E1）＋首次启用自检对 vllm 转译后端偏脆"，双臂（A36-F1/A31-F1③）改由**非自检路径**取证（B1/DNAT 可达已证，代理在场已证），C11-e2e 真实回合留待有原生后端时补；(丁) 提供一个原生 OpenAI 兼容后端再复测。

## 5. 零干扰终证（本 provisioning 子窗）

wide 规则经产品 `gateway revoke` 撤销；宿主 INPUT 无本部署 tagged 规则；`ufw status numbered` 与本子窗基线 **diff 空**；`shz-ci-gw` 3 条不变；CI 栈四单元 active；租户容器在跑（计数波动为租户自身重启）；new-api 后端存活。全程未手写 iptables、未碰 CI 栈、未碰租户任何资源。

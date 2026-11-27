# LoopForge 模型/凭据接入对照 · loopforge-provider-model-001

日期 2026-11-27（轮六十三）。触发＝218 生产部署轮停在安装器 runtime-auth 闸（`codex executable not found`），owner 问"必须 Codex/Claude 才能用吗、LoopForge 怎么处理的、是否还有其他形态"。镜像钉＝`be6918d`（v0.4.3 预备，距台账上钉零增量）。**性质＝只读源码研究（E1＝代码存在），未运行 LoopForge（其运行时行为＝E3 假设）；不构成引入授权、不自动立票、不改任何在册票状态。**

## 1. 两条根本不同的"模型接入"路线

| 维度 | Culler（我们） | LoopForge |
|---|---|---|
| agent 循环 | **自研内核**（harness 内部适配器） | **不写内核**，包第三方 coding-agent CLI |
| "provider"含义 | 直连的推理端点＋其凭据 | 一个**托管 CLI 二进制**（自带模型后端与登录） |
| 凭据持有 | provider-proxy 持凭据连 `inference_url` | 各 CLI 自己连自己后端；平台注入其 API-key env |
| 模型宽度来源 | 实现几个 harness/端点 | 包几家现成 CLI |

## 2. LoopForge 的 provider 枚举（`internal/managedcli/providers.go:31-77`，E1）

`providerSpecs` 九家：`agy`、`cursor`(cursor-agent)、`dsh`(DeepSeek `@deepseek-ai/dsh`)、`kimi`(Moonshot `@moonshot-ai/kimi-code`)、`opencode`(opencode-ai)、`pi`(`@earendil-works/pi-coding-agent`)、`codex`(`@openai/codex`)、`codebuddy`(腾讯 `@tencent-ai/codebuddy-code`)、`claude-code`(`@anthropic-ai/claude-code`)。经 npm 托管安装，allowlist＝`AGENTFLOW_MANAGED_CLIS`（`providers.go:20,74-97`），默认只装 claude-code/codebuddy/codex。每家 `--model` 等旗标各异；LoopForge 管自动更新/超时/观测，不碰模型端点本身。

## 3. LoopForge 的凭据分层（`internal/domain/project_credential.go:11-75`，E1）

- **项目凭据**四 kind：`username-password` / `token` / `aksk` / `custom`＝**运行时注入的命名环境变量**（≤64KiB，`ProjectCredentialEnvLimit`），供 agent 访问被开发系统的外部资源（DB/Git/云 AK-SK）。
- **模型/安全关键 env＝平台保留**：`projectCredentialReservedPrefixes`（:56-61）黑名单封 `ANTHROPIC_/OPENAI_/GEMINI_/DEEPSEEK_/KIMI_/CURSOR_/CODEBUDDY_/OPENCODE_/MOONSHOT_/GOOGLE_/AGENTFLOW_/LOOPFORGE_/LD_/DYLD_/GIT_CONFIG/GIT_SSH/GH_/OTEL_/ZCODE_`；`reservedNames`（:45-54）另封 `GITHUB_TOKEN/SSH_*/HTTP(S)_PROXY/NODE_OPTIONS/…`。⇒ 那些**闭源 CLI 的 BYOK 通道（OPENAI/ANTHROPIC/GOOGLE/DEEPSEEK/KIMI/MOONSHOT API-key）确实存在**，但属**部署级平台 env**，项目凭据不许占用（防覆盖＝防注入面）。

## 4. 回答 owner 三问

1. **必须 Codex/Claude 才能用吗（Culler 侧）**：不是产品只能两家，是**安装器交互登录那一步只实现了 `codex-oauth`＋`claude-oauth` 两条订阅登录流**（`mc/deployment/runtimeauth_acquire.go:151-264`，明文"never reads API keys"），且各需宿主装官方 CLI。Culler 凭据模型（`mc/config/config.go:36-44`）实有七 kind，catalog 收录 `codex-api-key`(api.openai.com/v1)＋`claude-api-key`(api.anthropic.com)＋`opencode-api-key`(经目录)＝**API-key 免登录 CLI 通道设计存在**，但不经该向导录入（`install-source.sh:178` 拒 `--static-*`；`config.go:692` 拒遗留 `static_env`）。
2. **LoopForge 有无其他形态**：它的"多"＝**多包现成 agent CLI**（9 家，含 Google/DeepSeek/Kimi/Cursor/腾讯＝Culler 无），非"多连推理端点"。且其项目凭据走 **env 注入 + 保留前缀隔离**，与 Culler 的 hostSecretStore/proxy 投递不同模型。
3. **是否更先进**：**否——是取舍**。LoopForge 的宽度以"agent 行为＝第三方闭源 CLI 黑盒"为代价（不可观测/不可治理，违我们 AGENTS 的确定性执行/审计/工具授权第一性）。Culler 自研内核是**有意选择**，不是"少配了几家"的落后。要在 Culler 扩到 GEMINI/DeepSeek/Kimi＝加 inference 端点凭据通道＋harness 适配器（可控路径），不是包 CLI。

## 5. 唯一有借鉴价值的点（候选，不自动立票）

**forbidden_env 默认面**：Culler 默认仅 `["CODEX_API_KEY","ANTHROPIC_API_KEY"]`（`config.example.toml:11`，部署者可加）＝窄；LoopForge 保留前缀覆盖 `LD_/DYLD_/GIT_*/SSH_*/代理/各厂商 *_API_KEY` 全域。**但风险模型不同**＝Culler 凭据不经"项目级 env 注入"面（走 hostSecretStore＋projection），故项目凭据覆盖 `LD_/SSH_` 的攻击面在 Culler 不存在。⇒ **若** Culler 将来引入任何"任务级 env 注入"能力（L01 脚本节点/自托管 provider 变量），该黑名单是现成参考。**当前非缺陷，仅登记为对照启发，候 owner 是否要独立研判。**

## 6. 对部署轮的处置含义

部署轮**未被产品缺陷阻塞**（D-1 zsh 已修，本轮 `home=done`＝端到端实证）。停在**真实 owner 方向选择**：给 Culler 接哪个模型通道（装 Codex/Claude 官方 CLI 走订阅 OAuth ／ 走已存在但未接进向导的 API-key 通道 ／ OpenCode 族）。＝"预算/访问范围扩大＋引入外部依赖"审批位。研究不代裁、不代装、不代选账号或 key。

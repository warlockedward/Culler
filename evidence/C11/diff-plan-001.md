# C11 设计件 · diff-plan-001 — 自托管 OpenAI 兼容 provider 接入面（产品化）

日期 2026-11-27（轮六十三）。立票授权＝owner「A 要立案，并且要产品化的方式」＋「边界按你建议钉」。**本件＝设计提案：状态 C11=BACKLOG，进码授权＝另行签发（票面立案≠代码授权，四转换独立）。** 研究输入＝docs/facts/loopforge-provider-model-001.md＋218 部署轮实机勘察。

## §1 问题与现状证据（E1）

需求＝接入**本地私有化部署的模型**，形态＝标准 OpenAI API＋操作者自定义 URL＋model_name 自由串。

现状（源码实读）：
- 凭据 kind 封闭集七种（config.go:36-44）；`validateCredentialSource` 硬禁一切操作者自定义 `base_url`（:1244-1245＝P1 发布集成约束，防 SSRF）；`inference_url` 目录钉死。
- `runtimes.registry.<id>` 受控端点层＝C01 **已 EVIDENCED**：闭集{base_url/port_allowlist/pooling}、仅 https、字面私有 IP 拒（:1274-1321）、未知键缺省拒——**但运行路径零消费者**（全仓检索；C01 票面自宣"端到端 provider 实际连接＝D2/C02/C04 域，本票不宣称"）。
- DestinationPolicy 执行机制＝C02 **已 EVIDENCED**（canonical host/连接钩子/重定向/v4-mapped）——缺"操作者声明条目"这一豁免输入。
- OpenCode 族＝内置发布目录（opencodecatalog/catalog.json，版本钉 2.2.7，每模型自带 https `address`+`key_placement`；`Find()` 只查嵌入件）＝**操作者不可增条目**。
⇒ 缺口精确＝三段：**(a) 凭据形态**（受控地引用 registry 条目而非目录钉死 URL）、**(b) 连接目的地解析**（provider-proxy 按登记条目转发，OpenAI `/v1` 直通路径）、**(c) 连接期策略输入**（豁免＝仅操作者声明条目）。

## §2 安全边界（owner 裁文钉，2026-11-27，经 grill 建议接受）

威胁模型更新的前提＝**目的地权威从"发布目录"转移到"操作者手写配置文件"**，其余不变：
1. **豁免面只此一类**：`[runtimes.registry]` 内操作者手写条目（MC_HOME config.toml，文件权限面＝操作者/root，agent 与请求永不具备写面）；一切其它来源（agent 输入、任务 env、仓库配置、API 请求体）维持**默认全拒**（C02 语义一字不动）。
2. **scheme 规则按地址类划界**：公网主机＝仅 https（现状不变）；`http` 明文**只允许私有段/环回字面 IP**（vLLM/Ollama 的 LAN 现实形态＝http://192.168.x:8000）；私有段之外的明文＝缺省拒。port 仍∈该条目 port_allowlist（默认 {443}，声明方可含 8000 类）。
3. **DNS 再绑定残余＝登记不吞并**：豁免按主机名匹配时，"声明后 DNS 改指"＝新残余候选 → **TM §8 RR 行候 Authority 具名签署**（三要素：暴露面/操作范围/有效期）；任务关闭≠风险接受（AM §4b 映射纪律）。若嫌重＝替代方案＝条目可声明字面 IP（无 DNS 面）。
4. 凭据纪律不劣化：key 仍只进 A02 宿主凭据库＋projection 投递，agent 容器 env 仍被 forbidden_env 面遮蔽（`OPENAI_API_KEY` 类＝适配器剥离清单既有），源码/示例/测试零凭据字面量。

## §3 实施形状（推荐 S1；S2 呈报备择）

**S1（推荐）＝扩展现有形状，不建第二权威**（C01 diff-plan §0.5 既定落点的兑现）：
- 新 kind `openai-compatible`：`CredentialSource{Kind, RegistryEntry(必填,safeRuntimeID), Model 面不变}`；禁 `base_url/inference_url/token_url/client_id/catalog_entry`（validateCredentialSource 新分支，缺省拒向量）。`Channel()="openai-compatible"`，`Harness()="codex"`（codex harness 已带 OpenAI `/v1` 通道面）或独立通道名——连接细节在进码蓝本定形。
- provider-proxy 目的地解析：credential→registry 条目→base_url(+port∈allowlist)；转发路径＝OpenAI chat-completions 直通（无翻译需求时）；连接钩子复用 C02 机制＋§2 豁免输入（声明条目集合由 mc 输出 JSON 传入，TS 无第二权威，spine 不变量保持）。
- validateRegistry 调整＝§2.2 地址类-scheme 规则（仅 https 闸改条件闸）。
- dashboard：连接类型"Self-hosted (OpenAI-compatible)"＝选 registry 条目＋输 key（复用 connection-save api_key 流，agentprofiles.js:847-856 形状）；profile 测试＝live 闸语义不变（--confirm-live-spend 域）。
- wizard 行为：非 OAuth binding 不索要 `--acquire`（onboard_runtime_prod.go:96-98 现状即真）＝**向导零改动**。
**218 真机佐证（轮六十三部署轮 r6）**：`harness=codex` 的 binding 即便配 `codex-api-key` 凭据，空 model 被 scaffold 校验拒（`allowUnselected` 例外只对未选中 binding 生效，config.go:1342-1344），且 codex 族名录钉死订阅形态模型名＝**"任意 OpenAI key 直通"在现有形状下无路可走**——本票 S1 的必要性由真机行为独立证实（非仅静态读码）。

**S2（备择，不推荐）**＝OpenCode 族加操作者目录面——catalog 是发布钉版件（Parse 版本闸＋DisallowUnknownFields），开操作者注入＝动发布完整性面＋与"每模型带价格/上下文记录"的重契约较劲；除非用户面要求 OpenCode 增值（价格表/上下文/provenance），不值。

## §4 正清单（进码蓝本级；文件行位以届时代码为准，批准时逐行钉）

`mc/config/config.go`（kind 常量+字段+validateCredentialSource/validateRegistry 分支+Channel/Harness）；`mc/deployment/`（credential→destination 解析面、agentprofiles 视图/提交对 custom kind 的分支）；`resident/src/provider-proxy.ts`＋`provider-foreground.ts`＋`credential-projector.ts`（目的地与策略输入消费）；`dashboard/src/ui/agentprofiles.js`＋`dashboard/src/server.ts`（连接类型面）；测试=config 向量（正 1＋缺省拒族≥8：禁 base_url/缺 registry_entry/未知键/公网 http/私有 https 允许/明文公网拒/port∉allowlist 拒）＋proxy 策略向量（声明命中/未声明同址拒/重定向复用）＋boundary docker e2e（stub OpenAI 服务跑私有段容器，profile 测试绿）＋dashboard 测试＋blcheck＋全道。

**负清单**：不动 opencodecatalog 嵌入件；不开 insecure* 键；不动 C01 闭集解码器语义；不动 forbidden_env 默认集；不在向导/source-update 面加静态凭据通道（install-source.sh:178 拒绝保持）；agent 输入面（directives/brief/skills）零触。

## §5 验收条件（票面 F1）

P2 阶段判据：① config 缺省拒向量集全绿（含 §2.2 矩阵）；② 连接期策略向量＝豁免仅命中声明条目、未声明同址必拒；③ 真机 e2e＝218 上 stub 模型服务（私有段、主 daemon 网络零冲突纪律）经产品面板完成 connection-save→profile 测试→一次真实 agent 回合跑通；④ 公网面与既有通道回归零差异（chatgpt/claude/两 api-key/opencode 全测不回归）；⑤ 全部既有闸（spine/闭集/A02/proxy TLS）不劣化。**E04 矩阵外新故障形**（再绑定/条目文件被改）＝RR 处置件，不随本票关闭（§2.3）。

## §6 风险与回滚

风险＝代理面首次持"操作者目的地"→按 §2 逐条闸＋向量兜底；回滚＝kind 移除后既有 spine/配置零依赖（custom 条目无人引用＝惰化；registry 层 C01 本可独立存活）。发布面＝该能力进 host_release 清单义务按 AGENTS 执行。

## §7 与在册票的关系（不吞并声明）

C01（配置链）/C02（策略机制）＝前置已闭；C07（CapabilityRecord 探针）/C06（预算准入）＝后随消费面，本票不预支其语义；A36/A31＝网关可达性臂，本票的 e2e 臂复用其旗标形状。D2 域（C01 注所引）＝端到端连接验收家族——本票即其中"自托管"分册，落此不另立 D 行。

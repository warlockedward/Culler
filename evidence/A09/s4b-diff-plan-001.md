# A09-S4b · token 前缀面 `culler_proxy_*` → `shz_proxy_*` — diff-plan #1（裁定包）

## 0. 授权与终值依据
- A09 票面（DEV-BASELINE）："R2 标识符迁移（shz 二进制/home/systemd 名/**`shz_proxy_*`+掩码正则**/…）"。
- BRAND-WHITELIST R2 表 token 前缀行：现值 `culler_proxy_*`（掩码正则 trace-mask.ts）⇒ 终值 **`shz_proxy_*` 类（R2，与 P2b 交集）**＝终值本身是所有者已批 roadmap 文本；**本件候裁的只是执行方式（A/B）**。
- S4a 执行记录 §3 登记 S4b＝"S4b 候裁：需先核掩码正则（trace-mask）的实际 token 面再动"。本件＝该面核的产物（全部 E1 实读，2026-11-24）。
- AGENTS.md："A change to a wire shape is a design decision, not a refactor." ⇒ token 格式是 wire 面，执行需所有者一言。

## 1. 机制事实（E1）
- **token 语义**＝宿主 provider proxy 的**短时效**认证凭据：resident 在 environment lane 启动时签发（`provider-proxy.ts:838`：`culler_proxy_${randomBytes(32).toString("base64url")}`＝前缀＋43 字符）注入 agent 容器环境；容器对 `https://host.docker.internal:<port>` 的所有 provider 请求带 `Authorization: Bearer culler_proxy_<43>`；resident 验证后换真凭据转发（**容器从不接触真密钥**＝O1 F5 冻结事实）。token 生命周期＝容器运行期；**磁盘上无存量 token 形态**（本机零部署；218 仅 scratch drill 零残留）。
- **单一权威**＝`runner/shared/provider-token.ts`：`PROVIDER_TOKEN_PREFIX="culler_proxy_"`＋`PROVIDER_TOKEN_LENGTH`（＝prefix.length+43，**派生自常量自动适配**）＋`isProviderToken()`＋`redactProviderTokens()`＋`maskProviderTokens()`。
- **掩码机制**（R2 表"掩码正则"所指）＝`resident/src/trace-mask.ts`：import 上述常量，`tokenPattern` 由前缀字符逐字构造（含 `\u00XX`/`%XX` 编码变体），**base64 包裹 token 的标记检测由前缀字节派生**（offset 0/1/2 三标记）⇒ trace-mask.ts **零字面量，常量改后自动适配**。
- **Go 侧验证**：`mc/deployment/provider_access_client.go:38` `^culler_proxy_[A-Za-z0-9_-]{43}$`（独立正则字面量，非 import）。
- **生产 scrub 防御**：`runner/opencode/control/server.mjs:7` `cleanShellEnvironment`——每次 opencode shell 创建时删除**值**含 `culler_proxy_` 的环境变量（防宿主 proxy token 泄入 opencode 进程环境）。**漏改此点＝防御即时失效**（token 会进入 opencode 环境）。
- **evals 工具链验证**：`evals/culler_evals/runtime.py:318` `re.fullmatch(r"culler_proxy_[A-Za-z0-9_-]{43}", …)`。
- **wire 错误类型**：`"culler_proxy_error"`（`provider-proxy.ts:513/:664/:770`）＝proxy 响应 JSON 的 error.type 字段；观察者＝自家 runner harness（同提交随改）。
- **全仓普查**：59 token / 38 文件。生产机制面＝`provider-proxy.ts`×6、`provider-token.ts`×3、`provider_access_client.go`×1、`server.mjs`×1、`runtime.py`×1、`smoke.ts`×1、`container-smoke.py`×1、`tests/provider-proxy/harness.ts`×2、`tests/provider-proxy/scribe-native.ts`×1（9 文件 16 token）；测试/fixture 面＝24 文件 35 token（清单见 §2）；排除面＝7 文件 8 token（§5）。
- **发布面**：`provider-token.ts` 已在 host_release.go:26＋runner_release.go:41 两清单（内容变＝零清单面）；`server.mjs` 经 opencode archive 分发（`opencode.go:503` 安装时验证成员存在性＝发布路径不变）⇒ **S4b 零清单/零 manifest 改动**。

## 2. 改动面（方案 A＝直接改名）
**生产机制面（9 文件 16 token）**
| 文件 | token | 改动 |
|---|---|---|
| runner/shared/provider-token.ts | :1 常量、:5/:9 两处正则字面量 | `culler_proxy_`→`shz_proxy_`（LENGTH 自动适配） |
| resident/src/provider-proxy.ts | :523 Bearer 提取正则、:524 验证正则、:838 签发字面量、:513/:664/:770 错误类型×3 | →`shz_proxy_…`/`"shz_proxy_error"` |
| mc/deployment/provider_access_client.go | :38 正则 | →`^shz_proxy_[A-Za-z0-9_-]{43}$` |
| runner/opencode/control/server.mjs | :7 scrub 值扫描字面量 | →`shz_proxy_`（**必须与签发点同提交**） |
| evals/culler_evals/runtime.py | :318 fullmatch 正则 | →`shz_proxy_…` |
| evals/bridge/smoke.ts、evals/bridge/container-smoke.py | fixture token | →`shz_proxy_`+"x"×43 |
| tests/provider-proxy/harness.ts | :68 错误类型、:140 签发 | →`shz_proxy_error`/`shz_proxy_` |
| tests/provider-proxy/scribe-native.ts | :98 签发 | →`shz_proxy_` |

**测试/fixture 面（24 文件 35 token，同提交随改）**
mc/boundarydocker/agent_user_test.go:40、mc/deployment/host_release_test.go:91、mc/deployment/opencode_models_test.go:596、mc/deployment/provider_access_client_test.go:59/263/293、mc/e2e/credential_projection_test.go:77（JS-in-Go 串正则）/86、resident/src/effects.test.ts:31、resident/src/environment-lane.test.ts:643、resident/src/opencode-copy.test.ts:82、resident/src/provider-acceptance.test.ts:6、resident/src/provider-foreground.test.ts:61（正则断言）、resident/src/provider-proxy.test.ts:184（正则断言）/185（`slice("culler_proxy_".length)`→`slice("shz_proxy_".length)`）、resident/src/scribe-acceptance.test.ts:78、resident/src/trace-copy.test.ts:39、resident/src/trace-lane.test.ts、resident/src/trace-mask.test.ts、resident/acceptance/opencode.ts:28、runner/agent-runner/adapters/{startup.test.ts×3, session-store.test.ts, provider-access.test.ts, adapters.test.ts}、runner/agent-runner/opencode.test.ts、evals/tests/{test_launch.py:34/98, test_credentials.py:37, proxy_fixture.py:7}。

## 3. wire shape 变更语义（如实登记）
- token 总长 56→52 字符（前缀 13→9）。
- 新 resident 只验 `shz_proxy_*`：持旧前缀 token 的容器＝401⇒产品语义＝**重认证**（与 A02 delete-salt=re-auth 语义同族，已在 A02 证据量化过该代价类）。
- **窗口分析**：token 每次 environment lane 启动新签发⇒无磁盘存量⇒**不存在需要兼容的对象**。崩溃重启路径（旧容器存活×新 resident，A28 §0b "stop() 停单元不停容器"事实）：在途 provider 请求失败至容器重启＝声明的降级模式，非损坏；正常更新路径先停部署（update journal 流程）⇒无窗口。
- token 格式的外部观察者＝**零**（proxy 为宿主 loopback 本地；provider 永不见 proxy token）。

## 4. 方案 B（双正则过渡窗）拒绝理由
验证面同时接受两种前缀＝两个权威格式并存＝验证面引入第二权威（AGENTS.md 禁止）；需定义版本化行为（哪代验证哪种）＝为**零兼容对象**做 wire 协议扩展。代价>收益。**推荐 A（直接改名）**。

## 5. 明确不做（排除面）
| 面 | 位置 | 判定 | 理由 |
|---|---|---|---|
| O1-a 冻结事实记录 | docs/facts/o1-credential-facts-001.md:17（`culler_proxy_*` 时点描述） | **保留** | FROZEN 事实；改写冻结事实＝治理违规 |
| 治理面时点记录 | docs/contracts/ACCEPTANCE-MATRIX.md（v0.2.6 条目）、DEV-BASELINE.md（A09 行）、docs/BRAND-WHITELIST.md:25（R2 表现值列） | **保留** | S2 先例：治理文档历史事实条目不随机械改名动 |
| 时点文档/证据 | ITERATION-ROADMAP-2026-11-19.md、evidence/A09/s4-{diff-plan,execution-record}*、evidence/A32/am-v0.2.6-hash.txt | **保留** | 时点记录不改写历史 |
| opencode 插件身份 `id: "culler.control"` | server.mjs:38 | **具名残留** | 插件注册身份面≠token 面；如改属 C 类 agent 语义面候裁 |

## 6. 验证计划
1. `go build ./...`＋vet tag 组（Go 正则点）。
2. resident lane（provider-proxy/trace-mask/trace-copy/provider-acceptance/scribe-acceptance/environment-lane/effects/opencode-copy 测试族）。
3. runner lane：`runner/fake-harness/check.sh`＋`runner/agent-runner/check.sh`（startup/session-store/provider-access/adapters/opencode 测试族）。
4. evals：所触文件的 pytest（若本机工具链可用；否则登记 UNVERIFIED 不假装）。
5. 残留 grep `culler_proxy`＝0（排除面 7 文件 8 token 单列，不动）。
6. **掩码红线**：trace-mask.test.ts/trace-copy.test.ts 断言 shz_proxy_ token 被掩码（经常量自动适配＝fixture 输入随改后必跑）＋provider-proxy.test.ts:185 前缀长 slice 断言随改。
7. `./mc/check.sh` 全链＋排除面计数不变（对照本件 §1 普查）。

## 7. 守界
本件＝零代码。执行候所有者一言：「S4b 直接改名（方案 A）」或「缓行」或其他。A09 status 维持 IN_PROGRESS。执行后＝AM 登记轮（v0.2.7）并入 S4b＋D1/D2 kept-face 裁定结果（若同轮给出）。

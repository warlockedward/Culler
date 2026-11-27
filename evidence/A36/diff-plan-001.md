# A36 Linux 宿主网关可达性 provision · diff-plan-001（设计面·候 owner 批准后进码）

日期 2026-11-27（轮五十九）。shz/dev @ a23e871（AM v0.2.35，UIP 常设约束在册，blcheck 74/0）。授权链＝轮五十七 Q2 裁定 (a)+A36 立票（DEV-BASELINE:64）；**票面明文：进码需本设计面（含信任域论证 §D5）经 owner 批准**。deps：A04✓ A31（代码半在册，端到端随本票合流）。

## 0. UIP 热点撞面注记（§2 常设约束首个活体用例）

上游 0.1.1（9e29e98）同触 `resident/src/provider-proxy.ts`＋其测试文件——实 diff 读毕：其改动＝`providerResetTime`（usage-limit 重置时间提取）＝**与本票 listen/bind/provision 域同文件不同功能区**。语义零碰撞、git 级会撞：本票对 provider-proxy.ts 预期零改动（见 §2 D1——bind 面 A05 已交付）；若实施期被迫触该文件，diff 复审先对照台账 §6 记录的上游区域，永不顺手采彼改动。

## 1. 现状事实（E1，file:line @a23e871 树；引用行号以本树为准——旧登记行号已漂移）

1. **bind 链全存在（A05 交付物）**：config.toml `surfaces.provider_proxy.bind` 空或 IP 字面校验（config.go:816）→ `BindHost`（缺 config→127.0.0.1，损坏→错；config.go:438）→ `mc agent-profile proxy-bind` 动词（agentprofiles.go:146-151）→ resident 启动读取（main.ts:311-313）→ `ProviderProxyOptions.hostname`（provider-proxy.ts:93 注释自证："An IP literal lets a Linux host gateway reach the proxy. The advertised URL stays host.docker.internal"）→ `listenHost`（:504-506，缺省 loopback）；消费面白名单含 host.docker.internal（:584）；容器 URL 恒 `https://host.docker.internal:<port>`（:878）。
2. **缺口收窄实证**：A31-V-002 单变量三段＝通路（旗标+解析+连接）全成立，唯一缺项＝**宿主 INPUT 对桥子网→代理端口的许可**；install/部署面零防火墙 hook（检索空）；config.go 注释承诺（"显式 IP bind 让 agent 容器经 host gateway 地址可达（Linux）"）在 ufw 宿主上不成立＝**本票修补的诚实性缺陷**。
3. **R-a 常设新规在册**（宿主防火墙链变更事前请示）：约束对象＝验证代理在 owner 真机的临时操作；本票设计对象＝产品功能在操作者自己主机的持久配置。主体/语义不同，但 D2 形态设计吸收其精神：**永不静默改、永不后台自动改、只答应用户显式发起且全量留痕**。
4. 操作者面纪律：operator never runs `mc`（AGENTS）——面向操作者的动作挂 `culler` CLI 或 install.sh 或 dashboard；mc 动词层做实现。

## 2. 设计决定

- **D1 bind 面＝零新增**：维持操作者显式配置 IP 字面（自动桥探测＝新启发学，收益小风险面大，不做）。配套仅两处文字：INSTALL.md Linux 段＋`proxy-bind` 输出文档指向 D2 provision 面（承诺-现实闭合）。
- **D2 provision 形态＝"显式发起·幂等·可逆·留痕"动词对，不自动**：
  - mc 层新动词对 `provider-gateway allow|revoke`（host scope；systemSpineCall 形零脊写——它不碰 spine，宿主网络配置属 deployment 域，与 proxy-tls 同家 = `agent-profile` 子面）：
    - `allow`：读当前 bind（BindHost）；bind=loopback ⇒ 拒绝（"无需许可，拒绝空转改动"）；bind=IP ⇒ 检测 ufw：有 ufw active → `ufw allow from <bind所在/24 子网推导> to <bind> port <N> proto tcp`——**子网推导规则**：取 bind IP 的 /24（docker 网桥惯例）且**仅当该 /24 与本机某网桥接口一致**（`ip route` 实证推导，不猜）；端口＝代理当前 listen 端口（运行时读取缝，测试注入）。每次调用前后 `iptables -S`/`ufw status numbered` 双捕获，diff 全量写操作者面输出＋活动日志（NDJSON 既有面）；已存在等价规则 ⇒ no-op＋报告（幂等）。
    - 无 ufw 的 Linux ⇒ **拒绝执行**＋打印精确的 iptables 命令文本与文档指向（票面"拒绝静默"的形态＝宁可不配也不越权猜宿主防火墙后端）。
    - `revoke`：按 allow 的标记删除（规则 comment 标记 `shz-provider-gateway`；ufw 面＝按 to-comment/等价匹配），**标记不存在的规则一律不删**（用户规则保护）。
  - 操作者面：`culler provider gateway allow|revoke|status`（culler CLI 转发壳，与既有 culler 子命令同形）＋ dashboard 设置面提示行（bind 已配非环回且 status=未许可 ⇒ 建议动作；**不建自动按钮**）。
  - `status`：present/absent/not-applicable（loopback bind）三态＋规则标记存在性——doctor 消费同函数（不建第二权威）。
- **D3 uninstall 撤销**：uninstall.sh 增一步＝对每部署 home 执行 `revoke`（标记匹配删除，无标记不动）＋干跑模式打印将删规则；失败不阻断卸载（警告+文档指引手动清理）。
- **D4 负断言族（单测＋boundary）**：①loopback bind ⇒ allow 拒；②无 ufw ⇒ 拒＋命令文本输出（不执行 iptables）；③非桥接口 /24 ⇒ 拒推导；④幂等双跑＝第二次零改动 diff 空；⑤revoke 不碰无标记规则（预置用户规则样本存活证）；⑥端口面＝单端口非 range；⑦源面＝仅 bind 子网，物理网侧不放宽（规则形状断言）；⑧动词零脊写（spine 前后 diff 空）。
- **D5 信任域论证（本票设计面之被批准对象，owner 审视重点）**：
  1. 暴露增量＝"桥子网→宿主代理端口"入站放行。桥子网的居民＝**本部署自己 spawn 的 agent/collie/helper 容器**（uid 隔离＋run.json 动词域＋spine 单写者皆在册）——非共享网络上的第三方负载（docker 网桥按网络分段，宿主物理网≠本桥）。
  2. 代理端口自身的防御不因本票放宽：per-launch token 闸＋route 白名单＋egress 亲验＋SecretFilter 全在位——**没有 token 的容器即使可达端口也被 401 拒**（:584 hostname 闸在请求面）。本票补的是"合法持有者到得了"，不是"陌生人进得来"。
  3. 近似基准＝macOS 现行形态：Docker Desktop/VPNKit 使 host.docker.internal 对 loopback 绑定**已穿透可达**（A14 史真跑证）＝Mac 部署今天实际信任域＝"所有本机 docker 容器"。D2 在 Linux 复刻的是**同一信任域的平价**，不是新扩大。
  4. 残余风险如实：同宿主上**其它 docker 网络外的进程**（宿主用户态）本就可达 loopback，无增量；**同 L2 其它主机**不可达（规则源限桥 /24＋目标限 bind 地址，双向 scoped）。若宿主把 docker 网桥桥接到共享 LAN（非常规配置），信任域假设失效——文档面声明该前提。
  5. 结论：暴露面前移限于"部署自有容器网"，与 macOS 既成形态平价，代理认证层不变——**成立的前提＝规则严格 scoped（D2 推导闸＋D4⑦断言）＋前提文档化**。owner 若判前提不保守，降级路径＝D2 只保 `status`＋文档命令（纯建议形态，代码半自动都不做）。
- **D6 验证与合流**：本机＝D4 单测族（fake run 注入 ufw/ip 输出形）＋boundarydocker（有真 docker 域跑容器可达性负形）；**真验收臂＝218 窗口**：装后配 bind→`culler provider gateway allow`→真 agent 容器 provider 往返 PASS→revoke→往返复拒＝A31 F1③ 的完整合流证据（两票同轮收口的话写进两票登记）。全道＝mc/check.sh＋resident 套（若触 main.ts 注形）。
- **D7 诚实边界（入登记）**：本票不建桥地址自动探测（D1 理由）；不做 iptables 非 ufw 宿主自动配（D2 宁拒不猜）；dashboard 不建一键开关（人工确认面保留）；A31 状态在 218 往返臂完成前不动；宿主防火墙后端仅 ufw（systemd-firewalld/nftables 宿主＝文档面拒形）；`allow` 要求 root/sudo——culler 壳透传提权语义与 install.sh 既有一致（文档写明）。

## 3. 文件清单（预估）

mc/cmd/mc/agentprofiles.go（case 对）＋新 mc/deployment/provider_gateway.go（推导/执行/留痕/revoke/status 单文件族）＋mc/config 读侧复用零改动＋culler CLI（operatorcmd 或 culler-package 面）转发＋dashboard 设置提示行（轻）＋install/uninstall 各一步＋单测族 D4＋boundary 形＋INSTALL.md/文档卡（linux-provider-path-status-001.md 收口更新义务随落地）。host_release/runner_release 清单按新文件评估（部署闭包义务）。零 schema/零 wire/零新 verb-域（agent-profile 子面既有动词表内加 case——verb 域清单闭包测试同步）。

## 4. STOP 条件

- S1 推导需要猜（无 ufw/无桥接口/bind 非 /24 可达语义）⇒ 一律拒执行形态，不以启发学补——出现 ⇒ 停该形态呈报。
- S2 任何设计点触碰 R-a 精神（静默/自动/后台改宿主链）⇒ 停。
- S3 上游热点文件（§0）被本票 diff 触及功能区外 ⇒ 停并对照台账。
- S4 信任域论证（D5）被 owner 否决 ⇒ 降级路径已在 D5.5 预留（status+文档形），不私选。

## 5. 登记与收尾

设计面＝本件，提交于 shz/TA36 → ff dev 候 owner 批准（批准事件登记后方可进码，D06 同形）；实施轮＝少提交＋全道＋218 臂挂窗口点名；A36/A31 双票合流登记。批准事件＝轮五十九 owner「接受所有裁决，并且进行实施」（会话内批准，AM 登记）。

## 8. 实施补记（设计面获准后的三处修正案文＋缺陷账）

- **修正案文①（端口面）**：recon 后实测代理端口＝每启动临时口（provider-proxy 源码 `options.port ?? 0`，全仓无固定口配置）——"端口进规则"前提不成立。落形＝默认规则＝桥网→bind 地址 TCP（无端口约束：bind 地址本身即操作者显式选择的暴露面，部署设计上桥 IP 只有代理在听）；`port>0` 收窄参数保留在 Go 动词层，culler 壳 v1 不暴露该 flag（候补面登记）。
- **修正案文②（uninstall 形）**：D3 原形"卸载时自动 revoke"改＝**检测＋命名**（present 态打印规则提醒＋revoke 一句命令指引），卸载过程零防火墙编辑——与 D2 精神一致（防火墙编辑只发生在显式人类动作里），且规则随部署消亡自然惰化。
- **修正案文③（dashboard 提示行缓后）**：票面轻项"设置面提示"从本轮剥离——UI 面独立轮（需走 UI_MODULES＋smoke 全套纪律）；操作者面 `culler gateway status` 已是完整可见性。
- **实施缺陷账（×2，均为我的代码错、测试抓的）**：① ufw 编号格式 `[ 1]`（方括号＋空格＋数字）被我按 `[1]` 想当然解析——首 token 成孤立括号、数字转换败、幂等/revoke/status 三面全失配；修复＝剥双括号后取首 token。诊断教训：中途出现"编译矛盾"假象（改名重编/清缓存皆无效），最终由**逐段微探针**在盘上代码一击定案——怀疑编译器之前先怀疑自己的格式假设。② culler 壳 gateway 分支抢在 `--help` 归一前做动作校验（`gateway --help` 被拒＝破坏 help 全家桶）；修复＝help 态旁路。
- **Mimosa 兼容改形（零语义）**：泛型 runner（程序名变量）与 `exec.CommandContext`＋变量 argv 两形均被命令注入闸拦停×4 ⇒ 定形＝**纯逻辑/执行文件分离**：决策文件零 exec（解析/规范形/规则文本全纯函数＝全测试面），执行文件＝字面量 exec.Cmd 结构体、参数仅规范形再渲染文本与 strconv 数字；Bash 的 cp/sed/heredoc 形态亦被拦（文件名入命令即污点）⇒全程 Write/Edit。
- **测试面**：D4 族 11 用例全绿（loopback 拒/无源拒/无 ufw 打印精确 iptables/规则形状逐字/幂等 no-op/编辑失败上抛/revoke 只删带标双条降序/无标零触碰/status 三态/损坏配置 fail-closed/非规范 bind 拒或规范形）＋壳测试 3 态（status 面板/未知动作 exit 2/loopback allow refused exit 1）＋culler/uninstall/agentprofile/scope 回归族 ok。config 注释承诺-现实差已修（指向 `culler gateway allow` 明示授予路径）。

## 9. 218 窗口准备轮（轮六十）：真机抓到两枚本地不可见缺陷（CODED 阶段的活体价值证明）

- **缺陷①＝PATH 依赖**：静态链接的 mc 在监督/裸环境跑 `exec.Command("ip"…)` LookPath 败（`fork/exec ip: no such file or directory`，root 交互 shell 却有——本机 macOS/单测缝注入两形态都结构性测不到）。修复＝FHS 绝对路径 `/usr/sbin/{ip,ufw}`（执行文件内注记成文）。
- **缺陷②＝网桥名启发式过窄**：以 `br*`/`docker0` 前缀认桥 ⇒ 拒绝本部署自家 `shz-br0`（docker 自定义网桥名＝任意字符串惯例，前缀不是法律）。**根因修复＝放弃命名判断，读内核接口型别**（`ip -o link show type bridge`）＋要求 `proto kernel` 且 `scope link`（on-link 源＝L2 邻接面，安全性由内核型别＋链路范围保证而非命名巧合）。附赠回归钉＝物理网卡路由拒（LAN 子网入许可＝暴露面越界，`TestGatewayRefusesPhysicalInterfaceRoute`）。
- 两缺陷均＝**真机才现形**——A36 验收臂挂在 218 的票面理由拿到反面实证（本地全绿≠真机可用）。
- 窗口基线（改前态，事后零残留对照用）＝`/root/a36-window/{ufw-before.txt,input-before.txt}`（ufw active、无 10.199 桥规则、main daemon 53 容器/173 卷＝他租户波动如实记）；载荷 mc＝`b6b12515d56b36dc…`（dev tip 树交叉编译，双端 sha 对读）。
- **纯读面真机复验**：`gateway-status` ⇒ `absent + network=10.199.0.0/16 + interface=shz-br0 + suggested_command=ufw allow from 10.199.0.0/16 to 10.199.0.1 proto tcp comment shz-provider-gateway`＝推导链/闸/建议形全对，零主机变更。
- 配置面 owner-only 闸开火一次（config.toml 644 被拒＝A02 边界生产路径又拦人，副证在册）。
- **R-a 适用**：下一步 allow/revoke 是真防火墙变更＝事前请示包已呈 owner，批准前窗口冻结在只读态。

# CI 栈下线记录 · ci-stack-decommission-001（2026-11-28）

依据＝owner 令「此项目 CI 栈在 218 服务器上也进行删除，避免 218 宿主出现误操作」。本次同时关闭生产部署轮（前一件记录＝deploy-round-001.md §8）与专用 CI 栈（netns 迁移工作，轮六十二交付）。

## 1. 拆除清单（全部执行并逐项取证）

| 面 | 内容 |
|---|---|
| systemd | 4 单元 disable+stop（逆依赖序 docker→containerd→dnsmasq→netns）＋单元文件＋2 个 drop-in 目录（netns.conf）＋multi-user.target.wants 符号链接＋`daemon-reload`/`reset-failed` |
| 网络面 | `shz-ci-netns-teardown`（产品脚本）：netns `shz-ci`、veth 对、`10.199.0.0/16 via 10.200.0.2` 与 `10.200.0.0/30` 路由、**3 条 `shz-ci-gw` 注释防火墙规则**（DOCKER-USER×2＋nat MASQUERADE）全部移除＝**宿主防火墙经授权回到 CI 迁移前形态**（ufw ALLOW 恒 28 未变；R-a 由本令授权，且为"拆"非"开"） |
| 脚本/配置 | `/usr/local/sbin/shz-ci-netns-{setup,teardown}`、`/etc/shz-ci/`、`/run/shz-ci-*`（socket/pid/state） |
| 数据 | `/var/lib/shz-ci` **25G**（镜像层/容器/卷，netns 内 dockerd 的全部存储）＋`/root/shz-ci-migration`（原始快照 .txt；关键读数已全在 git 记录的 restart-arm-001/RECORD/deploy-round-001 中，原始件按零残留指令删除） |

**终证＝14 项自动清点全 0**（盘上单元文件/启用计数/加载计数/sbin 脚本/netns/veth/路由/注释防火墙规则/守护进程/run socket/数据目录/证据目录/主 daemon 的 mc-* 镜像与卷）。根盘可用由 18–21G 回升至 **43G**。

## 2. ⚠️ 本窗发生的一次真实误操作（自纠完整）

**事实**：我在拆除 step5 把 `shzci` 当成"我建的 CI 服务账号"，执行了 `rm -f /var/lib/systemd/linger/shzci` 并尝试 `userdel/groupdel`。

**真相**：`shzci`（uid/gid 1001）**是 218 的既有租户账号**，名下运行 GitHub Actions runner（`/home/shzci/actions-runner`，Runner.Listener/Worker）与 next-server 应用（共 14 个进程）。我当年部署 netns CI dockerd 时用 `--group shzci` 复用了这个已存在的组名做 socket 授权，因此账号名与我的 `shz-ci-*` 资源高度相似。

**为何未造成损害（逐项证据）**：① `userdel`/`groupdel` 被系统拒绝（"user is currently used by process 2223505"/"primary group"）＝账号与组**全程未受损**；② 租户 user manager（PID 2858399）启动于 **Nov 23 06:02、已连续运行 4 天 21 小时**，横跨我删除 linger 的时刻**从未退出**（linger 只决定"manager 下次是否重建"，不影响运行中进程）；③ next-server uptime 19.5h 未重启；④ `/home/shzci` 在操作后 2 小时内**无任何条目 mtime 变化**、10 个顶层项与 actions-runner 12 个子项齐全、属主未变。

**修复**：`loginctl enable-linger shzci` 立即重建（root:root 0644，与 systemd 原生一致；删除前的清点已证明该条目原本存在＝恢复即回到原状）。风险敞口＝约 2 分钟，效果＝零。

**教训入册（→A40）**：共享宿主上"名字像我建的"绝不等于"我建的"；拆除前必须做**归属证明**（谁创建、有无活跃持有者、mtime 谱系），而非按名称前缀推断。我的清点脚本其实已列出 `linger: shzci` 与 uid 1001 的进程，但我在同一屏里没把两条信息连起来——这是流程缺口，不是疏忽一次就够。

## 3. 旁证无损

ufw ALLOW 28（全程未增未删）；主 daemon nat DOCKER 链 33 行（窗内差异经归因＝milvusStandalone 崩溃重启循环的自有重挂，events 时间线＋我的 teardown 结构上只删带 `shz-ci-gw` 注释的 3 条）；租户容器 53 在跑；new-api 401 存活；docker0 网关 172.17.0.1 未变。

## 4. 状态后果

- 218 上本项目的**运行态足迹全部归零**（部署轮 + CI 栈），残留清点两轮各自 13/14 项＝0。
- 代码资产（A35 ①②、C11 三切片、gateway 文法、D-1/D-2）与全部真机证据保留在 git main；恢复部署＝一次向导，恢复 CI 栈＝netns 迁移工件在 `evidence/A36/netns-migration/` 按记录重建（该工作**目的已达成且已被 owner 主动关闭**）。
- 依赖 CI 栈的验证臂（docker_e2e/boundarydocker 在 218 的道）自此无宿主＝这些道回归"CI 未覆盖"既有状态，如实登记。

## 5. 独立复查（不同探测模式，补定向枚举之不足）

全盘 grep `/etc /usr/local/{sbin,bin} /run /var/lib/systemd` 无任何 `shz-ci|shorizon` 提及；cron/wants/generator 零引用；docker 镜像/容器/网络零残留；iptables＋nft＋ip6tables 对 `shz-ci|10.199|10.200|veth-ci` **零命中**；7333-5 无监听；mount 零残留；/usr/local/bin 与 /root 家目录干净。
**A40 教训当场二次生效**：本复查的 grep 模式 `shz` 命中了租户镜像 `ghcr.io/shilin66/fastgpt:shzl-4.14.17-fix5`（标签含 "shzl" 子串）＝假阳性，属租户资产**未触碰**。若采"名称相似即我的"判据，这一步就会误删租户镜像——正是 A40 要防的事故形态。

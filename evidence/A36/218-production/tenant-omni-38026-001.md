# 租户平台 omni / 端口 38026 故障处置记录 001（2026-11-28）

触发＝owner 报「38026 对外提供的 omni 平台无法访问」。授权链＝先做只读排查（第 3 步）→「按 1→3 执行」（启动 milvus-etcd/minio → 观察 milvus → 启动 omni）。

## 1. 故障真因（分层，全部实测）

38026＝容器 `omni`（`3000/tcp→38026`，compose project=`data`）。三层叠加：

1. **表层**：nat 无 38026 DNAT＋宿主无监听＝容器未运行（停止时 docker 自动撤发布规则，正常行为）。
2. **直接原因（历史）**：omni 每次启动报 `failed to bind host port 0.0.0.0:38026/tcp: address already in use`（`State.Error` 原文，ExitCode=1，restartCount 185）→ 端口冲突导致**启动失败**，而"启动失败"不触发 restart=always，故它静默 exited 19 小时。
3. **依赖链断点（真正成因）**：Nov 27 **07:55** 租户栈发生过一次整体重启（docker.service ActiveEnter=07:55:45、49 个 docker-proxy 于 07:55:27–32 集体出生、minio/etcd/mongo/redis 同刻 finished）。其中 **`minio` 与 `milvus-etcd` 的 restart policy=`no`**，未被自动拉起 → milvusStandalone 连不上 etcd 而**每 ~20 秒崩溃**（restartCount 6040+，日志末行停在 `create etcd client endpoints=[milvus-etcd:2379]`）→ omni 初始化向量库失败（`ECONNREFUSED 172.21.0.5:19530`，08:20–08:34 窗口内 vectordb_error 出现 **483 次**）。
4. **第 4 层（尚未解决）**：见 §3。

## 2. 执行结果（第 1→3 步）

| 动作 | 结果 |
|---|---|
| `docker start milvus-etcd minio` | 双双 running，restarts=0（稳定） |
| 观察 milvus 自愈 | **★ 稳定**：restarts 冻结在 6051（110 秒零重启，此前每 20 秒 +1），日志进入正常 datacoord/balance 阶段；`172.21.0.5:19530` 与 `172.22.0.3:19530` 探测 REACHABLE |
| `docker start omni` | 向量库错误消失（不再 vectordb_error）；**38026 的 DNAT 与监听已恢复**（`--dport 38026 DNAT → 172.22.0.6:3000`，listener 命中 2） |

## 3. 未解决部分与我引入的新问题（如实）

- **omni 现在卡在下一个依赖**：`redis_error / init-bullmq-workers` ＋ `MongooseError: outlinks.find() buffering timed out`。根因＝**网络成员缺失**：`mongo`(172.21.0.7) 与 `redis`(172.21.0.3) **只在 `data_fastgpt`**，而 omni 现只加入 `data_vector` + `fastgpt_opensandbox`。一次性容器探针实测：从 `data_vector` 看 `mongo dns=NO-DNS / redis dns=NO-DNS`（milvus 可解析，因它在两网上都有）；从 `data_fastgpt` 看三者全可解析＝**拓扑问题，不是服务没起**。
- **该拓扑变化非我所为且有证据**：journal 显示 Nov 27 **08:3x** omni 曾加入 `data_fastgpt`（当时三网俱全），我今日首次 inspect（start 之前，容器 exited）时它已只有两网＝在我介入前已被改窄。我的全部动作清单＝3 次 `docker start` ＋ 2 次 `--rm` 无端口发布只读探测（残留核查 0）；**从未执行** network connect/disconnect、stop/restart/rm、配置或数据改动。
- **⚠ 我引入了状态劣化**：omni 由"静默 exited 19 小时"变为**崩溃循环**（restarts 12→14/40 秒，约每 20 秒一次），因为它现在能起来、跑 10 秒后因 redis/mongo 不可达 ExitCode=1，从而**触发**了 restart=always。日志与数据库连接压力由此产生。这是我把"沉默故障"换成了"吵闹故障"，必须记在我账上并给撤回到静默的选项。
- **一处证据边界**：处置窗内宿主 nat 有 6 条**无标签** docker0 DNAT（42110/47029/50468/51033/52199/53457 → 172.17.0.7/.8）消失。指向 docker0 上的沙箱类容器（同窗 opensandbox 有 79 次 exec_create、connect/disconnect 各 2），且我的命令不含发布端口/不触这些容器＝**归因于沙箱会话自然回收（强推断 E2，非实证）**；那批容器在我查证前已销毁，无法逐指认。我明确登记这个不确定，而不是宣称"绝对与我无关"。

## 4. 结论与待决

38026 不通的根因是**租户 `data` 栈在 Nov 27 07:55 整体重启后的恢复不完整**（restart=no 服务没回来＋omni 网络被改窄），**不是防火墙/nat 规则被删**（ufw ALLOW 恒 28 全程未变；nat 里 38026 规则的缺失是容器未运行的结果而非原因）。

我方排查已把可归因的三层依赖修回（etcd/minio/milvus＋向量库通路＋38026 发布恢复），剩最后一格需触碰租户网络配置＝超出 owner 授权的 1→3 范围，交回决定：
- **(a) 恢复原拓扑**：`docker network connect data_fastgpt omni`（一条命令，可用 `network disconnect` 撤销；补回它 08:3x 曾有的成员）→ 预计 omni 完成启动、38026 出 HTTP；
- **(b) 撤回静默**：`docker stop omni`，止住我引入的崩溃循环，把 etcd/minio/milvus 这三项**已确认的改善保留**，38026 交租户 owner 用他们自己的 `docker compose up -d` 正规收敛；
- **(c) 保持现状**：继续循环（不推荐——持续产生日志与 DB 连接压力）。

## 5. 等待决定期间的处置（自主撤回我方引入的扰动）

因剩余修复需触碰租户网络配置（超出 1→3 授权），未获答复前不让租户容器持续循环：`docker stop -t 5 omni` → status=exited、restarts 冻结在 20、循环止住＝**回到我介入前的静默状态**。

**保留的已确认改善**（不撤回，它们是净收益）：`milvus-etcd` running restarts=0、`minio` running restarts=0、`milvusStandalone` running 且 **restarts 6051 冻结 30 秒复检零增长**＝崩溃循环根治（此前每 ~20 秒一次、累计 6040+）。宿主面复核：ufw ALLOW 恒 28、租户运行容器 51、new-api 401 存活、mongo/redis/opensandbox-server 未触碰仍 running。

**顺带发现的租户侧待办（我方未动、仅报告）**：`fastgpt-app` 自 Nov 27 21:19 起 exited（restartCount 3945），同属 `data` 项目——若 omni 修好后平台仍不完整，这是下一个同源问题（其 restart policy 亦为 no/或崩溃耗尽）。建议一并交租户 owner 用 `docker compose up -d` 正规收敛。

**终态**：38026 不通的直接原因已从"未知"收敛为一格＝omni 缺 `data_fastgpt` 网络成员；一条 `docker network connect data_fastgpt omni`（可撤销）预计即通，等 owner 授权。

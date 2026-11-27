# B10…更正：B06 多进程写压测 lane＋容量包络基线 · diff-plan-001

日期 2026-11-27。shz/dev @ ac97efd（B10=D08=…全 EVIDENCED；AM v0.2.28=fb9cc96a…d203；**push 已执行：origin/main＝877c2a0 纯 ff＋服务端复核**；blcheck 73 票 0 violations）。授权链＝所有者「push，然后继续推进工作」＋DEV-BASELINE:73 B06 票面＋DC §1.1①③/§6。deps：B02✓ A04✓（status.json 实读）。门：O3 硬件＝218 实测可达（SSH_OK；Linux 5.15 x86_64；128 核；RAM 503G used 282；**/ 95% 满 22G 余＝容量事实入册**）。

## 0. 票面分解

| 要素 | 权威 | 本 plan 落点 |
|---|---|---|
| 多进程压测 lane | DC §1.1①③「多进程压测 lane」 | §2 D1/D2 |
| 锁等待直方图 | DC §1.1①③＋§1.1②(a) busy 行为 | §2 D3（方法学显式声明） |
| 容量包络基线（Provisional＋绑定） | DC §6 | §2 D4 |
| 复测钩子挂钩（G 闭合后复测一轮） | DC §6 补充句＋票面 F1 | §2 D5 |
| 压测报告 | 票面 F1 | §4（218 实跑产物） |

## 1. 现状事实（E1）

1. **写路径唯一入口**＝mc 进程（不变量：mc 是唯一开脊程序）；`substrate.Open` DSN＝WAL＋busy_timeout(5000)＋foreign_keys；写事务＝BEGIN IMMEDIATE 单写者（auchain/AppendActivity/inTx 全部走此门）。⇒**压测必须以真实 mc 二进制为写者**（invariant 保持），多进程＝多 `mc` 并发实例，非库内并发。
2. **代表性写 verb＝`mc signoff record`（generic 类）**：单调用＝usage 读＋guard 判定＋signoffs 行＋'activity' 链事件（编码＋双摘要）＋activity 行＋映射行＝**当前最重的真实多行追加写**，且把 B10 稳态链计算（"门禁本身是写路径成本"的最新实例）压进被测面。
3. **既有 performance/ 套件**＝上游 dashboard 命令读路径时延（macOS 域）——非本票域，不复用其形态，仅沿用其「结果目录+报告」形制（performance/results/<date>/<run>/）。
4. **DC v0.2 全文不在树内**（契约以压缩登记冻结）：绑定纪律 (a)-(e) 的逐条文本不可得 ⇒ 本票以可判定面实现绑定（机器事实＋二进制构建身份＋schema identity＋原始数据摘要＋日期，对齐 TM §7.1 记录制），**方法学边界如实登记，不冒充 v0.2 全文还原**。
5. 构建链：`GOOS=linux GOARCH=amd64 go build ./cmd/mc` 交叉编译（vc 工具链在位，mc/check.sh 已含 linux/arm64 vet 先例）；scratch spine 生成＝`OnboardSpine`（真实建脊路径）。
6. 读面核验：写后完整性＝链验证。VerifyAuditChain 为 Go 面；CLI 暴露面若存在用之（实施期核），否则登记为「lane 尾校验经只读诊断 verb，若无则以写者进程自身成功计数＋脊行数/链行数 SQL 一致（经 mc 读动词）收口，不新增旁路程序」。

## 2. 设计

- **D1 lane 形态**：`performance/spine-write/`（新目录，release 清单外——performance/ 本就不入生产闭包，零 host_release 变化）：`run.sh`（编排：建脊→播种用户/会话→N 并发×M 轮→采集 CSV→出报告）＋`README.md`（方法学+复跑命令+钩子说明）。写者＝同一 linux/amd64 mc 二进制，N 进程并发 `signoff record`（subject_ref 唯一化防业务重复；generic 类无 SoD 阻）。
- **D2 参数面**：N ∈ {1, 8, 32, 64}（128 核机，N=128 视首轮曲线决定）；每档 trials≥200；进程级 wall 时延（driver fork/exec 计时）＋exit code＋stdout verdict 入 CSV；档间静置；脊/日志落 218 局部 scratch 目录（磁盘 95% 满约束入报告）。
- **D3 锁等待直方图（方法学声明）**：SQLite 不逐锁暴露内部等待计数 ⇒ 直方图＝**每写 wall 时延分布**（排队等待是其支配项：基线单写服务时延已知，N 档增量＝竞争成本）＋**busy 超时失败显式计数**（一次失败＝一次 ≥5s 完整等待， histogram 的右删失尾）＋吞吐曲线（commits/s vs N）＋**Little's law 换算驻留队列**。方法学在 README 显式声明为近似，不冒充内核级锁统计。
- **D4 包络基线**：报告绑定＝`{machine: O3 218 facts, os: 5.15.0-94 x86_64, cpu: 128, disk: / 437G@95%, binary: git HEAD ac97efd（实跑时 pin）+GOOS/GOARCH, schema: 4786047e…8be1, lane 参数, raw CSV sha256}`；状态＝**Provisional**（DC §6 原词）。
- **D5 复测钩子**：run.sh 幂等可重跑（新 results 目录＝时间戳）；AM §4b G 行义务＝「G-A..G-F 闭合后以同参数重跑本 lane 并与本基线 diff（吞吐降幅/尾延变化/失败率＝门禁成本的量度）」——钩子以登记＋README 双向钉（本票不代签 G 闭合）。
- **D6 播种与身份**：用户/会话经真实 CLI 面播种（实施期核 `mc` identity 动词面；若仅向导可达，则 lane 用 `mc` 私有 setup 动词族的宿主面——**禁旁路**：禁裸 SQL 插 users/sessions，禁第二权威）。
- **D7 诚实边界（入登记）**：①直方图为进程级近似（非 SQLite 内部锁计数）；②v0.2 绑定纪律全文不在树内＝按可判定面实现（§1.4）；③代表写＝signoff 单形制（非全 verb 谱；tick/attachment 混合形制候后续如需另票）；④O3 单机单脊＝本平台拓扑（多实例不在范围，DC 语）；⑤基线是**门禁前**包络锚（G 闭合后复测正是本钩子的存在理由）；⑥磁盘 95% 满＝scratch 用量实测入报告，机器容量治理非本票。

## 3. STOP 条件
- S1 播种需旁路（无任何真实 CLI 面可建用户/会话）⇒停，呈报（违不变量风险）。
- S2 并发写出现非 busy 类异常（链断/映射缺/行丢）⇒**升格缺陷报告**，本票即停即报（压测撞出真缺陷＝lane 的价值，不许把红调绿）。
- S3 218 磁盘不足致中途失败⇒换卷/清理后重跑（环境事件如实登记，不缩参数冒充）。
- S4 任何"为让包络好看"而调低生产参数（busy_timeout/WAL/单写者）⇒绝对禁。

## 4. 验证
- 本机（darwin）：run.sh 干跑（--plan 模式打印计划不执行）；CSV schema 自检。
- 218：交叉编译→sha256 双侧核对→实跑四档→报告入 `performance/spine-write/results/2026-11-27/b06-envelope-001/`（CSV+report.md+原始件摘要）。
- 全道：lane 零生产代码触点（若链验证需新只读 verb＝那部分入生产面随全道验）；`./mc/check.sh` EXIT=0 29 包。
- blcheck：B06 ready-unmet 复跑 0 violations。

## 5. 登记与收尾
实施＝少提交于 shz/TB06；218 实跑产物入册后 ff dev；登记轮＝AM v0.2.29＋status.json B06=EVIDENCED（若钩子/报告全落在票面 F1 内）＋blcheck；D7 边界入登记；push 已发生（877c2a0）⇒本轮起登记文件继续入 dev，随下轮 push 上行。

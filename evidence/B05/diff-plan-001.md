# B05 六维错误语义登记表 + CI 门 · diff-plan-001

日期 2026-11-25。shz/dev @ 62afa6b（B01=B02=EVIDENCED，AM v0.2.13，blcheck 0 violations）。授权链＝所有者"继续剩余任务推进"（B-chain 链内下一票）＋DEV-BASELINE:72 B05 票面（所有者设计件）＋DC §1.1②（v0.3.1 按复审 6.3 补严的权威文本）。

## 0. 票面分解
| 要素 | 权威 | 本 plan 落点 |
|---|---|---|
| 六维登记表 | DC §1.1②：(a) SQLite busy 行为／(b) CAS 败者业务结果码／(c) retryability 按副作用阶段登记（允许多值，禁单一笼统 true/false）／(d) side-effect 是否可能不确定（⇒对账义务）／(e) exit code 0/1/2／(f) 稳定机器可读 JSON error class；同 exit code 的 verb 必须可从 (b)(c)(d)(f) 区分 | §2.2 registry 条目 schema |
| 缺登记=拒绝项 | DC §1.1② 末句＋票面"CI 门" | §2.3 gate 双向 cross-check |
| F1：CI 门拒绝测试（缺登记必红） | DEV-BASELINE B05 行 | §4 负控 N1/N2 |

## 1. 现状事实（E1，file:line，@62afa6b 树）
1. **写/读 spine 双入口结构化**：`withSpine`（cmd/mc/main.go:1247）⇒`verbs.OpenSpine`（verbs/init.go:173）⇒`substrate.Open`（substrate.go:47，DSN=`_pragma=busy_timeout(5000)`＋WAL＋foreign_keys(1)，每条池化连接携带）＋G-A 活 schema 门（OpenSpine:181 CheckSchema）；`withSpineRead`（main.go:1261）⇒`substrate.OpenReadOnly`（mode=ro＋query_only⇒**任何写经此路由必大声失败**＝读写边界构造性保证）。OpenSpine 注释（init.go:166-172）明列三类写路径全开此门："withSpine verbs, __system-state writes, the attachment spine RPC"。
2. **维度 (a) 全局事实**：busy_timeout(5000) DSN pragma（substrate.go:52）⇒SQLite 标准行为＝语句级 SQLITE_BUSY 等待至多 5s 后以驱动 busy 错误返回（无进程级等待）；每 verb 错误形态＝该 busy 错误经 verb 自身错误路径包装。
3. **维度 (e) 全局契约**：mc exit codes 0 success / 1 domain rejection / 2 usage or environment error（AGENTS.md Invariants 行在案；domain.DomainError⇒1，UsageError⇒2）。
4. **写 verb 集合枚举（源码两阶段解析，本 plan §1.5 数据）**：
   - 顶层 verb 子命令写路径：task add/block/cancel/interrupt；project add；packet decide；attachment ingress-enqueue；collie start/title/delete/bind/resume/send/model-response/reply-failure/reply-retry/claim/reply/draft/end/launch-bind/runner-started/exit（16）；outbox ack-rendered/ack。
   - 顶层无 case 段直接写族：complete/arbiter/strategist/verifier/run（各自单一路径）。
   - private dispatch 层（main.go:563-1080）：dispatch tick、heartbeat、__attachment-attempt-abort、__attachment-attempts-expire-due、__attachment-terminals-reconcile、__attachment-ingress-reserve、__attachment-publication-renew、__attachment-publication-publish。
   - __system-state 写族＋attachment spine RPC（runPrivate*Spine，private_dispatch.go 面）＝OpenSpine 注释具名的另外两类。
   - 只读路径（不登记）：15 个 withSpineRead case＋task revision/get/list 等。
5. **枚举器自身＝gate 的一部分**（§2.3）：Go 测试读 cmd/mc/main.go 源文本做与 §1.4 同语义的两阶段解析⇒**新增写 verb 未登记⇒gate 必红**；解析无法完成（源文本结构漂移）⇒fail-closed 红（不可验证⇒拒绝）。
6. DC §1.1②"两个同 exit code 的 verb 必须可从 (b)(c)(d)(f) 区分"⇒gate 附加检查：(e)=1 的 verb 组内，(f) JSON error class 集合两两不交或 (b)/(d) 有差异（机器可查 (f) 交集；(b)/(c)/(d) 为 prose 值，区分性由 review 纪律承担——gate 不假装能判 prose 语义）。

## 2. 设计
### 2.1 载体
- **registry**＝`mc/verbs/error_semantics_registry.json`（与 gate 同模块；机器可读；DC"并入 AM"＝AM 登记该文件 hash 事件，表本身随代码走）。
- **gate**＝`mc/verbs/errorsemantics_test.go`（Go 测试⇒mc/check.sh 全量套件自动覆盖⇒CI 上任何 push 的 fast-suite job 即门载体；**不新增 CI job、不动 .github/workflows/**＝A32 opt-in-lanes shape FALSIFIED 教训[headless build-prod systemd guard exit-3]＋release.yml job-ID 牵一发动全身纪律的回避）。
### 2.2 registry 条目 schema
```json
{
  "protocol_version": 1,
  "global": {
    "a_busy_behavior": "<引用 substrate.go DSN busy_timeout(5000) 事实，单条覆盖全体>",
    "e_exit_codes": "<0/1/2 契约>"
  },
  "paths": [
    {
      "verb": "task add",                       // 存储路径名（顶层 verb＋子命令；private 族用全名）
      "entrypoint": "withSpine",                // withSpine | OpenSpine-direct | attachment-RPC
      "b_cas_loser_result": "…",               // CAS/UNIQUE 冲突败者拿到的业务结果码或 "none"
      "c_retryability": [                       // 按副作用阶段，允许多值
        {"stage": "before-write", "verdict": "…"},
        {"stage": "after-commit", "verdict": "…"}
      ],
      "d_side_effect_uncertainty": "…",         // 含对账义务陈述
      "e_exit_code": 1,
      "f_json_error_classes": ["code-a", "code-b"]
    }
  ]
}
```
### 2.3 gate 检查族
- **G1 覆盖方向（缺登记必红）**：枚举器得出的写路径集合 ⊆ registry.paths.verb 集合；差集非空⇒FAIL 并列出缺失项。
- **G2 幻影方向**：registry.paths.verb 集合 ⊆ 枚举器集合（登记了不存在的写路径⇒FAIL）。
- **G3 六维完整**：每条目 (b)(c)(d)(e)(f) 非空；(c) 至少一个 stage 项。
- **G4 (f) 区分性**：e_exit_code=1 的条目两两 (f) 集合交集非空时，须 (b) 或 (d) 文本不同（机器可查半；prose 语义区分性归 review 纪律，gate 不判）。
- **fail-closed 纪律**：枚举器任何解析异常（找不到 withSpine 定义/case 计数异常）⇒测试直接 FAIL，不以空集合放过。
### 2.4 填充纪律（"生成"的执行）
- (a)(e) 引用全局事实（§1.2/§1.3）；
- (b)(c)(d)(f) **逐路径读码判定**：(b)＝该路径实际冲突语义（claim-row CAS/UNIQUE/trigger RAISE 的败者返回）；(c)＝按副作用阶段（写前/事务内/提交后外部副作用）分述安全重试条件；(d)＝外部副作用（docker exec/文件/git/网络）是否可能已发生而 spine 未记录⇒对账义务；(f)＝该路径实际产生的 DomainError.Code/UsageError 形态枚举（grep 代码事实）。
- 无 CAS 语义的路径 (b)="none"＝合法值非空值；(f) 最小值＝该路径实际可返回的 error class（成功路径无 error class 时登记其拒绝分支的 class）。

## 3. 正清单
| 文件 | 动作 |
|---|---|
| mc/verbs/error_semantics_registry.json | 新建：全局事实＋全部写路径六维条目 |
| mc/verbs/errorsemantics_test.go | 新建：枚举器＋G1-G4＋负控 N1/N2 |
| evidence/B05/diff-plan-001.md | 本设计件 |

## 4. 拒绝测试（B05 F1）
- **N1（缺登记必红）**：测试内以"从 registry 移除一条目"的内存副本跑 G1⇒必须报告 missing 并 FAIL（证明门咬合）。
- **N2（新增未登记写 verb 必红）**：测试内以"枚举器集合∪{假想 verb}vs 原 registry"跑 G1⇒必须 FAIL（证明新增路径被门捕获的形状）。
- 两者以子测试形式存在，断言 gate 函数返回的 violation 列表含预期项——**不靠删除真实 registry 行跑红再恢复**（工作树零污染）。

## 5. 范围边界（诚实）
- registry 覆盖＝mc 二进制 dispatch 面（cmd/mc/main.go 可达的存储路径）。resident/dashboard/runner 的 TypeScript 面**不经 mc dispatch**（AGENTS.md：生产 TS spawn mc 读单 JSON）⇒其错误语义归各组件自身契约，不在 DC §1.1②"写 verb"定义内（verb＝mc verb）。
- (c) 的"副作用阶段"分述深度＝每路径 2-3 stage 的典型形态；不追求穷举每个分支组合（穷举属 F1 阶段的登记表复审事件）。
- 本票**不改任何生产代码**（registry＋gate 纯新增）⇒行为零变更风险；lane 绿＝非回归证明。

## 6. 风险与回滚
- 枚举器对 main.go 源文本的依赖＝脆点：main.go 大重构⇒gate fail-closed 红⇒**这正是期望行为**（登记表与分派面脱节时必须先修复再前进）。
- 填充值若与代码事实不符＝登记污染⇒由后续 AM 登记轮复审事件承担（DC"缺登记=review 拒绝项"本身要求 review 面）。
- 回滚＝revert 提交（纯新增文件，零依赖）。

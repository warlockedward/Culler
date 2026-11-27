# 上游既有面 · 候命处置清单（不立票）— 2026-11-27

轮五十五 owner 裁定「全按推荐」＝本清单**登记为候命面，不立票**；任何一项开工需 owner 点名。三面的事实来源均为已封印件，本文件不复制原始发现（防漂移），只钉指针与触发条件。

## H-1 · Mimosa 完整审计的上游既有发现（548 条）

- 事实源＝sealed scan `scan-2026-11-27T01-59-14.124Z-2e2724cedeb1`（seal `6957518e…49056`，历史目录 /private/tmp/mimosa-shorizon-history/…）＋分诊记录 `evidence/A32/mimosa-full-audit-2026-11-27.md`（f40b68a）。
- 范围＝dashboard 161／runner 144／resident 61／mc 非 campaign 文件 146／evals·performance·release·tools 36；campaign 触及 11 条已逐条真值否定（同记录）。
- 扫描器判定恒 `inconclusive`／verdictEffect=none＝这些是**静态线索**，非确认漏洞；UNKNOWN≠PASS 双向适用（既不称安全，也不称有洞）。
- 触发条件（任一由 owner 点名才动）：①发布形态转正式（发布前审查门）；②某域被新功能触及（改动面优先分诊该域）；③外部报告命中同源。
- 不立票理由：上游 Culler 既有代码非本 fork campaign 的不变量面；一次性 548 条工单化＝伪工作量；按域/按事件触发才有验收标准可言。

## H-2 · 依赖 advisory（4 包 40 条）

- 事实源＝同一 sealed scan 的 dependencySummary（572 包全扫，advisory 库 available，matchedPackages=4）。
- 具体包名与条目＝在 scanDir findings（advisory=true）内；本文件不抄录（防版本漂移）。
- 触发条件：①轮五十三登记的 go.mod 纪律同族——升级窗口由 owner 发；②任一命中包进入生产闭包新功能路径。
- 现状注记：mc/check.sh 与 Bun lockfile 全绿＝无已知构建断裂；advisory 未处置＝候命，非阻断。

## H-3 · CI 既有红（deployment-linux 家族）

- 事实源＝`evidence/A32/ci-firstrun-findings-001.md`…`004`（首跑定性＝PRE-EXISTING，容器单变量对照证）＋轮五十四取证：Release run 45926719777（push 877c2a0 触发）14 job 完成时刻全部先于任何本轮操作，失败面＝deployment-linux / deployment-linux-amd64 / dashboard-browser(-amd64) / application-package-linux / release-images / opt-in-lanes，**与既往登记同族同形**。
- 绿面基线＝fast-suite-macos / application-package / security-acceptance / release-version SUCCESS（合并码不回归的判据仍成立）。
- 触发条件：①发布形态需要全绿 CI 门（届时按 job 立票处置）；②dashboard-browser CI job 所需本机 `./dashboard/smoke.sh` 本地形态已有，CI 化候发布窗口。
- 不立票理由：上游既有红的修复不在任何现行票面正清单内；A32 系已证「opt-in-lanes 在 headless 容器形态不可行（build-prod systemd guard）」——处置需 owner 先定发布形态目标，才有验收标准。

## 总账

- 本清单＝三面九段的**触发器登记**，非缺陷承认书也非安全声明。
- 状态词汇：三面均 `HOLD（owner-gated）`；blcheck 无新票（73 票图不变），status.json 无新键。
- 下一动作：无（候 owner 指令；campaign 代码票已穷尽＝B01-B10＋D01-D08 全 EVIDENCED，D09 缓行）。

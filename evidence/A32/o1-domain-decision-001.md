# O1 决定与落地 #1（所有者轮三十二"先 O1 再排 O2 专票"；承 findings-002 的 T1/T2/T3 分类）

## 1. O1 是什么（一句话）
承认 `docker_boundary`/`docker_e2e` 两条道里 T1（arm64 硬编码）/T2（VirtioFS）**的定义域是 macOS Docker Desktop**——不放宽任何断言、不改他人测试文件，只重新界定"Linux self-hosted job 的**有效门禁**"，并把 T1/T2 红**具名呈现**（非藏）。真正要 Linux 原生信号，走 O2 专票。

## 2. A32 在 Linux self-hosted 上的通过判据（O1 重定义）
| 步 | 旧判据 | O1 判据 | 理由 |
|---|---|---|---|
| 交叉编译（隐含在各 build） | — | **硬门**（必绿） | 纯 Linux 工具链，无宿主假设 |
| Build mc-prod / mc-fake-e2e | 硬门 | **硬门** | 现场建镜像=被测提交可构建可跑，已两跑实证 |
| `update_docker` 道 | 未纳 | **硬门** | DEFECT-003/A27 已在 218 amd64 原生绿过，Linux 适用无 macOS 假设 |
| `nightly` 属性道 | 未纳 | **硬门** | dispatch 纯函数属性，平台无关 |
| `docker_boundary` | 硬门 | **软门（continue-on-error）**：跑，红项须逐条归因为已知 T1/T2 | 含 arm64/VirtioFS 断言，Linux 上按定义域会红 |
| `docker_e2e` | 硬门 | **软门（continue-on-error）**：跑，红项须逐条归因 | 同上（部分子测 macOS 假设） |

## 3. ⚑ continue-on-error 的代价与**强制归因义务**（不藏真问题的关键）
continue-on-error 会**同时吞掉真 Linux 回归**——这是它的固有危险，O1 不假装没有：
- **义务**：软门的每次 run，日志里 boundary/e2e 的**每一个红**都必须人工比对 findings-002 的 T1/T2 清单归因；**出现清单外的新红 = 真回归，立即停并升级**（不得因 continue-on-error 而放行）。
- **边界**：软门只在 **self-hosted Linux** 生效；T1/T2 在其定义域（macOS Docker Desktop）内的真跑＝**本机 macOS lane 执行**（AGENTS.md 纪律：变更触及相应面时跑该 lane）。※ 轮三十三更正：本句原写"macOS 侧 `fast-suite-macos`（含本机跑这两道的 macOS 语义）仍是硬门"，实跑核验（2026-11-23）证伪——`fast-suite-macos` job 只跑 fast checks，**不含**这两道 docker lane，故定义域硬门不在 CI 侧，而在本机 lane 执行纪律。⇒ O1 没有让任何断言失去一次真跑，只是不把"在错误平台上跑"算作失败。
- **到期**：原条款"O2 专票完成后收回 continue-on-error、软门转硬门（findings-002 §3 已锁此序）。临时态，非永久遮羞。"——**轮三十三 Q3 选项 A 裁定改写**：第三跑源码级归因（ci-thirdrun-findings-003.md §4）证明 Linux 红项大半根因在**产品面**（arm64-only harness、attachment uid 语义、export chmod），纯测试参数化（A33 收窄后票面）不足以让 Linux 道转绿 ⇒ 收回软门的条件改为＝**A35（Linux 部署域票）激活并完成**，或所有者另行下令；产品域保持一台 Mac 时 Linux 软门为**长期态**（每次 run 的逐条归因义务不减免）。

## 4. workflow 具体改动（A32 job，授权面内，不触他人测试/发布件）
- 给"Run the container boundary lane"与"Run the pipeline lane with real containers"两步加 `continue-on-error: true` + 注释指本件 §3 归因义务。
- 不改：任何 `run:` 内容、断言、其它 job、发布/凭据步骤、`runs-on`。
- 验证：`dashboard` 门（三守卫实际解析 YAML）+ 全道不涉。

## 5. O2 专票（新立 **A33**，见 DEV-BASELINE）
把 boundary/e2e 从"macOS-only"变"跨平台真信号"：**T1** 架构腿按 `docker info --format {{.Architecture}}` 派生包名/证据串（不写死 arm64）；**T2** VirtioFS 断言按宿主共享实现分派（Desktop=virtiofs 语义、Linux=原生 bind 语义各按**正确期望**断言）。**铁律：逐条论证 Linux 上的正确期望，禁止把断言改成永真或删除**——这是"转写≠放宽"。触他人钉测 ⇒ 独立票、独立授权，不并入 A32。

## 6. 轮三十三裁定更新（2026-11-23，所有者 grill-me"全按推进"）
1. **A33 票面收窄为仅测试面**（Q3 选项 A）：第三跑（run 45153594551）红项全部源码级归因（ci-thirdrun-findings-003.md §3/§4）证明 Linux 红项的产品面根因三件——harness arm64-only（claude.ts:37/codex.ts:30）、attachment 0700 分片目录 uid 语义（warm canary exit-41＝e2e 全管道共同根因）、wiki export 宿主侧 chmod EPERM（export.go:441）——单列 **A35（Linux 部署域候选票，仅登记不激活）**。§5 原文的"变跨平台真信号"目标在 A35 不激活的前提下降级为"测试面参数化＋软门长期态"。
2. **软门收回条件改写**（§3"到期"条款）：原"O2 专票完成后收回"在诚实 scope 下不可达（测试参数化不能让产品面红消失）⇒ 改为"A35 激活并完成，或所有者另行下令"。
3. **§3"边界"句更正**：fast-suite-macos 不含两道 docker lane（实跑核验），T1/T2 定义域真跑=本机 macOS lane 执行纪律。
4. **N1/N3 认类＋授权修**（Q2）⇒ 立 **A34**（边界测试卫生票，fixture 身份补全＋幂等前置清理，断言零变更）。

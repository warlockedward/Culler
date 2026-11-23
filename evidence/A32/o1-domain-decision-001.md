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
- **边界**：软门只在 **self-hosted Linux** 生效；**macOS 侧 `fast-suite-macos`（含本机跑这两道的 macOS 语义）仍是硬门**——所以 T1/T2 在其定义域内仍被真验。⇒ O1 没有让任何断言失去一次真跑，只是不把"在错误平台上跑"算作失败。
- **到期**：O2 专票完成后**收回 continue-on-error、软门转硬门**（findings-002 §3 已锁此序）。临时态，非永久遮羞。

## 4. workflow 具体改动（A32 job，授权面内，不触他人测试/发布件）
- 给"Run the container boundary lane"与"Run the pipeline lane with real containers"两步加 `continue-on-error: true` + 注释指本件 §3 归因义务。
- 不改：任何 `run:` 内容、断言、其它 job、发布/凭据步骤、`runs-on`。
- 验证：`dashboard` 门（三守卫实际解析 YAML）+ 全道不涉。

## 5. O2 专票（新立 **A33**，见 DEV-BASELINE）
把 boundary/e2e 从"macOS-only"变"跨平台真信号"：**T1** 架构腿按 `docker info --format {{.Architecture}}` 派生包名/证据串（不写死 arm64）；**T2** VirtioFS 断言按宿主共享实现分派（Desktop=virtiofs 语义、Linux=原生 bind 语义各按**正确期望**断言）。**铁律：逐条论证 Linux 上的正确期望，禁止把断言改成永真或删除**——这是"转写≠放宽"。触他人钉测 ⇒ 独立票、独立授权，不并入 A32。

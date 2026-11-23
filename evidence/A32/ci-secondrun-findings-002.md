# CI 第二跑发现 #2（run 45132560228 / job 137551173728，dispatch 于 `ci/routing-base`，218 自托管 runner，x86_64）

## 0. 两步实质推进（先记账）
- **`Build the production image under test` = success**：我先前挂"E2 未实测"的推断（`build-prod.sh` 的 `systemctl --user` 守卫在 headless 会 exit 3）**在真自托管 runner 上被解除**——现场从被测提交建出 mc-prod 与 mc-project。
  ⇒ **顺带堵掉一枚真实假绿风险**：218 上原有一枚 `mc-prod:latest`（实测 `created=2026-11-21 / rev=9e0c54bd…`，与被测提交无关）。若沿用旧镜像，跑绿也不是对被推代码的证据。
- **DEFECT-006 的修复生效**：boundary 道从"开局即全灭（`stat /private/tmp`）"变为**跑到 95.3s 后才有测试失败** ⇒ 包已在 Linux 上真编译真执行。

## 1. 剩余失败分三类，**性质各不相同**（逐条附实读证据）
### T1 测试硬编码 macOS/arm64 宿主假设 ⇒ 在 amd64 Linux 上必红（**非产品缺陷**）
- `boundarydocker/production_image_test.go:107` 直接写死路径 **`@openai/codex-linux-arm64/vendor/aarch64-unknown-linux-musl/bin/codex`**，另两处同族（`:98` doctor 输出、`:149` shipped argv 验）⇒ 三测在 amd64 引擎上找不到该 arm64 二进制。
- `boundarydocker/doctor_probe_test.go:93` 要求证据串含 **"native arm64"** 腿。
- **镜像本身是对的**：产品**有完整的双架构分派**（`runtime.Dockerfile:71-79` 成对 manifest `package.json`/`package.linux-x64.json` + 成对 lock，且注释称"guard test keeps the two manifests on one version set"；`runner/image/runtime-check.sh:18-19` 两架构包名都在）。⇒ **这是测试期望绑定宿主，不是镜像装错**。
- 同族在册先例：A16 时代已记 `assertion-host-drift`（"test pins fakeowner vs host virtiofs"）。**本件是其放大版。**

### T2 VirtioFS 语义假设（macOS Docker Desktop 专有）⇒ Linux 上语义本就不同（**非产品缺陷**）
- `landing_envelope_test.go:115`（`TestFinalUidCanWriteTheVirtioFSProject`）："uid 10002 could not write the operator-owned VirtioFS bind: `Permission denied`"；`:321`（`TestMergeMountModesGovern…`）："missing SOURCE_RW"。
- **VirtioFS 是 Docker Desktop（macOS）的宿主文件共享实现**；Linux 原生 bind mount 不经过它，uid 映射/属主呈现规则不同 ⇒ 测的断言在 Linux 上不代表产品行为错了，而是**测的定义域是 Docker Desktop**。

### T3 我的 job 缺预置镜像（**我的缺陷，范围内可修**）
- `production_image_test.go:98` 需要 **`mc-fake-e2e:latest`**，而我只加了 `mc-prod` 的构建步 ⇒ 该道还需要 fake-harness 镜像（本机由 `runner/image/build.sh` 产，`mc/e2e/e2e_test.go:61-65 imageCommand` 实证）。**属 A32 job 自身返工，不需要新授权。**

## 2. ⚑ 我必须纠正自己先前的一个判断（写进登记，不藏在战果后）
我在推荐 route B 时说"218 有真 systemd + Docker ⇒ runner 能跑 boundary/e2e 拿 Linux 全信号"。**这句话把价值高估了**：这两条道里有 T1/T2 两类**按 macOS/Docker Desktop 语义写死**的断言，换到真 Linux 引擎**不会自动变绿**。
runner 真正买到的东西是（仍然有价值，但不是我说的那个）：① 现场构建验证了双架构链与 build-prod 在 Linux 可跑；② 把 DEFECT-006 这类"只有真 Linux 才暴露"的缺陷逼出来；③ **首次量化了"这些道其实是 macOS 车道"这一事实**。
⇒ 诚实结论：**"A32 的 Linux CI 绿灯"不是一个接上 runner 就能拿到的结果**，它取决于下面选一。

## 3. 需要所有者定的一件事（我不自选，因触及他人钉测）
| 选项 | 内容 | 代价 / 风险 |
|---|---|---|
| **O1 承认定义域** | 把 `docker_boundary`/`docker_e2e` 的**票面定义域**明确为"Docker Desktop（macOS，含 arm64 假设）"；CI 侧把 A32 收窄为**交叉编译 + Linux 纯 Go 道**（不含这两条）；Linux 引擎覆盖另立**跨平台化专票** | 最诚实、零放宽；但 A32 拿不到"Linux Docker 道"信号（那本来就是这些测的定义域外） |
| **O2 把测参数化** | 改他人钉测：包名/架构腿按 `docker info --format {{.Architecture}}` 派生、VirtioFS 断言改为"按共享实现分派（Desktop 用 virtiofs 断言，Linux 用原生 bind 断言）" | 工作量大且**必须逐条保证不放宽**（例如 uid-10002 写保护这条，Linux 上的正确期望是什么要重新论证，不是删断言）；触他人钉测=需明确授权 |
| **O3 只补 T3** | 先只修我自己的（加 `mc-fake-e2e` 构建步），让能跑的尽量跑，红项按 T1/T2 具名登记为"定义域外" | 与 O1 组合最自然；单做则红仍在 |

**执行侧倾向（非裁）**：**O3 + O1 的组合**——T3 我立刻能修（我自己的 job），T1/T2 先**如实登记为"这些道的定义域是 Docker Desktop"**并让 CI 结果按此呈现（不因"预期红"而放宽任何断言）；Linux 引擎真覆盖若你要，走 O2 但作为**独立跨平台化专票**、逐条论证新期望，绝不为凑绿改语义。
给了我就按你的选继续；不给 T3 我不擅自并入（免得 job 呈"半修"状态误导读账）。

## 4. 顺带（与本车道无关，但同属"runner 落地后才看得见"）
隔离方案（`runner-isolation-plan-001.md`）的 **S0 前置已具备**（job 已结束、不再占用共享 daemon）；我按你自己裁的"专用"待你给 O1/O2/O3 方向后一并动，或你令"先做隔离"我就先做 S0–S6（两件事互不阻塞）。

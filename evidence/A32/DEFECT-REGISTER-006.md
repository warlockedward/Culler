# Defect register 006 — 两条 Docker 道的临时根硬编码 macOS 专有路径 `/private/tmp` ⇒ 整个包在 Linux 上开局即死

状态：**REGISTERED → FIXED via this ticket（代码见 §4；Linux 侧验证由 CI 自托管 runner 下一跑产生，本机不可验，具名于 §5）**。

## 1. 症状（自托管 runner 首跑，run 45127097061 / job 137533831556）
`opt-in-lanes` 首步 "Run the container boundary lane" 失败，日志内 **12+ 个不同测试文件同一句**：
> `stat /private/tmp: no such file or directory`

（实读样本：`agent_user_test.go:500`、`attachment_canary_test.go:52/123/140`、`doctor_probe_test.go:67`、`landing_envelope_test.go:105/147/246/302`、`operator_transport_test.go:24`、`production_image_test.go:58/122` …）

## 2. 根因
- `mc/boundarydocker/landing_envelope_test.go:56-64` 的共享 helper `sharedTempDir()` 写死 `os.MkdirTemp("/private/tmp", …)`；**8 个文件调用它** ⇒ helper 一失败，全包所有用它的方法 `t.Fatal` 开局即红。
- 同一模式第二处：`mc/e2e/e2e_test.go:545` `base, err := os.MkdirTemp("/private/tmp", "mc-e2e-")` ⇒ **docker_e2e 道在 Linux 上会以完全相同方式全灭**（本轮未跑到那一步，但代码同形，不靠运气）。
- 动因是真实的 macOS 问题（该文件注释原文）：Docker Desktop 会把曾用过嵌套 bind mount 的 `/var/folders` 目录打上 `com.apple.provenance` xattr，令 `os.RemoveAll` EPERM——**测试体通过、teardown 失败**。
- **⚑ 该注释自称 "Verified both ways rather than assumed" 被本次 CI 首跑证伪**：只验了 macOS 一种方式；Linux 侧从未跑过（fork 首跑之前不存在 Linux CI）。这正是"文档主张≠在位事实"的在册同族（cf. `creds@249` 那次），**记入 §F 更正链**。

## 3. 为什么先前所有检查都没抓到
本机（macOS）跑 `mc/check.sh` 与 boundary/e2e 全道**全绿**——因为缺陷只在 Linux 出现。首跑之前 fork 无 Linux CI；`deployment-linux`（Go 包测）红的是**另一件事**（监督测需 user-systemd，托管 runner 无，另计）。⇒ 一枚**只有真 Linux Docker 引擎才暴露**的移植缺陷，属 runner 上线的直接产出。

## 4. 修复（平台分派，macOS 行为逐字保留）
新增 OS 条件根选择（编译期分文件，避免运行时猜）：
- `_test.go` 辅助：darwin → `/private/tmp`（**原行为与原因注释原样保留**）；linux → `/tmp`（非符号链接、无 provenance xattr 问题，且 `t.TempDir()` 在 Linux 本可直接用，但为保持两道**同一物理根**与既有排障经验，统一用 `/tmp` 显式根）；其他 OS → 明确 skip（Docker Desktop 绑定语义无保证）。
- 两处调用点改为该 helper（`boundarydocker` 与 `e2e`），**不动任何断言、不动清理逻辑语义**。
- 禁触：测试断言、镜像构建、spine/凭据面。

## 5. 验证边界（不假称）
- **本机可证**：darwin 分支编译 + `go vet` 双平台 × 两种 tag（darwin/linux × boundary/e2e 四组全通过）+ 全包 gofmt 净 + **`mc/check.sh` 全道绿（=macOS 行为零回归）**；无 tag 构建的 `build constraints exclude all Go files` 与**基线逐字一致**（该目录文件本就全带 tag，非本次引入）；`t.TempDir` 在本包已有 2 处先例，故 Linux 根选择非新发明；`e2e_test.go:95` 陈旧注释已修。
- **本机不可证**：Linux 上 Docker 绑定/卸载真跑（本机 docker 是 Docker Desktop＝macOS 语义）。⇒ **Linux 侧 EVIDENCED 由自托管 runner 的下一跑产出**；其完成前本件只到 CODED，A31/A32/A02 的延付具名**不解除**。
- **残留未知**：Linux 是否真无同名 xattr 困扰（按 tmpfs/无 provenance 推断，非实测）；若下一跑出现新的 Linux 特有 teardown 问题，本件续记更正段（append-only）。

## 5.1 ⚑ 扫描门对本件**不可用**（分域纪律第 2 类命中：tag-only 目录）
seal `b9cedfea3afcabacf15d218d6b996f18c228987c39fbf41e858737302548d3fc`（551 条，`partial/inconclusive`）：blame-join 落本件改动行 **0**，但**该"0"是空断言**——间接覆盖核验：`mc/boundarydocker/` 与 `mc/e2e/` **整目录 0 个文件有 finding**，而同为 Go 的 `mc/verbs/` 有 6 条（全仓仅 49 个 .go 文件带 finding）。两目录**每个文件都带 `docker_boundary`/`docker_e2e` 构建标签**（默认构建集排除），据此判定**扫描器未解析 tag-only 目录** ⇒ "0 落行"无法区分"干净"与"未扫"，**本件不援引扫描作背书**。
同类边界：`.github/*.yml` 在扫描中 0 命中 ⇒ workflow 改动**在扫描范围外**；其验证=`dashboard` 门 984/0（三守卫实际解析该 YAML）+ 全道绿。
⇒ 本件与 A32 返工的**有效证据链**（不掺水）：① `go vet` 四组（darwin/linux × 两 tag）②gofmt 净 ③`mc/check.sh` 26 包/0/0 ④`dashboard` 984/0 ⑤**真 Linux 运行**（自托管 runner 产；macOS Docker Desktop 语义不可替代）。
**纪律产出**：扫描覆盖判定须按 **①语言 ②构建标签可见性** 双维核——只查"文件是否在覆盖集"仍不够，**tag-gated 文件默认不在任何解析集**。前例=v0.1.90（shell 不被解析）；本例=tag-only Go 目录。

## 6. 同族其它命中（**登记不修**：不在本次 CI 车道，且改它们超出授权面）
| 位置 | 形态 | 为何本轮不动 |
|---|---|---|
| `tests/provider-proxy/leak-controls.ts:10`、`tests/provider-proxy/harness.ts:18` | `mkdtemp("/private/tmp/…")` | 由 `runner/shared/provider-leak-detector.test.ts` 与 `tests/live-model/{proxy,scribe}.ts` 引用；**`resident/check.sh` 的 `bun test` 在 `resident/` 下跑，覆盖不到它们** ⇒ 不在本次 opt-in-lanes 两道内。若在 Linux 上被调用会以同族方式失败 = 潜伏缺陷，留待其所属车道上线时处理 |
| `mc/cmd/mc/culler_prod_test.go:146`、`mc/verbs/environment_cleanup_test.go:11`、`mc/cmd/mc/setup_preview_initial_prod_test.go:67` | `/private/tmp/...` 作为**字符串夹具数据** | 非文件系统操作（是被解析/比较的文本），跨平台无影响 |
⇒ 本票只修**实际阻碍 Linux 车道**的两处（§4）。

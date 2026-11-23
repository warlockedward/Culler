# C01 验证记录 #1（TM §7.1 · L2 Provider Registry 配置链 · 读法 N 落地）

- verification_id: **C01-V-001**
- target_component: `mc/config` 的 L2 Provider Registry 配置链——`runtimes.registry.<id>` 独立命名空间（读法 N，Authority 轮二十七锁定"选 N，R 不采用"）；每条字段闭集 `{base_url, port_allowlist, pooling}`；三 https 闸条目化校验 `validateRegistry`；目的地与凭据解耦（不触碰 `validateCredentialSource` :1242-43 发布集成约束）
- tested_version: 票支 `shz/TC01`（提交=amend 后含 gofmt 修），基线 dev `32b23d7`；改动=config.go（+RegistryEntry/Runtimes.Registry 字段+validateRegistry+net/url import+调用）+ registry_test.go（新）
- 判据（票面 F1）：配置链闭集测试（缺省拒）

## 结果（逐件真实读取）
1. **config 包测**：`./test.sh ./config/` **ok**（含新 `registry_test.go`：1 合法用例 + **12 缺省拒用例** + 端口显式放行 + 空 registry 零回归 + 拒绝原因具体性）。
2. **缺省拒覆盖**（票面核心）：http scheme／userinfo／缺 base_url／loopback(127.0.0.1)／private(10.x)／link-local(169.254.x)／unspecified(0.0.0.0)／端口越 allowlist／pooling 非法／**`insecure_ok` 未知键**（闭集解码器 :601-602 拒）／**`insecure` v0.1 遗留键**（同拒）／非法 entry id。全部 rc=拒。
3. **P1 不变量守**：`base_url`（credentials/bindings 上）**继续禁**（:1226-1228 未动）；registry 目的地=操作者登记、agent 只引用 id；**未放宽**任何现有约束（读法 N 相对 R 的关键优势，正是 Authority 选 N 的理由）。
4. **空 registry = no-op**：`config_version 4` 基例（无 registry 块）测绿 ⇒ 对既有全部配置零行为改动（回归面最小）。
5. **合并前全道**：`mc/check.sh` **`C01_MC2_EXIT=0`**（26 包 / 0 FAIL / 0 UNVERIFIED / gofmt 净）。**⚑ 自记**：全道首跑 `C01_MC_EXIT=1` 被 **gofmt 拦下**（registry_test map 字段对齐），正是"全道绿才并"门生效、dev 全程未受影响；gofmt 归位后重跑绿——诚实记，不粉饰。
6. **扫描门**（分域纪律）：seal `351ba1e17f6a28ee856431911d419ad92b004f4a782b9f5e40f06a7b978184d8`（551 条，partial/inconclusive）；blame-join **落 C01 改行 0**；阳性对照=`config.go` 全文有 3 条既有 finding（证该文件确被解析、join 有效）；`registry_test.go` 全文件 0 finding（纯测试无安全面，**不假称背书**，其证据=config 测绿）。**静态边界，不宣称项目安全。**

## 未做 / 边界（诚实）
- **C01 不含"用 registry 目的地实际连 provider"** = D2 执行面（Destination 完整对象判定/TLS/pooling 执行），属 C02/C04，本票只管**配置链闭集校验**（diff-plan §5 明列、票面 F1 同）。故 C01 达"配置面 EVIDENCED"，不宣称端到端连接能力（那是 C02/C04 延付）。
- registry 被 binding/credential **引用**的运行期解析（agentprofiles 消费 registry id）：本票只加配置校验+结构；若消费面接线非票面 F1 所需，具名为后续（不擅扩）。

- reviewed_by: host-agent（agent E1；非独立复验）
- verdict: **C01 = EVIDENCED（配置链闭集/缺省拒这一票面 F1 全句）**；端到端 provider 连接另计 C02/C04。义务完成判定归 F0a Exit+治理链。

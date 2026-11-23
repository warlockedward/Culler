# A09-S1 · CULLER_* 环境变量前缀迁移 → SHZ_* — diff-plan #1

## 0. 授权
- PENDING-DECISIONS-002 P4（轮二十五，"全按推荐"）＝"准我分步 S1→S5、每步全绿再并开做，S1 先行"。
- 轮三十三 grill-me Q5"全按推进"＝两默认追认（destination_branch 名 `culler` 不改=功能语义；migrate-home 删）⇒ recon-decomposition-001.md §2/§3 两个歧义点已清。
- 本步=S1（最低风险步）：仅 env 前缀，不动 home（S2）/单元名（S3）/二进制名（S4）/schema（S5）。

## 1. 范围（实测，非估算）
- 含 `CULLER_*` env token 的文件=113（42 go / 34 ts / 12 sh / 15 md / 10 py / 其余 sql/html 等），不同 env 名=87。
- **排除面（守界）**：
  - `openwiki/`（3 个 md 命中）＝Scribe 专属维护，Culler 拒绝任务提交改动它（AGENTS.md）⇒ 不动，留给 Scribe 下次更新时自然带入。
  - `evidence/`（5 个文件）＋`docs/facts/`（1）＋`docs/design/`（1）＝时点记录/侦察件，改写=伪造历史 ⇒ 不动（其中引用的 env 名是"当时事实"）。
  - `.git`、已发布产物。
- **功能耦合点（必须同改）**：
  1. `mc/verbs/environment_types.go:177` 前缀过滤列表 `{"MC_","CULLER_","LD_","DYLD_","BASH_FUNC_"}` → `"SHZ_"`（D20 不建兼容层=纯替换不留双前缀；漏改⇒自家 env 不再被过滤出 agent 容器环境）。
  2. `.github/workflows/release.yml` 的 env **生产者**（CULLER_COMMIT/CULLER_RELEASE_OUTPUT/CULLER_RELEASE_PLATFORM/…）与其消费者（release/build.sh、validate_package.py、mc/deployment 读点）同一提交内同改⇒不产生半状态。
  3. `mc/deployment/cli_install.go:279/:282` 的 `CULLER_COMMAND=` shell 探针协议 token（脚本与验证器两侧都在仓内）⇒ 全局替换自然成对。

## 2. 方法
- 对范围内文件做字面 token 替换 `CULLER_` → `SHZ_`（该 token 在仓内语义唯一=env 名前缀，无其他大写撞名——已 grep 核：非 env 语境命中=0）。
- 不重命名任何文件/目录/函数/变量标识符（那是 S4 面）。

## 3. 验证
1. 范围内残留 `CULLER_`=0（grep 复核，排除面单列其"仍含"为预期）。
2. `mc/check.sh` 全链（含 env 读点的单测：host_secret_fixture 的 CULLER_TEST_HOST_SECRET_DIR 接线、environment_types 过滤测试等）。
3. resident/check.sh + dashboard/check.sh（TS 侧 env 读点+三守卫解析 release.yml）。
4. `go vet` 各 tag（fixture 域编译不破）。

## 4. 不做
- 不改 `~/.culler` home（S2）、`com.culler.*` 单元（S3）、`mc`→`shz` 二进制名（S4）、schema/delivery 文本（S5，且 S5 另需 B02 基线冻结窗协调）。
- 不改 openwiki/、evidence/、docs/facts/、docs/design/（见 §1 排除面）。
- 不推任何远端。

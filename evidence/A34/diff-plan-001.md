# A34 边界测试卫生（N1 fixture 身份 + N3 幂等前置清理）— diff-plan #1

## 0. 授权与来源
- 所有者轮三十三（grill-me Q2"全按推进"）：N1/N3 认类＋并入同一卫生票＋**授权修**（触边界测试=独立授权面，已获点名）。
- 归因源：evidence/A32/ci-thirdrun-findings-003.md §4 N1/N3（每行 E1 证据）。

## 1. 改动面（最小集，逐文件）
| 文件 | 改动 | 断言变更 |
|---|---|---|
| mc/boundarydocker/attachment_canary_test.go | ① N1：子测"a Collie run envelope loses host scope before any frame"的 collie envelope fixture 补 `release_build_id`，值=**动态读镜像** `/opt/culler-runtime/release-id`（同型 precedent：mc/deployment/update_checkpoint_docker_test.go:65 `docker run --rm --network none image /bin/cat …/release-id`；不用 4fa3863 的硬编码 "development"，因为 boundarydocker 被测二进制=镜像内 mc-real，release=<commit sha>）② N3：`TestAttachmentCanaryRecoversExactInterruptedAttempt` 的 `docker create --name` 前置 `_ = exec.Command("docker","rm","-f",name).Run()`（同型 precedent：mc/boundarydocker/landing_envelope_test.go:152） | **零**（两个子测的期望串 "host scope only" / recover 语义不动） |
| docs/contracts/DEV-BASELINE.md | A33 行票面收窄（Q3 选项 A）＋新增 A34/A35 行 | — |
| evidence/A32/o1-domain-decision-001.md §3 | "到期"条款改写（软门长期化+收回条件）＋"fast-suite-macos 仍是硬门"句更正（实跑核：该 job 只跑 fast checks，不含两道 docker lane；T1/T2 定义域真跑=本机 macOS lane 执行） | — |
| evidence/A34/diff-plan-001.md | 本件 | — |

## 2. 为什么不是放宽（转写≠放宽自证）
- N1：fixture 变"真"——真实 matched-pair resident 的 collie envelope **恒带** release_build_id（resident/src/effects.ts:637 requiredReleaseBuildId，413880a 起）。补字段=让 fixture 还原真实运行形态，测的本意（Collie tier 在任何 frame 前失去 host scope）**不变且恢复可达**（当前 CI 上该子测根本走不到 host-scope 检查就被 release 检查抢先拒绝）。
- N3：幂等前置清理=测试卫生（确定性容器名+共享 daemon 语义），不改任何断言语义。
- 两处均不动 G-B bind 产品行为、不动检查次序、不动 macOS 侧任何期望。

## 3. 预期红集变化（CI 重派验证用）
- boundary：20 FAIL 行 −2（N1 父测+子测转绿）=18；`TestAttachmentCanaryRecoversExact…` **仍红**但签名=exit-41（T2 族共同根因，S3 run 已实证该签名），撞名签名不再出现。
- e2e：25 FAIL 行不变（根因未动）。
- 两道硬门：绿不变。

## 4. 验证路径
1. 本机（macOS，Docker Desktop）：若本机 mc-prod 镜像+docker 可用，跑 `go test -tags docker_boundary -count=1 -run 'TestAttachmentCanaryPublisher|TestAttachmentCanaryRecovers' ./mc/boundarydocker/`（macOS 侧定义域=修复不得破坏绿）。不可用则 UNVERIFIED 具名，靠 CI 重派。
2. CI：dev ff 后推 ci/routing-base 重派一次（该 run 同时=隔离稳态第 2 次验证）：判据=§3 红集差值。
3. 三守卫不涉（不改 release.yml）。

## 5. 不做（守界）
- 不改 A13 产品行为/检查次序（Q2 明示"不改检查次序"）。
- 不碰 T1/T2 其余断言（那是 A33 面，且 A33 未获代码授权）。
- 不推 main。

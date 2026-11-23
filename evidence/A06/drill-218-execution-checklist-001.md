# A06 恢复演练 · 218 真 docker 臂执行清单（Q4 有界扩权下的逐动作登记）

## 0. 授权边界（轮三十三 grill-me Q4"全按推进"=有界扩权已发出）
允许：scratch MC_HOME + 隔离 XDG_STATE_HOME、专用 docker 栈（`unix:///run/shz-ci-docker.sock`）、交叉编译 amd64 mc 在 218 执行 `__recovery-drill`、凭据经 hostSecretStore（OS 密钥路径，零字面量）。
禁区：他人项目状态（nexusflow 等 ≥8 套）、主 dockerd（`/var/run/docker.sock`）容器与卷、`shzci` 账户面、`build-prod.sh`、任何生产 MC_HOME（218 上不存在 Culler 生产部署=只跑 CI job）。
回退：scratch 域整体 `rm -rf` + 专用栈 volume 删除 = 全案可逆。

## 1. 逐动作清单（每步单动作+通过判据）
| # | 动作 | 判据 |
|---|---|---|
| 1 | 本机交叉编译 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tmp/shz-mc-drill ./cmd/mc`（a06 worktree） | 产物存在、`file` 报 x86-64 ELF |
| 2 | 218 前态实读：主 daemon `docker ps -q \| wc -l`=60、专用栈 `docker -H <shzsock> ps -q \| wc -l`=0、专用栈 images 计数 | 记录进本件 §3 |
| 3 | `scp /tmp/shz-mc-drill root@218:/root/shz-drill-mc`（0755 仅 root 可读目录内） | 218 `sha256sum` 与本机一致 |
| 4 | 218 建 scratch 域：`mkdir -p /root/shz-drill/{home,state}`（0700）；`MC_HOME=/root/shz-drill/home`、`XDG_STATE_HOME=/root/shz-drill/state` | 目录 mode 实读 0700 |
| 5 | 专用栈建 scratch 卷：`docker -H <shzsock> volume create mc-a06-drill-spine` | volume ls 可见 |
| 6 | 卷里种一个真 spine：本机 `substrate.Init` 生成 spine.db（meta 行=32×f uuid+当前 schema）→ scp 218 → `docker -H <shzsock> run --rm --network none --user 0:0 --mount type=bind,src=/root/shz-drill/spine.db,dst=/input.db,readonly --mount type=volume,src=mc-a06-drill-spine,dst=/destination mc-prod:latest /bin/sh -ec "cp /input.db /destination/spine.db; chown -R 10002:10002 /destination"`（同 update_checkpoint_docker_test.go:87 先例形） | 卷内 spine.db 存在且 10002 属主 |
| 7 | 执行演练：`MC_HOME=/root/shz-drill/home XDG_STATE_HOME=/root/shz-drill/state DOCKER_HOST=unix:///run/shz-ci-docker.sock /root/shz-drill/mc __recovery-drill` | rc=0；stdout JSON receipt verdict=PASS |
| 8 | 证据采集：receipt JSON + `MC_HOME/backup-drill/<id>/record.json`（TM §7.1 17 字段）+ `MC_HOME/backups/keyring.json` + `backups/encrypted/*.db.enc` 摘要；scp 回本机 evidence/A06/ | 各件字节落地+sha256 登记 |
| 9 | 负控（防假绿）：篡改 `.enc` 一字节后重放演练的解密臂（或直接跑 `openSHB1` 等价：用篡改件替换后再次 `__recovery-drill` 应 FAIL 且 record verdict=FAIL） | 篡改件被拒=证明 GCM tag 臂真在守 |
| 10 | 零残留收口：`rm -rf /root/shz-drill` + `docker -H <shzsock> volume rm mc-a06-drill-spine` + 删 `/root/shz-drill-mc` | 后态实读：scratch 域不存在、专用栈 volume 回到基线集、主 daemon=60 未动 |

## 2. 时序依赖
- S4 验证 run（45173724893）占用专用栈期间**不执行**本清单第 5–7 步（卷/容器计数会互相污染前后差）——run 结束后执行。
- 第 9 步负控若意外触发真实恢复路径的副作用：演练语义=只读生产卷+只写 scratch 域，无生产副作用面。

## 3. 前态（218 实读，执行前填）
- 主 daemon running=**60**；专用栈 running=**0**；专用栈 volumes=**0**（2026-11-23 21:22 本地实读）

## 4. 后态与残留判定（执行后填）
- 主 daemon running=**60**；专用栈 running=**0**；专用栈 volumes=**0**；`/root/shz-drill` 不存在（21:25 本地实读）⇒ **前后差为零**
- 演练产物（销毁前已取回本机）：record.json＋keyring.json＋spine-20261123T132333Z.db.enc（evidence/A06/218-drill-7cbcb71b/ 三件）；验证记录=verification-record-A06-V-001.md
- 负控实况：NC-1 keyring 指纹篡改=rc1 domain-rejection（"corrupt or was altered"）后 keyring 复原；NC-2 credential-salt 删除=rc1 同拒＋**盐未再生**（目录内仅剩 blobs）⇒ A02 缺盐语义在备份键路径真机成立

## 5. 执行期更正（计划 vs 实况，如实登记）
- **卷名**：本件 §1 第 5/6/10 步原写 `mc-a06-drill-spine`，实跑前发现演练动词的备份面=`RuntimeNames(MC_HOME).Volume`=**确定性派生名**（names.go:26：`mc-spine-<sha256("mc-deployment-v1\0"+canonicalHome)[:6]`）⇒ scratch home `/root/shz-drill/home` 派生 **`mc-spine-f2a82672eb72`**；误建卷 `mc-a06-drill-spine` 已删（从未被产品路径使用）。教训登记：演练域的卷名不由执行者挑选，由 home 身份派生。
- **负控臂**：§1 第 9 步原拟"篡改 .enc 一字节重放"——实跑前发现演练动词每次**新建**备份、不读既有 .enc 件（设计如此），故生产动词面无可回放的"篡改既有件"臂；GCM 篡改拒的机制证据=本机单测 TestSHB1RoundTripAndTamperRefusal（PASS），218 面改证漂移臂（NC-1）与缺盐臂（NC-2）两臂真机成立。

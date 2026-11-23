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
- （待 S4 run 结束后填写：主 daemon 计数 / 专用栈计数 / 专用栈 volume 清单）

## 4. 后态与残留判定（执行后填）
- （同上三口 + scratch 域删除确认 + 主 daemon 容器清单 diff vs S0 基线）

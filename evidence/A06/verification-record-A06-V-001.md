# 验证记录 A06-V-001（218 真 docker 臂恢复演练，TM §7.1 全字段）

- verification_id: 7cbcb71b8208f3790b6cb43fd4a766de
- target_component: spine encrypted backup and recovery drill (A06)
- tested_version: release=41ce8b0（amd64 静态 ELF，sha256 前缀 2dd606b93677689e1914e56b1796f285，双端实读一致）；镜像=mc-prod:latest（专用栈 S3/S4 run 现场构建件）
- environment: host=F1104 platform=linux/amd64 ubuntu 22.04；专用 docker 栈（shz-ci-docker，data-root=/var/lib/shz-ci/docker，shz-br0@10.199/16）；docker CLI server=29.4.0；卷=mc-spine-f2a82672eb72（scratch，确定性派生自 MC_HOME=/root/shz-drill/home）；hostSecretStore=Linux 文件态后端（XDG_STATE_HOME=/root/shz-drill/state/culler-host-credentials：credential-salt＋blobs/Culler-840928b5…/backup-key:dfd018fd….enc）
- attack_scenario: recovery drill——live spine volume 的加密备份→回读→解密→snapshot 只读门→integrity→身份→聚合根抽样；负控两件：keyring 指纹篡改、credential-salt 删除
- expected_result: 解密脊经 settled 门打开，quick_check ok，报告当前 schema/uuid/表数；篡改/缺盐必拒且不再生盐
- actual_result: **verdict=PASS** integrity=ok tables=75 schema=5c56d3132ea33fb85f9204462df94396c9dfb5c4801271b9b6019ff108ba0963 uuid=32×f tasks=0 activity=0；备份件 spine-20261123T132333Z.db.enc=1044577B（=明文 1044480B＋SHB1 头 97B：magic4+version1+ref64+nonce12+tag16）；NC-1 指纹篡改=rc1 domain-rejection "corrupt or was altered"；NC-2 删盐=rc1 同拒＋**盐未再生**（blobs 目录仅剩）
- evidence_ref: evidence/A06/218-drill-7cbcb71b/（record.json＋keyring.json＋spine-20261123T132333Z.db.enc 原件三件）
- evidence_digest: 2f123e156352e646513b8d7e4202c64265a43f9865a12d213ef88f7468156d23（解密脊 sha256）；备份件 sha256=7d6ef381b3153cc3be2f7b4ef86e7dd3fdfe872f918552c54cf78b8f1c04c985
- test_script_digest: mc binary release=41ce8b0（`mc __recovery-drill` 单动词，无脚本）
- execution_id: 7cbcb71b8208f3790b6cb43fd4a766de（2026-11-23T13:23:33Z UTC）
- supersedes_verification_id: （空，首条）
- verdict: PASS（作用域限定=本 tested_version×environment×scenario 组合；旧 PASS 不继承新环境）
- reviewed_by: 执行与记录=实施 agent（兼任显式记录）；人工复核=候（所有者轮三十三 Q4 有界扩权覆盖本演练域）
- verified_at: 2026-11-23T13:23:33Z
- z5_replication: NOT_APPLICABLE（独立信任域通道未实现，TM §2 Z5/D22 待专票）

## 演练域纪律实证（Q4 有界扩权的边界件）
- scratch MC_HOME＋XDG_STATE_HOME 隔离域（/root/shz-drill，0700）；专用 docker 栈唯一 docker 面（DOCKER_HOST 环境变量注入）
- **禁区零触碰**：主 daemon 前态 60 容器=后态 60（实读）；他人项目卷/容器零增减；shzci 账户面零改
- **零残留**：后态实读 main=60 / dedicated_running=0 / dedicated_vols=0 / scratch 域不存在（前态=同值，前后差为零）

## 具名未证面（不扩称）
- GCM 篡改密文拒的**机制级**证据=本机单测 TestSHB1RoundTripAndTamperRefusal（PASS）；218 面证的是漂移臂（NC-1）与缺盐臂（NC-2）——当前演练动词面不读既有 .enc 件（每次新建），故"篡改既有备份件"臂在生产动词面上无真机回放路径，机制证据以单测为准
- resident 周期自动演练集成、DC §7.4 双事件轮换、Z5 通道=具名残留（DEV-BASELINE A06 行在册）

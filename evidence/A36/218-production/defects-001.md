# 218 生产部署轮缺陷登记 · 001（轮六十三）

## D-1 · CLI PATH 验证硬依赖 /bin/zsh＝Linux 无 zsh 时安装器永久拒绝（真机抓取）

- 症状：`install-source.sh --activate` 走到 `mc __onboard home` 拒语 `A new zsh shell cannot resolve shz. Check its shell configuration.`（domain-rejection）。218 实况＝Ubuntu 22.04 root shell=bash，无 zsh。
- 根因：`mc/deployment/cli_install.go` 的 `installCLIPath` 对四个壳环境条目逐一 `verify`（`verifyCLIShell` = 真实 spawn `/bin/zsh -ic/-lic`）。二进制不存在时 spawn 失败＝verify 失败＝编辑循环反而往 `~/.zshrc`/`~/.zprofile` 写无人读的 PATH 导出，终验循环再失败→拒。macOS 默认面（zsh+bash 俱在）掩盖了该假设。
- 定性＝产品缺陷（非品牌面，非本部署配置可绕）：Linux 生产形态下向导 home 段不可达＝A25/A03 系 Linux 端到端前置。既有测试全用 stub verify（读 rc 文件、不 spawn 真壳）＝零覆盖该行为＝缺陷逃逸原因（如实）。
- 修复（最小面）：`verifyCLIShell` 开头对壳二进制做存在性判定，缺失＝视为通过（该壳家族不在本机＝不是操作者的登录环境；不 spawn、不写其 rc）。bash 面验证强度不变；zsh 在场（macOS）行为逐字不变。附带收益：Linux 不再产生无意义 .zshrc。
- 回归测试：新增直接单测＝不存在的壳路径→verifyCLIShell 返回 nil；真实 `/bin/sh`（或宿主必有壳）spawn 失败面保持错误语义。
- 影响面：`mc/deployment`（生产码，已在 host_release 清单）；无 wire/状态机/schema 触面；无文档面变更（拒语措辞不动）。
- 验证：`cd mc && ./test.sh ./deployment/`＋gofmt/vet＋全道后台跑；218 拉新提交重跑安装器第三轮过闸＝真机终证。

## 处置与授权边界

- 本修复属 owner「按建议顺序执行」部署轮内阻塞缺陷（预案 §3 步骤 2 的安装路径自身）＝执行域最小修复，不改任何已裁不变量。
- 拒语文案未动＝AGENTS 终端设置对抗评审门不触发（非 setup 文案变更）。

## D-1 验证记录（2026-11-27）

- 定向：`./test.sh ./deployment/ -run 'TestVerifyCLIShell|TestCLIPath'`＝ok（新 3 面单测通过：缺失壳跳过/在场壳不解析仍拒/解析成功通过）。
- 静态：gofmt 清零＋`go vet ./deployment/` exit 0。
- **全道**：`./mc/check.sh` → `MC_CHECK_EXIT=0`，29 包 ok、零 FAIL、零 UNVERIFIED（含新测试行）。
- 真机终证＝218 安装器第 4/5 轮 `home` 段过闸（spine provisioned＝缺陷路径实战通过）。

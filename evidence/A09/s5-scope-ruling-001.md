# A09-S5 · schema 文本面范围裁定记录（2026-11-24）

recon（recon-decomposition-001.md §2）当时计 schema.sql/delivery.sql 含 culler＝4＋3＝7 token，并标"S5 触 spine 不兼容，需 §2 分支名歧义先定"。Q5（轮三十三）裁定落地后（Git 分支名 `culler` 保留＝功能语义），对三 schema 文件做 E1 复扫（2026-11-24，shz/dev @ 6c9e808）：

## 7 token 的逐 token 终判
| 位置 | token | 语义 | 终判 |
|---|---|---|---|
| mc/substrate/schema.sql:215 | `destination_branch TEXT GENERATED ALWAYS AS ('culler') STORED` | Git 交付目标分支名（生成列默认值） | **保留**（Q5） |
| mc/substrate/delivery.sql:118 | `NEW.destination_branch <> 'culler'` | 触发器谓词（分支语义） | **保留**（Q5） |
| mc/substrate/delivery.sql:170 | `NEW.destination_branch <> 'culler'` | 同上（第二触发器） | **保留**（Q5） |
| mc/substrate/schema.sql:2799 | 注释 "merge of a task into culler" | 注释描述分支语义 | **保留**（Q5） |
| mc/substrate/delivery.sql:356 | 注释 "on the culler branch" | 注释描述分支语义 | **保留**（Q5） |
| **mc/substrate/schema.sql:1** | 注释 "The spine stores **Culler** state" | 品牌散文（非分支语义） | **S5 执行面** → "S-Horizon state" |
| **mc/substrate/schema.sql:2068** | **活文本** `'Culler health: ' \|\| NEW.detail`（alert 通知体写入 spine） | 用户可见通知正文（品牌散文） | **S5 执行面** → "S-Horizon health: " |

## 结论
- **S5 实际执行面＝2 token**（均 schema.sql），非 recon 时代的"7 处"。
- 两 token 均在 `mc/substrate/schema.sql` ⇒ 任何字节改动改变 schema identity SHA-256（AGENTS 不变量：schema.sql/delivery.sql/environments.sql 的哈希＝身份，改动⇒已部署 spine 不兼容且无转换器）。
- **今日零已部署 spine**（本机零＋218 零）⇒ 现在改＝零兼容对象；但票面硬注"schema 文本必须蹭 v1 基线冻结一并改（R2 原则：过 v1 再改＝付费迁移）"⇒ **执行窗＝B02 v1 基线冻结之前**（与 B-chain 同窗，非之后）。
- :2068 是触发器写入 spine 的**数据内容**（非 schema 结构）：若改后已有行持旧前缀＝新旧通知体混杂；零部署⇒无混杂面。
- S5 因此从"候 B02 窗＋候裁定"降级为"**候 B02 预冻结窗的 2-token 机械面**"（无新裁定项；R2 表终值 S-Horizon 已在案）。

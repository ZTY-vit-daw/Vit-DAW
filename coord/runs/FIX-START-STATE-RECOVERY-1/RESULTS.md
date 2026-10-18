# FIX-START-STATE-RECOVERY-1（138）真栈测量腿结果

- 时间：2026-10-18 21:20–21:36 +0800；owner=GLM-5.3 L2 PC ZCode 执行会话
- worktree `D:/Vit_DAW_wt_fxstartrec1`，分支 `port/fix-start-state-recovery-1`（基线 `origin/main`）
- 栈占用/释放：`coord/resources/PC-RUNTIME-STACK.md`（占用 85ba26ff / 释放 70df1976）

## 被测与场景

| 项 | 值 |
|---|---|
| 修复 agent | 本分支构建 `agent/bin/VitAgent.exe` SHA256=991230032C53A799… |
| 基线 agent | `origin/main`（85ba26ff 代码面=445c49ce）构建，worktree `D:/Vit_DAW_wt_fxsr_base`，SHA256=7537AB63779ED818… |
| kernel | stock `D:\Vit_DAW\Export\staging\runtime\VitApp.exe` SHA256=B6565DCF…（只读复用；`VIT_PROJECT_XML` 指向 run 目录副本，stock Workspace 零触碰） |
| 种子 | 真实旧 draft `draft_20260905T134132_3866b080` 的 42MB runtime state **副本**（源文件零触碰；仅改 project_path 指向隔离根 + 植入一条 running continuation + 补 session.json 使旧 session 可被 RecoverWorkingSession 续上——即 118 现场形态） |
| 内核启动 uuid | `vitproj_b3f2838183d64add9ca7824b8b3ac557`（=种子旧 uuid，复刻「内核报上一工程 uuid」） |
| 隔离 | 每腿独立 `VIT_HISTORY_DRAFT_ROOT`（run 目录内，事后裁剪不入库）；不启 UI/vsphub |

## 四腿结果（全部脚本自起自拆，teardown 端口=0、本腿进程=0）

| 腿 | run 目录 | agent | 激活判定 | 关键结果 |
|---|---|---|---|---|
| ① baseline_clean | realstack_baseline_clean_v2 | 7537AB63 | **activated**（认领种子旧 draft，recovered=true conversations=158） | 现象完整复现：uuid 扫描→切旧 draft→续旧 session→**42MB 状态启动期被原地重写**（sha+mtime 变化）；B/C/D 全过 |
| ② fixed_clean | realstack_fixed_clean | 99123003 | **pending**（reason=startup_recovery_disabled_new_session） | 启动零物化；**种子全腿零触碰**（sha+mtime+全树 identical，含 startup/probe POST/project_new/readers）；project_new 恰 1 draft；reader 只读 |
| ③ fixed_sentinel | realstack_fixed_sentinel | 99123003 | pending（同②） | 42MB 文件被脚本全程独占（FileShare.None）：行为与②零差异、agent 日志零 sharing violation → **零 open 反证（读次数=0）** |
| ④ baseline_sentinel | realstack_baseline_sentinel | 7537AB63 | **pending**（恢复未发生） | 阳性对照：独占句柄阻断基线 uuid 扫描读 → 恢复链断（verdict=pending、42MB 未重写）→ 证明③的哨兵机制确实探测读尝试 |

（另有 3 个 v1 中止/迭代目录：_v1script=预检误伤 headless Godot 后修脚本、_v1probe/_v1gate=断言口径修正，JSON 均留档。）

## 计时（agent 进程启动 → 激活判定行出现）

| 腿 | agent HTTP ready | 激活判定 |
|---|---|---|
| baseline_clean | 598ms | **1566ms**（恢复链：root 扫描 + 42MB×2 整读 + unmarshal + 旧 session 续接 + 158 组会话恢复入内存；此后 probe POST 触发 42MB 重写，settle 窗口 ~3.5s） |
| fixed_clean | 526ms | **535ms**（pending 即返；此后零 IO） |
| fixed_sentinel | 544ms | 562ms（与②同量级——哨兵无影响） |
| baseline_sentinel | 563ms | 576ms（扫描被阻断，未恢复） |

- 隔离根仅 1 个 draft（真实根 10,185 个 draft，基线扫描成本会更高）；本差分（1566 vs 535ms）是 42MB 状态处理 + 旧 session 续接的**下界**归因，未含真实根目录行走成本。
- 118 现场 ≈4s 窗口（建 A→42MB 重写）与本腿基线形态一致（重写由请求边界持久化触发）。

## 读/写计数（目标态=0 的证明链）

- **写=0**：②/③ 全腿种子全树 sha256+mtime+size 快照 identical（含 6×500ms quiescence 探针）；`.vit_agent`/`.sessions` 零新增。
- **读=0**：③ 独占句柄（任何进程 open 即 sharing violation）全程持有，agent 行为/日志与②零差异；结构面=唯一按 uuid 读 draft 根的代码路径（RecoverableProjectPathForUUID[InRoot] 生产调用点）已移除；④ 证明该哨兵对基线的读尝试确实可见。

## 118/137 零退化

- 118：`version.project_new` 新增 draft 恰好 1 个、携带新 uuid（②③ B 断言）。
- 137：启动/激活/reader 零物化、零白名单外写入（②③ A2/D 断言）。

## 发现（不在本卡修，上报主管）

**C2 切换窗口旧 uuid 残留**：project_new 后的影子身份滞后窗口内，一次调度 tick 以（进程缓存的新 draft 路径 + 滞后旧 uuid）走满激活，在新 draft 的 `.vit_history/.vit_agent` 物化一行惰性旧 uuid workspace（`migrated_from_legacy: true`），随后自愈到新 uuid（末条 activated=新 uuid）。该签名与 118 ATTRIBUTION §4 round2 记录同族；**基线二进制在恢复被哨兵阻断时同样出现**（腿④）——即凡旧 uuid 无家可归（无恢复重定向吸收）就落当前 draft，非本修复引入。锚点：`agent/internal/chat/server.go` activateCurrentProjectWorkspace 的 CurrentDraftProjectPath 兜底分支 + `agent/internal/history/history.go` BindProjectIdentity 迁移物化；候选修法（待裁定）：同路径不同 uuid 且已有活动工作区时保持 pending 而非跨 uuid 绑定。

## 诚实边界

- 测量在隔离根（1 draft）进行；真实根（10,185 draft）的扫描行走成本未计入基线数字（只会拉大差距）。
- 腿内 `version.project_new` 为脚本直发（前端等价通知形状，137 同款）；未启 Godot UI（UI 面 4 条手测不在本卡义务内）。
- §8：LLM=0（无 chat/LLM 调用；仅 kernel ZMQ 命令 + agent HTTP 面）。
- 种子 42MB 副本仅存在于 run 目录（已裁剪删除，不入库）；源文件（用户真实状态）全程零触碰零改写。

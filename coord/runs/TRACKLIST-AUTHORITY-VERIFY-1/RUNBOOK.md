# TRACKLIST-AUTHORITY-VERIFY-1（255）复验 RUNBOOK：139 缺陷形态在当前 main 的真栈复测

- owner：GLM-5.3 L1 PC 执行会话（ZCode，255 卡会话）；worktree `D:/Vit_DAW_wt_255impl`，分支 `port/tracklist-authority-verify-1`（领取提交 5733f17a=origin/main）
- 复验问题（卡面目标 1）：**当前 main 上，外部新增/导入轨后 tool face 读数是否反映 kernel reply**（139 形态：tool==shadow≠kernel，S1 两轮 tool=0 而 kernel=6）；以及 143 修复（88e88327）+VSP-SHADOW-REFRESH（ee4a991e）三面（A 工具面权威/B 结构 delta 触发/C 去抖刷新）是否在 143 之后三周演进中仍然成立。
- 探针：**原样复用 143 修复验收探针**（`coord/runs/FIX-TRACKLIST-SHADOW-FRESHNESS-1/run_tracklist_fix_probe.ps1` + `tracklist_fix_probe.py`，零改动）——其 PASS 判据即修复契约，A 面 silent-stale/undisclosed 断言直接对映 139 形态。
- 判定映射：
  - **探针 PASS（ps1 exit 0）** → 139 形态不在当前 main（已被 143 系覆盖且存活）→ 走"证据包收口"分叉；
  - **探针 FAIL 且 S1 阶段 tool==shadow≠kernel** → 139 形态仍在（回归/覆盖不全）→ 走修复分叉（按 port/track-list-gap-1 草稿，另行实现）；
  - **探针 FAIL 且形态与 139 不同型** → 如实分列上交（卡面停止条件 1）。

## 1. 被测与可比性

- agent/vsphub=本 worktree 构建（HEAD 5733f17a，代码=origin/main c56a1695）：VitAgent SHA256=`edd851a6e268c0f7f161186403273735285b1109e762a117e215759f42ae1002`，vsphub=`3e43fab18f0f4db662303a8d0d4ca7a4fa4629ce71689af4bbf450cb630fdfc1`（`-SkipBuild` 前提=二进制为本次亲手构建，含当前 main 全部代码）。
- kernel=stock `D:\Vit_DAW\Export\staging\runtime\VitApp.exe` SHA256 前缀 `b6565dcf85d1da86`——**与 139 诊断 run（run_20261018_175229）和 143 修复验收 run（run_20261018_192615）同一字节**，唯一变量=agent 侧代码（139 时代 vs 当前 main）。
- 栈=kernel+vsphub+agent（不启 UI）；berth 纪律、端口面（5555/5556/5557/7878/8787/4445）、run-dir 内核 workspace 隔离、只杀自起 pid——全部由 ps1 驱动自带。

## 2. §8 声明（跑前成文）

- **LLM 调用次数 = 0**（全部确定性：内核 ZMQ 直查 / hub HTTP / agent invoke 只读面+S3 add_track 写面均无 LLM 参与；139/143 两先例同探针均 0）。预算 ≈¥0（卡面 ¥0.5-2 下界之下）。
- **运行次数 = 1**，单 run 无重跑。
- **成功条件**：ps1 exit 0 且 probe PASS（七面全过：sanity 三面 ID+uuid 一致 / A 面 tool≡kernel 全阶段+silent-stale 空+undisclosed 空 / stale 披露双计数一致 / B 面 S1 两轮 heal / 自愈面 / S3 面 / 同帧 VSP ack 链）→ 判"139 形态不在"。
- **失败分类**：`environment_interrupt`（栈起不来/端口占用/进程残留，按 §9 记录后可重建栈重跑一次）；`assertion_fail`（probe FAIL→按阶段读签名定性：tool==shadow≠kernel=139 形态在 / 其他形态=新缺陷面上交）；`capability_blocked`（S3 被 authority 门拒→如实记录，非 S3 断言失败不算缺陷证据）。
- **止损线**：assertion_fail 后不原样重跑（§8 同签名两禁）；取证包（probe_summary+console+raw）上交决策侧裁定分叉。环境中断型可整栈重建重试至多 1 次。
- 计费核对：跑后核对 agent telemetry 无 LLM 调用行（预期 0）。

## 3. 运行命令

```powershell
powershell -NoProfile -ExecutionPolicy Bypass `
  -File D:\Vit_DAW_wt_255impl\coord\runs\FIX-TRACKLIST-SHADOW-FRESHNESS-1\run_tracklist_fix_probe.ps1 `
  -RepoRoot D:\Vit_DAW_wt_255impl `
  -KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe `
  -SkipBuild `
  -RunDir D:\Vit_DAW_wt_255impl\coord\runs\TRACKLIST-AUTHORITY-VERIFY-1\verifyrun1
```

- 工件：`verifyrun1/probe_summary.json`、`probe_console.txt`、`raw/*.json`、`kernel_workspace/Workspace/Logs/`、`agent_last.log`、`vsphub_last.log`、`kernel_vsp_evidence.json`、`teardown.json`、`run_meta.json`（随 port 分支入库）。

## 4. 复验收口件

- 复验输出 vs 139 形态对照表（六面 user-track 口径逐阶段）+ PASS/FAIL 判定 + 分叉结论（在/不在）。
- 分支残余定性：port/track-list-gap-1（ahead5，wt_tracklist1）的 fix 草稿卡与 143 修复的关系（若复验=不在，草稿被 143 超集覆盖→建议关线清理）。

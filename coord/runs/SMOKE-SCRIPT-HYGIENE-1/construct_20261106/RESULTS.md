# SMOKE-SCRIPT-HYGIENE-1（260）构造验证结果

- 执行：GLM-5.3 执行流（ZCode 会话，PC）/ 2026-11-06 晚窗
- 领取基线：`929d6cd9`（领取时 worktree 干净零已有 diff）；领取协调笔：`bfaeb324`（main）
- 实现分支：`port/smoke-script-hygiene-1`（worktree `D:\Vit_DAW_wt_260impl`）
- 验证口径：**零 LLM、零真栈**——所有腿走 %TEMP% 隔离桩仓库（AGENTS §10），不触碰源码树、不启动 VitApp/Godot/agent 任何真实进程；pre-fix 对照版本一律 `git show 929d6cd9:<path>` 可复现。

## 四处修改 × 证据矩阵

| # | 修改 | 腿 | 期望 | 实测 | 证据 |
|---|---|---|---|---|---|
| ① | `scripts/run_c1_frequency_cleanup_smoke.ps1` 三条 go test 后补 `$LASTEXITCODE` 检查 | legA pre-fix + FAIL 桩 | exit 0（假绿复现，257 G3） | **exit 0** ✓ | `legA_prefix_failstub_console.txt` |
| | | legB post-fix + FAIL 桩 | exit≠0 且 throw 点名失败组 | **exit 1 + "failed with exit code 1"** ✓ | `legB_postfix_failstub_console.txt` |
| | | legC post-fix + PASS 桩 | exit 0（绿路径无回退） | **exit 0** ✓ | `legC_postfix_passstub_console.txt` |
| ② | `coord/runs/FIX-TRACKLIST-SHADOW-FRESHNESS-1/run_tracklist_fix_probe.ps1:147` 删除无条件 `$RunDir=""` | legA pre-fix + `-RunDir` | 传入目录被忽略、落到默认时间戳目录 | **传入目录未建 + 默认目录×1** ✓ | `legA_prefix_ignored_rundir_console.txt` |
| | | legB post-fix + `-RunDir` | 传入目录生效（run dir + run_meta + teardown 落位） | **三件齐 + console 点名传入目录** ✓ | `legB_postfix_passed_rundir_console.txt` |
| | | legC post-fix 无 `-RunDir` | 默认时间戳目录路径不变 | **默认目录×1** ✓ | `legC_postfix_default_rundir_console.txt` |
| | | legD post-fix `-PreflightOnly` | 前置守卫面完好、零目录零进程 | **exit 0 + "preflight passed" + 0 目录** ✓ | `legD_postfix_preflight_only_console.txt` |
| ③ | `scripts/SMOKE_TESTS.md` 顶部新增 "Smoke-Run Prerequisites (G7)" 节（正斜杠路径 + 显式 -KernelExe 两前置） | 静态 | 卡面授权"落点执行侧定，最小面"；对齐既有 Draft Root 契约节模式 | **+21 行独立节** ✓ | 实现 diff（`git diff 929d6cd9 -- scripts/SMOKE_TESTS.md`） |
| ④ | `scripts/run_free_state_d1_smoke.ps1` LW/D1-SETTLE 相位门分支化（zero-mutation 记「分支不可达」skip-with-reason；1-mutation 门组零改动） | leg1 post-fix zero-mutation + LW + SETTLE | exit 0；两相位各出完整 skip 判定面；run 到达 PASS 尾 | **exit 0；两 gates JSON `phase_reachable=false`+全门 `skipped_branch_unreachable`+`failures=[]`；`[lw]`/`[d1-settle]` BRANCH-SKIP 行；`D1-S1 PASS` 尾** ✓ | `leg1_*_console.txt` + `leg1_*_loudness_watch_gates.json` + `leg1_*_d1_settle_gates.json` |
| | | leg2 post-fix 1-mutation + LW | 门组照跑、诚实失败、throw（语义零改动） | **exit 1；disclosure=false、failures=2、无 phase_reachable 键、throw 在场** ✓ | `leg2_*` |
| | | leg3 post-fix 1-mutation + SETTLE | 同上（settle 门组） | **exit 1；settle_turn/verdict_on_disk/d1_node_lit=false、failures=5、throw 在场** ✓ | `leg3_*` |
| | | leg4 pre-fix zero-mutation + LW + SETTLE | 258 verifyrun1 形态复现：LW 门 fail + throw 拦断 settle 相位 | **exit 1；LW failures≥1、无 BRANCH-SKIP、`d1_settle_gates.json` 缺席** ✓ | `leg4_*` |

## 汇总判定

- probe1 `ALL PASS: True`（exit 0）；probe2 `ALL PASS: True`（exit 0）；probe3 `ALL PASS: True`（exit 0）。
- 各 probe 机器可读汇总：`probe1_summary.json` / `probe2_summary.json` / `probe3_summary.json`。
- ④ 分支识别依据：runner 报告 `validation.branch=="zero_mutation_boundary"`（256 合法化的 zero-mutation 终端形态；applied 路径无 branch 键）——分支标记缺失/陌生时**落向全门路径**（诚实门，不静默跳过）。

## 桩件与复跑

- 桩（`stubs/`）：`stub_agent_face.py`（7878 HTTP 状态桩 + 5555 裸监听 + godot-root 命令行标记）、`free_state_d1_smoke.py`（runner 报告桩，分支由 `VIT260_STUB_BRANCH` 环境变量选择）、`dev_agent_smoke_stub.ps1`（栈启动桩）。
- 驱动：`probe1_c1_exitcode.ps1` / `probe2_rundir.ps1` / `probe3_phase_branch.ps1`（参数：`-ArtifactDir`、`-PreFixScript`、`-PostFixScript`；probe3 另有 `-StubsDir`）。
- 复跑示例（pre-fix 版本先 `git show 929d6cd9:<path> > %TEMP%\vit260_prefix\<name>` 导出）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\probe1_c1_exitcode.ps1 `
    -ArtifactDir <本目录> -PreFixScript <prefix>\run_c1_frequency_cleanup_smoke.ps1 `
    -PostFixScript D:\Vit_DAW_wt_260impl\scripts\run_c1_frequency_cleanup_smoke.ps1
```

## 桩法已知边界（诚实声明）

- 桩腿证明的是**脚本层的分支调度与判定面产出**，不证明真实栈上 zero-mutation 分支的 LW/D1-SETTLE 门真的不可达——该事实由 258 verifyrun1 真栈工件钉死（`coord/runs/D1-DISCLOSURE-FORENSICS-1/verifyrun1/`：runner exit 0 + `validation.branch=zero_mutation_boundary` + LW 门因 watch hook 未注册而 fail + throw 拦断）。
- probe2 legB/legC 以 exit 1 收尾（kernel 桩 3s 听门超时 throw）——RunDir 语义在 kernel 启动**之前**即已完全落盘（run dir/run_meta.json/console），teardown.json 在 finally 落盘；exit 1 是桩腿的预期形态，非断言对象。
- 桩仓库为通过 run_meta.json 的 `git rev-parse` 腿做了 `git init`+一次提交（真实仓库恒满足此前置）。

## 上交观察（文件域外，不修）

- `coord/runs/TRACK-LIST-GAP-1/run_tracklist_gap_probe.ps1:147`（139 runner，②的复制源）存在**同型** `$RunDir = ""` 无条件清空缺陷。是否同卡追认或另开小卡，交决策侧。

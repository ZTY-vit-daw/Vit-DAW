# FE-GUI-C-FLOATING：前端收缩 Phase C（浮卡/真窗口双档）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据同 91）
- 派发确认：已确认（Phase A 裁定随卡）
- 验收负责人：GLM 主管决策流
- 池序 92；目标仓库=前端仓
- **晚窗参考序①（2026-10-15 早窗 /morning 校准：今日主推=M2 体验场次（用户侧），92 早窗不派发；晚窗若派发按池序自取，92 为 PC 可执行首位——基线 1c61482 已就位，转交提示词待出）**
- 优先级 / 预估 / 依赖：P2 / 预估依规格 / **FE-GUI-B 合入后领取**
- 模型分级：L1 / PC 执行流

## 目标（Phase A 回执划入 C 面的断言⑤后两档）

浮卡与真窗口双档重塑（window_manager 现役三活调用面保留为锚，Phase A 已核）；含走带/底栏抽屉终态收口。

## 文件域与红线

同 91（红线 9 文件零触碰+headless probe 端侧门）。

## 验收标准

规格断言⑤终态逐条钉死+全回归+真栈三件套 exit 0+手测清单[等待用户]。

- 领取（2026-10-18 午窗）：2026-10-18 11:52 +0800 / origin/main=b61a773e77abd3331b913e5f29d1649878049cb9 / owner=**DSH PC 执行流 exec-fe-gui-c**（本机 DSH 团队会话；12:00-18:00 决策流按用户 2026-10-18 午窗指令派发）/ 前端仓分支 **port/fe-gui-shrink-c-floating**（自 port/fe-gui-shrink-b-1@1c61482 建；基线与 2026-10-14-BATCH1 裁定「Phase C(92) 解锁，基线=port/fe-gui-shrink-b-1@1c61482」一致）/ 前端仓 worktree **D:/Godot/project/vit-daw-frontend-fe-gui-c** / 主仓探针分支 **port/fe-gui-shrink-c-probe-1**（自 origin/main b61a773e 建）+ worktree **D:/Vit_DAW_wt_feguic_probe**（新增 scripts/run_fe_gui_shrink_phase_c_probe.ps1+工件；验收通过后由决策侧 cherry-pick，同 91 先例）/ 领取提交=（本提交）。领取核对：卡在 todo+派发已确认（Phase A 裁定随卡）；91 已 pass（2026-10-14-BATCH1）且 Phase B 基线 1c61482 在卷；doing 池 LOUDNESS-B 修订腿文件域（主仓 scripts/run_free_state_d1_smoke.ps1+fs_largeproject_smoke.ps1+coord/runs/LOUDNESS-B-TRIGGER-1/）与本卡前端仓域+主仓**新增**探针脚本互斥（不触既有主仓脚本）；真栈腿串行=同一 PC 栈同一时刻仅一 owner，本卡真栈三件套腿排在 LOUDNESS-B 修订腿释放后（§2.2，决策侧确认窗口）。协作模式（DSH 团队）：协调面（卡片/资源记录/验收）由 12-18 决策流单写，实现面由执行流在独立 worktree 写。**真栈窗口：2026-10-18 12:26 由 Lead 登记 PC-RUNTIME-STACK 当前占用（owner=exec-fe-gui-c，计划 12:30–15:30）并发 GO**；执行口径=RepoRoot 用探针 worktree（写隔离）+`-KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe`（C++ 零改动，记 SHA256）+agent 由工作树 Go 源构建（Go 零改动，记 SHA256）+`-GodotProjectRoot` 指本卡前端 worktree；跑完手动拆栈复核四端口/四类进程=0。
- 回执：（前端仓 commit / 探针结果 / 端测边界声明）
- 验收：（裁定文件）

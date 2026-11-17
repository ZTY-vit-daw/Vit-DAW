# 394-E 候选首次normal诊断：FAIL

- 实际run ID工具生成20261117_133141，开始13:31:41.910+0800；主仓6d592829、新前端a4a6ec5（E/OBS及3字段观测），knownM仅EXR水合。完整原始命令参数见RUNBOOK E附录，本run显式SkipFailureLegs/SkipLegacyLegs，fresh build；旧基础腿/C不重测，主旅程未降门。
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg A -CardId SHELL-WIN-BEHAVIOR-1 -WindowBehaviorChecks -PresentationScenario normal -SkipFailureLegs -SkipLegacyLegs -FrontendWorktree C:/Users/timoz/.codex/worktrees/shell-first-focus-394/vit-daw-frontend -FixtureSource D:/Vit_DAW/temp/f3_empty_project.vit -RunRoot <this>`。
- **内层exit1、外层exit1；主normal第1诊断失败**。native实际失败早于装置终止：13:34:16.336 bootstrap_ui_timeout_or_failure→真实failed；13:34:16.536 Godot退出0。dependent矩阵未跑，唯一剩余诊断须先根因缩小，不原样重跑。

真实GD日志（device/godot_failure_early_copy.log及外层godot_before_restore.log均完整583B）：PID29536、launch_id=`webui_launcher_br_mwk9c7md`，readiness.begin→window_bootstrap.config(feature_enabled=true, initial_mode=1,current_mode=1)，之后没有window_bootstrap.confirmed、direct.begin或mounted/snapshot/ready。native同ID/PID window_bootstrap日志state=`bootstrap_ui_timeout_or_failure`，当时daw_visible=true、daw_iconic=true、daw_noactivate=false、daw_topmost=false、hidden_boot_state同失败、foreground=other。当前不能判prepared成功，亦不能将FOREother说成shell在前；这是新的bootstrap失败，不能冲销旧1301345的真实DAW抢焦点反例。

装置等待处置：normal collector当时只等expected opened，收到真实failed仍等待250s。主管认可后，owner先于13:36:03用只读CDP复制全部WebView消息到device/native_capture_before_abort.json，并保全GD原日志；首次Stop筛选因命令行参数顺序未命中（未终止任何进程）。13:37:02.657再次亲核node20352命令行精确属于本run的vitshell_full_smoke_cdp.mjs后Stop-Process；该CDP退出-1，继而原ps1 catch/finally完成。**本attempt不能声称完整40ms sample数组已归档**（其内存被结束）；原native ring/消息与bridge/GD/真实failed日志已保留。没有新app请求、没有手动kill Godot制造失败，没有主机输入。

future装置小修：已授权在expectopened场景收到真实failed/canceled时立即停止采样、保全capture+GD、抛语义失败，保持消息到达顺序；不改成功判据。node --check/selftest exit0。原attempt不补造partial samples。

卫生终态13:37:37.194+0800：Vit系进程0、TCP7878/7879/8787/5555/5556/9333监听0、UDP4445=0。6快照before/after SHA全一致，fixture源前后AC0CE2E81C12E6F10E3D691CF062CD3E568365C274F12F2B963694AE3B48B41D，recents807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353。无供应商LLM（显式port1，本腿未到旧L3）；原build/hash/HEAD/status、GD/bridge/agentraw、native消息、stdout/exit/hygiene全在ZIP。主管统一释放，执行侧不改main资源。

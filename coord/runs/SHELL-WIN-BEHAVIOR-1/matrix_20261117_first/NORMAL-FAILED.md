# 391组合normal首腿：FAIL，等待原生层裁定

- 主仓HEAD1c104631de4ae68974b338ce18ea0bd0c0daf7dd（组合389/395/装置），前端130134564ade879bf29068ba146b52f9d75ff9f2（已知EXR水合M）；远端占用cb02ee55 owner=/root/entry388，启动前和各实际spawn前fetch+即时净空。
- 完整命令（最终组合主仓根目录）：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg A -CardId SHELL-WIN-BEHAVIOR-1 -WindowBehaviorChecks -PresentationScenario normal -FrontendWorktree 'C:/Users/timoz/.codex/worktrees/godot-entry-396/vit-daw-frontend' -FixtureSource 'D:/Vit_DAW/temp/f3_empty_project.vit' -RunRoot 'C:/Users/timoz/.codex/worktrees/entry-final-validation/Vit_DAW/coord/runs/SHELL-WIN-BEHAVIOR-1/matrix_20261117_first/normal'
```

- **内层exit1、外层exit1**。链路到L1c新建：native接受真实nonce后opened(stage=workspace_ready)，但加载窗口判据失败；未到normal后续管理全旅程。failure三腿/legacy/内核就绪/零绑定已PASS，不得把它们写成391最终PASS。dependent delayed/failure矩阵与LegC均未启动。
- 分类=**真实原生窗口行为反例（超执行文件域）**，不是无观测边界或模型失败。采样0错误，139条；81条active+DAW样本中5条违反前台/order/style。数据是有限40ms采样+事件，不外推所有瞬态。

## 原始锚点

`normal/device/r1_cdp_chat_new.json.presentation-partial.json`：

- native消息index6、status样本index17：observed_at_tick_ms=91521968，guard=true、hooks=true，shell_hwnd6555976、daw_hwnd265808、foreground=daw、shell_above_daw=false、daw_noactivate=false。395同样诚实呈现data-native-focus=daw、当前request seq3/pending。
- native index7 tick91522015已改order/style为true，但foreground仍daw；index8 tick91522359才shell前台，较首违规观察+391ms。原生事件补救不能使已有反例变成PASS。
- 同一真实request=`webui_launcher_br_mwk7qje9`；native seq4 opened stageworkspace_ready、vit_launcher_result opened与最终guardfalse release_reasonworkspace_ready均有原记录。只证明native阶段/释放进度，不替代未完成的真实GD时序验收。
- 原bridge `r1_bridge.log`含同nonce launch_result.accepted及opened；没有错nonce注入或合成UI负例。本腿未产生供应商LLM，显式本机port1 override，telemetry未生成；没有任何主机键鼠/SetForegroundWindow测试动作。

## 证据缺口与装置修复

CDP在拷Godot默认raw前先判窗口失败，外层随后按卫生还原Godot默认log，导致本attempt当前nonce的Godot原日志未被保全（目录搜索亦无该nonce）。故不能声称mounted→snapshot_applied→ready时序已验证。已补装置：完成捕获后在任何assert前先保全raw；fail()也尽力保全；外层还原Godot log前另保存godot_before_restore.log。原失败工件不补造日志、不改结果。node --check、静态日志parser self-test、PowerShell AST与diffcheck均exit0。

## 卫生与交接

- 2026-11-17 12:50:29 +0800 owner亲核Vit进程0，TCP7878/7879/8787/5555/5556/9333监听0，UDP4445=0。
- wrapper hygiene：6个快照前后hash一致（recents+Godot默认log+两个root的Settings/default XML）；fixture前后`AC0CE2E81C12E6F10E3D691CF062CD3E568365C274F12F2B963694AE3B48B41D`。recents终态`807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353`。
- VitShell SHA256=`CD7F42F76D1CE49ACFB735D06F0E1F43CEB1E0089F328D0529CF8E8F284F44A2`；agent=`1E6925CDF7481D90CCC2581BE6BA86BA3F89C98EC8A4C9899EA77449BBE6DCAC`；kernel/hub仍A5947361…348E/B10CC412…1F68，完整值见meta。
- 保留原stdout、inner/outer退出码、meta、bridge/raw-agent、全部partial采样/消息、hygiene、构建hash/HEAD/status于normal ZIP；原生脚本无d1report，未伪造。停止新run，未改C++/产品或窗口判据；原生修订/覆盖裁定由主管负责。

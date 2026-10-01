# FS-STOP-APPLY-1 真栈烟测工件（2026-10-01 19:33 run）

- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/run_free_state_d1_smoke.ps1 -RepoRoot D:/Vit_DAW_worktrees/fs-stop-apply-1 -SkipBuild -KernelExe D:/Vit_DAW/VitApp/build/VitApp_artefacts/Release/VitApp.exe -PublicManifest D:/Vit_DAW/temp/semantic-processor-agent-project-smoke-v1/fixtures/semantic_processor_project_smoke_v1_80085263a651cf20/fixture_manifest.json -StopSemanticsProbe`
- 退出码 0；报告 `status=stop_semantics_probe_pass`（本目录 d1_smoke_report.json）
- 被测二进制（§9）：
  - agent = worktree HEAD 6a338479 自建 `agent/bin/VitAgent.exe`（SHA256 DA7B372488839C38B09006A44BCB32E101E5F2112FAFDFD137A101F47ECB0902，2026-10-01 19:32:28）；worktree 领取时基线 origin/main=701e2c27，实现 commit 28551ef2
  - kernel = 主仓 `VitApp.exe`（SHA256 78DC4889921204472DDCA365FFA1915287EF8C5D69BD7BA8C91C68463AD0631F，2026-09-29 19:37:42）；本卡零 C++ 改动（`git diff 701e2c27..6a338479 -- VitApp/` 为空）
- 一次启动失败（环境中断类，未入栈）：bash 反斜杠引号吃掉 `-File` 相对路径，PowerShell 报「-File 形式参数的实际参数不存在」；改正斜杠绝对路径后成功。无功能性结论受影响。

## 探针证据摘要（d1_smoke_report.json → stop_semantics_probe）

- parked goal_53d57021f9e31a6d（run_32eb045f07c1dc72，round=user_judgment_pending）→ POST /agent/turn/stop
- goal 诚实 stopped（响应+持久投影）；closure → **fs9_terminal**，settlement reason=**cancelled**（非 satisfied——诚实停因）；controller owner → **settled**（释放）
- 停止后 project revision 不动（revision=4，零新干预）；锚点行在 agent_last.log：`[turn.stop] request received conversation=d1_s1_20261001T113429103434 goal=goal_53d57021f9e31a6d run=run_32eb045f07c1dc72 reason=user_stop prior_status=completed actively_executing=false`
- 新输入 → 新 goal_935f529d58d5f602 / run_61ca87d44fa494f7 +回复落地（M1 18:35 楔死面 green）

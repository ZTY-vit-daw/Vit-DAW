# 独立 Leg C（389）回执

- 目录名`entry389_C_independent_20261117_1335`是opaque run ID；真实启动UTC=`2026-11-17T05:05:15.8997076Z`，完成=`2026-11-17T05:06:51.8276129Z`。
- 命令：`powershell -NoProfile -ExecutionPolicy Bypass -File ./scripts/run_entry_management_acceptance.ps1 -Leg C -CardId SHELL-WIN-BEHAVIOR-1 -FrontendWorktree C:/Users/timoz/.codex/worktrees/godot-entry-396/vit-daw-frontend -FixtureSource D:/Vit_DAW/temp/f3_empty_project.vit -RunRoot <this>`。
- 内层`ui_entry_smoke.ps1` exit0，外层C exit0；仅389旧入口路径，不开`WindowBehaviorChecks`、不设402 smoke env、供应商预算0。不证明VitShell窗口焦点、394-E bootstrap或后续normal矩阵。

事实断言：failinject无agent时React窗可见、Godot主窗不可见且零restore；autoload真实open，Godot主窗恢复，`child_hwnd=397052`跨入口不变，`navigate_count=1`、page_url不变、scene_added/project_stage/surface.ack齐备；legacy进程存活且无boot/hidden行。395 UI由该真实页面入口挂载回归覆盖。

卫生：六份快照（recents、Godot默认log、两个root Settings.xml/default_project.xml）before/after hash一致；fixture前后`AC0CE2E81C12E6F10E3D691CF062CD3E568365C274F12F2B963694AE3B48B41D`；recents`807226F4AF3FC85C2B99BFD2FB827C1583E7C991826BDE5F0B4344A6DC13E353`；13:07:09 owner亲核Vit进程/TCP7878,7879,8787,5555,5556,9333/UDP4445全零。构建agent SHA`1E6925CDF7481D90CCC2581BE6BA86BA3F89C98EC8A4C9899EA77449BBE6DCAC`。

工件：`device/artifacts/ui_entry_smoke_report.json`、run根目录`stdout.log`/`hygiene.json`/`teardown_owner.json`及完整wrapper run ZIP。失败分类无；此次唯一C有效run exit0。此腿不解锁normal后续matrix。

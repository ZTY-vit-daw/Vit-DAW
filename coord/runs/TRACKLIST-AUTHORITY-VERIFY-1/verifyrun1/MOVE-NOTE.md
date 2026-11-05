# MOVE-NOTE（工件迁移留痕）

- 本 run 实际执行时命令含 -RunDir D:\Vit_DAW_wt_255impl\coord\runs\TRACKLIST-AUTHORITY-VERIFY-1\verifyrun1，但 run_tracklist_fix_probe.ps1:147 在 try 块前有 ` = ""` 无条件清空参数（脚本固有缺陷，143 先例均用默认路径未暴露；参数绑定经独立 argtest 证明传入正常），工件因此落在默认路径 coord/runs/FIX-TRACKLIST-SHADOW-FRESHNESS-1/run_20261105_205547。
- run 结束后（20:57）原样移至本目录 verifyrun1/，未改任何工件内容；run_meta.json 记录的 started/repo_root/各 SHA256 为执行时真实值，worktree_head=b619c5b8（占用登记提交）。
- 该偏差只影响工件落点，不影响被测栈与判定有效性。

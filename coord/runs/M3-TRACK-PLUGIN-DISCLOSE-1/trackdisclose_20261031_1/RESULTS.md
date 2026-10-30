# M3-TRACK-PLUGIN-DISCLOSE-1（228）真栈腿结果——trackdisclose_20261031_1

- 时间：2026-10-30 22:29–22:31 +0800（登记 22:26=874a4afe 在 main；占用前 22:2x 重读登记面+进程端口全零亲核；登记-启动间隔内 §2.2 增补即时复核过）
- 完整命令：

  ```powershell
  powershell -NoProfile -ExecutionPolicy Bypass -File "D:\Vit_DAW_wt_228\scripts\dev_agent_smoke.ps1" -StartKernel -RestartAgent -TrackPluginDiscloseSmoke -NoChatSmoke -NoStripSilenceSmoke -KernelExe "D:\Vit_DAW_wt_228\VitApp\build_release\VitApp_artefacts\Release\VitApp.exe" -RunArtifactsDir "D:\Vit_DAW_wt_228\coord\runs\M3-TRACK-PLUGIN-DISCLOSE-1\trackdisclose_20261031_1"
  ```

- 退出码：**ps1 exit 0**（"dev agent smoke completed"；腿内断言体 `track_plugin_disclose_smoke.py` exit 0）
- 被测：worktree `D:/Vit_DAW_wt_228` @ `port/m3-track-plugin-disclose-1`
  - 内核=worktree 自建 `VitApp/build_release/VitApp_artefacts/Release/VitApp.exe` sha256 `f9e3da0a…`（本卡改内核 C++，非主检出 C1E32DFE 只读复用形态）；运行前后零漂移
  - agent=ps1 `-RestartAgent` 在 worktree 根 `go build` 自建（含本卡 `free_state_observation.go` 白名单同步）=同源
  - 测试时 HEAD=`874a4afe`（协调基线）之上未提交实现 diff（见卡面回执 diff 锚点节）；工作树=本卡文件域内新增/修改
- 腿结果（摘要工件 `track_plugin_disclose_summary.json`，status=pass）：
  - **内核 VSP 紧凑快照腿**：6 可见轨全部携带数组型轨行 `plugins`（schema 稳定/§11 缺省空数组语义）；全部 6 轨披露 `type=volume` 条目——样例轨 1007（bass）→ `{"id":"1008","name":"Volume & Pan Plugin","type":"volume"}`（轨级 Volume&Pan 的 plugin_id 在紧凑面可见=222 次生面闭合）；每条目仅 id/name/type 最小集（enabled/参数零出行，payload 防膨胀）
  - **agent CCB project.structure 腿**：`project.state` 触发 shadow 刷新后，`ccb.observation_request view_ids=["project.structure"]` 披露 6/6 轨行 `plugin_count>=1`+`plugin_chain_summary`（样例 `plugin_id=1033`、type=volume、role_guess=utility、slot=1）；disclosure 14,660B ≤ 65,536B 预算；视图 payload 内 `"plugins"`/`"params"` 全量面键零泄漏（bounded disclosure 红线核对）；bundle `read_only=true`/`mutation_authority=false`
- 计费：零 LLM 只读腿（零调用）
- 隔离亲证：run 目录 `default_project.xml`（git 干净副本）mtime 22:29:45=运行窗内承接内核写入；主检出 `D:\Vit_DAW\VitApp\Workspace\default_project.xml` mtime 19:58:35=登记前遗留零触碰；`gui_project_isolation.json` 标记在案
- 拆栈：owner 手拆 VitAgent 30672 / VitApp 38080（pid 路径亲核=本腿所属）→ 复核 VitApp/VitAgent/VspHub/node=0、TCP 5555/5556/7878/8787/18787/5178 listen=0、UDP 4444/4445=0；释放记录随 22:31 条目上 main

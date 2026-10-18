# FIX-START-DRAFT-SIDE-EFFECT-1（137）真栈腿 RUNBOOK

- 分支：`port/fix-start-draft-side-effect-1`，worktree `D:/Vit_DAW_wt_startside1`（基线 origin/main `77a0b02f` + 本卡改动）
- 被测 agent：`D:/Vit_DAW_wt_startside1/agent/bin/VitAgent.exe`（本分支构建，SHA256 由脚本记录）
- 被测内核：`D:/Vit_DAW/Export/staging/runtime/VitApp.exe`（主检出 stock，SHA256 脚本记录）
- 栈占用：需 Lead 在 `coord/resources/PC-RUNTIME-STACK.md` 登记（本卡 owner=exec-start-draft，task-12）后发 GO；本执行流未启栈。

## 启动方式（脚本自起栈、finally 只杀自起 pid）

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File D:\Vit_DAW_wt_startside1\coord\runs\FIX-START-DRAFT-SIDE-EFFECT-1\scripts\run_stack_leg_137.ps1 `
  -AgentExe D:\Vit_DAW_wt_startside1\agent\bin\VitAgent.exe `
  -KernelExe D:\Vit_DAW\Export\staging\runtime\VitApp.exe `
  -RunDir D:\Vit_DAW_wt_startside1\coord\runs\FIX-START-DRAFT-SIDE-EFFECT-1\realstack_<UTC时间戳> `
  -KernelHelper D:\Vit_DAW_wt_startside1\coord\runs\FIX-START-DRAFT-SIDE-EFFECT-1\scripts\kernel_zmq_command.py
```

脚本行为：
1. 自检：六端口（7878/5555/5556/8787/4445/19876）监听=0、VitApp/VitAgent/vsphub 进程=0；用户 Godot 编辑器允许（记录 pid，Game 实例则中止）。
2. 隔离根 `<RunDir>/ProjectHistory/drafts`，预置与**内核启动工程 uuid**匹配的种子旧 draft（runtime state 指向种子，无 BOM）。
3. 自起 stock kernel（工作目录=exe 目录）与分支 agent（env `VIT_HISTORY_DRAFT_ROOT=<隔离根>`，agent 日志落 `<RunDir>/agent_last.log`）。
4. 等启动恢复/激活认领种子旧 draft；断言 **A) 启动/激活阶段新增 draft 目录=0**（旧行为=1 个 75B 空壳 A）。
5. 触发自主 reader 面（`version.list`、`version.worktree_list`、`/agent/state`）并等待 reader 窗口；断言 **D) draft 根全树（路径/内容 sha256/mtime）零变化、零新增目录**。
6. 内核 ZMQ `new_project`（新 uuid）→ agent `/agent/invoke version.project_new(新uuid)`；断言 **B) 新增 draft=恰好 1 个**（旧行为=2：A+B）、新 draft 含新 uuid；**C) 新 draft 无旧内核 uuid workspace 泄漏、种子 draft 保留旧 uuid**。
7. finally：只杀自起 agent/kernel pid；复核六端口监听=0、本腿进程=0；结果 JSON `<RunDir>/stack_leg_result.json`（含 prestart/startup/reader/project_new 快照、assertions、teardown、二进制 SHA、HEAD/status、reader 回包、refused-rebind/activation-pending 日志计数）。

## 验收口径

- **通过** = 脚本 exit 0 + `stack_leg_result.json.assertions` A/B/C/D 全 true + `teardown.ports_listening=[]` 且本腿进程=[]。
- 回执记录：run ID（RunDir 名）、agent/kernel SHA256、HEAD、四断言实测、reader 树 diff=0、新 draft uuid、拆栈证据、偏差。
- §8：LLM=0（本腿无 chat/LLM 调用；仅 kernel 命令 + agent 生命周期/历史读工具）。
- 失败分类：environment_interrupt（内核启动失败/端口被占）/ assertion_fail / script_exception；止损=同签名 assertion_fail×2。
- 不启 Godot UI；UI 类 4 条手测保留用户排期。

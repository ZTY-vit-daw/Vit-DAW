# RACK-PARAM-PERSIST-FIX-1 真栈 probe RUNBOOK（§8 跑前冻结）

- 卡：RACK-PARAM-PERSIST-FIX-1（321）；owner=ZCode GLM-5.3 L2-worker PC 执行子代理
- 冻结时间：2026-11-09 凌晨（栈登记前）

## 一、被测物

- 内核：本 worktree 构建 `VitApp/build_release/VitApp_artefacts/Release/VitApp.exe`
  （D:/Vit_DAW_wt_321persist @ port/rack-param-persist-fix-1；含两腿修复：
  写时 rack flush + 参数 override 写入；保存路径 rack 遍历 flush）
- agent：同 worktree 构建（dev_agent_smoke -RestartAgent，main 源零改动）
- probe 脚本：320 forensic probe **逐字复用**（三模块 sha256 钉死）：
  - forensic_persistence_probe.py = 6237e2c1fd3f4c3f0c0c2c64b4b6756930e66edebc8b335ed13e8bfa9ba1ad8a
  - proq3_free_cases.py = 75cd56e5671c66113d3c2cfb5da088dffe3389d8dd328a46e7750a8b464a12d5
  - proq3_free_driver.py = 485a01f5340068d001d8fdff8b6a44cc7c9b45240ac0a77fc353601e1d224264
  （副本在本目录，与 320 原件字节一致）

## 二、§8 口径

- **运行次数**：1 次栈窗口。窗口内序：起栈 → 布位 r1（placement_r1.py，
  320 driver.place_project_copy 复用）→ forensic probe ×3（逐次独立报告，
  每次=完整 写0.7→save→open→回读 周期=「三次重载稳定」证据）→
  工件收集 → 布位副本清除 → 拆栈亲核 → 释放登记。
- **零 LLM**：全部腿走 kernel ZMQ + agent HTTP 命令面，无模型调用，零计费。
- **成功条件**（修复验收）：
  1. 三次 probe 全部 `arm_a_rack_internal_proq3.persisted == true`
     （重载后 0.7±0.02）；
  2. `arm_b_track_volume.persisted == true`（track 级回归维持）；
  3. 探针退出码 0。
  注：probe 的 discriminator.shape 语义是**缺陷形态分类**（a回退+b保持
  ="rack_internal_save_flush_gap"）；修复后 a 不回退 + b 保持会落
  "unexplained" 桶——这是脚本预期行为，验收以两个 persisted 布尔为准。
  probe restore 段 initial_digest 硬编码 320 run3 的 5080116bec1afcfa；
  本窗口布位 digest 以 placement_r1.json 实测为准，restore_matches_initial
  对账用真值。
- **失败分类**：
  - F1 布位失败（registry 解析/装载/rack_add_node 拒绝）→ 装置环境类，
    按 §9 记录；
  - F2 probe 脚本异常 → 装置缺陷类；
  - F3 arm_a.persisted=false（修复无效）→ **取证优先**：解密保存后 .vit，
    dump RACKS→PLUGININSTANCE→PLUGIN 节点的 state/parameters 属性，判别
    失败层（chunk 有无 0.7 / override 有无落树 / 两者都在但加载侧问题）；
  - F4 arm_b.persisted=false → 回归，取证；
  - F5 栈环境故障（端口/进程/构建产物缺失）→ 环境中断，仅在有原始日志/
    退出证据时排除出有效轮次。
- **止损线**：同代码版本 probe 连续 2 次同型失败（F3/F4）→ 停止，取证上交；
  环境重建上限 1 次（320/312/318 先例）；止损只停止投入，不改写结论。
- **单 run 无重跑**：窗口内 probe×3 是预先声明的重复测量设计（稳定性证据），
  非失败后重试；失败后的任何再跑须先按失败分类处置。

## 三、栈协议全链（PROTOCOL §2.2）

1. 净空亲核（进程+端口全零）；
2. 占用登记推 main（coord/resources/PC-RUNTIME-STACK.md，卡 321）；
3. ≤60s 复核（重读记录+进程/端口即时核）；
4. 起栈：`dev_agent_smoke.ps1 -StartKernel -RestartAgent -NoChatSmoke
   -RepoRoot D:\Vit_DAW_wt_321persist -KernelExe <本卡构建> -AgentHttp
   http://127.0.0.1:7878`；
5. 运行（见 §二 序）；
6. teardown 亲核（TEARDOWN_ALL_ZERO：VitApp/VitAgent/godot/python 进程
   +端口 7878/5555/5556 等全零）；
7. 释放登记推 main（含 run 工件指针）。

## 四、收尾卫生

- 布位副本（%APPDATA%\Vit-DAW\projects\实验·ProQ3 裸控 r1.vit）验收读数
  收集后清除，projects 域恢复卡前状态（ledger+既有工程）亲核；
- worktree 运行时产物（VitApp/Workspace 运行态、日志）不进 commit；
- 工件全量落 stack_persist_probe/ 子目录。

# PROBE-SHRINKC-MIXERSYNC-1 复跑取证预声明（§8，跑前锁定 2026-10-24 18:28）

## 运行设计

- 被测：前端仓 shrink_c 探针 `tools/probe_fe_gui_shrink_phase_c.gd`（worktree D:/Godot/project/vit-daw-frontend-shrinkc @ port/probe-shrinkc-mixersync-1 @ ee8fa0e，本卡零改动基线）。
- 驱动：Godot 4.6.1 console headless 直调（`--headless --path <worktree> -s tools/probe_fe_gui_shrink_phase_c.gd`），import 缓存预热一次。
- 工件：本目录（主仓工件 worktree），随 port/probe-shrinkc-mixersync-1 入库，不落主检出盘。

## 腿与轮次

- **Leg A（无栈基线，隔离复跑 ×3）**：不起任何栈进程，探针独跑 3 轮（新 Godot 进程/轮）。
  - 预期：全 PASS（静态分析：注入链全同步，无栈时无回写源）。若 FAIL → 确定性缺陷直接定位。
- **Leg B（活栈复现，≥3 轮）**：真栈 kernel+agent（+vsphub）在栈，探针独跑 ≥3 轮。
  - 预期：复现 166 签名（mixersync add/delete/fader 腿 1-2 腿/轮间歇挂）。
  - 机理假设（跑前登记）：活栈下探针进程 VitTelemetryManager 绑定 UDP 4444 收到真电平流 → 真 track id 集 ≠ 注入基线 → `_maybe_detect_track_structure_change` 调度 resync → `vit_refresh_project_from_kernel`（VSP/IPC 双传输）→ `_apply_kernel_project_reply_and_emit(内核真状态)` → `project_loaded` → `_rebuild_strips` 用内核真轨集覆盖探针注入态，落点与探针 apply→check 3 帧窗口竞速 → add/delete/fader 腿间歇挂。
- 轮次预算：Leg B 若 3 轮零复现，加跑至 6 轮；6 轮仍零复现 → 停盲跑，转插桩探针定位（止损线）。

## 成功条件与失败分类

- 取证成功=Leg B 复现 ≥1 次 mixersync 腿 assertion_fail（捕获失败腿原始输出+detail 字典）；或 Leg A 复现（确定性缺陷）。
- 失败分类：environment_interrupt（栈起不来/端口冲突/183 撞栈）/ probe_infra（import 失败/装载失败 exit 2）/ assertion_fail（mixersync 腿，取证目标）/ no_repro（预算内零复现）。
- 止损：同签名 probe_infra ×2 → 停；Leg B 6 轮零复现 → 转 §11 known flaky 评估路径（须隔离+整包+diff 三证）。

## LLM 记账

- LLM=0（探针与确定性栈，无 chat 腿）。

## 栈与隔离

- kernel=主检出 K3（VitApp.exe SHA256=722A79D7…，起栈前亲核）只读，VIT_PROJECT_XML 指 run 目录 default_project.xml 副本+独立 kernel_workspace（JOURNEY 隔离形态），主检出 Workspace 零触碰。
- agent=主检出 Go 树（4c7736c9=c1285d68 Go 零差异）新构建，二进制落 run 目录，SHA 记录。
- 与 183（FE-SEND-LEVEL-FADER-1）错峰：登记占用成功推 main 后才起栈；183 已登记则等待。

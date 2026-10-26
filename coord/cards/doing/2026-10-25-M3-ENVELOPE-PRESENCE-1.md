# M3-ENVELOPE-PRESENCE-1：get_plugin_parameters 回执扩包络在场性字段（M3 步骤③支撑卡）

- 发卡：GLM 主管决策流 / 2026-10-25 晚窗（依据=用户 2026-10-25 包络形态设计裁定「显示时机=按需+手动混合」——按需自动展开腿需在场性数据源；186 标注点②案①在此最小面复活：仅 point_count 字段，不做 supports_automation 实判大面）
- 派发确认：已确认（用户形态裁定链）
- 验收负责人：GLM 主管决策流
- 池序 190；目标仓库=**主仓 D:\Vit_DAW**（VitApp C++ 域）；P3 / 0.25 天 / 依赖=无（M3-KERNEL-1 曲线命令已合入 ✓）；与 185/188/189 在飞文件域不相交（VitApp/Service 独占）
- 模型分级：L1 / PC 执行会话

## 目标

`get_plugin_parameters` 回执的每参数描述符行新增 **`envelope_point_count`**（int，0=无包络）：

1. **内核侧**：参数描述符构建循环处（PluginRackControlService.cpp 内 get_plugin_parameters handler；值来源=`param->getCurve().getNumPoints()`——M3-RECON 已锚 getNumPoints 同对象面 :2066/:2200/:2228/:2245）；插件参数与轨级参数（volume/pan 等）统一覆盖。
2. **兼容**：回执 JSON 纯增列——旧前端消费面（rack_control_layer_rules.gd:83 描述符解析）忽略未知键，零破坏；不触碰既有字段语义（AGENTS §11）。
3. **前端消费**：本卡不改前端；消费归 189（自动展开腿：刷新链拉取 get_plugin_parameters→任一参数 count>0→车道自动展开）。
4. **agent 侧**：零改动（catalog/描述符投影不涉；DAD 曲线回流扩型仍按 186 裁定②后置）。

## 文件域

`VitApp/Source/Service/PluginRackControlService.cpp`（描述符构建段单点）；冒测脚本扩展按 M3-KERNEL-1 模式（scripts/ 域——领取时与 188 在飞核对，若冲突则复用既有脚本改参数不新增文件）。越域即停。

## 验收标准

- 内核构建 exit 0；真栈冒测（M3-KERNEL-1 冒测模式扩展，§2.2 登记+185 裁定③三纪律）：装载插件→automation.set_points 三点→get_plugin_parameters 断言该参数 `envelope_point_count=3`→automation.clear_region→断言 `=0`；未动参数行字段在场为 0；既有字段全量零改动断言（diff 级）；
- 工件入 coord/runs/M3-ENVELOPE-PRESENCE-1/；端测边界声明。

## 停止条件

- **主管补注 2026-10-25（189 验收披露折入）**：活内核实证 get_plugin_parameters 对**内置轨级插件（volume 等）回 error only available for external plugins**（189 run L1 逐字在卷）——本卡在场性字段的自然覆盖面=**外部（rack VST）插件参数**；内置参数不在该回执面属内核既有形态而非本卡缺陷。实现时：外部插件参数行加 envelope_point_count；内置参数（volume/pan）在场性缺口如实登记边界并上交（自动展开对轨级参数的覆盖留主管另裁数据源——候选=轨级 automation.get_curve 探测或内核轨参数面小扩，不在本卡擅自扩）。

- get_plugin_parameters 描述符构建点与 M3-RECON/186 锚点族不符（内核已重构）→锚点上交；
- getNumPoints 对某参数族（轨级 volume/pan）不可达（无曲线对象挂载）→如实回 0 并记录边界，不硬造。

- 领取：（2026-10-26 18:36 +0800 / origin/main=6fc7b57ad6996c142c2cab8bf18ee524ea579ed4 / GLM-5.3 PC 执行侧 ZCode 会话（本卡会话，用户口令指定领取卡 190）/ 分支=port/m3-envelope-presence-1（worktree D:\Vit_DAW_wt_envp1）+ 协调 worktree D:\Vit_DAW_wt_envp1_coord（coord/envp1-claim）/ 领取提交=本提交。领取时核对：卡在 todo 派发已确认（用户形态裁定链）、依赖 M3-KERNEL-1 已合入、doing 池唯一卡 GEN-SUNO-WEB-DESIGN-1 文件域=coord/runs 零生产代码不相交、scripts/ 域无 doing 占用（复用既有脚本优先）；执行 worktree 基线 HEAD=6fc7b57a、git status --short=空）
- 回执：（commit / 冒测 run ID / 字段覆盖面说明 / 端测边界声明）
- 验收：（裁定文件 / 验收 commit）

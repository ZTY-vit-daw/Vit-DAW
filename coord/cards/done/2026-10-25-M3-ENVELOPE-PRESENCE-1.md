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
- 回执：（**实现 commit 72a4f0ea @ port/m3-envelope-presence-1 已推远端** / 冒测 run ID=coord/runs/M3-ENVELOPE-PRESENCE-1/envelope_presence_20261026_185557（**一轮 PASS，ps1 exit 0，12 检全绿**：plugin_search ValhallaSupermassive→rack_add_node 探针装载→无包络基线双查全等+20 参数行字段全在场=0→set_points 三平坦点→目标行 dry level envelope_point_count=3+未动行=0→既有字段 diff 级零改动→clear_region removed=3→全零+回基线全等）/ 内核构建 exit 0（VS 18 2026 全量 CMake，自包含 exe sha256 85b4e8df…）/ 真栈占用登记 14dee1d2→释放 4ce186e8（三纪律+188 守卫：登记后 18:55:36 即时复核全零距启动 <60s、无 already listening、VIT_PROJECT_XML=run 目录 git 干净隔离副本——副本被内核写 18:56:41 隔离生效亲证，worktree default_project.xml 运行前后同哈希 947e4d00、Settings.xml 零触碰）/ **字段覆盖面说明**：①外部（rack VST）插件参数=本卡自然覆盖面（真栈实证 20 行全在场随包络写入 0→3→0）；②内置轨级参数（volume/pan）不在 get_plugin_parameters 回执面（内核既有形态 error only-external-plugins，189 run L1 逐字在卷）——**轨级在场性缺口如实登记上交**，自动展开对轨级参数的覆盖留主管另裁数据源（候选=轨级 automation.get_curve 探测或内核轨参数面小扩）；③卡面目标节"插件参数与轨级参数统一覆盖"以停止条件节主管补注（2026-10-25）为准=仅外部插件行 / **端测边界声明**：a) 真栈腿打内核 ZMQ 5555 命令面（裸命令，与 Godot 宿主同面）——新字段装载→写→读→清全链真栈验证，被测 kernel=worktree 自建二进制含本卡改动；b) 既有字段全量零改动=diff 级断言（写包络前后全回执逐键比较，唯一披露差异=目标行 envelope_point_count 0→3；值字段零漂移——冒测点值取参数当前值平坦三点设计），同构建版本无包络双查询全等另证查询稳定性；parameterSurfaceSignature 只取 id 列表，增列调用点置签名计算后双保险；c) agent 侧与 webui/Godot 渲染面零改动（卡面目标 3/4），栈中 agent 仅作形态成员；d) 消费端自动展开腿归 189 二段（数据源本卡就位））
- 验收：**整卡 pass**（GLM 主管决策流 2026-10-26；裁定=rulings/2026-10-26-M3-ENVELOPE-PRESENCE-1-pass.md；合入=87b032e2→main。diff 亲读+判定面亲核+栈记录三纪律亲核；轨级参数缺口裁定=数据源另裁不阻塞消费面）

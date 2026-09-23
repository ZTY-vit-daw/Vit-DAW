# FIX-BROADBAND-SHARED-1：v6 broadband 条目允许共享阈值单参数形态（决策点 A 承接卡）

- 优先级 / 预估 / 依赖：P2 / 0.2-0.3 天 / FIX-PLUGIN-SELECT-1（v6 机制已族无关）；PORT-PCA-FULL-CANDIDATES-1 决策点 A 裁定（2026-09-23 PC 决策会话：允许共享形态）
- 模型分级：L1 / GLM-5.3 flash 可接（单包结构体+绑定小改，先例齐全）
- **背景**：现 broadband 白名单条目为双通道硬形态（`threshold_param_id_ch1/ch2` 必填，whitelist.go:72-78）；mac 的 12 个 Waves comp 实测全为单共享 Threshold——硬形态导致该族持续留空。裁定允许共享单参形态，先例：de_esser v5 条目即单 `threshold_param_id`（FAM1-S1），端口已支持单条目批写同 revision advance（staticeq_vsp Preflight 注释明示合法形状）。
- 目标：
  1. `BroadbandCompressionPlugin` 支持 `threshold_param_id` 单字段形态（或 ch2 可选——二选一以与 de_esser 字段名对齐为优），loader 双形态收（dual 照旧/单参合法），条目形态歧义（两字段都缺/都有）fail-closed
  2. D1 broadband 准入与绑定支持单参条目（param_id_ch2 省略=单条目批写，对齐 FAM1-S1 语义）；现有 dual 条目（PC Vertigo VSC-2）行为不变
  3. 红测试：单参条目加载+admission+绑定/双形态往返/dual 回归/歧义 fail-closed
- 文件域：`agent/internal/experimentplugins/whitelist.go`（+测试）+ `agent/internal/chat/free_state_d1_plan_table.go`（broadband 段，+测试）
- 验收：①红测试修前红/修后绿；②agent 全量+webui 绿；③PC 现网真栈不回归（现有 dual 白名单行为不变——单测+一次 ④/旅程 spot 即可）
- 停止条件：发现端口写路径对单参 broadband 需改 executionports 才能绿 → 上交（预期不需要——FAM1-S1 同款语义已支持）
- 领取：2026-09-23 22:35 +0800 PC 执行会话（GLM-5.3）；main@2b5b1513（`git pull --ff-only origin main` 后 Already up to date）；领取时 `git status --short`：M VitApp/Workspace/default_project.xml（运行时残留，非本卡 diff）+ coord/runs 各卡工件、extension 构建产物、godot-cpp/ 未跟踪（均非本卡 diff，不回退不提交）
- 回执：**执行完成 2026-09-23 23:05 +0800 PC 执行会话（GLM-5.3）**。实现 `fc2dfefa@port/fix-broadband-shared-1`（4 文件 +386/-15，自 main 分出，见并发注记）。
  - **并发注记**：工作期间另一会话向本地 main 提交 c52a4d2b（PORT-PCA-FULL-CANDIDATES-PC-1 步1盘点回执，coord-only 4 文件，与本卡文件域零交集）；本卡分支自该点分出，无冲突。
  - **① 红测试修前红/修后绿**：红=编译红（API 缺失：ThresholdParamID 字段/ThresholdParamPair 方法，`red_experimentplugins.txt`+`red_chat.txt`，与 FIX-PLUGIN-SELECT-1 同形态先例）；修后绿=新测试 7 枚全绿——单参条目 v6 加载+往返+accessor / 双形态混列（同族 dual+shared 共存，pin 各自解析+各自写形状）/ v5 单对象单参形态 / 歧义四例 fail-closed（both/no_form/ch1_only/ch2_only）/ 共享形态过 PCA 准入（空库拒→promoted 收）/ chat 侧单参绑定解析 + 计划 args 省略 param_id_ch2（FAM1-S1 ③ correction b 语义）。dual 回归=既有测试零改动全绿（v2 往返、半对/同对拒、dual 绑定 thr_a/thr_b、dual args 含 param_id_ch2）。
  - **② 全量绿**：agent `go test ./... -count=1` EXIT=0（84 ok / 0 FAIL，无 flaky 无环境失败）；webui `npm run test` EXIT=0（324/324）。
  - **③ PC 现网 dual 不回归 spot**：(a) 确定性腿——新 loader 只读加载本机 `~/.vit/free_state_experiment_plugins.json` EXIT=0，唯一 broadband 条目 Vertigo VSC-2 dual 形状原样（ch1=1416131121/ch2=1416131122，pair+accessor 同值）；探针程序临时放置跑后即删未提交。(b) 旅程腿——真实栈 journey1 **all_green / EXIT=0**（A1 无方向询问 / A2 装载落地 pca_load_gate_denials=0——该装载即走新 loader 加载现网白名单路径 / A3 实验链 proposal_applied+card_mounted / A4 会话洁净 stall_warn=0），隔离指纹前后一致（912.vit、repo default_project/settings、user history 全同值）。旅程 r1 一次使用错误（执行侧预创建 RunRoot 被脚本新鲜度纪律拒，栈未启动=环境中断类）清障后 r2 即绿；§8 纪律：N=1 主跑+1 次使用错误重跑，成功条件 exit 0 达成，无断言红无双连红。
  - **停止条件未触发**：executionports 零改动——staticeq_vsp.go:119-122 Preflight 既有"param_id_ch2 可选、单共享参单条目批写同 revision advance"合法形状（FAM1-S1 裁定③），按 write_mode 分派与域无关。
  - **实现要点**：BroadbandCompressionPlugin 增 `threshold_param_id`（omitempty，字段名对齐 de_esser）；validateBroadbandCompressionPlugin 双形态判别（dual 历史措辞保留：ch1/ch2 must be non-empty、must differ）；ThresholdParamPair 归一化写形状（单参→(shared,"")，dual→(ch1,ch2)）；BroadbandThresholdParams 兼答两形态；D1 broadband 绑定经 ThresholdParamPair 解析，dual 条目绑定值逐字节不变；披露/轴覆盖/C2 面 only 读 PluginIdentifier 零触碰。消费方核查：SelectBroadbandCompression 调用方仅 D1 绑定+披露+轴覆盖（后两者只读 identifier）。红绿+spot 工件指针：`coord/runs/FIX-BROADBAND-SHARED-1/`（red_*.txt / go_full_after.txt / webui_after.txt / live_whitelist_dual_spot.txt / journey1_bbshared_console{,_r2}.txt / red_green_summary.md）。
- 验收：**pass（2026-09-23 PC 决策会话）**——①diff 复核：`threshold_param_id`（omitempty，字段名对齐 de_esser）+双形态互斥判别（dual 历史措辞保留）+ThresholdParamPair 归一（dual 绑定逐字节不变）+executionports 零改动（FAM1-S1 既有合法形状），与冻结设计逐项一致；红测试 7 枚（编译红+行为红）与 dual 回归零改动采信。②决策侧独立复跑：worktree cherry-pick（main 侧 6eed13f2）agent 全量 84 包 ok/0 FAIL；webui 324 采信（本卡 4 文件零 webui 域）。③spot 双腿采信：现网 dual 白名单加载原样（Vertigo ch1/ch2 同值）+ 真栈 journey1 all_green exit 0（pca_load_gate_denials=0，装载走新 loader）+隔离指纹一致；RunRoot 预创建被拒记环境中断类（脚本新鲜度纪律，决策侧同日同踩）不构成返工。④已合入 main（cherry-pick 6eed13f2，分支 fc2dfefa 同内容）；broadband 族共享形态双端解锁，mac 的 C1 comp 补晋升（PC-FULL 裁定②）即可入列。

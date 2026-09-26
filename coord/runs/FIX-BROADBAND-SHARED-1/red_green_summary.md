# FIX-BROADBAND-SHARED-1 红绿证据摘要

日期：2026-09-23 · 分支：port/fix-broadband-shared-1 · 领取基线 main@2b5b1513（分支点 c52a4d2b，见并发注记）· 实现 fc2dfefa（4 文件 +386/-15）

## 修前红（实现前记录，同一工作树）

- `red_experimentplugins.txt`：experimentplugins 包对新测试编译红（unknown field ThresholdParamID / ThresholdParamPair undefined）。
- `red_chat.txt`：chat 包编译红（unknown field ThresholdParamID）。
- 红的形态说明：本卡为 schema/API 演进卡，与 FIX-PLUGIN-SELECT-1 同形态先例——红测试针对新 API 编写，修前红表现为编译红（API 缺失即红）；歧义 fail-closed/双形态混列等行为面断言随 API 落地后由修后绿覆盖（歧义四例对旧 loader 部分为"拒因不同"的拒收——`threshold_param_id` 在旧版是 unknown field 拒，新版是形态歧义拒，两者都 fail-closed，拒因措辞以新版为准）。

## 修后绿

- agent 全量 `go test ./... -count=1` EXIT=0，84 ok / 0 FAIL（`go_full_after.txt`），无 flaky、无环境失败。
- webui `npm run test` EXIT=0，324/324（`webui_after.txt`）。
- 新测试（7 枚）：
  - experimentplugins/whitelist_broadband_shared_test.go：单参条目 v6 加载+往返+accessor、双形态混列（同族 dual+shared 共存，pin 各自解析+各自写形状）、v5 单对象单参形态（规范化 v6+单元素列表）、歧义四例 fail-closed（both_forms/no_form/ch1_only/ch2_only）、共享形态过 PCA 准入（promoted 收/空库 not-PCA-promoted 拒）。
  - chat/free_state_d1_plan_table_broadband_shared_test.go：单参条目 D1 绑定解析（ParamID=shared/ParamIDCH2 空/PCA 准入空库拒→promoted 收）+计划 args 省略 param_id_ch2（FAM1-S1 ③ correction b 语义）。
- dual 回归（既有测试零改动全绿）：TestLoadParsesV2FileWithBothSectionsRoundTrip（dual 往返）、TestLoadRejectsMalformedCompressionSections（半对/同对/unknown field 拒）、TestResolveD1PluginParamBindingForCompression（dual 绑定 thr_a/thr_b）、TestD1S1CompressionPlanEmbedsWhitelistBinding（dual args 含 param_id_ch2=thr_b）、TestD1S1DeEsserPlanEmbedsWhitelistBinding 尾段 dual 对照。

## PC 现网 dual 不回归 spot（验收③）

- 确定性腿 `live_whitelist_dual_spot.txt`：新 loader 只读加载本机 `C:\Users\timoz\.vit\free_state_experiment_plugins.json` EXIT=0；唯一 broadband 条目 `VST3-Vertigo VSC-2-7e4e7243-aaeea0d` dual 形状原样（ch1="1416131121" ch2="1416131122"，ThresholdParamPair 与 accessor 同值，shared 空）。探针程序（agent/tmpspot）临时放置、跑后即删、未提交。
- 旅程腿 `journey1_bbshared_console_r2.txt`：真实栈（内核+agent+构建自本卡分支）journey1 `all_green` / EXIT=0——A1 无方向询问（option_marker_count=0）/A2 装载落地（pca_load_gate_denials=0，bx_hybrid V2 rack node 入图；该装载走新 loader 加载现网白名单的路径）/A3 实验链（proposal_applied=True card_mounted=True）/A4 会话洁净（stall_warn=0）；隔离指纹前后一致（repo default_project/settings、912.vit、user history 全同值）。r1 一次使用错误（执行侧预创建 RunRoot 被脚本新鲜度纪律拒绝，栈未启动=环境中断类）清障后 r2 即绿。
- 运行纪律（§8）：旅程 N=1 主跑（+1 次使用错误重跑）；成功条件=exit 0 达成；无断言红、无双连红。

## 并发注记

工作期间另一会话向本地 main 提交 c52a4d2b（PORT-PCA-FULL-CANDIDATES-PC-1 步1盘点回执，coord-only 4 文件，与本卡文件域零交集）。本卡分支自该点分出，无冲突。

## 停止条件核查

未触发：executionports 零改动——staticeq_vsp.go Preflight 既有"param_id_ch2 可选、单共享参单条目批写同 revision advance"合法形状（FAM1-S1 裁定③），按 write_mode 分派与域无关。

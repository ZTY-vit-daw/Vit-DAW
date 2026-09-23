# Ruling：FIX-D1-PLUGIDENT-1 pass（2026-09-23）

- 卡：`coord/cards/done/2026-09-23-FIX-D1-PLUGIDENT-1.md`（PC 决策侧开卡，mac 执行侧执行，Mac 决策会话验收）
- 实现：port/fix-d1-plugident @`1c27006`，经决策侧验收 cherry-pick 入 main = `12d799b`（回执 `7396bd2` 之后）；合入树 `go build` exit 0 + chat 包 `go test` 全绿（决策侧复验）
- 裁定：**pass（验收①②④；③=PC 真栈插件写 spot，按 dispatch 裁定留作合入后 PC 侧补验）**——mac 模型驱动 EQ 全链首次闭环

## 决策侧核验（Mac 决策会话，独立复算）

1. **红测试真实性**：`red_before_fix.txt` 两测试以精确预期断言消息失败（args 仅 path+name 无 identifier）；`green_after_fix.txt` 双绿；**分支 tip `1c27006` 决策侧独立复跑两测试通过**。测试设计采信：PluginBound 侧经生产 resolver 产出 binding，证明 identifier 源自白名单 section 而非测试字面量；两条 stub 路径（juce_eq 默认 schema）锁死不变。
2. **实现质量超卡面**：不止卡面两处 args map（`d1StaticEQActionArgs`/`pluginParamWriteArgs` 各一行），**七个族的 binding resolver 一次补齐 PluginIdentifier 填充**（static_eq 包裹层+comp/deess/transient/lim/gate/mbc）——未来任何族配 shell 主体直接受益。7 处八月 v5 时代旧断言（"must not carry a known-list identifier"）随修翻转为"携带且=白名单值"——旧断言锁定的正是本卡推翻的缺口纪律，同文件域合规。零端口/零内核改动达成。
3. **测试面**：go 全量 83 包 ok，唯一 FAIL=contextruntime `/var` 符号链接既有环境项（FIX-TEST-CTXSYMLINK-1 在 todo；执行侧已隔离复跑+领取基线复现排除关联）；webui 324/324。
4. **验收④ mac 旅程回归（闭环证据，决策侧逐条抽核原始工件）**：exit 0、9/9 all_green、`plugin_path is ambiguous` 1→0（基线对照）、capability_blocked=0。链条实证：ZMQ 线上报文 `rack_add_node` 携带 `plugin_identifier=VST3-Q10 Stereo-10456661-3a8f251e` → 内核 `rack_item_id=1039` + `WavesVST3UI class_selection match(selected_index=200,CID 精确)` → `component_create success` → `set_plugin_param succeeded(param 17=-1.5)` → `readback_verified=true` → A/B 事件链。**模型驱动 EQ 全链『假设→准入→带标识符计划→CID 实例化→参数应用→回读→A/B』在 mac 首次走通**——REALSTEMS→EQ-1→本卡三卡链条的最终闭环。
5. **§8 纪律**：N=1 主跑即成、无环境项触发、止损线未触及（section8_discipline.md）。

## 合入操作记录（决策侧）

- 执行侧本地 main（dde70c1 领取→7396bd2 回执）与 port 分支（1c27006）分叉于 5c16ef6，ff 不可行 → 按协议 §3 走 cherry-pick：`1c27006` → main `12d799b`（卡片冲突自动解决为 done/ 终态，无重复副本）。
- 执行侧建议采纳：`staticeq_vsp.go:68-73` 过时注释（"real-plugin path leaves identifier empty"）由决策侧随合入更新（comment-only，零行为改动）。

## 流程瑕疵（不构成返工，入档备忘）

- 收口三步（mv done+回执+推 main）由执行侧在验收开始后补齐（7396bd2）——连续第二张卡收口不完整（上卡=mv 不完整，本卡=回执滞后）。**已入派卡提示词惯例的执行侧注意项：完成回执写卡+mv done+coord 推送应在自验通过后立即完成，不等决策侧催办。**

## 遗留移交

- **[等待 PC 真栈插件写 spot 补验]**（验收③）：PC 侧在合入后的 main 上跑一次插件写场景 spot（预期绿——独占解析语义已被 C2/PCA 在 PC 生产栈例行行使）；结果回写本卡验收字段或后续 coord 提交。
- port/fix-d1-plugident 分支已合入，可由决策侧择机清理。

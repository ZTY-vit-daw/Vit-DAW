# FIX-PLUGIN-SELECT-1 mac 多候选验证腿——转交提示词草稿（实现合入 main 后使用）

> 前提：PC 实现（port/fix-plugin-select-1）已经决策侧验收合入 main。mac 侧从 main 同步。

## mac 侧操作序列

1. **写 v6 多候选白名单**（备份现有 v5 后）：
   `~/.vit/free_state_experiment_plugins.json` 改为 schema_version
   `vit.free_state_experiment_plugins.v6`，static_eq 条目数组放入 mac 已 PCA 认证的
   4 个 EQ 主体（Q10 Stereo / API-550A / API-560 / EMO-F2，条目字段与 v5 单对象逐字段
   同形：plugin_name/manufacturer/format/plugin_identifier/plugin_path/bands）。其余族如无
   认证主体可省略。注意：plugin_path 仍是 Waves 壳路径，identifier 为各成员 CID——
   内核按 identifier 独占解析（FIX-D1-PLUGIDENT-1 纪律）。
2. **跑 mac 旅程**（原命令原素材）：
   `./scripts/journey1_demo_journey_smoke_mac.sh --stems-dir ~/Desktop/cases/spv1_p01/stems`
3. **验证自选复活证据链**（验收④口径）：
   - agent 日志/提示中出现 `PLUGIN CANDIDATES` 披露（4 候选、结构性字段、无 plugin_path）；
   - 模型提案 parameter_bounds 携带 `plugin_identifier` = 披露集中某一成员（模型自选）；
   - FrozenPlan args 的 plugin_identifier = 所选成员 → rack_add_node 装载所选 EQ →
     set_plugin_param succeeded → readback_verified → A/B 事件链；
   - exit 0 / all_green。
4. **失败分类**：若出现 "requires a pinned plugin_identifier"（模型未选）或
   "whitelist admits one of"（选错）→ 这是准入 fail-closed 的正确防御，记录后重跑
   旅程（模型随机分支）；连续两次同签名 → 上交决策侧。

## 回执要求

两端 HEAD + 两端白名单版本（PC v5 单候选 / mac v6 多候选）+ run ID + 工件指针。

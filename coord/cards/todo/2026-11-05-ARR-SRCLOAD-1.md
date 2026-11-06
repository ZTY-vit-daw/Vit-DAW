# ARR-SRCLOAD-1：音源装载最小闭环——编曲语境装载链真栈验证 + 编曲 pack（编曲首张 / 注册链②）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[258 报告](../../reports/2026-11-05-arr-recon-1.md) 卡序草案 + [taxonomy](../../decisions/2026-11-05-capability-domain-taxonomy.md)）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 277；**注册链第 ② 位（前置=276 已验收提交）**；目标=capability_pack 新增 arrangement pack（或扩 midi pack）+ usage 披露补丁；P1 / 1.5-2 天；L1-L2 / PC；真栈烟测腿一次

## 目标（258 报告卡序卡 1 原案）

1. **真栈验证编曲语境装载链**：chat 驱动"加钢琴音源"→ plugin_search / semantic（type=synth）→ `rack_add_node`（synth 默认 Z2）→ 写 MIDI clip（create_midi_clip+add_midi_notes）→ **离线渲染非静音断言**（start_render 后产物非静音）——装载通道出声的第一次实证（A1 修正结论的落地）。
2. **编曲 pack**：装载编排次序（search→rack_add_node Z2→写 clip→试听）+ Z2 语义 + 试听引导 + 与符号分析的衔接占位（详细引导归 283）。
3. **usage 披露补丁**：semantic_search 的 type 值披露（synth/instrument 可见性）。
4. **环境预检**：`plugin_list_available` 断言 `is_instrument` 计数>0；为空按环境前置上交，不视为代码障碍。

## 约束

- 文件域=capability_pack.go（新 pack 段）+ catalog.go usage 披露段 + 烟测脚本扩展 + 测试；**零内核改动**（全部既有命令）；不触 M3 域；`plugin.instantiate` 维持排除（258 验收裁定），装载走 rack_add_node 工作流。

## 验收标准

- 真栈烟测腿 exit 0：装载→写 clip→渲染非静音全链断言过（run ID+工件入回执）；
- pack/usage 改动=引导级，既有九 pack 行为零变化（对照单测）；
- 回执附：装载链各环节返回摘要、渲染产物路径、commit hash。

## 停止条件

- 本机音源库存为空（is_instrument==0）→ 环境前置上交；渲染静音且排查超本卡域→取证上交。

## 领取

- （执行侧回填）

## 回执

- （执行侧回填）

## 验收

- （主管回填）

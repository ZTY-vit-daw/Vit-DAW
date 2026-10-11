# GEN-CTX-DISCLOSE-1：工程音乐上下文披露键（统一化实施①）

- 发卡：GLM 主管决策侧 / 2026-10-11 晚夜（依据=GEN-UNIFY-CTX-DESIGN-1 验收 pass 的实施拆卡规格件五 卡1；用户三线同步+生成≠MIDI 口径）
- 派发确认：已确认（用户 2026-10-11 同步裁定）
- 验收负责人：GLM 主管决策流
- 池序 75；目标仓库=D:\Vit_DAW；**规格全文=[设计件五 卡1](../../runs/GEN-UNIFY-CTX-DESIGN-1/05-impl-card-specs.md)（权威，本卡为池入口）**
- 归属：Mac 生成线（统一化脊柱落地）；P1 / 0.5 天 / 无依赖
- 模型分级：L1 / Mac 执行流（内核 C++ 披露块+冒测脚本）

## 要点摘录（以规格件为准）

get_project_state 新增 music_context 六键（tempo/拍号族，key_signature/style_note 占位 null）；文件域=CommandDispatcher.cpp 披露块（:3389-3420 区域，领取时实测）+scripts 冒测；验收=真栈只读冒测（set_tempo 联动断言）+既有键零变更快照 diff+agent 侧零 diff。

## 并行与资源

CommandDispatcher.cpp 与任何在飞卡（如 PC 混音线内核腿）叠域时按 PROTOCOL §3 核对；Mac 栈按 MAC-RUNTIME-STACK 登记。

- 领取：（时间 / origin/main hash / owner / 分支 / worktree / 领取提交）
- 回执：（commit / run ID / 冒测工件）
- 验收：（裁定文件 / 验收 commit）

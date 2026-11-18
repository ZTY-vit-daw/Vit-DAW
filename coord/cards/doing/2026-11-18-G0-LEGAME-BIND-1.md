# G0-LEGAME-BIND-1（W01-C）：工程包装配绑定（H3 收口腿）

- 池序 425；P0；PC；发卡=本会话 PC 主管；**todo 候 424 合入** + 真实栈排位。
- 权威：契约 v0-frozen §3.3（R2 启用元数据）/§6.3/§6.5；卡片草案 `impl-cards-draft.md` W01-C。
- 目标：把编译产物接进工程包链路——项目包元数据 `legame.{enabled,design}` 启用判定（R2）；M1 导入触发解析→编译→装配；binding UUID 化（修复父目录静态推导串工程）；缓存四元组生效；材料相对路径随包迁移。
- 文件域：`agent/internal/projectpackage/`、`agent/internal/chat/edu_jam_chat.go`（binding 读取点）、`scripts/`（扩展现有 ps1 烟测）；领取前主管核对在飞卡文件域。
- 验收（真实栈，exit 0 门槛）：导入设计源→编译装配→agent HTTP 验证乐局装载、binding 指向正确工程、region/alias 非轨首推导；A/B 两工程不串 binding、重开不切别版；revise 源→缓存失效→新卡生效；recents 快照还原纪律。
- 停止：三断言真栈 PASS；越界缺陷停手上交。

## 发卡/领取（2026-11-18 晚，424 合入后）

- owner=`lead/legameparse423`（续用 W01-A/B 执行流）；分支 `codex/legame-bind-425`；worktree=`C:/Users/timoz/.codex/worktrees/pc425-bind/Vit_DAW`；基线 main `780e5671`。
- **分两段**：先零栈实现 + Go/脚本单测（不启真栈）；真栈烟测腿必须候主管登记 PC-RUNTIME-STACK 并书面授权后单独执行。
- 硬约束：项目包元数据 `legame.{enabled,design}` 启用判定（R2，普通工程可随时转乐局）；binding 键=project UUID（修复父目录静态推导串工程）；缓存键四元组；材料相对路径随包迁移；A/B 两工程不串 binding、重开不切别版、revise 源→缓存失效新卡生效（真实栈三关）。
- 复用 423/424：`legamedesign.Parse/Compile/WriteCompiled/RefreshCompiled`；编译入口必须含 ValidateCompileReadiness。
- 文件域：`agent/internal/projectpackage/`、`agent/internal/chat/edu_jam_chat.go`（binding 读取点）、`scripts/`（扩展既有 ps1 烟测，不另起机制）；领取时主管核对在飞卡文件域（当前无同域）。

## 零栈段验收（2026-11-18 晚）

- commit `b78c9969`；主管亲跑 build/projectpackage/legamedesign/chat 全 PASS（chat 121.6s）；合入 main `bef5e9c1`。
- 交付：R2 元数据启用、M1 导入装配、binding UUID、四元组缓存、重开/移动/revise fail-closed、真栈 runbook。
- 零栈证据：A/B 隔离、重开不切版、revise 新卡、enabled=false 惰性、R5 无覆盖不落卡。
- 真栈腿未跑（runbook 待授权）；卡保持 doing。裁定：[zerostack-pass](../../rulings/2026-11-18-G0-LEGAME-BIND-1-zerostack-pass.md)。

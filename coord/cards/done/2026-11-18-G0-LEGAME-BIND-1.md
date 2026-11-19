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

## 真栈腿（2026-11-19，lead 亲跑）

- 栈位：main `9f65db40` 登记；berth=`agent/bin/VitAgent.exe` + `VIT_AGENT_SPAWN_RUNTIME=1`（agent 自拉 headless kernel+VspHub，无 VitShell/GUI）。
- 二进制：VitAgent `3D9BAC1C…`（main 33650e22 构建）、kernel `A5947361…`、hub `B10CC412…`。
- 装置修（已合入 main）：`029e1c6c` 默认端口 7878+agent 面预检；`745b6265` `args`/`confirmed`、`project.open file_path`、`current_project` 字段、S5 自然语句、S6 会话延续、UTF-8 BOM。
- 轮次：attempt1 编码解析失败；attempt2 S0–S4 PASS / S5 `edu_jam_no_session`（测试期望缺陷：命令词跳过 auto-intro）；attempt3 正式轮 **S0–S7 PASS**。
- 正式轮证据：[summary.json](../../runs/G0-LEGAME-BIND-1/real-stack/run_20261119_1818/legame_final/summary.json)（status=passed；A/B UUID `vitproj_4ce90ea8…` / `vitproj_73ce04f5…`；binding/card 路径相异；重开 cache_key 一致；revise 后 design_source_sha256 变化且新标题生效；recents 还原哈希一致；零 LLM 遥测）。
- 权威输入：Settings.xml 未变；**default_project.xml 被内核改写**（berth 前 `5BC9C756…` → 当前 `E9B668A0…`；仅存哈希未复制，无法逐字节还原；预跑内容为六 stems 测试残留，非用户工程）。装置缺陷记录见 [DEVICE-NOTES](../../runs/G0-LEGAME-BIND-1/real-stack/run_20261119_1818/DEVICE-NOTES.md)；后续真栈腿须在 berth 前复制权威文件并在 teardown 还原。
- 栈：teardown 后 Vit 系进程/TCP 六口/UDP 4445 全零；recents 还原 `63D0F3F7…`。
- 裁定：[realstack-pass](../../rulings/2026-11-19-G0-LEGAME-BIND-1-realstack-pass.md)。


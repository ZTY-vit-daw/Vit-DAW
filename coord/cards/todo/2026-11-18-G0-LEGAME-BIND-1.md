# G0-LEGAME-BIND-1（W01-C）：工程包装配绑定（H3 收口腿）

- 池序 425；P0；PC；发卡=本会话 PC 主管；**todo 候 424 合入** + 真实栈排位。
- 权威：契约 v0-frozen §3.3（R2 启用元数据）/§6.3/§6.5；卡片草案 `impl-cards-draft.md` W01-C。
- 目标：把编译产物接进工程包链路——项目包元数据 `legame.{enabled,design}` 启用判定（R2）；M1 导入触发解析→编译→装配；binding UUID 化（修复父目录静态推导串工程）；缓存四元组生效；材料相对路径随包迁移。
- 文件域：`agent/internal/projectpackage/`、`agent/internal/chat/edu_jam_chat.go`（binding 读取点）、`scripts/`（扩展现有 ps1 烟测）；领取前主管核对在飞卡文件域。
- 验收（真实栈，exit 0 门槛）：导入设计源→编译装配→agent HTTP 验证乐局装载、binding 指向正确工程、region/alias 非轨首推导；A/B 两工程不串 binding、重开不切别版；revise 源→缓存失效→新卡生效；recents 快照还原纪律。
- 停止：三断言真栈 PASS；越界缺陷停手上交。

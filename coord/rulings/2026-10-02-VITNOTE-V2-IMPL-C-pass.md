# Ruling：VITNOTE-V2-IMPL-C — pass（2026-10-02 决策会话；跨面目检挂 V2 手测场）

## 验收四层

1. **diff 直读**（Godot 仓 port/vitnote-v2-impl-c@4e0b862，6 路径 +685/-9）：
   - rack 供给器（§6.2）：契约四函数+对称注册；face_rects=全部在场 adapter 视图矩形（多块）；resolve 只读调用 F11 域源（get_selected_kernel_track_id+get_rack_nodes 快照深拷贝规整）；诚实降级注释在位。**聚合形态偏离申报采信**：§6.2 字面「每视口一个」在单静态供给器下聚合落地（枚举多块矩形+resolve 锚定首个可见 adapter）——逐视口拆分需 adapter 侧注册钩越本卡只读域；当前 UI 单机架视图（vp_rack_main）下无实际影响，探针真路径证 4 面解析正确、legacy 无机架视图正确缺席。
   - library 供给器（§6.3）：三 tab 分支（Files=browser_tree 可见行+Tree 缺失回退 _places_roots 如实降级/Plugins=_available_plugins 带 identifier/Groups）+MAX_ENTRIES=50+truncated_items 仅截断时出现+关闭抽屉空矩形；诚实降级注释在位。
   - manager spawn 扩至四供给器——四类面覆盖完成（裁定 4 兑现）。
   - **只读约束兑现**：graph_rack_dock_adapter/left_library_dock/project_repository 三文件 diff 零命中；lane 域零改动。
2. **我方独立复跑**：headless 探针我方自跑两轮 **127/127 PASS EXIT=0**（静态契约 18+沙盒 89+两面烟测各 10）；`--import` 零 SCRIPT/PARSE ERROR。
3. **工件亲读**：coord/runs/VITNOTE-V2-IMPL-C/ 5 件（README+run1 红[6 红=探针搭建缺陷 FakeRackAdapter 忘设节点名，非实现缺陷，诚实区分]+run2/3 全绿+import）——红绿链完整可回指。
4. **边界核对**：真路径证据在案（dock 4 面含真 rack@vp_rack_main+library 真 Tree 枚举 6 条）；真实抽屉开合/机架切换/真实内核 rack 链内容归手测 §10.2 第 3 步[跨面完整版]挂 V2 手测场；运行栈未起声明在案。

## 裁定

- **pass**。停止条件正确未触发（F11/F12 域源实锚全在位）。
- 分支链：port/vitnote-v2-impl-c@4e0b862 为现 HEAD；**V2-IMPL-D 依赖解锁**（多面辖区摘要呈现+九步手测一场销三卡——V2 五卡收官张）。

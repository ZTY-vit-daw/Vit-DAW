# Ruling：VITNOTE-INTERACT-V2-DESIGN-1 — pass（2026-10-01 决策会话；D1/D2 两裁定边界项转用户）

- 实现：ea8b9220（docs/VITNOTE_V2_INTERACTION_DESIGN.md 350 行+V1 修订标注）+0083fdc9（卡片移动+回执）@port/vitnote-interact-v2-design-1 → 合并 main d8659456/06737e32（决策侧 cherry-pick，领取提交未走 main 致路径冲突按终态解为 done/）。
- 亲核项：
  1. **七验收项对照属实**：①§4.1 手势状态机到状态/转移级（IDLE/PRESS_PENDING/DRAGGING/RESOLVING 全转移+消费边界「仅活跃期拦输入」+焦点文本区让位精确化）；②§5 命中解析五步管线+占比双口径+跨面合并单 note+空辖区；③§6 供给器契约四函数 duck-typing+组注册+四类面逐一+新面接入指南；④§7 输入 bug 三候选+可执行探针五步+三修复锚点；⑤§8 旧路径处置表（lane 通知退役/采集转供/组广播保留）；⑥§10.2 V2 九步清单（一场销三卡）；⑦§11 五卡排程。
  2. **八项裁定+宪法三原则零偏离**：触发/坐标命中/域对象/供给器 identity·affordance/控制面快照/跨面合并/窗口式/只读链全部落规格；区域抽象与触发隔离在 §4（pad 移植预留）。
  3. **可行性已证**（停止条件未触发）：全局捕获=manager 末位挂载+_input 逆序传播+活动态 set_input_as_handled，三先例（router/lane/manager）全走此路——设计级可行性成立。
  4. **输入 bug 根因静态链采信**（高先验候选 B）：router `_input` 先转发 lane 后查 interactive 旁路+`_candidate_interactive_hit_roots` 不含 VitNoteLayer 子树——精确解释「含收起按钮全灭」；legacy 同根推论+探针验证路径在案；三修复锚点（顺序修正/旁路根扩展/仲裁器同款短路）与供给器架构共用 `vit_floating_interactive` 组——输入修复与新面接入同一注册机制，设计内聚好。
  5. 零代码核实：仅 docs 两文件；Godot 仓只读。
- **D1/D2 转用户裁定**（卡面上交项，见随卡问题）：
  - D1：clip 体上 Alt+拖=克隆手势让位（圈选不发起）——设计默认非侵入式让位；备选=圈选优先、克隆迁 Ctrl+拖（改既有手势，须用户点头）。
  - D2：plain marquee（无 Alt 普通框选）回归纯选区（不再出胶囊）——解释性裁定：八项裁定「胶囊+[N] 双出口保留」读作圈选完成后的出口；备选=双触发并存（同屏竞争胶囊）。
- 后续：IMPL 五卡按 §11 排程由决策侧裁剪入池（V2-INPUT-FIX-1 输入修复锚点卡建议池头——解锁面板交互）。

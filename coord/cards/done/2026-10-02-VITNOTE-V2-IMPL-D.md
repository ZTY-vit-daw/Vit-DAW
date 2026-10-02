# VITNOTE-V2-IMPL-D：多面辖区摘要呈现+V2 九步手测执行——三卡同场销收官（V2 五卡第五张，P1）

- 池序 31；**目标仓库=D:\Godot\project\vit-daw-frontend（Godot 仓，自 port/vitnote-v2-impl-c@4e0b862 切出——已验收 HEAD）**；来源=[VITNOTE_V2_INTERACTION_DESIGN.md](../../../docs/VITNOTE_V2_INTERACTION_DESIGN.md) §5.3（跨面合并摘要）/§10.2（九步手测清单——**裁定 8：一场销 IMPL-1/DOCK-MOUNT/V2 三卡**）
- 优先级 / 预估 / 依赖：P1 / 0.5 天实现+手测半场 / **A/B/C+INPUT-FIX-1 四卡全部已验收**（分支链在位）；V2 收官卡
- 模型分级：L1 / GLM（panel 呈现改造轻量+手测编排需对设计九步逐条判据）
- 已核实事实（IMPL-B/C 落地基线，勿重勘）：
  1. note 载荷 v2 已真解析（origin/rect_global/faces[] 降序含 label+selection_share+domain）；面板现 `_rebuild_summary` 读 v1 顶层字段显「空辖区」（IMPL-B 申报的已知边界——本卡修）。
  2. 四类面供给器全在位（timeline/rack/library/control）；胶囊带面数文案；[N]/点击双出口沿 V1 hint。
  3. IMPL-A 修复（router 旁路/根枚举/仲裁器短路）在分支链基线内——手测第 8 步判据「含收起按钮全部鼠标交互有效」直接可验。
- 目标：
  1. **多面摘要呈现**（§5.3）：`_rebuild_summary` 升级读 faces[]——面板头部辖区摘要=「轨道时间线 72% · 机架 28%」式一行（label+selection_share 百分比，降序拼接，空辖区态保留）；faces 空走空辖区文案（退化路径不变）。文件域=vit_note_panel.gd（本卡首次入域，实锚后申报其余呈现位）。
  2. **九步手测执行**（§10.2 全表，逐条判据对齐）：执行侧先完成可自验项的 headless/探针面（如有）+准备手测环境说明；九步真人执行由用户按 AGENTS §5 途径（Godot 拉起前端→进工程）——执行侧职责=清单编排+陪同取证+结果记录，**不代替用户判定人耳/目检项**。
  3. **三卡同场销记录**（裁定 8）：手测第 9 步复核 VITNOTE-IMPL-1 条件项（圈选→胶囊→便签→IME→观察问答→收回→重开最小闭环）与 VITNOTE-DOCK-MOUNT-1 条件项（dock 默认入口挂载可见）——结果落卡面销项。
  4. 手测全程留证：每步结论+异常截图/日志归 coord/runs/VITNOTE-V2-IMPL-D/（或用户口述执行侧代录，注明来源）。
- 文件域：`app/tracks/vitnote/vit_note_panel.gd`（摘要呈现）+手测编排记录——预计 ≤2 文件改动；若呈现需动 hint/manager 由停止条件裁。
- 约束：新分支 `port/vitnote-v2-impl-d` 自 port/vitnote-v2-impl-c@4e0b862 切出；不动 main；运行时目录勿动；**手测需真实三件套时遵守 §9 栈排他**（与用户活栈错峰，泊位声明）；无自动化测试基建如实申报。
- 验收标准：多面摘要 diff 亲核+headless import 零错误+九步手测记录完整（判据逐条结论）+三卡销项落簿+用户确认手测结论。
- 停止条件：手测暴露 V2 链真缺陷 → 记录现象+锚点，卡移 blocked 附证据上交（缺陷归谁由决策侧裁）；panel 呈现发现 faces 形态与 IMPL-B 载荷不匹配 → 带锚点上交。
- 领取：2026-10-02 / Godot 仓基线 port/vitnote-v2-impl-c@4e0b8625967c6927cc62151680b281b3262de2a3 / 分支 port/vitnote-v2-impl-d（Vit-DAW 仓 HEAD=4413c12c，工作树仅 VitApp/Workspace/default_project.xml 运行时改动=领取前已有勿动；Godot 仓未跟踪文件全为运行时工程目录，未动）
- 回执：2026-10-02 实现侧自验完成（commit 948b73ca 簿记+runs/ 八件工件；决策侧代誊）——多面摘要落地 Godot 仓 port/vitnote-v2-impl-d@fd7e6af（2 文件 +176/-4）；panel 探针红绿链（run1 一红=face_kind 空串边界真缺陷→修复后 16/16 两轮）+三套回归零破坏（resolve 127/127+circle 52/52+input 全 EXIT=0）+import 零 SCRIPT ERROR；**九步手测清单编排就绪（runs/VITNOTE-V2-IMPL-D/MANUAL_TEST_9STEPS.md）待用户按 AGENTS §5 途径执行**；执行侧未起真实栈（泊位声明）。
- 验收：**conditional pass（2026-10-02 决策会话——实现面 pass；九步手测+三卡销项待用户执行后终裁）**——[rulings/2026-10-02-VITNOTE-V2-IMPL-D-conditional.md](../../rulings/2026-10-02-VITNOTE-V2-IMPL-D-conditional.md)；_rebuild_summary diff 亲核（空辖区/三级兜底/tooltip/v1 字段移除正当）+我方全套独立复跑（panel 16/16+三套回归+import 零错误）+九步清单编排质量优；九步全过→转 pass 终裁+IMPL-1/DOCK-MOUNT/V2 三卡销项；任一步不过→blocked 附锚点

# Ruling：VITNOTE-IMPL-1 — 代码面 pass，终裁待 M8 手测（2026-09-30 决策会话）

- 实现：Godot 仓 port/vitnote-impl-1 @098cd25（9 文件 +633/−5：vitnote 新模块 hint/manager/panel+marquee 挂点+快捷键注册；主仓零改动）
- 亲核项：
  1. **diff 亲核**：marquee 挂点唯一出口在拖框分支（单击清选区不出提示），选区写入与广播**零改变**（既有工作流零破坏=设计 §4.1 第 1 条）；复用 `_marquee_rect_global`+`_collect_marquee_hits_global`（RECON F1/F2 锚，辖区快照与选区共用同源）。
  2. **结构亲核**：vit_note_manager 具备全要件——present_marquee_hint/open_note/`_has_text_input_focus` 焦点守卫（RECON F5 先例复用）/get_agent_context（鸭子类型 host，RECON §1d 同源）/viewport clamp。vit_note_panel 322 行三态结构按设计 §4.2。
  3. **自验声明采信**：godot headless --import 零脚本错误；基线对比错误数一致（4 预存 Browser 节点错误非本卡引入）；vitnote 逻辑冒烟 23/23（挂载/注册/守卫/快照/三态/多 note；64x64 视口位置断言为 clamp 语义——精确位置留手测，申报诚实）。
  4. **边界采信**：7878 实发与 IME 交互未自动化（Godot 仓无该层测试基建），走 M8 手测清单——卡面验收标准的预期路径；手测步骤清单 9 步在卡面（含三出口/IME/会话延续/收回重开/移动/焦点守卫/双 note 并存），覆盖 M8 判据全项。
- **终裁条件**：用户按 AGENTS §5 途径手测（Godot 仓 checkout port/vitnote-impl-1 + 主仓 agent/bin/VitAgent.exe 已由决策侧重建至含 FS-SETTLE 修复的合并态 main）。M8 最小闭环走通（框选→形变提示→开便签→中文输入→观察问答→收回→重开；加分=双 note）→ 终裁 pass + Godot 仓合流裁定；发现缺陷 → 按 FIX 卡处置。
- 上帝ot 仓分支纪律核验：port/vitnote-impl-1 独立分支，main 未动，基线（5280822）之上单提交——合规。

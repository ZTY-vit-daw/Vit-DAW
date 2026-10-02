# Ruling：VITNOTE-CONTAINER-2 — conditional pass（探针面全过；用户手测复验留待）（2026-10-02 决策会话·gate 轮）

## 验收四层

1. **diff 亲核**（42b1856@port/vitnote-container-2，5 文件 +615/-50）：router **最小侵入方案采信**——仅 `_global_point_targets_floating_interactive_control` 一处加组成员体部判定（mouse_filter!=IGNORE+矩形命中，语义对齐仲裁器锚点 C），交互清单与其余调用点不动（画布手势零影响论证在位）；面板基类 PanelContainer→Panel 申报合理（把手驻角前提，vbox FULL_RECT 自管布局）；标题栏 STOP+gui_input 起闩/锚点三分（位移<4px 判重开/拖动钳制/右键即删走 manager.remove_note）/缩放把手 FDIAG+双端钳制/输入行常驻底部；钳制抽静态纯函数（探针可断言）。
2. **我方独立复跑**：probe_container **43/43 EXIT=0**+四套回归（resolve 127/circle 52/panel 16/input dock）全绿+`--import` 零 SCRIPT ERROR。
3. **工件亲读**：runs/VITNOTE-CONTAINER-2/ 10 件——新探针+四回归+import+**legacy 探针 EXIT=2 的基线对照归因**（fd7e6af 临时 worktree 两版输出逐字相同=既有 64x64 几何限制非本卡引入，对照件归档）+布局前后截图四张（窗口模式渲染）。
4. **边界核对**：渲染面/交互手感/IME 归用户手测（自验清单 8 项在卡面）；运行栈零启动零残留声明在案。

## 裁定

- **conditional pass**：用户手测复验（8 项清单）留待合并手测场；全过→转 pass。
- Godot 分支链：port/vitnote-container-2@42b1856 为现 HEAD——**VITNOTE-NOTESTREAM-2 的 Godot 腿自此切出**。

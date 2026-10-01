# VITNOTE-INTERACT-V2-DESIGN-1：便签圈搜式全域交互重设计——设计卡（用户八项裁定已定案，P1）

- 池序 26（P1 设计卡，M8 终裁总挂点）；**目标仓库=D:\Vit_DAW（设计文档）+参照 D:\Godot\project\vit-daw-frontend（代码勘察面）**；来源=用户 M8 第四轮手测反馈+两轮选项对齐（[decisions/2026-10-01-vitnote-v2-interaction-ruling.md](../../decisions/2026-10-01-vitnote-v2-interaction-ruling.md)——**八项裁定为权威输入，不得偏离**）
- 优先级 / 预估 / 依赖：P1 / 设计 0.5-1 天 / VITNOTE_V1_DESIGN.md+IMPL-1/DOCK-MOUNT 实现现状（Godot 仓 port/vitnote-dock-mount-1@66e59f0 已含双面挂载）；零代码设计卡（docs+coord 工件）
- 模型分级：L2 / GLM 首选（交互架构设计+跨面契约；新交互形态首例）
- 设计输入（已核实事实）：
  1. 现状：lane 级框选通知（vit_track_lane_2d.gd:2296-2311 组广播）+manager 双面挂载（vit_control+vit_dock_root）+面板窗口式三态；**面板 dock 面零交互未修**（键盘链通鼠标链灭，并入本 V2 修复——取证面含输入路由/叠层/命中测试三候选）。
  2. 用户八项裁定（decisions 文件全文）：Alt+拖触发/坐标命中模型+域对象第一原则/全局层+供给器（identity+affordance 契约）/v2 覆盖轨道+机架+资料库+控制面（状态快照）/跨面合并单 note 含占比/窗口式保留/输入 bug 并入/条件卡改挂 V2 同场销。
- 目标（设计文档 `docs/VITNOTE_V2_INTERACTION_DESIGN.md`，沿 V1 设计文档范式）：
  1. **全局圈选层设计**：Alt+左键拖手势状态机（按下/拖拽/松开/取消）+仅活跃期拦输入+视觉反馈（dim 遮罩+亮框）+与轨道面既有直接框选的共存判定（lane 手势让位或并存边界）+胶囊/N 键双出口保留。
  2. **坐标命中解析**：矩形→枚举命中面（各面全局矩形求交+占比）→供给器调用→域对象合并辖区摘要；「GUI 命中仅圈定范围、载荷=域对象引用」第一原则贯穿（摘要形态：面标签+占比+对象清单）。
  3. **供给器契约**：identity（这是什么）+affordance（能干什么）+resolve(rect)→域对象摘要；v1 面=轨道（lane 改造接入）/机架/资料库；控制面=状态快照供给器（走带=BPM/播放位置/循环区；轨道头=轨名/音量/声像/静音当前值）；新面接入指南。
  4. **输入 bug 根因取证面**：面板零交互三候选（上层拦截/命中测试未达/mouse_filter 链）的设计级排查路径+修复锚点（实现卡执行）。
  5. **旧路径处置**：lane 级 hint 通知/`_notify_vit_note_hint` 组广播的退役或保留判定（兼容期策略）；V1 文档的修订标注。
  6. **测试与验收**：9 步手测清单 V2 重写版（按新交互）；headless 可断言面（手势状态机/命中解析/供给器契约）+手测覆盖边界声明。
  7. **实现排程建议**（IMPL 拆卡：全局层/供给器各面/输入 bug 修复/收口，决策侧裁剪用）。
- 约束：八项裁定不偏离（要推翻回决策侧带用户）；零代码；Godot 仓代码勘察只读；Godot 仓无自动化测试基建如实申报。
- 验收标准：①手势状态机到状态/转移级 ②命中解析含跨面合并 ③供给器契约函数级+四类面逐一 ④输入 bug 排查路径可执行 ⑤旧路径处置明确 ⑥V2 九步清单 ⑦排程建议表
- 停止条件：发现 Godot 输入架构使 Alt+拖全局捕获不可行（如 dock 拦截层无法让位）→ 实证上交回决策侧（可能需用户改触发键位裁定）
- 领取：（2026-10-01 PC 执行侧 / origin/main=7649fc3a / port/vitnote-interact-v2-design-1，worktree=D:\Vit_DAW_worktrees\vitnote-interact-v2-design-1；Godot 参照=port/vitnote-dock-mount-1@66e59f0 只读）
- 回执：（文档 commit hash / 验收对照 / 核心设计决策）
- 验收：（裁定文件 / 验收 commit）

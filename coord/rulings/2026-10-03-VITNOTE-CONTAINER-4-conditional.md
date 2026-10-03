# Ruling：VITNOTE-CONTAINER-4 — conditional pass（2026-10-03 决策会话）

## 验收

1. **diff 亲核**（a2930a7@port/vitnote-container-4，Godot 6 文件=4 产品+2 工具 +712/-182）：图钉回归视窗（圈选范围**左上角** 22px=旧点 1.83x、压内容半透明 0.42、manager overlay 顶层直接子节点结构性不被轨道头遮、固定不可拖、单击重开/右键删/悬停 peek 沿用）；底部 pin tray 双侧退役（字段+方法探针断言锁定）；资料库 Notes 第四 tab 便签列（group/plugin 同款模式、单击重开仅收起态防焦点扰动、右键删真实链、空态文案+计数）。自由量实锚申报齐全（PIN_SIZE/ALPHA/左上角归属/实例方法非 static 的 headless 装载期理由）。
2. **我方独立复跑**（c4 worktree 亲跑，输出 runs/VITNOTE-CONTAINER-4/verify_pc/）：container 探针 **141/141 EXIT=0**；face_resolve 127/127+circle_state 52/52+panel_summary 16/16+input EXIT=0；`--import` 零 SCRIPT ERROR——与回执逐面吻合。
3. **环境耦合披露采信**：用户手测内核在场期间探针的 IPC 客户端被动只读连接（无写命令）如实申报；legacy 探针非逐字对比的归因（预期 diff+临时 id+环境耦合三类）合理。
4. ⚠ **可见性风险标注（决策侧独立核验）**：裁剪放大件与全幅收起态截图两读均**未能目辨图钉**（22px+半透明 0.42 在浅色背景上对比度不足的嫌疑；执行侧自报"分析器未辨"同款）。结构面（位置/alpha/存在性）已由 141 项探针锁定，但用户裁定原文的原始抱怨就是"看不清"——**转正条件加一条：用户目辨图钉无障碍**；若用户仍觉不清，随转正做一档视觉小修（尺寸/对比度/不透明度下限），不另开大卡。
5. 署名事故（commit 尾误带 Co-Authored-By: Claude Opus，实际全程 GLM 无 Opus 参与）如实留档更正，按纪律不 amend——采信。

## 裁定

- **conditional pass**：转正条件=用户手测复验（回执自验清单 7 项+图钉可见性确认）；同场转正 CONTAINER-3→CONTAINER-2→V2-IMPL-D。
- Godot 分支 `port/vitnote-container-4`@a2930a7 为现 HEAD——**NOTESTREAM-2 Godot 腿基线自此切出**（取代 CONTAINER-3 基线）。

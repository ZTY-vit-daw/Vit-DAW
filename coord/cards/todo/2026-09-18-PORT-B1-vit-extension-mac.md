# PORT-B1：vit_extension mac 目标（godot-cpp 就位 + SCons macos 构建 + gdextension 条目 + headless 加载验证）

- 优先级 / 预估 / 依赖：P2 / 0.5-1 天 / 无（层 B 首张实现卡）
- 模型分级：L2 / GLM-5.3（构建链接面 + Godot 扩展加载语义）
- 目标：仓内 `extension/` GDExtension 在 mac 可构建可加载：① repo 根取得 godot-cpp 源树（`extension/SConstruct:1` 引用 `../godot-cpp/SConstruct`；clone **godot-4.6.stable** tag，与本机 Godot `4.6.stable.official.89cea1439` 对齐——**未跟踪依赖不入仓**，确切 pin 记回执）；② `scons platform=macos arch=arm64` 双 target（template_debug/template_release）构建 exit 0 出 .dylib；③ `vit_extension.gdextension` 补 `macos.*` 条目（现仅 windows 两条）；④ 本机 Godot headless 加载验证（`--headless` import/等价机制，扩展加载零 error 日志）。**卡面口径注记**：PORT_AUDIT §1.3(c) 所记 `vit_webview_host` 源**从未入仓**（PC 本地未提交文件，决策侧 2026-09-18 git 考古核实：`git log --all -- '*vit_webview_host*'` 零命中）——仓内面仅 register_types + vit_waveform_reader，无 webview2 面，按仓内事实执行
- 文件域：`extension/`（SConstruct mac 适配如需、vit_extension.gdextension）；godot-cpp 为根级未跟踪依赖；不改 Godot 工程（仓外）、不改 `agent/`、`VitApp/`
- 验收标准：① godot-cpp pin 记录（tag+commit hash）；② scons 双 target exit 0 + 产物路径/大小/sha256；③ .gdextension macos 条目 diff；④ Godot 4.6 headless 加载 exit 0 且扩展注册无 error（日志入工件）；⑤ 工件目录：run ID + 构建/加载日志 + 产物哈希
- 停止条件：godot-cpp 4.6 与现有 SConstruct 构建面不兼容需非平凡改造 → 最小适配方案上交；headless 加载扩展报错 → 按类型记录证据转 blocked；需动仓外 Godot 工程才能验证 → 上交
- 领取：
- 回执：
- 验收：

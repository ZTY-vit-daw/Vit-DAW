# PORT-B4B：前端登 mac + 真栈就位 + macOS 导出预设（B4 主体，移植收官件）

- 优先级 / 预估 / 依赖：P1 / 1-1.5 天 / 依赖 PORT-B4A 运输包（先到达再开工）
- 模型分级：L3 / GLM-5.3（跨工程集成 + 真栈语义；关键点或证据冲突时找参谋决策模型讨论）
- 目标：
  1. **重建工程**：transfer 拉取运输包 → 解包至 `~/Documents/vit-daw-frontend/` → 按 README sha256 逐件核对 + 未提交状态清单比对（重建保真度声明）
  2. **CEF 就位**：按 B2P 手册 §2.2（main 上 `docs/CEF_MAC_PROBE_2026-09.md`）从本地探针资产（`~/Documents/vit-b2p-probe/` 上游 zip 或重取）补 `addons/godot_cef/bin/universal-apple-darwin/`
  3. **扩展就位**：B1 产物双 target dylib + mac .gdextension 条目拷入 `<前端>/extension/bin/`（镜像 `build_release.ps1:81` 落点契约 res://extension/bin/）
  4. **首次 import**：`godot --headless --import --path <前端>` **×2**——首轮 SIGSEGV 是 B1 ruling 已知引擎环境事实（首 import 文档生成段崩、二轮自愈），照实记录两轮退出码；终态验证 `.godot/extension_list.cfg` 含 vit_extension 与 godot_cef，headless 下 CefTexture 类可见
  5. **真栈二件在位**：mac 内核+agent 构建产物在位（复用 A5/C2 冒测脚本或其产物路径），**start_page mac 候选解析对实盘**：脚本级 dump 候选（B3 探针模式）逐条核对到真实二进制存在
  6. **导出预设**：`export_presets.cfg` 补 macOS 条目（dev 期 ad-hoc/无签名口径）；export templates 未装则条目就位+边界声明（不强制完成导出；templates 下载属网络面如实记录）
  7. **`[等待真栈验收]` 用户手测清单**：按用户裁定（手测入口=用户从 Godot 拉起 DAW 前端→启动页进入工程）写分步清单：拉起命令（含如需的环境变量在同一 shell 先设）、预期目检点（三件套拉起、启动页状态文案、WebUI 面板 CEF 渲染）、异常时日志取证路径——交决策侧转用户
- 文件域：`~/Documents/vit-daw-frontend/`（仓外重建与配置）+ 主仓 `scripts/`（mac 侧组装/校验脚本如需，命名对齐惯例）+ 工件目录；**不改** Godot 源仓文件内容（PC 侧资产只读）；`agent/`、`VitApp/Source` 零触碰
- 验收标准：①重建核对记录（sha256+状态清单比对）；②extension_list.cfg 双注册证据 + CefTexture 可见；③双 import 退出码记录（首轮 134+二轮 0 为预期形态，其他形态按类型记录）；④start_page 候选→实盘二进制核对表；⑤导出预设 diff + templates 边界声明；⑥用户手测清单成品；⑦工件目录 run ID/各段日志
- 停止条件：运输包核对失败（哈希不符/缺件）→ blocked 上交；二轮 import 仍崩或 CEF 类不注册 → 对照 B2P 手册 §4 坑位排查，仍败按类型转 blocked（B1 环境事实升级处置由决策侧裁定）；发现需改前端 .gd 逻辑才能起栈 → 域外上报（B3 域已验收，改动需新卡授权）
- 领取：2026-09-19 00:05 CST（Mac 执行流，用户直令「开工 PORT-B4B」，PC 侧确认 B4A 已完成推送）/ origin/main `d04d01fc0867c6f2215f0ff841f85fb575e1bd4d`（B4A done 回执之后；工作树干净单流）/ 分支 `port/b4b-frontend-on-mac`（仓外重建为主，仓内如需脚本再走分支）
- 回执：
- 验收：

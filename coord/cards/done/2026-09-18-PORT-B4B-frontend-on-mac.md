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
- 回执：**七步全过，`[等待真栈验收]` 用户手测清单见下**。工件 `~/Documents/vit-b4b-artifacts/b4b-20260919-0005/`（SUMMARY.md + 18 项证据文件）；前端仓（仓外无 remote）基线 `fa1d74b` + 集成提交 `58c6a3e`；主仓工作树零改动（三处落点 `VitApp/build/`、`agent/bin/`、`VitApp/Workspace/Logs/` 均 gitignore 覆盖）。分项：
  - **①重建**：tarball/bundle sha256 双符 README §0（`f5c4fdd4…`/`5600f054…`）；解包 exit 0，973 成员与 §1 一致；36 条排除清单逐条 ABSENT_OK、必备文件（project.godot/start_page.gd/两 .gdextension/3 字重/export_presets.cfg）全在场；README §3 状态清单比对：8×M PRESENT、9×D ABSENT、13×?? 按 §4 排除清单两分全符合；基线树 866 文件 sha256 存档；增量 bundle prerequisite `c7bcd29` 缺失拒解包（§2 预期形态，留档 transfer 不动）
  - **②CEF**：`~/Documents/vit-b2p-probe/downloads/godot_cef-v1.15.4.zip` 抽 mac 侧 → `addons/godot_cef/bin/universal-apple-darwin/`（965 条目 665M，手册 §2.2 同源）；libgdcef universal ✓、codesign --deep ✓、无 quarantine ✓；helper 执行位 644=手册 §4.2 已知坑，import 期 addon 自愈 `[CefInit] Set executable permissions for:` ×5 锚点全中
  - **③扩展**：B1 双 dylib 拷入 `extension/bin/`，sha256 与 B1 回执逐字节一致（`d0c0be10…`/`f31ac752…`）；`vit_extension.gdextension` 以主仓 B1 版（含 macos 两行）同步——前端运输版仅 windows 条目（PC 态如实）
  - **④双 import**：**首轮 0 + 二轮 0**（另加预设后第三轮 0 复验）——形态偏差：卡面预期首轮 134（B1 已知引擎事实）**未复现**，首轮即 reimport DONE+编辑器布局加载 DONE 干净退出；按验收③"其他形态按类型记录"，B1 E1 系最小测试工程、本工程未命中，非阻塞
  - **⑤注册/可见**：`.godot/extension_list.cfg` 双注册（godot_cef + vit_extension）；`--doctool --gdextension-docs` exit 0 出四类 XML：CefTexture/CefTexture2D/CefIpcInspector/VitWaveformReader——headless 类可见实证
  - **⑥真栈二件+候选对实盘**：内核复用 A5 run7 Debug 产物拷至 `VitApp/build/VitApp_artefacts/Debug/VitApp`（sha256 `fd0d3d84…` 与 A5 一致；同源性 `git diff 846d4f6 HEAD -- VitApp/` 零差异）；vitagent 当前 main 新构建（go build exit 0，agent/ 自 A5 后 3 个已验收提交含 7793927 hub 重试收编；sha256 `502a1aef…`；`/health` ok + SIGTERM 优雅 0 探活通过，7879 隔离端口）；候选核对表（startpage_candidate_check.txt，逐条转写自 start_page.gd@58c6a3e）：kernel Debug 候选 HIT（freshness 唯一命中即胜出）、agent `agent/bin/vitagent` HIT、hub 两候选缺席→`vsphub not found` warning 为**预期文案**（A5 形态无 hub 栈）；**F5 editor feature 实证**（一次性探针）：game-run 形态 `OS.has_feature("editor")=true`→dev 候选会被消费（与 PC 语义一致）；`VitApp/Workspace` 下 4 个 tracked 文件（Settings.xml 等）内核运行时会写入=与 PC 开发形态一致的预期变更
  - **⑦导出预设**：preset.1 就位（macOS/arm64/zip/`codesign/enable=false`/`com.vitdaw.dev`），diff 49 行入工件；**templates 边界：`export_templates/` 目录为空未安装，导出未执行**（卡面允许），下载属网络面未动；预设后 import 复验 exit 0 零 error
  - 端测覆盖边界声明（AGENTS §5）：本卡覆盖=重建保真+CEF/扩展就位+headless import/类注册+二进制在位与候选解析+vitagent 独立探活；**未含**：前端→内核/agent 实际 spawn 链路（`[等待真栈验收]` 手测）、CEF 实例渲染 WebUI 面板、模板导出链路
  - 停止条件未触发；vsphub 未放（卡面二件口径，hub 形态另议）

  **`[等待真栈验收]` 用户手测清单（按 2026-09-12 裁定手测入口书写，交决策侧转用户）：**

  **A. 前置自检（可选，终端执行，均应有输出）**
  ```bash
  ls -x ~/Documents/Vit-DAW/VitApp/build/VitApp_artefacts/Debug/VitApp   # 内核在位
  ls -x ~/Documents/Vit-DAW/agent/bin/vitagent                            # agent 在位
  ls -x ~/Documents/vit-daw-frontend/project.godot                        # 前端工程在位
  pgrep -fl "VitApp|vitagent" || echo "无残留进程（预期）"
  ```

  **B. 拉起（手测唯一途径=用户从 Godot 拉起前端）**
  1. 终端（或 Finder 打开 Godot 后 Import 选择工程目录 `~/Documents/vit-daw-frontend`）：
     ```bash
     ~/opt/bin/godot --path ~/Documents/vit-daw-frontend
     ```
     **默认无需任何环境变量**（mac dev root 缺省即 `~/Documents/Vit-DAW`，B3 语义）。如需可选项，必须在**同一 shell** 先设再拉起：`VIT_SKIP_DEV_AUTOSTART=1`（禁自动起栈）、`VIT_DAW_DEV_ROOT=<路径>`（改 dev 根，慎用——内核/agent 候选随根变动）
  2. 编辑器打开后，按 **F5（运行项目）** —— 必须是编辑器内运行（dev 候选仅在 editor feature 运行态消费，已实测实证；导出模板形态不适用）
  3. 出现 DAW 启动页窗口

  **C. 目检点（按序）**
  1. **自动起栈**：编辑器底部 Output/调试器面板出现内核与 vitagent 的启动输出；`start_page: vsphub not found; VSP Hub autostart skipped` **warning 为预期**（mac 未放 hub，A5 形态）
  2. **进程三查**（另开终端）：`pgrep -fl "VitApp/build/VitApp_artefacts"`、`pgrep -fl "agent/bin/vitagent"` 均有 pid；`lsof -nP -iTCP:5555 -iTCP:7878 -sTCP:LISTEN` 见内核 5555 + agent 7878（5556/5557 同属内核）
  3. **启动页就绪**：状态区**不出现** `Vit engine is not reachable. Check VitApp, vsphub, and vitagent.`（出现即 FAIL——文案无 .exe 后缀=B3 mac 分支生效的旁证）；最近工程列表正常渲染
  4. **进入工程**：从启动页打开/新建 `.vit` 工程进入 DAW 主界面（波形 dock、轨道等）
  5. **WebUI 面板 CEF 渲染**：打开含 WebUI 面板的界面，面板应出内容（CEF 首开冷启 ~3.5s 属正常，暖启 ~0.2s）；Output 面板可 grep `[CefInit]`/`CefTexture` 锚点
  6. **收尾**：正常退出 Godot；CEF helper 随 Godot 全灭（B2P 实证无残留）；**内核无优雅退出面**（A1 事实），若 `pgrep -fl VitApp` 仍有残留：`pkill -TERM -f "VitApp/build/VitApp_artefacts"`（退出码 143 属正常形态）；vitagent 同法 `pkill -TERM -f "agent/bin/vitagent"`

  **D. 异常取证路径**
  - 编辑器 Output 面板全文（起栈段 + `[CefInit]` 段）复制存档
  - agent 日志：`~/Documents/Vit-DAW/VitApp/Workspace/Logs/agent_last.log`（+ `agent_llm_telemetry.jsonl`、`agent_message_loop_debug.jsonl`）
  - 进程/端口态：`ps aux | grep -E "VitApp|vitagent|CEF"`、`lsof -nP -iTCP -sTCP:LISTEN | grep -E "5555|5556|5557|7878"`
  - 前端仓状态：`cd ~/Documents/vit-daw-frontend && git status --short`（预期：`.godot/` 等运行态外无源码改动）
  - 主仓 `VitApp/Workspace` tracked 文件（Settings.xml 等）被内核写入属**预期变更**，非污染，勿 reset
- 验收：pass（2026-09-19，ruling [2026-09-19-B4B-pass.md](../../rulings/2026-09-19-B4B-pass.md)：决策侧实盘抽验全过（sha256/dylib 哈希=B1/双注册/候选表/双二进制）；双 import 0/0 形态偏差采认（B1 引擎首崩范围收窄入档）；主仓零改动无 cherry-pick；**手测清单已转用户，[等待真栈验收]**）

# Python Bridge 退役评估（PORT-B6 决策输入）

- 日期：2026-09-18｜执行：Mac 执行侧（PORT-B6，L2/GLM-5.3）｜性质：零代码改动只读评估
- 评估对象：`scripts/bridge_core.py`（582 行）/ `scripts/bridge_prod.py`（61 行）/ `scripts/godot_bridge.py`（dev 入口变体，仅 14 行装配 `BridgeConfig(verbose=True, dev 日志)`）
- 问题：mac 移植期该链**退役**（Go agent 已全覆盖）还是 **mac 化保留**（含 python 运行时 mac 化）？
- **结论先行：建议 mac 侧退役。B3 卡按"剔除该链"设计，无需 python 运行时 mac 化子项。**
- 证据边界声明：Godot 前端工程（`D:\Godot\project\vit-daw-frontend`）不在 Mac 本机，`start_page.gd` 相关事实引用 `docs/PORT_AUDIT_2026-09.md` §1.3(d)（PC 决策侧 2026-09-15 审计 + 2026-09-18 复核），Mac 侧未做本地复核；本仓其余结论全部来自本地代码锚点（领取基线 `05ddda4`，工作树干净）。

---

## ① bridge 实际职能清单（逐函数，附锚点）

bridge 是 **Godot(UDP) ↔ VitApp(ZMQ) 的协议转换器**，契约见 `docs/VIT_IPC_CONTRACT.md:5-6`（遥测 ZMQ SUB→UDP 4444；指令 UDP 4445→ZMQ REQ，应答原路 UDP 返回）。逐职能：

| # | 职能 | bridge_core.py 锚点 |
|---|---|---|
| F1 | 控制转发：UDP :4445 收 Godot 命令 → 原文转发 ZMQ REQ `tcp://127.0.0.1:5555` → 应答回传 UDP | `run_bridge` 控制循环 :472-540；`BridgeConfig` :138-146 |
| F2 | REQ 超时/重试/错误应答（timeout 时回 `{"status":"error","message":"bridge timeout to VitApp"}`） | :506-519（req_max_retries=1、req_timeout_ms=180000 :144-145） |
| F3 | 控制大应答落盘：>60KB 写临时目录 `vit_daw_bridge_replies`，回 `transport=file_reply` envelope（含 `reply_file` 绝对路径） | 常量 :17；`_make_file_reply` :48-63；`_send_control_reply` :105-134 |
| F4 | 遥测转发：ZMQ SUB `tcp://127.0.0.1:5556` → 规范化为严格单行 JSON → UDP 发 Godot :4444 | `bridge_telemetry` 线程 :384-465（规范化注释 :434-435） |
| F5 | 遥测大包落盘：>32KB 写文件，回 `file_reply` envelope（含 `telemetry_file`） | 常量 :18；`_make_telemetry_file_reply` :84-102；:438-454 |
| F6 | 影子工程：`get_project_state` ok 全量初始化 + `delta_update` 增量合并（`nodes_by_uid`）、初始化前 delta 缓冲重放、非快照轨 orphan 历史（maxlen=20） | `VitShadowProject` :191-334（pre-init 缓冲 :269-277；orphan :198,214-219；初始化挂接 :521-538） |
| F7 | 停录强制刷新：`recording/recording_stopped` PUB → 必发一次 `get_project_state` 覆盖影子（V0.6 契约，`VIT_IPC_CONTRACT.md:893-899`） | :419-423（入队）+ :474-482（消费）+ `_shadow_refresh_after_recording_stop` :346-371 |
| F8 | delta `seq_id` 断档告警 | :407-418 |
| F9 | 滚动末日志（默认 `VitApp/Workspace/Logs/bridge_last.log`，尾部 300-400 行） | `BridgeLogger` :152-189；`bridge_prod.py:29-43` |
| F10 | Windows UDP ICMP port-unreachable 规避（SIO_UDP_CONNRESET） | :27-34 |
| F11 | 配置装配：env `VIT_BRIDGE_*` > `bridge_prod.config.json` > 默认值 | `bridge_prod.py:16-57`（dev 入口 `godot_bridge.py` 为 verbose 变体） |
| F12 | 诊断类：`tile_ready` 转发计数日志 :424-433；UDP 发送失败节流告警 :455-463；TRANSPORT position 高频噪声日志降频 :210-212 | 仅影响日志，不改变转发行为 |

运行链位置（PORT_AUDIT §1.3(d) 转引）：Godot `app/startup/start_page.gd:355-380` 经 `python_embed/python.exe` 拉起 `runtime/bridge_prod.py`（或 bridge_core.py），设 `PYTHONPATH`/`VIT_BRIDGE_CONFIG` 并注册子进程看护。桥的消费方仅此一处运行链；`scripts/b5_workspace_switch_forensics.py:61`、`scripts/b6_mixtick_terminal_forensics.py:78` 仅在注释里提及 pyzmq 属脚本环境依赖，不是桥进程消费者。

## ② Go agent 对照覆盖情况

**"Go 零引用"独立验证**（Mac 本机，领取基线 05ddda4）：

```
$ find . -name "*.go" -not -path "./.git/*" | wc -l
     791                                    # 全仓 .go 文件，全部位于 agent/（agent/ 外为 0）
$ grep -rn "bridge_core\|bridge_prod\|godot_bridge" --include="*.go" agent/; echo exit=$?
exit=1                                    # 零匹配（grep exit=1 = 无选中行）
$ grep -rn "bridge_core\|bridge_prod" VitApp/Source 2>/dev/null | wc -l
       0                                  # 内核 C++ 侧同样零引用
$ grep -rn "python_embed\|python\.exe" --include="*.go" --include="*.ts" --include="*.tsx" agent/ | wc -l
       0                                  # agent/webui 对 python 运行时零依赖
```

**职能对照**（Go 侧实现在 `agent/internal/bridge/bridge.go`（639 行）+ `agent/internal/shadow/`，由 `agent/cmd/vitagent/main.go:57-87` 无条件启动——Go agent 本身就是桥）：

| # | python 职能 | Go 覆盖锚点 | 判定 |
|---|---|---|---|
| F1 | 控制转发 | `bridge.go:111-152`（runControl，UDP :4445→kernel.ZMQ 5555）+ `:228-248`（forwardCommand→kernel.SendRaw） | **已覆盖** |
| F2 | 超时/重试/错误应答 | `:232-243`（重试循环 + `kernel.ErrorReply("agent timeout to VitApp: …")`）；超时/重试参数 `main.go:33-34` | **已覆盖** |
| F3 | 控制大应答落盘 | `spillControlReply` `:192-226`（envelope 同构：transport/reply_file/reply_bytes，增补 status/cmd/request_id）；落盘目录默认 `os.TempDir()/vit_daw_agent_replies` `:593-595` | **已覆盖**（阈值 32KB:86 vs python 60KB——差异见下） |
| F4 | 遥测转发+严格 JSON | `telemetryOnce` `:279-323` + `normalizeTelemetry` `:400-435`（json.Unmarshal→Marshal 重序列化，同 python 语义） | **已覆盖** |
| F5 | 遥测大包落盘 | `spillTelemetryPacket` `:363-398`（同 32KB 阈值:87，envelope 含 telemetry_file） | **已覆盖** |
| F6 | 影子工程 | `agent/internal/shadow/shadow.go`：preInitDeltas 缓冲重放 :22,:90-93,:225；orphanDeltas maxlen=20 :14,:285-289；测试 `shadow_test.go:9`（Initialize+delta）、`:35`（orphan） | **已覆盖**（且扩展了 change receipt/summary 能力，python 无） |
| F7 | 停录强制刷新 | refreshCh `:417-421` + `runShadowRefresh`/`refreshShadow` `:466-491` | **已覆盖** |
| F8 | seq 断档告警 | `normalizeTelemetry` `:409-415` | **已覆盖** |
| F9 | 滚动末日志 | `logx` 包 + `main.go:36-37`（`--last-log-path`/`--keep-last-log-lines`，默认 `VitApp/Workspace/Logs/agent_last.log` `:209-218`） | **已覆盖** |
| F10 | connreset 规避 | `:139`（"forcibly closed" 错误串过滤，WSAECONNRESET 等价处理；Go/net 在 darwin 无此问题） | **已覆盖**（等价） |
| F11 | 配置装配 | `main.go:27-41`：flag > `VIT_AGENT_*` > **`VIT_BRIDGE_*` 兼容回退** > 默认值（端口/超时/重试/日志全参数同名对齐） | **已覆盖**（配置可直接迁移） |
| F12 | 诊断类 | tile_ready 无专门计数（转发由 F4 泛化覆盖）；UDP 失败逐次告警 `:315-320`；另有 python 没有的 `legacyDiag`（`VIT_BRIDGE_LEGACY_TELEMETRY_DIAG`）:325-354 | **已覆盖**（仅诊断粒度差异，无功能语义） |

**Go 超集**（python 桥没有的）：`TelemetryHook`→chat 服务消费（`main.go:65`）；VSP realtime/transport latest-only 发布通道（`bridge.go:76-81,:92-94`）；legacy UDP 抑制（`:437-449`——VSP hub 启用时 `delta_update`/`levels`/`transport` 不再走 legacy UDP，改由 VSP realtime 供数，`VIT_BRIDGE_KEEP_LEGACY_*_UDP=1` 可还原）；HTTP chat API :7878（`main.go:68-80`）。

**逐项对照结论：已覆盖 12/12，部分/空白为零。** 停止条件（发现 Go 未覆盖且无法判定可否迁移的运行时职能）**未触发**。

**权威佐证**：`docs/vsp/VSP_V1_CLOSEOUT_REPORT.md:108`（现行收口报告）原文——"当前生产/开发桥接主路径是 Go VitAgent：Godot command UDP 4445 -> VitAgent -> Kernel ZMQ 5555，Kernel telemetry ZMQ 5556 -> VitAgent -> Godot UDP 4444。因此收口证据必须以 `agent/internal/bridge/bridge.go` / `agent/cmd/vitagent` 为准；Python `scripts/bridge_core.py` 仅作为历史/打包 fallback 参考。"`docs/vsp/VSP_MIGRATION_FROM_LEGACY_BRIDGE.md:26-33`（CURRENT-STATE 标记现行）已将 "historical Python bridge scripts" 列为兼容遗留件。当前 dev 烟测栈（`scripts/dev_agent_smoke.ps1`、`scripts/dev_agent_smoke_mac.sh:221`）均只构建/拉起 VitAgent，无 python 桥环节。

## ③ mac 侧退役影响面

**start_page.gd 链路改动点**（B3 文件域，行号引 PORT_AUDIT §1.3(d)）：

- :355-380 python 桥整段（拉起 `python_embed/python.exe` + `PYTHONPATH`/`VIT_BRIDGE_CONFIG` + 子进程看护）在 mac 分支中不包含；PC 分支原样保留。
- 连带同文件既有 mac 化项（B3 主任务本就覆盖）：:144-182 内核候选 `VitApp.exe`、:191-194 agent 候选 `vitagent.exe`、:116 状态文案 `VspHub.exe`。
- **互斥事实（本地可证）**：vitagent 无条件运行桥 goroutine 并绑 UDP :4445（`main.go:57-87`、`bridge.go:111-119`），python 桥绑定同一端口（`bridge_core.py:380`）——同链路两者**不能并存**。故 start_page.gd 的桥拉起必为条件分支（vitagent 缺失时 fallback，与 `assemble_windows_release.ps1:115` "VitAgent.exe not found … will fall back to Python bridge" 的打包语义一致）或已是死路径；**确切触发条件在 Mac 侧无法独立核实（Godot 工程不在本机），B3 现场确认**。若确认是无条件并行拉起，须先改条件分支再退役，否则会出现 4445 双绑失败+看护重启循环。

**发布打包剔除 python_embed 的连带**（全部为 Windows 打包脚本，mac 无存量依赖）：

- `python_embed` 唯一消费者是桥 fallback：`scripts/bundle_python_embed.ps1` 头注即"Bundle Windows embeddable Python + pyzmq … (no system Python required)"，唯一用途为运行桥；Go/webui 侧 `python_embed|python.exe` 零引用（见② grep）。
- 受理脚本：`build_release.ps1:73-75,174-190`（拷贝 bridge 三件）`:298-300`（python_embed 缺失警告，文案即"桥接仍依赖系统 Python"）；`scripts/assemble_windows_release.ps1:64,115-123`（bridge fallback 拷贝）`:51,181-183`（python_embed 拷贝）`:224,:233`（清单文案）。
- mac 打包路径（未来若建）不含上述段即无连带；本建议不要求改动任何 Windows 打包脚本。

**PC 侧是否受影响：零。** 本结论限定"mac 处置=退役"：`scripts/` 三文件、`bridge_prod.config.example.json`、Windows 打包 fallback 链全部保留在仓，PC 发布链原样。

**文档卫生（供决策侧另行处置，不在本卡文件域）**：`VIT_IPC_CONTRACT.md:6` 仍写"发布入口：scripts/bridge_prod.py，开发入口：scripts/godot_bridge.py"，与② 的 Go 主路径现状不符（该文档在 CURRENT-STATE 现行清单，CURRENT-STATE.md:110 已注"UDP 4444/4445 桥仍在现役链路中"——桥角色在役，但由 VitAgent 承担）；《Vit-DAW 发布模式一键启动配置清单》（CURRENT-STATE.md:113，现行运维节）内容仍是 python 链口径（:12 "启动桥接 python scripts/bridge_prod.py"）。建议 B3 收口时由决策侧同步修订两处表述。

## ④ 建议 + 理由与风险

**建议：mac 侧退役。** B3 卡按"剔除该链"设计——start_page.gd mac 分支不拉桥、不设 python_embed、不包含 python 运行时 mac 化子项；vitagent（darwin 构建）作为唯一桥接承担者。

**理由**：

1. **职能全覆盖无空白**：12 项逐职能对照全部"已覆盖"（②），且 Go 侧是超集（chat 消费、VSP realtime、HTTP API）。
2. **权威口径已定**：VSP 收口报告（现行）明示生产/开发主路径=Go VitAgent，python 桥仅历史/打包 fallback；VSP 迁移收口门槛（`VSP_MIGRATION_FROM_LEGACY_BRIDGE.md:56-65`）已达成。
3. **契约兼容，切换零迁移成本**：端口（4444/4445/5555/5556）、`VIT_BRIDGE_*` env、CLI 参数、file_reply envelope 全部同名同构（`main.go:27-41`）。
4. **保留的真实代价**：python 官方 embeddable 发行版仅 Windows；mac 化保留需为桥单独解决 python 运行时分发（系统 Python 版本漂移、捆绑 framework 等均是新增维护面）+ pyzmq 依赖，换取的只是一个已被 Go 覆盖的 fallback。

**风险与缓解**：

| 风险 | 评估 | 缓解 |
|---|---|---|
| R1 start_page.gd 桥拉起条件未在 Mac 本地核实（无条件并行拉起则与 vitagent 端口冲突） | 事实缺口，但恰在 B3 自己的文件域内 | B3 现场读 gd 源确认；若无条件拉起，先改条件分支再剔除 |
| R2 Go 桥在 VSP hub 启用时抑制 legacy UDP（delta/levels/transport 改走 VSP realtime），与 python 桥全量转发不同 | 有意设计（`bridge.go:437-449`），PC 生产链已是此行为，前端 VSP realtime autoload 已具备（VSP closeout :100 验证项） | mac 沿用 PC 同链路；层 C 烟测覆盖 realtime 面 |
| R3 控制 inline 阈值 60KB→32KB、落盘目录 `vit_daw_bridge_replies`→`vit_daw_agent_replies` | envelope 契约同构，Godot 按 envelope 内绝对路径读文件，消费方无感 | 无需动作；B3 手测时留意大应答场景即可 |
| R4 两份现行文档仍写 python 入口口径 | 文档卫生问题，不影响运行时 | 决策侧在 B3 收口时修订（③ 末段） |
| R5 未来需要 python fallback 回归 | 仓库文件不删，仅 mac 运行链剔除 | 任何时刻可恢复；PC 打包 fallback 不受影响 |

**对 B3 卡的直接设计输入**：退役路线成立，B3 无 python 运行时 mac 化子项；B3 须包含"确认/整改 start_page.gd 桥拉起条件"这一前置检查项（R1）。

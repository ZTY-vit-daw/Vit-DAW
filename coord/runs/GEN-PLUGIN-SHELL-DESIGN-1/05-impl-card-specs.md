# 设计件五：实施拆卡规格（6 张，可直接发卡）

- 归属：GEN-PLUGIN-SHELL-DESIGN-1 / 2026-10-12
- 拆卡原则：壳体骨架先行（后续卡的被宿主实体）→ 宿主侧服务（通道+队列）→ agent 执行腿（依赖 GEN-CTX-INJECT-1 合入，卡面归属行已钉）→ GUI/PCA/试听三腿并行。每卡独立可验收、可独立回滚（AGENTS §12 一卡一 commit）。
- 每卡验收含**端侧烟测 exit 0**（AGENTS §5：真实内核+agent 栈，脚本体系按 `scripts/gen_chain_survey_smoke_mac.sh`/`gen_api_adapter_smoke_mac.sh` 同模式新增 `scripts/gen_shell_*_smoke_mac.sh`；PC 执行时按 `dev_agent_smoke.ps1` 参数模式对写）。GUI 视觉面不属自动化验收（各卡端测边界声明）。

## 卡 1：GENSHELL-CPP-1（壳体骨架卡）

- 优先级 / 预估 / 依赖：P1 / 2-3 天 / 无
- 目标：`/GenShell/`（仓库根新目录，先例=/PluginProbe/native-host）CMake 工程——VST3 SDK 直实现壳体：`IEditController`+`IAudioProcessor` 骨架（instrument，静音输出）、T0 消费（IProcessContext 读 tempo/拍号，仅缓存）、T1 五参数注册（件一 §3 枚举）、T3 chunk 读写（件一 §5 schema）、VSTGUI 编辑器静态骨架（六区占位，无业务逻辑）、T2 IConnectionPoint 壳侧端点（收发信封 vit.genshell.msg.v1，消息先回 echo 计数）。构建产物 `GenShell.vst3` 部署到本机标准 VST3 目录。
- 文件域：`/GenShell/**`（全新）；`scripts/gen_shell_skeleton_smoke_mac.sh`（新）。**第一步（卡内必做）**：核实 VST3 SDK 在执行端的获取路径（tracktion_engine 子模块内 SDK / PluginProbe native host 的 SDK 来源——PC 完整检出核实，Mac worktree 子模块空目录为 worktree 工件）；VSTGUI vendoring（Apache-2.0）方式入卡记录。
- 验收标准：
  1. 构建：CMake 构建 `GenShell.vst3` 双平台（执行端先一端，另一端随其端首个卡）exit 0。
  2. 真栈烟测（exit 0）：内核起 → `scan_plugins`/`plugin_list_available` 发现 GenShell（identifier 断言）→ `rack_add_node`（**UI 路径形态**=直接内核命令，不经 agent 加载门——PCA 卡 5 前的合法装载方式）→ `get_plugin_parameters` 断言 T1 五参数在场 → `set_plugin_param`(SceneMode) 读回一致 → chunk 往返（工程 save→load 表单草稿字段保真）→ `show_plugin_editor` 不崩（进程存活断言，不做视觉断言）。
  3. T2 自环：烟测脚本经宿主服务（卡 2 前可临时用壳单测进程模拟宿主端）发 `context_snapshot` 消息 → 壳回执计数暴露在 debug 参数。
- 停止条件：VST3 SDK 获取路径在两端均不可复用（需全新 vendoring 超预估）→ 上交换 SDK 策略；VSTGUI 在目标编译器不可用 → 降级裁定（JUCE 插件+通道降级 A，件一 §7 fallback 入风险表 06 §5）。
- 端测边界：编辑器视觉面=零断言（进程存活+参数面为数据级验收）；手测入口按 AGENTS §5 Godot 前端拉起。

## 卡 2：GENSHELL-HOST-1（宿主侧服务+通道卡）

- 优先级 / 预估 / 依赖：P1 / 1-1.5 天 / GENSHELL-CPP-1 合入后
- 目标：内核 `GenerationShellService`（VitApp/Source/Service/ 新文件）——①壳实例注册表（rack_add_node 后按 identifier 识别，持有 IConnectionPoint host 端实现，件一 §2 主选路径接线+降级序核实）；②提交队列（submit_request 入队→job upsert queued+ghost 置位；cancel→upsert cancel_requested）；③新读命令 `gen_shell.pending` 注册（应答 jobs+providers，件二 §4 结构）；④`context_snapshot`/`provider_catalog`/`job_progress`/`job_terminal`/`audition_ready` 下行推送（GUI 轮询 500ms 进程内读服务状态缓存）。
- 文件域：`VitApp/Source/Service/GenerationShellService.{h,cpp}`（新）；`VitApp/Source/Service/CommandDispatcher.cpp`（仅 `gen_shell.pending` 注册行+路由，~2784-2803 邻域新增，**不触** handleGetProjectState 披露块 3291-3448——GEN-CTX-DISCLOSE/KEYFIELD 域避让）；`VitApp/CMakeLists.txt`（源文件登记）；`scripts/gen_shell_host_smoke_mac.sh`（新）。
- 验收标准：
  1. 真栈烟测（exit 0）：装载壳 →（经服务直调测试钩子或壳 GUI 骨架按钮）伪提交 → `gen_shell.pending` 应答含 queued 作业+请求载荷 → 伪推进（job upsert running→done）→ 壳 debug 参数 JobState 镜像一致 → cancel 路径 upsert cancel_requested 可见 → ghost 置位/清除（`get_project_state` jobs 快照断言 job_state/ghost_state）。
  2. IConnectionPoint 主选路径核实结论入回执（成立/降级 A 启用——件一 §2）；降级启用时队列语义不降（烟测同断言）。
  3. 既有命令面零回归：全量 `go test ./... -count=1` 0 FAIL（agent 侧零改动应零 diff）+内核全量编译零警告增量。
- 停止条件：从 te::ExternalPlugin/JUCE 包装层取裸 VST3 接口在完整 tracktion_engine 检出中无路径（主选+降级 A 均不通）→ 上交（降级 B loopback socket 决策点，风险表 06 §5）。
- 端测边界：内核+agent 栈数据面；无 Godot 前端旅程断言。

## 卡 3：GENSHELL-AGENT-1（agent 执行腿卡）

- 优先级 / 预估 / 依赖：P1 / 1-1.5 天 / **GEN-CTX-INJECT-1 合入**（genctx 快照+渲染器可用）+ GENSHELL-HOST-1
- 目标：`agent/internal/genshell/` 新包——后台 runner（随 agent 主进程启动的 goroutine）：2s 轮询 `gen_shell.pending` → claim → genctx 快照组装+`RenderGenerationPrompt(profile=gen_plugin, scene)` → `genapi.Generate` 原样调用 → WAV 产物 `bridge_ingest_generated_asset`（track/start/take_stack/job 从请求）→ 终态 upsert+job_terminal；`render_only` dry-run 路径（预览，件三 §4）；取消监听+`interrupted` 孤儿清扫（件二 §7）；失败分类透传（件二 §3）。harness 本地工具接线（`gen_shell` 管理面：启停/状态查询，RiskDirect）。
- 文件域：`agent/internal/genshell/**`（全新）；`agent/internal/tools/catalog.go`（gen_shell.pending argspec+gen_shell 管理工具注册）；`agent/internal/harness/harness.go`（本地工具分发区 ~2463-2474 邻域增 case）；agent 启动接线一处（server/main 装配点，领取时按 LOUDNESS-A-CONFIG 锚点模式定位）；`scripts/gen_shell_agent_smoke_mac.sh`（新）。
- 验收标准：
  1. 真栈烟测（exit 0，回环 provider 模拟器先例=gen_api_adapter_smoke_mac.sh 模式）：壳伪提交（in_project）→ runner 领取 → 发送 prompt 含四块结构（注入块在场证据——MUSIC CONTEXT/CONSTRAINTS 块在模拟器收到的请求体中断言）→ 产物 WAV 落盘 → ingest 落轨（clip_id/asset_ref/take_stack 断言）→ job 终态 done+壳参数镜像 → 多候选 count=2 同栈双 take+`switch_asset_take` 切换 → 取消负探针（提交后立即取消→cancelled、无落轨）→ 失败类别负探针（401 provider→auth_error 镜像）→ free_creation 模式（零注入块断言+不落轨+工件路径回执）。
  2. 孤儿清扫：runner 重启（模拟 agent 重启）→ running 作业→interrupted 断言。
  3. 密钥纪律：烟测断言模拟器日志/内核日志/`gen_shell.pending` 应答中零 key 材料（genapi SecretFree 先例沿用）。
  4. 全量 `go test ./... -count=1` 0 FAIL+gofmt 净；避让声明（capabilitycontext/contextruntime/promptruntime 零触碰）入回执。
- 停止条件：GEN-CTX-INJECT-1 落地形态与本卡假设（RenderGenerationPrompt 签名/profile 枚举）不符→回设计侧修订对接清单（件三 §5）；`gen_shell.pending` 应答形态与卡 2 实现漂移→两卡对齐修订（同基线设计件二 §4 为准）。
- 端测边界：agent HTTP 面数据断言；无 webui/旅程改动。

## 卡 4：GENSHELL-GUI-1（模板面+GUI 卡）

- 优先级 / 预估 / 依赖：P2 / 1-2 天 / GENSHELL-AGENT-1（dry-run 预览可用）
- 目标：模板引擎（壳 C++ 侧：`vit.genshell.template.v1` 解析+`compose` 纯函数+受控字段四种控件渲染+内置四模板+`~/.vit/gen_shell_templates.json` 用户模板加载/fail-closed）+ GUI 六区业务接线（件三 §4：模板下拉/动态表单/自由文本/上下文面板+预览/provider+scene+区域/作业区含候选切换与重试）。
- 文件域：`/GenShell/**`（壳体域内新增模板与 GUI 源文件）；内置模板 JSON 资源；`scripts/gen_shell_gui_smoke_mac.sh`（新，数据面）。
- 验收标准：
  1. compose 确定性单测（同表单值同 free_text）+字段类型四控件 roundtrip+模板文件 fail-closed 负例（未知 schema_version/坏 JSON 拒载且 GUI 提示）——壳体自带单测入构建。
  2. 真栈烟测（exit 0，数据面）：选模板→表单值→（GUI 或驱动钩子）提交→模拟器收到 free_text 含编排行（断言字段行在场+用户原文置尾）→dry-run 预览回显四块（预览=实发：预览文本与实发 prompt 逐字节一致断言——同请求参数下）。
  3. 手测清单交付（[等待用户]：六区视觉/交互经 Godot 前端拉起目检——AGENTS §5 手测入口）。
- 停止条件：VSTGUI 控件集不足以表达四类型表单（降级自绘）→ 范围上交；无其他。
- 端测边界：GUI 视觉=手测清单，不入自动化门（如实声明）。

## 卡 5：GENSHELL-PCA-1（PCA 准入面卡）

- 优先级 / 预估 / 依赖：P2 / 0.5-1 天 / GENSHELL-CPP-1（有可指纹二进制）
- 目标：`generation_shell` processor family 三处词表（processorintent/intent.go 家族枚举、processorattestation v2 家族表 v2.go、processorregistry/registry.go 注册——coverage v1=空集，件四 §2）+本地认证路径跑通（FingerprintPath 对 GenShell.vst3 bundle）+agent 侧装载门断言：经 agent 的 `rack_add_node` 在 promoted attestation 在场时放行、缺席时 `pca_load_gate:` 拒绝（fail-closed 双向探针）。
- 文件域：`agent/internal/processorintent/intent.go`、`agent/internal/processorattestation/`（v2 家族表+测试）、`agent/internal/processorregistry/registry.go`；`scripts/gen_shell_pca_smoke_mac.sh`（新）。
- 验收标准：
  1. 真栈烟测（exit 0）：认证签发（pcactl/认证入口按现行面）→ agent 发 rack_add_node 装载 GenShell 成功（job/audit 留痕）→ 撤 attestation（v2 store 移除）→ 同装载被拒（错误串断言）→ UI 路径（直接内核命令）装载不受影响（既有先例回归）。
  2. 单测：家族词表三处一致性+未知家族 fail-closed 回归；全量 `go test ./... -count=1` 0 FAIL。
- 停止条件：CertifyProcessor 硬性要求非空 inspect/apply 工具对（local_processor_certification.go:234-247 约束）→ 上交换方案（gen_shell.inspect 只读工具/认证 token 通道），不擅自造 typed apply。
- 端测边界：agent HTTP 面断言。

## 卡 6：GENSHELL-AUDITION-1（缓冲播放+打印腿卡）

- 优先级 / 预估 / 依赖：P2 / 1-2 天 / GENSHELL-AGENT-1
- 目标：壳 instrument 实播（audition_ready→后台线程解码+线性重采样+走带同步流播+Bypass 静音）；print 路径（runner `print` 模式：壳图内输出就绪等待→`start_render(file_path=工件目录, range=目标区域)`→WaitRender（renderResults 既有面）→WAV→ingest——件二 §6）；非 WAV 产物可解码性负探针。
- 文件域：`/GenShell/**`（audio 路径源文件）；`agent/internal/genshell/`（print 模式分支）；`scripts/gen_shell_audition_smoke_mac.sh`（新）。
- 验收标准：
  1. 真栈烟测（exit 0）：产物落轨后 print 路径全链（渲染 job_id→render_done→WAV RIFF 断言→ingest clip_id 断言，GEN-MAC-BASELINE render 腿 A 同模式）→fast/print 双路径同请求产物 clip 等价（时长断言）→非 WAV 负探针（mp3 工件→按可解码性二值断言：落轨成功或 unsupported_format_for_landing 终态，不静默）。
  2. 图内流播数据面：装载壳→注入测试缓冲→`start_render` 壳轨区域→输出 WAV 非静音断言（RMS>阈值）——音频链证据不依赖人耳。
  3. 采样率不匹配单测（44.1k 工件进 48k 工程→重采样后时长保真 ±1 样本）。
- 停止条件：render 期白名单互斥与新命令冲突（CommandDispatcher.cpp:2242-2276 面变化）→ 锚点上交。
- 端测边界：播放听感=手测清单（[等待用户]），自动化到 WAV 数据级为止。

## 序列与并行图

```
GENSHELL-CPP-1 ──→ GENSHELL-HOST-1 ──→ GENSHELL-AGENT-1 ──→ GENSHELL-GUI-1
     │                                     │        │
     │                                     │        └──→ GENSHELL-AUDITION-1
     └──→ GENSHELL-PCA-1（二进制可指纹即领） ┘
（GEN-CTX-INJECT-1 合入 = GENSHELL-AGENT-1 硬前置；卡面归属行钉死）
```

- 总量：6.5-11 天；关键路径 CPP→HOST→AGENT ≈ 4.5-6 天（受 GEN-CTX-INJECT-1 排期约束）。
- 最小可用产品链=卡 1+2+3（无 GUI 也能经驱动钩子全链生成落轨）；4/5/6 为产品完整度与准入腿，可并行/后置。

## 互斥文件清单（并行领取核对用）

| 卡 | 独占/共享文件 |
|---|---|
| 1 | /GenShell/**（全新）；scripts/gen_shell_skeleton_* |
| 2 | VitApp/Source/Service/GenerationShellService.*（全新）；CommandDispatcher.cpp **注册区 2784-2803 邻域**（与 GEN-CTX-DISCLOSE-1/KEYFIELD-1 的 3291-3448 不同区同文件——领取前按 PROTOCOL §3 冲突规则核对）；VitApp/CMakeLists.txt |
| 3 | agent/internal/genshell/**（全新）；tools/catalog.go（与 GEN-CTX-KEYFIELD-1 同文件——串行核对）；harness.go 分发区邻域 |
| 4 | /GenShell/** 模板与 GUI 源（与卡 1/6 同目录树——串行领取或按 PROTOCOL §3 核对） |
| 5 | processorintent/processorattestation/processorregistry 三包 |
| 6 | /GenShell/** audio 源+agent/internal/genshell print 分支（与卡 3/4 域重叠——串行） |

- /GenShell/** 为壳体线共享域：卡 1→4→6 按序领取（或决策侧明确并行拆分后分派）；卡 5 独立于壳体域。

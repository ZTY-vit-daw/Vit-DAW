# Mac 移植审计报告（PORT-AUDIT-1）

- 日期：2026-09-15；执行：GLM 执行会话（卡 PORT-AUDIT-1，P1 只读勘察）
- 勘察基线：`D:\Vit_DAW` HEAD `a7bbc3b`（main），工作树 6 项既有改动与本卡无关未触碰
- 方法：静态扫描（grep/源码阅读/GitHub API/web 核实）+ 本机可执行验证（darwin 交叉编译）；**零生产代码改动**
- 起点核对：以 PORT roadmap 2026-08-27 快照（`queue/todo/2026-08-28-PORT-roadmap-mac-porting-after-phase-d.md`）为基线，逐项核对腐化并找新增项
- 环境边界（如实收口）：MacBook Air runner 未就位，本报告所有结论为**静态审计 + Windows 侧实证**；需真机的动态项列入 §7 待办，不冒充已验证

---

## 1. 六项勘察结论

### 1.1 内核 Win32/平台面全扫描

自研 C++ 面（`VitApp/Source`，排除 `build/`、`Tests/build/` 下的第三方依赖树）扫描结果：

**未守卫的 `windows.h` 依赖：3 个文件（其中 1 个为 roadmap 未记录的新增项）**

| 文件 | Win32 用法 | roadmap 状态 |
|---|---|---|
| `VitApp/Source/Service/TiledSpectrogramBaker.cpp:7` | `#include <windows.h>`（裸）；:927/:943 `CreateFileMappingA`/`MapViewOfFile` | 已知（2 baker 之一） |
| `VitApp/Source/Service/WaveformEnvelopeBaker.cpp:23` | 同上；:893/:908 | 已知（2 baker 之一） |
| `VitApp/Source/Service/SharedMemoryTester.h:3` + `.cpp` | 裸 include；`HANDLE`/`CreateFileMappingA`/`MapViewOfFile`；**无条件编入主目标**（CMakeLists `VITAPP_SOURCE_FILES` :231-232，`Main.cpp:6/45/69` 引用） | **新增项**。触发虽由 `VIT_ENABLE_SHARED_MEMORY_TEST=1` 环境变量控制，但编译期即阻断 mac 构建 |

**其余扫描面全部干净（显式结论：除上述 3 文件外未发现新增项）**：

- 硬编码反斜杠路径/盘符：`Source` 内零命中（命中的反斜杠均为日志字符串转义引号与缓存 key 的 `replaceCharacters("\\/:.","____")` 分隔符归一——后者双平台兼容）。
- Windows 特有 CRT/调用（`Sleep`/`_snprintf`/`strcpy_s`/`CopyFile`/`OutputDebugString`/`__declspec`/`#pragma comment`/`__try` 等）：零命中。
- 其他 Windows 头（`io.h`/`process.h`/`shlobj.h`/winsock 等）：零命中。
- `VitApp/Source/Core/VitPaths.h`（路径中枢）：全走 `juce::File`，跨平台干净。
- `inferPluginFormatNameFromPath`（CommandDispatcher.cpp:1687 与 PluginRackControlService.cpp:1229 两处重复定义）：已 mac 感知——`.vst3` 后缀对 mac bundle 目录同样命中，`audiounit:` 前缀已支持；`.vst`/`.dll` 分支是 VST2 遗留死分支（未开 `JUCE_PLUGINHOST_VST`），不构成 mac 阻断。
- 平台宏：自研 C++ 内零命中；CMake 侧仅 `VitApp/CMakeLists.txt:107` `if (WIN32)` 包 DPI manifest（**确认已守卫**，与 roadmap 一致）。
- `VitApp/Tests/CMakeLists.txt`：含 `D:/Vit_DAW/tracktion_engine` 绝对路径兜底，但有 `EXISTS` 守卫且相对路径优先——无害。
- `tracktion_engine`：**裸 gitlink**（mode 160000，pinned `0981e7ce`，无 `.gitmodules`），remote 指向公开上游 `https://github.com/Tracktion/tracktion_engine.git`，工作树干净（无本地补丁）。mac 侧 `git clone` 主仓后**不会自动获取**（无 submodule URL 映射），需手工 clone 上游并 checkout 到 pinned commit（§5 R-物流）。
- Main.cpp 使用 `te::PluginManager::startChildProcessPluginScan`（JUCE 插件扫描子进程）——JUCE 跨平台机制，mac 行为需真机确认（§7）。

### 1.2 构建面

**CMake 主文件（`VitApp/CMakeLists.txt`）**：非 MSVC 分支现状良好——

- :286-290 MSVC `/W4 /permissive-` 与非 MSVC `-Wall -Wextra -Wpedantic` 分支在位；
- :292-294 Linux `atomic` 链接分支在位；
- libzmq/cppzmq 走 FetchContent（GitHub tarball+SHA256），跨平台无 Windows 假设；目标选择优先 `libzmq`（动态）后 `libzmq-static`——**mac 建议显式选 static**（省 .dylib @rpath 处理，Windows 侧现状是拷 libzmq.dll）；
- libsodium 从 `third_party/libsodium-1.0.20.tar.gz` 离线解包（tarball 在位，哈希校验）。

**libsodium bundle（`VitApp/cmake/libsodium_bundle/CMakeLists.txt`）——roadmap 未展开的具体风险点**：

- :9-10 `file(GLOB_RECURSE)` 全树收 `.c`——**非标准姿势**（上游自身构建会按平台过滤）；全树会同时编入 `randombytes/internal`、`randombytes/sysrandom` 等多平台实现，靠源内 `#ifdef` 区分。Windows 侧已实证可构建；**mac 未经实证**（§5 R1）。
- :41-43 Clang+MSVC 前端加 `-mavx2 -maes` 等 SIMD flag——Apple Clang 前端变体不是 MSVC，不会误伤 arm64；逻辑安全。
- `VitApp/Tests/CMakeLists.txt` 的 RenderWatchdog 测试镜像了同一套 libsodium 抽取逻辑（同样的 GLOB 姿势），风险同源。

**Windows 假设脚本清单（mac 需替代或新建，均为发布面不阻断编译）**：

| 脚本 | Windows 假设 |
|---|---|
| `build_release.ps1`（根） | VitApp.exe 路径、`vit_extension.windows.*.dll` 拷贝、libzmq.dll 拷贝、Godot "Windows Desktop" 导出预设、python_embed\python.exe、D:\Godot 递归搜 exe、产出 .exe/.bat 启动器 |
| `scripts/assemble_windows_release.ps1` | 名称即 Windows 专用（Inno/zip 便携发布组装） |
| `scripts/build_installer_v0.*.ps1`（10 个） | Windows 安装器族 |
| `scripts/bundle_python_embed.ps1` | Windows embeddable python 打包 |
| `scripts/build_agent.ps1` | PowerShell 主体（go test+build 逻辑本身跨平台，仅需 mac 等价入口） |
| `scripts/download_vcredist.ps1` | VC++ 运行库（mac 无对应物，直接剔除） |

**结论**：roadmap 所记「libsodium/libzmq bundle 脚本按 Windows 场景写」成立但范围偏窄——实际 Windows 专用面覆盖整个发布/安装器链；mac 侧需要一个新的 assemble 脚本（建议 zsh 或直接用 CMake install + 少量胶水），不移植 PowerShell 体系。

### 1.3 Godot 前端

前端工程 `D:\Godot\project\vit-daw-frontend`（仓库外）：Godot **4.6** + Forward Plus 渲染器，`export_presets.cfg` 目前**只有 "Windows Desktop"** 一个预设（名为 Vit DAW v0.93）。

**(a) WebUI 面板实际走 CEF，WebView2 是休眠路径（新增发现）**：

- `app/browser/cef_browser_view.gd`、`cef_probe.gd`、`browser_panel.gd` 使用 CEF（`CefTexture`）；
- `extension/src/vit_webview_host.cpp` 的 WebView2 路径在 `.gd`/`.tscn` 中**零引用**——休眠代码，mac 不依赖它（其守卫 `#if defined(_WIN32) && defined(VIT_WITH_WEBVIEW2)` 本身规范）。

**(b) godot_cef mac 可用性：从 roadmap 的「待查」升级为「确认可用」**：

- 本地 addon 是 [dsh0416/godot-cef](https://github.com/dsh0416/godot-cef)（Rust 版，`gdext_rust_init`，compatibility_minimum 4.5）；
- 上游 README 平台表：macOS **Metal ✅**（GPU 加速 OSR）、Vulkan ❌（mac 无此路径）、软件渲染 ✅ 兜底；
- 最新 release **v1.15.4（2026-09-09）**单资产 `godot_cef-v1.15.4.zip`（约 1 GB，全平台包），其中含 `bin/universal-apple-darwin/Godot CEF.framework`（本地 `.gdextension` 清单已声明该 macos 条目与依赖框架）；该版 changelog 含「macOS export bundle relocatable」修复——**mac 维护活跃**；
- 本地 `addons/godot_cef/bin/` 只装了 `x86_64-pc-windows-msvc`——mac 侧需从上游 zip 取 universal framework 放入；
- 双平台同限：预编译 CEF 无 H.264/AAC/MP3（上游许可限制），WebUI 若无相关媒体内容则无影响。

**(c) vit_extension mac 目标：可行**：

- 标准 GDExtension SCons 布局：`extension/SConstruct` + `src/`（3 组源文件：register_types、vit_waveform_reader、vit_webview_host）+ `godot-cpp/` 源码树在位；
- SConstruct 对 webview2 的处理已守卫（`os.path.exists(WebView2.h)` + `VIT_WITH_WEBVIEW2` 宏 + `env["platform"]=="windows"` 才链 ole32/user32/gdi32/dcomp）；
- mac 编译 = `scons platform=macos arch=arm64`（godot-cpp 标准流程）+ 在 `vit_extension.gdextension` 补 `macos.*` 库条目（当前清单只有 windows 两条）。

**(d) 新增项：前端进程链的 Windows 硬编码（roadmap 完全未记）**：

`app/startup/start_page.gd`（用户手测入口）：

- :144-182 内核候选路径硬编码 `VitApp.exe`（runtime/ 与 dev 构建目录两套候选）；
- :191-194 agent 候选硬编码 `vitagent.exe`；状态提示文案 :116 提及 `VspHub.exe`；
- :355-380 **python bridge 在运行链上**：拉起 `python_embed/python.exe` 执行 `runtime/bridge_prod.py`（或 bridge_core.py），设 PYTHONPATH/VIT_BRIDGE_CONFIG 环境变量并注册子进程看护——Go agent 侧代码对 bridge_core.py 零引用，该链仅由前端维系；
- `OS.create_process` 调用本身跨平台，但二进制名/路径/python 运行时全按 Windows 写。

**(e) 备选路径（浏览器打开 WebUI）差距评估**：WebUI 由 Go agent 在 localhost HTTP 提供，Godot 侧 `OS.shell_open(url)` 即可拉起系统浏览器（跨平台 API，成本约半天+UI 提示位）；差距 = 失去 DAW 内嵌（窗口编排、CEF IPC 输入路由）。当前 `browser_panel.gd` **未实现**该兜底。CEF mac 已确认可用，此路径降级为应急备选。

### 1.4 插件生态

**指纹机制全量落定（代码锚点）**：

- `ParameterSurface` = 参数面描述符 JSON 的 sha256（`agent/internal/pluginprobe/adapter.go:382-397`；字段：ID/Type/Min/Max/EnumValues/DisplayDomain/Unit/Scale）——**「名称哈希跨平台稳定」的 roadmap 预期在机制上成立**：同版本插件在 mac VST3 暴露相同参数面则指纹一致；仍须逐个重探针实测（枚举顺序等实现差异可能存在）。
- `Installation` = 插件**文件内容**哈希（`agent/internal/processorattestation/fingerprint.go:14` `FingerprintPath`；VST3 bundle 目录按「排序的相对路径+文件字节」框架化哈希）——mac 二进制必然不同 → **重认证是设计内动作**，与 roadmap「指纹不同是预期」一致。准入查询按 binary fingerprint 匹配（`v2_store.go:217 QueryLibraryAdmissionV2`）。
- 白名单位置：`~/.vit/processor_control_attestations.v2.json`（`DefaultPathV2`，`v2_store.go:25-33`）——**机器本地确认**；辅以 `~/.vit/plugin_semantics.json`（5 条）与 `free_state_experiment_plugins.json`（8 条）。

**校准规模（本机实测，工作量的量化基础）**：

- **31 个插件主体 × 5 个语义族**（每主体一族）：Waves 23（C4/C6/L1/L2/LinMB/DeEsser/RDeEsser/Sibilance/C1/PSE/Smack Attack/TransX 的 Mono/Stereo 变体）、Plugin Alliance 5（HUM LAAL、Lindell 354E、Lindell MBC、SPL Transient Designer Plus、bx_limiter True Peak）、FabFilter 3（Pro-DS、Pro-G、Pro-L 2）；全部 promoted 状态。
- Waves 系走 **WaveShell** shell 架构（installed_path 指向 `WaveShell1-VST3 17.1_x64.vst3` 内部）；tracktion_engine pinned commit 本身就是「fix(vst3): update JUCE for stable WaveShell loading」——**WaveShell 加载是已知敏感区，mac 侧需重点重验**。

**新增项（roadmap 未记）**：

- `FingerprintPath` **拒绝 bundle 内 symlink**（fingerprint.go:59-61，fail-closed）——mac 插件包若含符号链接会直接报错而非误认证；概率低（PA/Waves/FabFilter mac VST3 通常无 symlink）但需真机确认。
- `defaultPluginScanPaths()`（`agent/internal/harness/harness.go:8585-8592`，**无 build tag 的共享代码**）：依赖 `CommonProgramFiles`/`ProgramFiles` 环境变量（Windows 概念），兜底硬编码 `C:\Program Files\Common Files\VST3`——darwin 编译通过（护栏盲区）但 mac 运行时扫不到任何插件。需平台化（mac 默认 `~/Library/Audio/Plug-Ins/VST3` 与 `/Library/Audio/Plug-Ins/VST3`）。

### 1.5 Go agent 护栏

- **darwin 交叉编译今日实测**：`cd agent && GOOS=darwin GOARCH=arm64 go build ./...` → **exit 0（2026-09-15，HEAD a7bbc3b）**——roadmap「全绿」未腐化。
- **平台代码 build-tag 合规抽查（全量 5 文件，非抽查）**：`harness/shm_windows.go`+`shm_other.go`、`vsphub/audio_feature_shm_windows.go`+`_unsupported.go`、`vst3host/worker_windows.go`+`_unsupported.go`、`orchestration/filelock_windows.go`+`filelock_unix.go`——`//go:build` 标签与 fallback 兄弟文件模式全部合规；fallback 均为显式错误降级（无隐性空实现）。
- **护栏边界（重要）**：交叉编译只抓编译面。`defaultPluginScanPaths` 硬编码 Windows 兜底（§1.4）在护栏全绿下存在，证明**运行时行为缺口是护栏盲区**——移植期应在 mac 上跑只读冒测（`g_runtime_readonly_smoke.ps1` 白名单 GET 模式的 mac 等价物）作第二道护栏。
- vet 注意（roadmap 已记，复核仍有效）：harness 包对 `shm_windows.go` 的 uintptr 模式有既有警告，用 `go vet -unsafepointer=false ./internal/harness` 或忽略。

### 1.6 用户侧配合项清单（供用户确认）

| # | 事项 | 说明 | 状态 |
|---|---|---|---|
| U1 | PA 许可证 mac 覆盖 | roadmap 记「用户已有 mac AAX = 许可证覆盖 mac，PA 全格式安装包含 VST3」。需确认 mac 上实际安装 PA VST3 后 5 个主体可见 | 待确认 |
| U2 | **Waves 与 FabFilter 的 mac 安装/许可**（新增，量最大） | 31 主体中 Waves 占 23：Waves 许可证通常双平台同版本覆盖，但需 mac 上安装 WaveShell VST3 17.x；FabFilter 3 个同理 | 待确认 |
| U3 | MacBook Air runner 就位 | 显示器支架位、随工作站同步启动、夜间空闲跑内核构建（2026-08-27 已定策） | 未就位（本报告动态项因此收口为待办） |
| U4 | GitHub 私有仓决策 | 当前纯本地；推私有仓 = CI 前提 + 灾备 | 待决策 |
| U5 | Air 基础环境 | Xcode Command Line Tools（内核 clang 编译）、Godot 4.6 官方 mac 版、Go、SCons/python（vit_extension 用） | 随 U3 一并 |

---

## 2. 与 roadmap 2026-08-27 快照的 diff

| roadmap 快照条目 | 本次审计结论 | 判定 |
|---|---|---|
| Go agent：darwin 全绿；平台代码 5 处 build-tag+fallback | 今日复测 exit 0；5 处全量核验合规；**补充**：护栏存在运行时盲区（defaultPluginScanPaths 类缺口） | ✅ 未腐化 + 边界澄清 |
| 内核：已知 Win32 = 2 个 baker 文件共享内存 + DPI manifest（已守卫） | baker 2 文件属实；DPI 守卫属实；**但实为 3 文件**——SharedMemoryTester.{h,cpp} 无条件编入主目标，同为编译期阻断 | 🔶 腐化：计数过时，移植接口化时须一并处理 |
| 内核：需抽 ISharedMemorySegment + POSIX mmap | 方向成立；补充：Go 侧配套需新增 shm_darwin.go（现 shm_other.go 显式报错降级），两端一起动 | ✅ 成立 + 补配套项 |
| libsodium/libzmq bundle 脚本按 Windows 场景写 | 属实但范围窄：libzmq 走 FetchContent 无假设；libsodium bundle 的真风险是 GLOB 全树姿势 mac 未实证；Windows 专用面实为整条发布/安装器/python_embed 链 | 🔶 细化 |
| vit_extension 有源码可加 mac 目标（SCons） | 确认可行（godot-cpp 在位、守卫规范、清单补条目即可）；补充：WebView2 路径休眠，mac 不依赖 | ✅ 成立 |
| godot_cef 只有预编译二进制，mac 可用性待查 | **已核实：上游 v1.15.4 支持 mac**（Metal OSR + 软渲染兜底，2026-09-09 刚修 mac 导出包重定位）；本地 bin 缺 mac 产物需从上游 zip 取 | ✅ 风险降级（待查→确认可用） |
| 插件生态：param ID 预期跨平台稳定但须逐个重探针；白名单机器本地；PCA mac 重认证是指针不同预期 | 全部证实并量化：ParameterSurface=参数元数据哈希（跨平台稳定口径成立）；白名单=`~/.vit/` 机器本地；规模 31 主体×5 族；**新增**：symlink 拒绝策略、WaveShell mac 重验敏感（pinned commit 语义）、Go 扫描路径 Windows 兜底 | ✅ 成立 + 量化 + 2 新增风险 |
| （roadmap 未记）前端进程链 | start_page.gd 硬编码 VitApp.exe/VspHub.exe/vitagent.exe/python.exe + python bridge 由前端拉起在运行链上 | 🔴 新增面 |
| （roadmap 未记）导出预设 | export_presets.cfg 仅 Windows Desktop | 🔴 新增面（小） |
| 护栏 2：新增平台分支走 build-tag+fallback | 既有 5 处合规；**SharedMemoryTester 是首个反例**（内核侧无此纪律的 C++ 等价物——内核护栏是「禁止未守卫 windows.h」文字规则，无机器检查） | 🔶 建议内核侧补机器检查（§4 CI 层） |

---

## 3. 分层工时表

口径：mac runner 就位后，以熟练执行会话（GLM L2 或受卡约束的下 Flash）计；粒度=可开卡的任务×预估人日。**静态审计估算，真机后须修正**（§7 待办会回填）。

### 层 A：内核（VitApp + Go 侧配套）

| 任务 | 预估 | 备注 |
|---|---|---|
| A1 ISharedMemorySegment 接口抽象 + 3 使用点改写（2 baker + tester）+ POSIX mmap 实现 | 2-3 天 | roadmap 既定方向；tester 顺带处理（或降级为 test-only 编译） |
| A2 Go 侧 `shm_darwin.go`（shm_open+mmap 读段，对齐 shm_windows 语义） | 1 天 | harness + vsphub 两处读点 |
| A3 mac CMake 试编译整备（clang 警告清理、libsodium GLOB 实证/必要时改显式源列表、Tests 镜像同步） | 1-2 天 | R1 消解点 |
| A4 tracktion_engine mac 构建链（上游 pinned clone 核对 + Xcode CLT 编译） | 0.5-1 天 | CMake 非 MSVC 分支已在，预期主要是环境问题 |
| A5 内核 mac 冒测最小集（dev_agent_smoke 的 mac 等价：内核起停、ZMQ/HTTP 探活、插件扫描 child-process） | 1-2 天 | 真机；WaveShell 枚举并入 |
| **层小计** | **5.5-9 天** | |

### 层 B：前端（Godot）

| 任务 | 预估 | 备注 |
|---|---|---|
| B1 vit_extension mac 目标（SCons macos + .gdextension 条目 + 休眠 webview2 验证） | 0.5-1 天 | |
| B2 godot_cef mac 集成（上游 zip 取 universal framework + Metal OSR 实测 + helper 进程跑通） | 1-2 天 | 真机；上游 mac 可用性已确认，剩集成细节 |
| B3 start_page.gd 平台分支（二进制名/路径候选/python 与 bridge 链 mac 化或剔除） | 1 天 | 先做 python bridge 退役评估（见 B6） |
| B4 macOS 导出预设 + dev 期签名策略（ad-hoc/无签名跑 runner；正式分发延后） | 0.5-1 天 | |
| B5 （可选）浏览器兜底 OS.shell_open + 提示位 | 0.5 天 | 应急备选，非阻塞 |
| B6 python bridge 退役评估（若 Go agent 已全覆盖其职能，mac 直接剔除该链） | 0.5 天 | 产出=决策卡，非实现 |
| **层小计** | **4-6 天** | |

### 层 C：校准（插件生态）

| 任务 | 预估 | 备注 |
|---|---|---|
| C1 `defaultPluginScanPaths` 平台化 + mac 插件目录默认值 | 0.5 天 | 阻塞 C3，先行 |
| C2 mac 校准脚本（一键探针→白名单草稿→PCA 重认证→冒测，两端可重复；roadmap 卡链 3） | 2-3 天 | 把移植永久降级为重校准的关键件 |
| C3 31 主体重探针 + 重认证执行（脚本就位后） | 1-2 天 | 真机；Waves WaveShell 重点盯；symlink 策略确认 |
| C4 只读冒测 mac 等价物（g_runtime_readonly_smoke 白名单 GET 模式移植） | 1 天 | 运行时缺口第二道护栏 |
| **层小计** | **4.5-6.5 天** | |

### 层 D：CI/基建

| 任务 | 预估 | 备注 |
|---|---|---|
| D1 GitHub 私有仓推送（含大文件策略：gitlink tracktion_engine 的处理决策——补 .gitmodules 指上游或文档化手工步骤） | 0.5 天 | 依赖 U4 决策 |
| D2 Air 自托管 runner 安装（launchd 常驻、随工作站启动） | 0.5-1 天 | 用户配合 |
| D3 workflows：Go 矩阵 push 触发（linux/windows/darwin 交叉编译护栏进 CI）+ 内核每夜 mac 构建 | 1 天 | |
| D4 （建议新增）内核侧「未守卫 windows.h / 硬编码反斜杠」机器检查（grep 断言脚本进 CI） | 0.5 天 | 把 roadmap 护栏 3 从文字规则变机器规则 |
| D5 每夜构建产物归档 + 失败通知 | 0.5 天 | |
| **层小计** | **3-3.5 天** | |

**总计：约 16.5-25 人日**（含真机动态项；不含用户侧 U1-U5 等待时间）。

---

## 4. 风险清单

| # | 现象 | 影响 | 缓解 |
|---|---|---|---|
| R1 | libsodium bundle GLOB 全树编译 mac 未实证（多平台 randombytes 实现同编） | 层 A 首个卡点，可能重复符号或编入错误实现 | A3 首日即试；必要时改为按平台显式源列表或直接用上游 CMake 构建 |
| R2 | WaveShell mac 加载：tracktion pinned commit 是 Windows 语义修复，mac shell 枚举/加载未经证 | 31 主体中 23 个 Waves 不可用则校准面塌掉大半 | A5 真机首测项；失败则升级 tracktion/JUCE 版本或补丁（独立卡） |
| R3 | CEF mac 集成细节：Metal OSR 与 Godot Forward+（MoltenVK）纹理合成的组合行为、CEF helper 进程权限 | WebUI 面板渲染/输入异常 | 上游已支持该组合（README 平台表），先软渲染兜底实测再开加速；Forward+ 异常时 Compatibility 渲染器备选 |
| R4 | `FingerprintPath` 拒绝 bundle 内 symlink（fail-closed） | mac 插件包若含 symlink，探针直接报错 | C3 首批即遇则按失败类型记录；必要时为 mac bundle 定义 symlink 解引用策略（决策卡） |
| R5 | darwin 交叉编译护栏只抓编译面，运行时缺口（defaultPluginScanPaths 类）静默存在 | mac 上功能哑火难定位 | C4 只读冒测进 CI；移植期新平台分支一律双文件（纪律不变） |
| R6 | tracktion_engine 是裸 gitlink（无 .gitmodules），mac clone 不会自动带上 | 环境搭建踩坑/版本漂移 | D1 决策：补 .gitmodules 指上游（pinned）或 README 固化手工步骤 |
| R7 | 双平台永久双份机器本地状态（白名单/PCA/plugin_semantics） | 每次插件升级双端重校准 | C2 校准脚本两端可重复（roadmap 既定），把成本压到跑脚本 |
| R8 | 发布面全 Windows 专用（ps1 体系 + python_embed + 安装器） | mac 无发布产物路径 | 明确 mac 发布面=新 assemble 脚本 + .app/zip，不移植 ps1；python bridge 先做退役评估（B6） |
| R9 | JUCE 插件扫描 child-process 与 mac 沙箱/权限交互（历史上有 Gatekeeper 相关问题） | 插件发现失败 | A5 真机验证；dev 期可绕签名，分发期再评估公证 |

---

## 5. 建议的分层移植顺序修正

roadmap 卡链 4「内核→前端→校准→双端冒测」主骨架成立。修正建议：

1. **C1（扫描路径平台化）提前并入层 A 首批**——它阻塞校准层且是半小时级改动，却决定 mac 上一切探针能否看见插件。
2. **层 B 内部重排：B1+B3（不依赖 CEF）先行，B2（CEF 集成）与层 A 并行真机验证**——CEF mac 可用性已从「待查」变「确认」，从关键路径降为并行项。
3. **C2（校准脚本）在内核 mac 构建通过后立即启动**，与层 B 剩余项并行——校准是总工时里真机占比最高的块，尽早开始攒样本。
4. **B6（python bridge 退役评估）提到层 B 开头**——若可剔除，直接省掉 python_embed mac 化与一条运行时依赖链。
5. **D4（内核 Windows 假设机器检查）提前到 CI 首版**——SharedMemoryTester 证明文字护栏已漏过一次，先把门关上再继续移植。

---

## 6. 产出物与验收对照

- 本报告：`docs/PORT_AUDIT_2026-09.md`（唯一产出文件；扫描全部以只读命令完成，未写扫描脚本——无复用必要，卡面允许）。
- 六项勘察各有着落：§1.1-§1.6；「未发现新增项」的显式结论见 §1.1（硬编码路径/CRT 调用/平台宏均零新增）。
- 零生产代码改动：git status 终态核对见回执（新增本报告 1 个未跟踪文件，白名单外零触碰）。

## 7. 待办：需真机的动态项（runner 就位后回填）

1. libsodium GLOB mac 实编译（R1 消解，A3）；
2. WaveShell mac 枚举/加载 + 31 主体重探针（R2/R4，A5+C3）;
3. CEF Metal OSR 在 Air 实测（含 helper 进程、Forward+ 组合）（R3，B2）；
4. JUCE 插件扫描 child-process mac 行为（R9，A5）；
5. 只读冒测 mac 全量跑（R5 消解，C4）。

# SHELL-ENTRY-REFRESH-1 正式刷新 FINAL（PC 主管 lead 放行，410/411/412 已验收合入）

- 执行：2026-11-18 18:35–18:45 (+08:00)，执行子代理 `lead/entry413`（DeepSeek-V4.1-Flash，PC）
- 放行依据：主管 lead 2026-11-18 晚窗消息 —— 前端 main=`cf33195`（410 producer + 411 readiness），主仓 main=`dfa86cac`（412 395 呈现 merge + 协调提交），均已推 origin/main
- 临时工作目录 T = `C:/Users/timoz/AppData/Local/Temp/shell413-ext-449f0a7d`（预阶段隔离构建目录，复用其预构建 DLL）
- **结论**：两仓同步到位；VitShell Release（410 后源码）重建成功；409 守卫新扩展 DLL（预构建可复用产物）已替换前端主检出 tracked 构建产物（未提交）；VitAgent/webui 重建成功；真实工程 headless 导入两条命令 exit 0 无崩溃；部署 DLL 无窗探针三断言全 true exit 0。全程零真栈、零 GUI、零源码改动、零提交/推送。

## 1. 两仓同步与基线

| 项 | 值 |
|---|---|
| frontend HEAD（前/后） | `cf33195bb3c3a86d3c77c8338efba014d8097f6b`（开工即等于 origin/main，lead 已同步） |
| frontend origin/main | `cf33195bb3c3a86d3c77c8338efba014d8097f6b` |
| 主仓 HEAD（前/后） | `dfa86cac9457b0473ea53597bd02a8c32398da2a` = origin/main |
| 前端 tracked 构建前状态 | clean（`git status --porcelain -uno` 空）；`git status --short` 12 行既有未跟踪工程目录 |
| `git diff --quiet bc8daf6..HEAD -- extension/` | **exit 0（extension/ 源码自 409 基线未变，预构建 DLL 可复用得到证实）** |
| bc8daf6..HEAD（410/411）摘要 | 24 files changed, +1797/−14：VitShell/* 11 文件（CMakeLists、godot_manager.cpp/.h、launcher_bridge.cpp/.h、main.cpp、status_server.cpp/.h、tests/CMakeLists + 2 个新测试）+ app/shell/vitshell_launch_readiness.gd + app/startup/* + tools/* 探针 |

命令与退出码：
```powershell
git -C D:/Godot/project/vit-daw-frontend fetch origin main      # exit 0
git -C D:/Godot/project/vit-daw-frontend pull --ff-only         # exit 1（main 分支未配置 upstream，非内容冲突）
git -C D:/Godot/project/vit-daw-frontend pull --ff-only origin main  # exit 0 -> Already up to date.
git -C D:/Vit_DAW fetch origin main                              # exit 0
git -C D:/Vit_DAW pull --ff-only                                 # exit 0 -> Already up to date.
```
说明：前端 main 未设 upstream，普通 `git pull --ff-only` 报 "no tracking information"（exit 1）；随后按显式 `origin main` 快进 exit 0，HEAD 已等于 origin/main，无内容变更。主仓 pull exit 0。

## 2. 命令原文与 exit 码

### 2.1 VitShell Release（410 后源码）
```powershell
cd D:/Godot/project/vit-daw-frontend
cmake -S VitShell -B VitShell/build -G "Visual Studio 18 2026" -A x64 "-DVITSHELL_WEBVIEW2_DIR=D:/Godot/project/vit-daw-frontend/extension/third_party/webview2"   # exit 0
cmake --build VitShell/build --config Release   # exit 0，17.2 s
```
- cmake 4.3.0-rc2；既有 build/CMakeCache.txt 已是同一 generator（Visual Studio 18 2026）与同一 WEBVIEW2_DIR，configure 增量成功。
- 构建产物：VitShell.exe（新）+ 410 零栈测试 exe（shell_window_guard_test / window_status_serialization_test / launcher_presentation_test，仅构建未运行）+ WebView2Loader.dll 拷贝。
- 日志：`T/logs/final/vitshell_configure.log`、`vitshell_build.log`。

### 2.2 扩展 DLL 校验与替换（主管指令：不提交）
```powershell
$fe = "D:/Godot/project/vit-daw-frontend"
git -C $fe diff --quiet bc8daf6..HEAD -- extension/   # exit 0
Copy-Item "$T/extension/bin/vit_extension.windows.template_debug.x86_64.dll" "$fe/extension/bin/vit_extension.windows.template_debug.x86_64.dll" -Force
```
- 旧：`E23872FAFEC8F5216E20E56860DD958AD9104E9D4F85683BCCC50406602ACEF0`（495,616 B，2026-11-14 23:45:28，409 之前）
- 新：`B270362D1E3380D27A709747611FED149A8A0F54B11BA00C1229834510CD2EB7`（520,192 B，2026-11-18 18:24:45，隔离目录干净构建产物）
- 旧 DLL 已备份：`T/backup/vit_extension.windows.template_debug.x86_64.dll.before-413`（SHA256 同上旧哈希，可还原）
- dumpbin /exports（部署后的主检出 DLL）：1 个导出符号 **`vit_extension_library_init`**（RVA 0x6F70），exit 0

### 2.3 agent / webui（主检出）
```powershell
cd D:/Vit_DAW/agent
go build -o ./bin/VitAgent.exe ./cmd/vitagent      # exit 0，17.2 s
cd D:/Vit_DAW/agent/webui
npm run build                                      # exit 0，10.4 s（tsc --noEmit && vite build；node_modules 已在，未 npm ci）
```
- 日志：`T/logs/final/go_build.log`、`webui_build.log`；vite 提示部分 chunk >500 kB（既有提示，非错误）。

### 2.4 真实工程 headless 导入验证（关键）
```powershell
$godot = "D:/Godot/Godot_v4.6.1-stable_win64_console.exe"
& $godot --headless --import --check-only --path D:/Godot/project/vit-daw-frontend   # exit 0，19.8 s，156 行
& $godot --headless --import --path D:/Godot/project/vit-daw-frontend                # exit 0，18.6 s，49 行
```
两条命令**均 exit 0、无 crash（无 CrashHandlerException / signal 11）**。末 5 行（两命令相同，ANSI 已去）：
```
[  50% ] loading_editor_layout | 正在加载中央编辑器布局……
[  66% ] loading_editor_layout | 正在加载插件窗口布局……
[  83% ] loading_editor_layout | 编辑器布局就绪。
[ DONE ] loading_editor_layout
```
- 日志：`T/logs/final/realproject_import_checkonly.log`（21,508 B）、`realproject_import.log`（6,476 B）。
- 说明：该引擎 `--check-only` 的帮助标注为 "Only parse for errors and quit (use with --script)"，与 `--import` 组合被接受（exit 0），未观察到独立 check-only 语义；以第二条正式 `--import` 为准。
- 补充取证（非主管要求的额外一次）：`--headless --import --verbose` exit 0，167 行；日志显示 `正在校验 GDExtension……` 步骤完成、godot_cef 库 PDB 缺席 WARNING（既有），**无 vit_extension/GDExtension 加载错误**（对照：预阶段伪造 entry_symbol 时该引擎会打印 "Can't resolve symbol …" ERROR，本次真实工程无此错误）。日志：`realproject_import_verbose.log`。
- 既有噪声（非致命）：首次导入日志出现 `ERROR: Couldn't read OBJ file 'res://VitShell/build/.../*.obj'`（Godot 把工程内 CMake 构建目录的 .obj 当 mesh 导入）；exit 0，属构建目录位于 Godot 工程内的既有现象，本次重建新增 .obj 再次触发。

### 2.5 部署 DLL 无窗加载探针
```powershell
# 把主检出部署后的 DLL 复制进隔离探针工程（hash 校验一致）
$env:GODOT_EDITOR_CUSTOM_FEATURES = "vit_shell_hidden_boot"
& "D:/Godot/Godot_v4.6.1-stable_win64_console.exe" --headless --path "T/probe_project"   # exit 0
```
完整 stdout：
```
Godot Engine v4.6.1.stable.official.14d19694e - https://godotengine.org

EXTENSION_REGISTERED=true
METHODS_OK=true
FEATURE_ENABLED=true
[probe] godot=4.6.1-stable (official) editor=true debug=true feature_env=vit_shell_hidden_boot
[probe] instantiate=true
```
- 探针工程 DLL = 主检出部署 DLL（SHA256 `B270362D…`，520,192 B，两处哈希一致）。
- 对照（不设 feature env）：`EXTENSION_REGISTERED=true / METHODS_OK=true / FEATURE_ENABLED=false`，exit 1（探针按设计以非零退出）。
- 日志：`T/logs/final/probe_run_deployed_dll.log`。

## 3. 最终产物哈希

| 产物 | 新 SHA256 | 大小 | 时间 | 旧 SHA256（刷新前） |
|---|---|---|---|---|
| `VitShell/build/Release/VitShell.exe` | `5D420DEDB482D6CA2FADA050477C3319722B1AEE6A7CF7DFE922485A1F3FCBA0` | 311,808 B | 18:36:43 | `B1CE68138A1EEE3D0A490BD042A47C2A6179E3496D8A2AD89CAD3B86A80B1E7D`（261,120 B） |
| `VitShell/build/Release/WebView2Loader.dll` | `BB5FDA5F2D3AAE5E0977AB28D8A8A00ACFD63D73748AAAC32DAF4FC6645B6677` | 159,112 B | 11-15 04:09 | 同（未变） |
| `extension/bin/vit_extension.windows.template_debug.x86_64.dll` | `B270362D1E3380D27A709747611FED149A8A0F54B11BA00C1229834510CD2EB7` | 520,192 B | 18:24:45 | `E23872FAFEC8F5216E20E56860DD958AD9104E9D4F85683BCCC50406602ACEF0`（495,616 B） |
| `agent/bin/VitAgent.exe` | `27D09FA09D0CD6F710F9BFAD7E06F44242159987178241953431B13C435C3768` | 39,502,848 B | 18:37:25 | `9D1835C27ED04046C85048F9BB13A12982FFE844ACC59A22C5355BF381FCC122`（39,500,288 B） |
| `agent/webui/dist/index.html` | `0886058EF839F421FBA09FDEAE381583FE5E8911D561DD9C4E5866161EFCFB9F` | 630 B | 18:37:35 | `01CE9F173A9BAD41C9DC38727062606AC683F1097BA9922AC5F115568C7D2375`（630 B） |
| 入口 `VitShell/build/Release/启动VitShell.cmd` | `293E9B48F6974E5A8CA436B35AADDFDBC1C245FDE18BB548ED80119822BE7CD9` | 336 B | 11-15 18:09 | 同（未变） |

webui dist 其余文件（新构建，供需要时核对）：
```
favicon.svg        362 B  3D290486DD8B352ED4E6492BC666CC9D45B73F487A2A5C8105227497DFF771BC
gallery.html       605 B  8D130572B4B52C3BC9B5D4CE20EA352ABEFBCD44911BF5F788C6398B9040D309
assets/gallery-BPLLf_0o.js   34175 B  43AD8EAF035893DC55DE4D31D2D918877140821E2661A41828BA67EBC5ACFBFE
assets/gallery-CY77NL5L.css   5544 B  BAF08CEA001973385359CC87F1326258E82DD72002FBC8D1A93264FA0587789C
assets/main-BzQgVzPm.js       5867 B  0DF6EE37ABEAC0D020AA12AB72A706008870588BE90F745449E54079CE02ED4C
assets/main-DELB1KAd.css     77785 B  88C2F8FF1DED6817BDC6DEB5BFF6E09A93EA3CA310304CE1482F7141461D8B35
assets/sprite-aPYNIkYB.css   65643 B  515B7961A020825ED946C03B23999EFBCB87997E6CE20D4DF70F1176E3482280
assets/sprite-B8avabzQ.js  1754257 B  298B878C05710A7A6C20BBA2C72750483663CB00C02045C6F7655E9C649469FE
```

## 4. 实际手测入口 `启动VitShell.cmd` 核对

内容（未改动，336 B，hash 前后一致）：
```
@echo off
rem SHELL-3 handtest v6 wrapper: VitShell.exe + dev assembly args (arg-free launch is a recorded boundary)
start "" "D:\Godot\project\vit-daw-frontend\VitShell\build\Release\VitShell.exe" --dev-root D:\Vit_DAW --godot-exe D:\Godot\Godot_v4.6.1-stable_win64_console.exe --godot-project-dir D:\Godot\project\vit-daw-frontend
```
路径参数逐项存在性核对：VitShell.exe=True、dev-root D:/Vit_DAW=True、godot console exe=True、project dir=True。**因此不改**（与卡面边界一致：路径仍有效则不改）。

## 5. git status 摘要与未提交产物

### 前端主检出（HEAD cf33195）
- tracked 工作树改动（**唯一**）：` M extension/bin/vit_extension.windows.template_debug.x86_64.dll`（Bin 495616 → 520192，主管指令就地替换，**未提交**）
- `git status --short` 共 13 行 = 1 个 modified + 12 个既有 `??` 未跟踪工程目录（.vit_agent/、.vit_derived/、若干 .vit_project 等，均非本代理产生）
- 被 ignore 的构建/缓存产物：`VitShell/build/`（VitShell/.gitignore:build/）、`.godot/`（根 .gitignore）、`*.exe/*.tmp/*.obj`（根 .gitignore）、`extension/src/*.obj`
- `extension/bin/` 另有旧 `~vit_extension…~RF*.TMP` 隐藏残留（历史产物，未触碰）

### 主仓（HEAD dfa86cac）
- 本代理产出的构建产物均被 ignore：`agent/bin/VitAgent.exe`（.gitignore:bin/）、`agent/webui/dist/`（agent/webui/.gitignore:dist/）
- 本文件 `coord/runs/SHELL-ENTRY-REFRESH-1/FINAL.md` 为新增未跟踪文件（按要求落盘，未提交）
- **并发写入观察（非本代理所为，未触碰）**：会话期间主仓 tracked 改动由 4 个变为 5 个 —— 新增 ` M scripts/vitshell_zproduct_smoke.ps1` 与 `?? scripts/vitshell_combo_cdp.mjs`（另有既有 `M CURRENT-STATE.md`、`M VitApp/Workspace/Settings/Settings.xml`、`M VitApp/Workspace/default_project.xml`、`M tracktion_engine`）。本代理只执行了 go build / npm build / FINAL.md 写入，未改动上述任何文件。

## 6. 工件与日志

- run 目录：`D:/Vit_DAW/coord/runs/SHELL-ENTRY-REFRESH-1/`（PLAN.md、PRESTAGE.md、FINAL.md）
- 正式刷新日志：`T/logs/final/`
  - vitshell_configure.log、vitshell_build.log
  - go_build.log、webui_build.log
  - realproject_import_checkonly.log、realproject_import.log、realproject_import_verbose.log
  - probe_run_deployed_dll.log
- 预阶段日志与回执：`T/logs/`（scons_build.log、dumpbin_exports.txt、probe_run_with_feature.log、RECEIPT-SHELL-ENTRY-REFRESH-1-pre.md 等）
- 旧 DLL 备份：`T/backup/vit_extension.windows.template_debug.x86_64.dll.before-413`
- T = `C:/Users/timoz/AppData/Local/Temp/shell413-ext-449f0a7d`

## 7. 边界声明

- **零真栈**：未启动 VitShell / Godot GUI / agent / 内核栈；仅 headless 导入与 headless 探针，进程均正常退出；收尾核对进程表无 godot/VitShell/VitAgent/VitApp/VspHub 残留。
- **零 GUI**：所有 Godot 调用均带 `--headless`；未做任何 GUI 操作。
- **零源码改动**：两仓 tracked 源文件（.cpp/.h/.gd/.ts/.tsx/CMakeLists/go 等）未改；唯一 tracked 工作树改动是主管指令要求替换的扩展构建产物 DLL（未提交）。
- **零 git 写**：未 add/commit/mv/push/rebase/checkout 丢弃改动；仅 fetch/pull --ff-only。
- `.godot/` 缓存与 `VitShell/build/`、`agent/bin`、`agent/webui/dist` 为 headless 导入/构建产生的 ignored 产物，未提交。

## 8. 待主管决定 / 残留

1. **tracked DLL 替换是否提交**：前端工作树当前 dirty 1 file（extension/bin/vit_extension…template_debug.dll，495,616→520,192 B）；本卡执行侧不提交，等裁定。
2. **template_release DLL 仍为 2026-11-14 旧产物**（未含 409 守卫）：本次卡只要求 template_debug（editor/项目运行路径）；导出 release 路径若要用新守卫需另卡刷新。
3. **预阶段观察的 headless 编辑器导入崩溃未在真实工程复现**：真实工程有既有 `.godot` 缓存，两条 `--import` 均 exit 0；隔离最小工程 + 任意一版 DLL（含 409 之前旧 DLL）可复现 → 非 409 回归、非产品 `--path` 启动路径，是否另开取证卡由主管定。
4. **真栈/GUI 行为未验证**：本卡范围内为零栈产物刷新；启动 VitShell 的实际呈现与 409/410/411 手测旅程另计（用户手测入口仍是 `启动VitShell.cmd`，其指向的 VitShell.exe/DLL 现已为新产物）。

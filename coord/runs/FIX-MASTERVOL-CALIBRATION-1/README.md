# FIX-MASTERVOL-CALIBRATION-1（池序 134，P1）实现报告：工程模板 master volume 默认值修正（+6 dB → -3 dB）

- 卡片：`coord/cards/doing/2026-10-18-FIX-MASTERVOL-CALIBRATION-1.md`（池序 134，P1，L2）
- 发卡/验收：12:00-18:00 决策流（Lead）；执行 owner：DSH PC 执行流 **exec-mastervol**
- 依据：
  - `coord/rulings/2026-10-18-BATCH3-rulings.md` §一（132 KERNEL-RENDERGAIN 取证 pass + P1 升级 + 修复卡立项）
  - `coord/runs/KERNEL-RENDERGAIN-1/RESULTS.md`、`ARCHAEOLOGY.md`（+6.000 dB = 工程 master volume 节点 fader 位置 1.0，证据 run_20261018_135710）
- 工作区：主仓 worktree `D:/Vit_DAW_wt_mastervol1`，分支 `port/fix-mastervol-calibration-1`
  - 基线 `origin/main = a99117a0`（已含 132 三条证据提交：`836d4400`/`1fa8c85f`/`a99117a0`；派发提交 `536d509c`、卡面领取基线 `aec47ad8` 均为其祖先）
  - 本卡执行分支不 push main；主工作树 `D:/Vit_DAW` 全程只读（其 `VitApp/Workspace/{default_project.xml,Settings/Settings.xml}` 的未提交改动为 132 已定位的既存运行时状态，非本卡写入）
- 生成时间：2026-10-18（午窗）

## 0. 结论速览

1. **模板/新建工程 master volume 默认值 = -3.0 dB**，以引擎 fader 位置 `0.6376281516` 写入（`VitPaths.h` 默认模板 + blank 模板、仓库 `VitApp/Workspace/default_project.xml` 三处 `volume="1.0"` → `volume="0.6376281516"`）。
2. **存量工程不静默迁移**：既有工程 XML 的 `volume="1.0"`（= 引擎 fader 顶 +6.0 dB）保持原值，只登记差异；迁移候选另评。
3. `scripts/gen_vit_project.py` 数值保持不变（`0.6376281380653381` 即 `exp((-3-6)/20)` 的 float32 精确表示，同为 -3.0 dB），只补注释说明语义与精度口径。
4. 新增回归测试 `VitMasterVolumeCalibrationTests`（VitApp/Tests 现有模式）：模板/启动工程 fader 位置 = -3 dB、旧值 1.0 仍 +6 dB（不迁移证据）、16/24-bit 削波边界改善、生成脚本仍 -3 dB、存量用户工程未被改动。
5. `master` 读回/控制面不可见（`list_tracks` 的 master `volume_db=null`、无 master 命令）仍是**已知边界**，不在本卡范围（裁定③）。

## 1. 语义说明：fader 位置 vs dB（本卡关键概念）

pin 引擎 `tracktion_engine/modules/tracktion_engine/utilities/tracktion_AudioUtilities.cpp:136-156`
（`volumeFaderPositionToDB` / `volumeFaderPositionToGain`，`volScaleFactor = 20`）：

```
volumeFaderPositionToDB(pos)   = 20 * ln(pos) + 6
volumeFaderPositionToGain(pos) = 10^(volumeFaderPositionToDB(pos)/20)
decibelsToVolumeFaderPosition(db) = exp((db - 6) / 20)
```

| fader 位置（工程 XML `volume` 属性） | dB | 线性增益 |
|---|---|---|
| `1.0` | **+6.0 dB**（fader 顶端） | 1.99526 |
| `0.7408182206817179` | 0.0 dB（unity） | 1.0 |
| `0.6376281516217733` | -3.0 dB | 0.70795 |

- 工程 XML 里 `<MASTERVOLUME><PLUGIN type="volume" ... volume="..."/>` 的 `volume` 属性是**fader 位置**，不是线性增益。
- 旧模板值 `1.0` 在新引擎语义下 = **+6.0 dB**（不是 unity）。这是 132 真栈实测注入点（E2 master 节点 A/B 1.995181 vs 0.999959；E3 五值映射精确对应）。
- 工程内两处独立证据表明**预期默认 master = -3.0 dB**：
  - `VitApp/Source/Core/VitKernelUtils.cpp:128-140`：`Edit::Options{ ..., defaultMasterVolumedB = -3.0f }`（仅当工程 XML 无 `MASTERVOLUME` 子树时生效）；
  - `scripts/gen_vit_project.py`：生成工程写入 -3.0 dB fader 位置。
- **本卡取值**：模板与仓库默认工程统一写 `0.6376281516`（-3.0 dB 位置，裁定①）。
- **精度口径**：`0.6376281516`（模板十进制）与 `0.6376281380653381`（生成脚本字面量）都解析为同一个 float32 引擎属性值（后者是 `exp((-3-6)/20)=0.6376281516217733` 的 float32 精确表示），语义差 ≈ 1.8e-7 dB，不可听/不可测。为避免无意义字节漂移，`gen_vit_project.py` **不改数值、只补注释**（Lead 2026-10-18 批准）。

## 2. 改动清单

代码/测试（本分支）：

| 文件 | 改动 |
|---|---|
| `VitApp/Source/Core/VitPaths.h` | 默认模板（headless 兜底）与 blank 模板（`new_project` 用）的 `MASTERVOLUME` 插件 `volume="1.0"` → `volume="0.6376281516"`；新增 fader 语义注释，防止被"归一化"回 1.0 |
| `VitApp/Workspace/default_project.xml` | headless 启动加载的默认工程同改（仅该属性一行） |
| `scripts/gen_vit_project.py` | **仅注释**：说明该数值 = -3.0 dB fader 位置（float32 精确表示），与模板 0.6376281516 等价 |
| `VitApp/Tests/MasterVolumeCalibrationTests.cpp` | 新增最小回归测试（JUCE-core 链接，VitApp/Tests 现有独立 CMake 工程模式） |
| `VitApp/Tests/CMakeLists.txt` | 注册 `VitMasterVolumeCalibrationTests` 目标与 ctest |

**明确未改（按裁定/文件域）**：

- **存量工程**：`VitApp/Workspace/VitDAW_Project.tracktionedit`（`modifiedBy="timoz"` 的用户存量工程，`volume="1.0"` 保留）——裁定②，只登记不迁移；
- `coord/` 历史工件与 132 run 快照里的工程副本（证据面，不修改）；
- 引擎 `tracktion_engine` 子模块（pin `b4397498` 不变，未提交子模块指针变更）；
- Renderer/写入器/PCA/typed executor/前端/Go agent 生产代码（零改动）。

模板 XML 其余字节保持原样（diff 仅目标属性行 + C++ 注释；`default_project.xml` 仅 1 行）。

## 3. 存量工程策略（裁定②：不静默迁移）

- **策略**：只修模板/新建工程；既有工程 XML 的 `volume="1.0"` **保持原值**。原因：旧值 `1.0` 是"陈旧默认"还是"用户有意 +6 dB"在数据上不可区分；静默改写用户工程有误伤风险（且引擎旧工程迁移面 `tracktion_OldEditConversion.h:274-281` 只处理 AuxSend `gain`→`auxSendSliderPos`，不迁移 volume 属性；改写需另立方案）。
- **存量样本登记（只登记，不修改）**：
  - `VitApp/Workspace/VitDAW_Project.tracktionedit:13` = `volume="1.0"`（用户存量工程，含用户 VST/素材路径）；
  - 仓内其它工程/工件 XML 副本（132 抽查 16/16 份为 `volume="1.0"`，如 `coord/runs/JOURNEY-3/*/kernel_workspace/default_project.xml`、`coord/runs/OPT-IMPL-2/*`、`coord/runs/L4-LEDGER-DIR-1/*`、`coord/runs/QUOTA-RECOVERY-LEGS-1/*`、`coord/runs/PORT-WIN-HYGIENE-SMOKE-1/*`、`coord/runs/L2-2-SEG-SMOKE-1/*` 等历史 run 工件）。
- **已知代价（如实登记）**：修复后**新建/默认工程 = -3 dB**，**存量工程仍 = +6 dB** → 存量↔新建工程 master 来源差 **9 dB**。这是"不迁移"策略的直接代价；若不希望长期保留，需另立迁移卡（建议带备份/用户确认/一次性迁移脚本或 UI 提示）。
- **迁移候选评估要点（另评，本卡不做）**：需定义"陈旧默认 vs 用户有意"判定与用户确认路径；改写用户工程前要备份与可回滚；引擎/写入器层面的迁移面（`OldEditConversion`）改动必须另裁（本卡文件域禁止改引擎迁移面——停止条件）。

## 4. 回归测试（VitApp/Tests 现有模式）

新增 `VitApp/Tests/MasterVolumeCalibrationTests.cpp`（链接 `juce::juce_core`，编译快、不依赖真栈）：

| 检查 | 断言 |
|---|---|
| T1 | 内嵌默认模板 + blank（`new_project`）模板：`MASTERVOLUME` 插件 `volume` 解析为 fader 位置 `≈0.6376281516` 且 `faderPositionToDB(pos) ≈ -3.0 dB`；模板不再含 `volume="1.0"` |
| T2 | 仓库 `VitApp/Workspace/default_project.xml`（headless 启动工程）同上 |
| T3 | 旧值 `1.0` 仍映射 `+6.0 dB` / gain 1.99526（**存量不迁移**诚实证据）；unity 位置 `0.7408182` → 0 dB |
| T4 | PCM 削波边界：0.9 源峰 + 旧 +6 dB → 1.7957（16/24-bit 会 clamp）；新 -3 dB → 0.6372（不 clamp）；差 > 1.15（本次消除的 6 dB） |
| T5 | `scripts/gen_vit_project.py` 生成值仍为 -3.0 dB fader 位置（float32 字面量容差 1e-6） |
| T6 | 存量用户工程 `VitDAW_Project.tracktionedit` 仍保留 `volume="1.0"`（未被静默迁移/改动） |

构建/运行命令（worktree 根目录）：

```powershell
cmake -S VitApp/Tests -B VitApp/Tests/build -G "Visual Studio 18 2026" `
      -DVIT_TRACKTION_ENGINE_DIR=D:/Vit_DAW/tracktion_engine
cmake --build VitApp/Tests/build --config Release --target VitMasterVolumeCalibrationTests --parallel 3
ctest --test-dir VitApp/Tests/build -C Release -R VitMasterVolumeCalibrationTests --output-on-failure
```

结果（2026-10-18，本 worktree）：
- `cmake -S VitApp/Tests ...` configure exit **0**（含 tracktion/juce 配置 58.3s）
- `cmake --build ... --target VitMasterVolumeCalibrationTests` exit **0**（产物 `VitApp/Tests/build/VitMasterVolumeCalibrationTests_artefacts/Release/VitMasterVolumeCalibrationTests.exe`）
- `ctest --test-dir VitApp/Tests/build -C Release -R VitMasterVolumeCalibrationTests --output-on-failure` exit **0**：`1/1 Test #8: VitMasterVolumeCalibrationTests ... Passed`（0.05s）
- 直跑测试二进制输出（实测值）：

```
  default_project_xml_template: MASTERVOLUME volume="0.6376281516"
  blank_project_xml_template: MASTERVOLUME volume="0.6376281516"
  Workspace/default_project.xml: MASTERVOLUME volume="0.6376281516"
  PCM clip boundary: legacy peak=1.795736 fixed peak=0.637151
  scripts/gen_vit_project.py: MASTERVOLUME volume="0.6376281380653381"
  Workspace/VitDAW_Project.tracktionedit: MASTERVOLUME volume="1.0"
VitMasterVolumeCalibrationTests: all checks passed
```

## 5. kernel Release 构建（不占栈）

- 配置：
  ```powershell
  cmake -S VitApp -B VitApp/build -G "Visual Studio 18 2026" `
        -DVIT_TRACKTION_ENGINE_DIR=D:/Vit_DAW/tracktion_engine `
        -DFETCHCONTENT_SOURCE_DIR_LIBZMQ=D:/Vit_DAW/VitApp/build/_deps/libzmq-src `
        -DFETCHCONTENT_SOURCE_DIR_CPPZMQ=D:/Vit_DAW/VitApp/build/_deps/cppzmq-src
  ```
- 构建：`cmake --build VitApp/build --config Release --target VitApp --parallel 4`
- 结果（2026-10-18 本 worktree）：
  - configure exit **0**（14:12:56；见 `build_kernel.log` 第 88 行）
  - `cmake --build ... --target VitApp --parallel 4` exit **0**（14:17:16；见日志第 4221–4222 行）
  - 产物：`VitApp/build/VitApp_artefacts/Release/VitApp.exe`，size=**31,757,824** B，
    SHA256=**B8CD2E4D7C77A2D695BBE86D09A27EDBAF3F03CDFABA285BC0FD47CA6455A594**
    （该 exe 即真栈验证腿的被测内核；MSVC 19.50 / VS 18 2026 / Release）
  - 构建日志含大量既有 C4819/C4244/C4996 warning（tracktion/JUCE 在 code page 936 下的既有告警），无 error
- 环境口径（如实登记）：
  - `tracktion_engine` 子模块在 worktree 内 `git submodule update --init` 首次从 GitHub 重 clone 过慢（>5 min 未完成，已取消），改为以**主检出同 pin 的 `D:/Vit_DAW/tracktion_engine`（HEAD=`b4397498`，clean）只读引用**作为 `VIT_TRACKTION_ENGINE_DIR`；不提交子模块指针/工作树变更；
  - libzmq/cppzmq 无系统包（`cppzmq_DIR`/`ZeroMQ_DIR` NOTFOUND），用主检出 `VitApp/build/_deps` 的 FetchContent 源做 `FETCHCONTENT_SOURCE_DIR_*` 覆盖，**无新网络下载**；
  - 构建产物 `VitApp/build/` 为 gitignored，不入库。

## 6. 已知边界（诚实声明）

- **master 读回/控制面不可见**：`list_tracks` 的 master 行 `volume_db=null`，内核命令面无 master volume 读写命令（132 E8 真栈实测）；用户无法在 UI/命令面看到或调整 master —— 属独立缺口，裁定③明确不在本卡。
- **监听（播放）电平未在本卡直接实测**：132 以同一 master 节点 + 播放图 `includeMasterPlugins` 默认 true 的代码路径推出"监听与渲染同热"，本卡修模板后同样只由渲染腿直接量化；如需监听实测另立 meter 探针腿。
- **存量工程 9 dB 差异**：见 §3（不迁移策略的直接代价）。
- **default_project.xml 是运行时状态文件**：内核启动/保存可能改写它（既有行为）；本卡只把模板默认值改对，不改变运行时写回逻辑。
- **32-bit float 导出不 clamp** 的既有语义不变（132 E5：float 保留越界值）；本卡只把默认电平拉回 -3 dB，不改变写入器行为。
- **端测边界**：本文档的构建/单元测试为离线证据；**真栈渲染比（≈0.707918）/16-bit 削波改善/内核 SHA256 的最终量化证据在真栈验证腿**（§7），受用户 92 手测与 Lead 栈窗口约束，未获 GO 不启栈。

## 7. 真栈验证腿计划（等待 Lead 栈窗口 GO）

- 复用 132 的 **kernel-only berth** 模式（`coord/runs/KERNEL-RENDERGAIN-1/run_render_gain_diag.ps1` + `render_gain_diag.py` 只读参考；复制/改造到本卡 run 目录，不改共享 `scripts/`）。
- 被测内核 = 本卡 worktree Release 构建的 `VitApp.exe`（记录 SHA256、HEAD、工作树状态）。
- 断言：`new_project`（blank 模板）/默认模板渲染 master 比 ≈ **0.707918**（-3 dB）；16-bit 0.9 源峰不提前削波（peak ≈0.6372，不再 clamp）；master 节点存在但读回不可见（E8 复验）。
- 栈纪律：启栈前亲核六端口（5555/5556/5557/7878/8787/4445）监听=0 + 四类进程=0；只杀自起 pid；跑完拆净复核=0；占用记录由 Lead 在 `coord/resources/PC-RUNTIME-STACK.md` 单写（执行侧不碰协调面）。
- 脚本就绪：`master_volume_diag.py`（L0 启动读回/MASTERVOLUME 值 + master 不可见、L1 启动默认工程渲染比、L2 new_project、L3 16/24/32bit、L4 0.9 源峰削波边界、L5 存量 volume=1.0 仍 +6 dB、L6 44.1k 源）+ `run_master_volume_diag.ps1`（berth 运行器，已在 worktree 内通过 Python 语法检查/PowerShell 解析检查；未占用栈、未启动）。
- 状态：**Lead 已裁定处置 B（要正式 GO 窗口重跑，本 run 仅作工程参考、不作验收证据）**。前一次运行 `run_20261018_141849` 发生在 GO 之前（未登记占用即启动可启栈 runner，违反 PROTOCOL §2.2），违规定性、真实重叠窗口与自报处置见 `RESULTS.md` §〇；L0–L6 实测数值（工程参考）见 `RESULTS.md` 表。
  - 摘要（工程参考 run）：L1/L2 渲染比 **0.707918（-3.0003 dB）**；L4 0.9 源峰 16/24/32bit peak≈**0.6371**（不 clamp，旧 +6dB 反事实 1.7957）；L5 存量 `volume="1.0"` 仍 **+5.9996 dB**（不迁移实测）；L0 master 读回不可见；L3 位深恒定；L6 44.1k 恒定。
  - 整改已落地：berth runner 增加只读 **`-PreflightOnly`** 纯预检模式（只做端口/进程/隔离检查与占用拒绝自检，**绝不启栈、不建 run 目录、不写文件**；exit 0=栈空闲 / exit 2=栈占用 would_refuse），已用 dummy listener 自测 0/2 两条分支（`RESULTS.md` §四）。以后任何会启栈命令必须先 Lead GO + PC-RUNTIME-STACK 登记。
  - 正式验收级 run：**等待 Lead 的 134 占用登记 + GO**（用户 92 测完释放栈后），将使用新 run ID 重跑 L0–L6 并更新本报告/RESULTS。

## 8. 工件（本目录）

| 文件 | 说明 |
|---|---|
| `README.md` | 本报告 |
| `build_kernel.log` | kernel configure/build 完整日志（exit code 在日志首尾行） |
| `build_tests.log` | VitApp/Tests configure/build 日志 |
| `test_ctest.log` | 回归测试运行日志（ctest exit=0） |
| `master_volume_diag.py` | 真栈腿诊断脚本（kernel-only，L0–L6；Python 3.13 + pyzmq 27.1；本卡新写，未改共享 scripts） |
| `run_master_volume_diag.ps1` | 真栈腿 berth runner（端口/进程占用预检 + run-dir 隔离 + 只杀自起 pid + 拆净复核；本卡新写） |
| `RESULTS.md` | 真栈验证腿结果（run_20261018_141849：L0–L6 实测表 + 流程偏差登记 + 复现命令） |
| `run_<timestamp>/` | 真栈腿工件（`run_20261018_141849/`：diag_summary.json / diag_console.txt / run_meta.json / teardown.json / sources / fixtures / renders / kernel_workspace；重件留盘、gitignored） |

## 9. 决策点/裁定引用

- 裁定来源：Lead（12:00-18:00 决策流）2026-10-18 代理裁定（用户可覆写），见 BATCH3 §一与 doing 卡面领取记录；Lead 开工答复（2026-10-18 14:22）批准：①目标 `0.6376281516`；②存量不迁移；③master UI/控制面不在本卡；④`gen_vit_project.py` 保留数值+注释；⑤`VitDAW_Project.tracktionedit` 不改只登记；⑥真栈腿等 Lead 栈窗口 GO。

# FIX-MASTERVOL-CALIBRATION-1（134）真栈验证腿结果 —— kernel-only berth，L0–L6 全 PASS

## 〇、正式验收级 run（Lead GO 窗口内）：**run_20261018_142520**

- 流程：收到 Lead GO（PC-RUNTIME-STACK 已登记 134 占用 14:30–14:40）→ 先 `-PreflightOnly` = exit **0**（栈空闲）→ 正式 berth。
- 起止：2026-10-18T14:25:21.748 +0800 → 14:25:38.462 +0800（约 16.7s）；脚本 exit **0**；`master volume diag: PASS`
- 被测内核：`VitApp/build/VitApp_artefacts/Release/VitApp.exe`，size=31,757,824 B，
  SHA256=**B8CD2E4D7C77A2D695BBE86D09A27EDBAF3F03CDFABA285BC0FD47CA6455A594**；HEAD=**d7e11ff9**（worktree `git status --short` 空）
- 进程：自起内核 pid=**25832**；只杀该 pid；拆栈后六端口（5555/5556/5557/7878/8787/4445）监听=**0**、VitApp/VitAgent/vsphub/Godot-frontend 进程=**0**（`teardown.json`）
- 隔离：部署目录 `D:\Vit_DAW\Export\staging\runtime` 在 14:25 后**零写入**；本 run 内核日志 4 件全部落在 `run_20261018_142520/kernel_workspace/Workspace/Logs/`（如 `VitHeadlessServer2026-10-18_14-25-21.log`）→ run-dir 隔离契约满足
- L0–L6 实测（全部 PASS）：

| 腿 | 实测 | 期望 | 判定 |
|---|---|---|---|
| L0 启动读回 | 启动工程 `volume="0.6376281516"`；master 行 `volume_db=null`；无 master 命令面 | 模板 -3dB + 读回不可见 | PASS |
| L1 启动默认工程渲染 | ratio=**0.70791812**（**-3.000339 dB**），16bit | 0.7079458（-3.0 dB） | PASS |
| L2 `new_project`（blank 模板） | ratio=**0.70791812**（**-3.000339 dB**） | 0.7079458 | PASS |
| L3 位深 16/24/32 | 0.707918 / 0.707896 / 0.707895（32bit=float32） | 恒定 ≈0.7079 | PASS |
| L4 0.9 源峰削波边界 | peak16=**0.63716544**、peak24=**0.63712533**、peak32_float=**0.63712525**，均 <1.0（不 clamp）；旧 +6dB 反事实线性峰=**1.795736**（会 clamp） | 不提前削波 | PASS |
| L5 存量 `volume="1.0"` fixture | ratio=**1.99518054**（**+5.999644 dB**） | 1.995262（+6dB，不迁移） | PASS |
| L6 44.1k 源 | ratio=**0.70786142**（-3.001035 dB） | 0.7079458 | PASS |

- 工件 sha256（`run_20261018_142520/`，重件留盘、gitignored；run dir 共 30 文件 / 9.73 MB）：
  - `diag_summary.json` = `B6B023AD705A040E1A7A4A902415868F9E825543DE097384CDB37FB8E7232D0F`
  - `diag_console.txt` = `A87220BE0A3F4AFB8F331753B0BB8603126B26F6EE66C4B0791F248B0C6FDDBC`
  - `run_meta.json` = `406CD18DCF9EFF436E3B99984E0422D7B5CBEB22680BD6D140B921C124E1B251`
  - `teardown.json` = `6E44D59110DECEA6A6FE437BF95A502689161DA3AF4CDD1C2E8FA5578C8D712B`
  - `startup_default_project.xml` = `F3AC7BFCD906C54E88F69A5A9BC703C445E3A377C1A267211D4F3974D5131ACD`
- 结论：与工程参考 run 数值完全一致（确定性）。修复前沿 132 E4 `new_project`=1.995181（+6.0dB）→ 修复后 **0.707918（-3.0003dB）**；16/24bit 削波提前已消除；存量工程仍 +6.0dB（不迁移实测）。**本 run 作为本卡真栈验收级证据。**

## 〇-附、前一次工程参考 run（未登记违规，不作验收证据）：run_20261018_141849

> **状态标记（Lead 裁定，2026-10-18）：本 run 为「工程参考 / 未登记违规」，不作验收证据。**
> 处置=Lead 裁定 **B（正式 GO 窗口重跑）**；正式 run 将使用新 run ID 并作为验收级证据。
> 违规定性：未在 `coord/resources/PC-RUNTIME-STACK.md` 登记占用、未获 Lead GO 即启动可启栈 runner（违反 PROTOCOL §2.2；"预检拒绝 dry-run"不构成豁免）。启动 14:18:49 与 118 腿拆栈窗口（14:18:50–52）有真实重叠（118 报告记录本 kernel pid 20128 在场），双方均未触碰对方进程、无功能踩踏；self-report 及时、隔离契约满足、拆栈干净。
> 整改（已落地）：berth runner 新增只读 `-PreflightOnly` 纯预检模式（`run_master_volume_diag.ps1`，见 §四），绝不启栈、不建 run 目录、不碰外来进程。

- 运行：**run_20261018_141849**（目录在本卡 `coord/runs/FIX-MASTERVOL-CALIBRATION-1/run_20261018_141849/`，重件留盘 worktree、不入库）
- 起止：2026-10-18T14:18:49 +0800 → 14:19:07 +0800（约 18s）；脚本 exit **0**；diag 输出 `master volume diag: PASS`
- 被测内核：`D:/Vit_DAW_wt_mastervol1/VitApp/build/VitApp_artefacts/Release/VitApp.exe`
  - SHA256=**B8CD2E4D7C77A2D695BBE86D09A27EDBAF3F03CDFABA285BC0FD47CA6455A594**
  - size=31,757,824 B；HEAD=**d3dd31e3**（worktree status --short 空）
- 驱动：本卡新写 berth runner `run_master_volume_diag.ps1` + `master_volume_diag.py`（kernel-only，无 agent/Godot UI；run-dir 隔离：`kernel_workspace/{CMakeLists.txt,Source/}` 标记 + `VIT_PROJECT_XML` 指向 run 目录）
- 环境：Windows / ZTY / Python 3.13.12 / pyzmq 27.1.0；六端口=5555/5556/5557/7878/8787/4445

## 〇、流程偏差（如实登记，待 Lead 裁定）

- 本次运行**发生在 Lead 给出「真栈窗口 GO」之前**。原意是执行一次「预检应拒绝启动」的 dry-run（验证 runner 遇到栈占用会安全拒绝），但 14:18:49 预检实测六端口监听=0、VitApp/VitAgent/vsphub/Godot 进程=0（即当时栈空闲），runner 因此按设计继续启动内核并跑完全部腿。
- **未与任何其他流栈重叠**：启动前预检=0；运行期间只使用自起内核 pid=**20128**；拆栈只杀该 pid；拆栈后复核六端口监听=0、四类进程=0（`teardown.json`）。
- **隔离契约满足**：本 run 内核日志/Settings 全部落在 run 目录 `kernel_workspace/Workspace/`；部署目录 `D:\Vit_DAW\Export\staging\runtime` 最近写入时间=**14:18:38**（早于本 run 内核启动 14:18:49，属其它会话/118 栈），本 run 对外部目录**零写入**。
- **资源协调面**：本执行侧未写 `coord/resources/PC-RUNTIME-STACK.md`（协调面由 Lead 单写）；如需后补本条占用记录（14:18:49–14:19:07，kernel pid 20128，已拆净），由 Lead 登记。
- 待裁定：**已裁定（Lead，2026-10-18）= B：按 GO 正式窗口重跑；本 run 采信为工程参考、不作验收证据。**

## 四、整改：`-PreflightOnly` 纯预检模式（已落地并自测）

- 实现位置：`run_master_volume_diag.ps1` 参数 `[switch]$PreflightOnly`（头部 usage 注释同处）。
- 语义：只读执行端口/进程占用检查 + kernel exe/HEAD 信息采集；**绝不启动内核、不创建 run 目录、不写任何文件、不触碰外来进程**。退出码：**0**=栈空闲（真实 berth 可启动，但仍必须 Lead GO + 占用登记）、**2**=栈被占用（would_refuse=true）。
- 自测证据（2026-10-18 14:21–14:22，栈空闲时）：
  - 自由态：`-PreflightOnly` → exit **0**，JSON `"stack_free": true`，未创建 run 目录；
  - 占用态自检：本地临时绑定 127.0.0.1:4445 的 dummy TcpListener（**非栈进程**，模拟占用，随后释放）→ `-PreflightOnly` exit **2**，JSON `"would_refuse": true` 且列出 `port:4445,pid:<dummy>`；内核未启动、无文件写入；释放后 4445 复核=free。
- 纪律：以后任何会启栈的命令必须先有 Lead GO + `PC-RUNTIME-STACK` 占用登记；`-PreflightOnly` 不是启动许可。

## 一、L0–L6 实测结果（全部 PASS）

| 腿 | 实测 | 期望 | 判定 |
|---|---|---|---|
| **L0** 启动读回 | 启动工程 `MASTERVOLUME volume="0.6376281516"`；kernel snapshot 无 MASTERVOLUME 读回；`list_tracks` master 行 `volume_db=null` | 模板 -3dB 值 + master 读回不可见 | PASS |
| **L1** 启动默认工程渲染 | ratio=**0.70791812**（**-3.000339 dB**），16bit | 0.7079458（-3.0 dB） | PASS |
| **L2** `new_project`（blank 模板） | ratio=**0.70791812**（**-3.000339 dB**），16bit | 0.7079458 | PASS |
| **L3** 位深 16/24/32（同 blank 工程） | 0.707918 / 0.707896 / 0.707895（32bit=float32） | 恒定 ≈0.7079 | PASS |
| **L4** 0.9 源峰削波边界（blank 工程） | 16bit peak=**0.637165**、24bit=**0.637125**、32bit float=**0.637125**；三者均 <1.0（不 clamp）。旧 +6dB 反事实线性峰=**1.795736**（>1.0，132 E5 实测会 clamp） | 不提前削波 | PASS |
| **L5** 存量工程 `volume="1.0"` fixture | ratio=**1.99518054**（**+5.999644 dB**） | 1.995262（+6dB，不迁移） | PASS |
| **L6** 44.1k 源（blank 工程） | ratio=**0.70786142**（-3.001035 dB） | 0.7079458 | PASS |

结论（对照 132 KERNEL-RENDERGAIN-1 基线）：

- 修复前：`new_project`/启动默认工程 ratio=**1.995181（+6.0dB）**（132 E4）。
- 修复后：同一路径 ratio=**0.707918（-3.0003dB）** —— 与裁定目标 -3.0 dB 一致（容差 1% 内，差 0.0003dB）。
- 16/24bit 削波边界：0.9 源峰下修复后 16/24bit 渲染 peak≈0.6371（不 clamp）；修复前 +6dB 时线性峰 1.7957 会 clamp（132 E5）。**"16/24bit 削波提前 6dB"已消除**。
- 存量工程实测仍 +6.0dB：**不静默迁移**策略在真栈上得到实测证据（修复只影响模板/新建工程）。
- master 节点存在但读回不可见：`list_tracks` master 行 `volume_db=null`、无 master 命令面（与 132 E8 一致）；master 节点本身仍参与渲染（132 E2 A/B 已锁定注入点）。

## 二、工件（run_20261018_141849/）

| 文件 | 说明 / SHA256 |
|---|---|
| `run_meta.json` | 内核 SHA256、HEAD=d3dd31e3、worktree status、机器；sha256=`48F093C5E74394D87810867E4D6ED64D45612D513F9465D042864643132A865D` |
| `diag_summary.json` | 全部腿实测数值/源与 fixture 统计；sha256=`C4FA3438DBCFBC94907A6716217FA0191BAFEDAED99E9EA6794B78D31A994E60` |
| `diag_console.txt` | 控制台完整输出（含 PASS 行）；sha256=`34094DBF57EF174655BF918F95B20183422A7DAC3C804515848AAABB7287C02A` |
| `teardown.json` | kernel_pid=20128、torn_down=true、剩余监听=0、剩余进程=0；文件尾有 rename 前路径字符串（目录已由 run_DRYRUN_a 更名为正式 run ID，内容未改） |
| `startup_default_project.xml` | 启动工程副本（=本卡修复后的 `VitApp/Workspace/default_project.xml`，volume=0.6376281516） |
| `sources/` `fixtures/` `fixtures_live/` `renders/` `kernel_workspace/` | 源 wav / legacy fixture / 渲染 wav / run-dir 内核工作区（重件留盘，gitignored） |

## 三、复现方式

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\coord\runs\FIX-MASTERVOL-CALIBRATION-1\run_master_volume_diag.ps1 `
    -RepoRoot D:\Vit_DAW_wt_mastervol1 `
    -KernelExe D:\Vit_DAW_wt_mastervol1\VitApp\build\VitApp_artefacts\Release\VitApp.exe `
    -RunDir D:\Vit_DAW_wt_mastervol1\coord\runs\FIX-MASTERVOL-CALIBRATION-1\run_<new>
```

- 预检：六端口监听=0 且 VitApp/VitAgent/vsphub/Godot-frontend 进程=0 才启动；否则拒绝且不杀任何进程。
- 拆栈：finally 只杀自起 kernel pid，复核端口/进程=0，写 `teardown.json`。

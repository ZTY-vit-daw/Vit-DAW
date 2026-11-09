# WAVESHELL-OFFRENDER-FIX-1 (309) 栈腿 RUNBOOK §8 跑前口径（执行前成文，2026-11-09）

一次栈窗口合并 M1+插桩+判别+修法 B 验证（五项裁定①）。任一腿启动前本文件定稿。

## 0. 被测二进制与源面

- **被测内核**：`D:/Vit_DAW_wt_309recon/VitApp/build_release/VitApp_artefacts/Release/VitApp.exe`
  （构建命令：`cmake -S D:/Vit_DAW_wt_309recon/VitApp -B .../build_release -G "Visual Studio 18 2026" -DVIT_TRACKTION_ENGINE_DIR=D:/Vit_DAW_wt_309recon/tracktion_engine` + `cmake --build --config Release --parallel`，exit 0 零 error，2026-11-09 19:04）。
  **SHA256 = `92b09da5f940ee5aa9fc877f99e54fb87c4b4a578d9a37bdab1ecf5cbee891e7`**（31,832,576 B）。
- **engine 源**：worktree 子模块 `D:/Vit_DAW_wt_309recon/tracktion_engine` @ 分支 `vit309-stackleg`
  = 60ab1a49（301 插桩 VIT301_GUARD 在位）+ cd957f4（VIT309 mode 遥测，log-only）+ 9b8cc8a（fix-C 原型，env 门控 VIT309_C_CONSISTENCY）。
- **VitApp 源**：port 分支 `port/waveshell-offrender-fix-1` @ 61e29e1e（修法 B + 形态 3 + VIT309_RTLEVEL + 驱动器/ps1）。
- 主仓 `D:/Vit_DAW/tracktion_engine` @ 60ab1a49 与 wt_228 内核 `F9E3DA0A…` 全程只读不动。
- 被测 agent=wt_309recon 现场构建（Go 树=port 分支，agent 源零改动，非本卡判定面）。
- 内核日志落点：`D:/Vit_DAW_wt_309recon/VitApp/Workspace/Logs/VitHeadlessServer_*.log`。

## 1. 实验矩阵（每腿单 run 无重跑；kernel 生命周期 K1-K4 同一占用窗口内由 owner 亲停亲启）

| 腿 | 内核 env | 命令（ps1 公共前缀 `-RepoRoot D:\Vit_DAW_wt_309recon -StartKernel -RestartAgent -NoChatSmoke -NoStripSilenceSmoke -WaveshellStacklegSmoke -SkipBuild -RunArtifactsDir <run>\e<n>`） | 预期观测 | 判定 |
|---|---|---|---|---|
| E1 M1 实时对照 | VIT309_RTLEVEL=1（K1） | `-Vit309RtLevel -WaveshellLegMode realtime -WaveshellLegPlugin grand_rhapsody` | RTLEVEL 采样 ≥4s；verdict∈{sound,silent,marginal} 皆为有效判定 | RT-SOUND→non-realtime 特异族确认（S3 弱化）；RT-SILENT→S3/装载面升格（裁定⑤预案）；exit 0=判定完成/2=环境/4=装置（零 RTLEVEL 行=tap 缺陷取证） |
| E2a 基线遥测 | VIT309_DISABLE_BLACKLIST=1（K2） | `-Vit309DisableBlacklist -WaveshellLegMode render -WaveshellLegPlugin grand_rhapsody` | 修法 B 旁路：首渲 offline 静音预期 + VIT309_MODE 遥测钉 S1 时序（setupProcessing kRealtime ×≥1 + firstProcessAudio perCallMode=kOffline/setupMode=kRealtime）+ 形态 3 自动降级（VIT309_DEGRADED+degraded_to_realtime_render=true+重渲出声） | S1 运行时确证面；exit 0=链完成（静音/非静音均为观测）/4=装置 |
| E2b C 判别 | VIT309_DISABLE_BLACKLIST=1 + VIT309_C_CONSISTENCY=1（K3） | `-Vit309DisableBlacklist -Vit309CConsistency -WaveshellLegMode render -WaveshellLegPlugin grand_rhapsody` | fix-C 一致化（per-call=kRealtime）：**出声=S1 证实+S2 排除+C 原型有效**；仍静音=S1 排除→S2/S3 升格 | 一次实验同时判别 S1/S2+验证 C（裁定②）；exit 语义同上 |
| E3 修法 B 验证 | 无 VIT309 env（K4） | `-WaveshellLegMode render -WaveshellLegPlugin grand_rhapsody` | GR 直挂渲出声（RMS 非零）+ reply realtime_fallback=true/shell_class_plugin_hint 在场 + VIT309_BLACKLIST 警示日志 + 无 degraded（首渲即 realtime） | M0a2 场景复跑=出声的验证面之一（直挂形态）；exit 0 必须出声（静音=B 失效→止损上交） |
| E5 rack 路径 | 无 VIT309 env（K4 复用，py 直跑同 kernel） | `-WaveshellLegMode render -WaveshellMountMode rack -WaveshellLegPlugin grand_rhapsody`（同 K4 不重启） | GR rack 挂载（seg3a_r2/M0a2 同形）渲出声 + fix-B 披露在场 | M0a2 场景复跑=出声（rack 形态=原缺陷形态）；exit 0 必须出声 |
| E4 M3 不回退 | 无 VIT309 env（K4） | `-WaveshellLegMode render -WaveshellLegPlugin surge_xt` | Surge XT 出声（-13.63 dBFS 附近）+ **无** shell hint/realtime_fallback/degraded（非 WaveShell 类不受 B/3 波及）+ VIT309_MODE 遥测（perCallMode=kOffline 且出声=Surge 容错复证） | M3 不回退验证；exit 0；若 Surge 被 B 误伤（fallback=true）=谓词过宽→取证 |

- 腿序：E1→E2a→E2b→E3→E5→E4（K4 三腿共享内核；E2b 结果不影响后续腿执行——B 是交付修法不依赖判别结果）。
- E2b/E2a 的 fix-B 旁路与 C 原型均为**实验门控**，生产默认（K4）全部关闭。

## 2. §8 概率性声明

- **运行次数**：每腿单 run 无重跑；六腿合计 6 次驱动 run、4 个内核生命周期（K1-K4）。
- **预算**：**零 LLM**（全部腿 -NoChatSmoke -NoStripSilenceSmoke；agent 启动仅为栈契约；telemetry jsonl 零行=零计费亲证）。无付费轮。
- **成功条件**：
  - 全腿 exit 0 且判定面齐全（E1 verdict ∈ {sound,silent,marginal} 记录在案；E2a S1 遥测三键齐；E2b 判别结论成文；E3/E5 wav RMS 非零+B 披露齐；E4 出声+零回退）。
- **失败分类**：exit 2=environment（栈/端口/装置/VSTi 库存）；exit 4=command_defect（装载/可见性/渲染发布/RTLEVEL 零行/B 失效致 E3/E5 静音）；环境中断只有原始日志在卷才可排除出有效轮次。
- **止损线**：同一确定性断点连续两败（跨腿同型计入）→停手上交取证不重跑；内核崩溃/端口互染类环境中断先记录再决定。E3/E5 任一静音即修法 B 失效证据（不等两轮），记录后仍继续剩余腿（E4 回归不依赖 B）——除非栈本身不可用。
- **M1 应急（裁定⑤）**：E1=RT-SILENT 时——B 降级纯警告的代码不改动（B 仍在库），E3/E5 预期改为「渲染静音+VIT309_RT_STILL_SILENT+render_silence_observed 披露」，装载取证另立卡，本腿如实记录不扩修。

## 3. 栈协议

- 候栈排队：322 腿二 03:05 已释放，登记顶部=空闲；每腿登记前 fetch 重读资源记录+净空亲核（六口 TCP 5555/5556/5557/7878/8787/18787 listen=0 + UDP 4444/4445 + VitApp/VitAgent/vsphub/Godot/python 进程=0）→占用登记推 main→≤60s 复核→启动。
- 本窗口=一次占用登记（K1-K4 内核重启为窗口内 owner 亲停亲启，进程号记录）；窗口结束 TEARDOWN_ALL_ZERO 亲核（六 TCP+两 UDP+五类进程全零+killed PID 在案）+释放登记回填 run 记录。
- 工件：`coord/runs/WAVESHELL-OFFRENDER-FIX-1/stackleg_<ts>/e<n>_<tag>/`（每腿新目录不覆盖；console+summary+vit_trace+fixture+wav 留盘 gitignore 同 301）。

## 4. 验证矩阵外延（回归面）

- go build ./... + go test ./...（agent 全量；本卡 Go 零改动，防 scripts/环境意外）。
- webui `npm run test`（无该域改动，例行）。
- §6 五条健康 Go 测试（G0 基线面）。
- AB 烟测（run_ab_result_smoke.ps1）边界：若窗口时间内允许则跑，否则如实声明未跑+理由（本卡改动不触 mixboard/MOM 面）。

# 2026-10-18 午窗 批量三裁定（132 KERNEL-RENDERGAIN 取证 pass + P1 升级 + 修复卡立项）

- 验收会话：12:00-18:00 决策流（DSH PC，本机）；裁定来源=132 已确认卡（用户 2026-10-18 午窗选择「118+132 并行」派发）。
- 基线：origin/main=b428ad90（验收前）；本批合入后 main=__MAIN_AFTER__。
- 执行侧：DSH 团队执行流 exec-rendergain；协调面由 12:00-18:00 决策流单写。

## 一、KERNEL-RENDERGAIN-1（132）——取证腿 **pass** + 定性 **P1 候选升级**

- 交付：分支 **port/kernel-rendergain-1@e3b4ac32**（19b15661 ARCHAEOLOGY.md → f2038da9 berth runner/诊断脚本 → e3b4ac32 RESULTS.md+run 工件）；零生产代码改动，未 push main。
- **结论（决策侧亲核）**：离线渲染 +6.000dB 注入点=**工程 master volume 节点**（Edit 级 VolumeAndPanPlugin 状态=fader 位置 1.0；pin 引擎 `tracktion_AudioUtilities.cpp:136-156` `volumeFaderPositionToDB(1.0)=+6.000000dB`，0.7408182=0dB、0.6376282=-3dB）；模板三处硬编码 `volume="1.0"`（VitPaths.h:25-29/73-77 + 仓库 default_project.xml:14-18），与 gen_vit_project.py 的 -3dB 期望不一致；历史 blame=初始提交 `991cffb6`（2026-04-22）。**排除** Renderer 归一化/写入器/import/track 增益。
- **真栈证据（run_20261018_135710，10/10 腿 PASS）**：E1 ratio 1.995181=+5.9996dB（peak 0.99762 复现 K1）；E2 `use_master_plugins` A/B **1.995181 vs 0.999959**（注入点锁定）；E3 五份仅 MASTERVOLUME 不同 fixture 映射 1.995181/0.999959/0.707918/0.707918/0.999959 与 fader 换算精确对应；E4 new_project(blank) 1.995181；E5 16/24/32bit 恒定且 16bit peak 1.00003 clamp、24bit 1.00000 clamp、**32bit float 1.79566 不 clamp**；E6 幅度扫描/44.1k/mono/左独占逐声道一致；E8 `list_tracks` master `volume_db=null`+无 master 命令=不可见。
- **定性**：缺陷成立（模板陈旧值 + 引擎 volume 属性语义迁移缺失），**非故意 headroom 补偿**。
- **用户面**：所有新建/存量工程 master 总线恒 +6dB 过 unity；16/24bit 导出削波提前 6dB；无 UI/命令可读可控；跨工程来源电平不一致（new_project +6 vs gen_vit_project -3 vs 无 MASTERVOLUME -3）。比值类烟测免疫；绝对幅度面受影响清单见 132 RESULTS §0③。
- **裁定**：取证腿 **pass**；**升级 P1 候选**；立修复卡 **FIX-MASTERVOL-CALIBRATION-1（池序 134，todo，待用户确认派发）**，卡内决策点（模板目标 0dB unity vs -3dB；存量工程迁移 vs 只修模板）先上交裁定后落代码。
- 边界采信：kernel-only berth（不启 agent/UI，Lead 采信）；监听电平未真栈实测（由同一 master 节点+播放图代码路径推出，code-anchored；如需要另立 meter 探针腿）；重件留盘 worktree、精简件入库。
- 卡 132：**done / 验收 pass**。

## 二、偏差与栈

- 偏差登记（如实）：132 首轮非隔离尝试（run_135453）内核按 exe 目录解析 Workspace，向 **`D:\Vit_DAW\Export\staging\runtime\Workspace\Logs\` 写/追加 4 个运行时日志**（baker_diag.log 14.8MB / l3_acoustic_analyzer_diag.log 4.3MB / waveform_envelope_baker_diag.log 4.7MB / VitHeadlessServer2026-10-18_13-54-54.log 129KB）——非源码、非其它卡工件；13:56 后修复为 run-dir 隔离（VIT_PROJECT_XML 重定向）并复验部署目录零写入；日志保留未删作证（是否清理后续另行裁定）。
- 栈：132 占用 13:56–14:05（owner=exec-rendergain，stock kernel SHA256 B6565DCF…），release 后六端口（5555/5556/5557/7878/8787/4445）监听=0、四类进程（含 Godot 变体）=0；**当前栈空闲、用户 92 手测优先**。
- 池中外部新增（另一决策会话提交 1d6ee3be，用户已授权出卡）：**MEM-SCHEMA-1**、**CAP-BOUNDARY-1**，池序待主管排定。本裁定：**MEM-SCHEMA-1 池序 135**（P2，纯 docs 域，可与其它卡并行）；**CAP-BOUNDARY-1 池序 136**（P2，与池序 54 `FS-BLOCKED-SURFACE-CLOSURE-1` 同触 `agent/internal/chat/goalrunner_chat.go`，须主管排序后串行）。派发待用户/主管按窗口确认。

# G0-SCORE-SPIKE-1：谱面库选型 spike——真实编辑回读出声证据（展演关键路径去风险）

- 发卡：GLM 主管决策流 / 2026-11-15 深夜（依据=[展演目标线审计 H4](../../reports/2026-11-12-demo-execution-audit/README.md)+[进度驱动裁定](../decisions/2026-11-15-progress-driven-ruling.md)；无工期估算）
- 派发确认：已确认（主管发卡，第一波去风险）
- 验收负责人：主管决策流（选型结论+证据）
- 池序 369；P0；依赖=无；PC

## 目标

1. 谱面库候选盘点（含不限于：VexFlow/abcjs/OpenSheetMusicDisplay/自研 SVG+内核模型）——按本课硬需求打分：2/4 与 4/4 拍、附点/切分/休止/延音的真实**编辑**（非仅渲染）、音符 ID↔内核 clip/note 映射、随谱播放游标时钟、MIDI/记谱双向（W18 确定性转换承接）。
2. 对 ≤2 个最优候选做最小 spike（本地 scratch，不入生产）：附点+切分+休止+延音在 2/4 下的编辑→数据→重绘往返；记录与内核回读链路的接口假设。
3. 产出选型结论+实施卡草案（谱面工作台主开发卡的文件域/里程碑/风险），报告 `coord/runs/G0-SCORE-SPIKE-1/REPORT.md`。

## 非目标

- 不实施谱面工作台本体；不改生产文件；真实出声验证留主开发卡（本卡可做内核回读命令面静态盘点）。

## 文件域

只读勘察+scratch（`coord/runs/G0-SCORE-SPIKE-1/` 工件；如需代码 spike 放同目录 scratch/，不碰 src/）。

## 验收标准

报告三节（盘点打分/spike 证据/实施卡草案）；候选结论可支撑主管直接发主开发卡。

## 领取/回执/验收

### 领取

- 时间：2026-11-15 23:21（Asia/Shanghai）
- origin/main 基线：`9cc3048ccdda11c3f889476ebfd85eea3cd46c25`
- 领取时基线状态：worktree 为 origin/main 全新检出，`git status --short` 干净、`git diff --stat` 空（主检出 D:/Vit_DAW 的既有未提交改动不涉本卡文件域，未触碰）
- owner：GLM-5.3 执行子代理（ZCode L2 会话）+ PC（Windows，Git Bash）
- 分支：`coord/369-claim`（领取协调分支）；worktree：`D:/Vit_DAW_wt_369coord`
- 领取提交：`d7d84c94a9e4a87e375b4d242f54cae3d56e0460`（已推 main 并核对远端卡面 owner）

### 回执

- 完成：2026-11-15 23:43（Asia/Shanghai）；零栈（未启 VitApp/Godot/agent 真栈，未占 PC-RUNTIME-STACK，无 run ID）
- 工件：[coord/runs/G0-SCORE-SPIKE-1/REPORT.md](../../runs/G0-SCORE-SPIKE-1/REPORT.md)（报告三节+内核回读命令面静态盘点+运行口径）+ scratch/ 两候选（vexflow-spike、verovio-spike，node_modules 已 gitignore，未碰生产 src/ 与主仓 package.json）
- 测试命令与退出码：
  - `cd coord/runs/G0-SCORE-SPIKE-1/scratch/vexflow-spike && node spike.mjs` → **exit 0**（27 断言）
  - `cd coord/runs/G0-SCORE-SPIKE-1/scratch/verovio-spike && node spike.mjs` → **exit 0**（32 断言）
- 结论一句话：**主路线=VexFlow 5（MIT/TS 渲染引擎）+ 自研 beat-grid 记谱模型**（附点/切分/休止/延音编辑→序列化→重推导→重绘、tick→x 游标、模型↔SMF 精确往返全部 spike 亲证）；Verovio（LGPL、edit() 实验性、MIDI 时值带 note-off gap）不进首版编辑链，登记为教材材料 MusicXML 只读显示的可选件；主开发卡草案（SCORE-WORKBENCH-1：文件域/里程碑 M1-M5/风险 R1-R8）在 REPORT §5，可直接发卡。
- 关键接口锚点（静态盘点）：回读=`get_midi_clip_notes`（notes[]{id,pitch,start,length}，beats 相对 clip）+`music_context`（拍号/速度+单段门）+`current_selection.playhead_seconds`；写入=`apply_midi_note_patch`（ops+time_unit:beats，RiskConfirm）；**无 set_time_signature 生产命令（拍号写入须独立内核卡）**。
- 端测覆盖边界声明（AGENTS §5）：本卡为零栈勘察卡——验证面=确定性 node 断言（exit 0），**无浏览器渲染面烟测、无真栈烟测**（非目标）；真实出声与渲染面验证属主开发卡 M5。
- 回执提交：即本次 coord(369) 收口提交（卡移 done/、REPORT+scratch 工件、POOL-INDEX 同批；hash 见 `git log -- coord/cards/done/2026-11-15-G0-SCORE-SPIKE-1.md`）。

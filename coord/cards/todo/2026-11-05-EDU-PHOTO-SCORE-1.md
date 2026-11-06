# EDU-PHOTO-SCORE-1：拍谱开局节点——离线转换接线 + 录入即问询即教学编排（乐局 5）

- 发卡：GLM 主管决策流 / 2026-11-05（依据=[rulings §五-12](../../rulings/2026-11-05-EDU-DEMO-SCOPE-RULINGS-1.md) + [256 报告](../../reports/2026-11-05-edu-cap-recon-1.md) 点③拍谱路径）
- 派发确认：已确认（用户 2026-11-05 批准入池）
- 验收负责人：GLM 主管决策流
- 池序 286；前置=281+282+**用户已部署 Audiveris/music21**；P1 / 0.5-1 天；L1-L2 / PC；真栈烟测腿搭车或独立

## 目标

1. **离线转换腿**（scripts/ 小工具）：MusicXML→MIDI 转换脚本（music21，`.write('midi')`，单旋律导出；PPQ）——用户离线运行（拍谱→Audiveris→MusicXML→本脚本→MIDI 文件）；脚本零常驻、依赖 pin 进 requirements 文本。
2. **乐局开局节点编排**（279 拍谱节点的运行时行为，281 拦截面内）：`import_midi_to_track`（start_time_beats=0）→ **显式 `set_tempo` 对齐**（import 不读 tempo meta，256 点③在卷）→ 回读核对（note_count）→ `transport.play` 回放 → **问询纠错话术引导**（"我读出来的旋律是这样，和你的谱对吗？哪里不对？"——模型锚定坐标=自设循环段，§五-8 规则）→ 用户指出错音 → `analyze_midi_transformations`/读面定位 → 修正写入（走确认链）。
3. **工程侧零改动验证**：全链只用既有命令（256 已证），本卡新增=脚本+引导文本+（如需）拦截面开局分支接线。

## 约束

- 文件域=scripts/ 新脚本+乐局引导文本+（如需）edu/chat 最小接线+烟测脚本；零内核；Audiveris/music21=用户部署离线工具（引擎故障=环境中断不重跑超两次）。

## 验收标准

- 转换脚本单测（fixture MusicXML→MIDI 音符往返一致）；真栈烟测：注入→set_tempo→回放→问询节点触达 exit 0；回执附转换样例+run ID+commit hash。

## 停止条件

- 用户工具未部署→本卡等待（前置声明）；Audiveris 识别质量不属本卡验收（OMR 精度=市场，rulings §五-13）。

## 领取

- （执行侧回填）

## 回执

- （执行侧回填）

## 验收

- （主管回填）

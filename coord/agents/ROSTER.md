# 子代理名册（复用优先）

> 用途：主管发卡前先查此表，优先 `send_message` 复用同一产线子代理；新卡/完结后更新。最后更新：2026-11-20（tutorprod438b 续任完成 438 v2 实现，done 待验收）。

| owner | 产线 | worktree / 分支 | 最近任务 | 状态 |
|---|---|---|---|---|
| `convrail418` | 入口/对话流 UI | pc418-convrail / pc421-cross / pc426-settings / pc428-window / pc429-motion / pc430-motion / pc431-motion | 431 左栏 800ms | inactive（可续） |
| `legameparse423` | 乐局 Go 线（W01） | pc423-parse / pc424-compile / pc425-bind | 425 零栈绑定；真栈腿待栈位 | inactive（可续） |
| `score397stack` | 评分绑定/真栈 | pc397-merge / codex/score-binding-397-integ | 397 真栈 PASS | inactive（可续） |
| `materials420` | 教学材料线 | pc420-materials | 420 材料批2-4 PASS | inactive（可续） |
| `railb1` | 入口壳层 B1 | pc417-rail | 417 B1 PASS | inactive（可续） |
| `convopen419` | agent 组合工具 | pc419-open | 419 conversation_open PASS | inactive（可续） |
| `w02clip` | W02 乐局上下文/clip | pc432-clip / codex/w02-a1a-clip-432 | 432 A1a PASS 66ad1d66 | inactive（可续） |
| `w02ctx` | W02 课程上下文注入 | pc432-context / codex/w02-a1b-context-432 | A1b + free-state/diag fix dcee3135 | inactive（可续） |
| `w02real` | W02 真栈旅程 | pc432-real / codex/w02-a1c-real-432 | 432 A1c PASS 20cb3657 | inactive（可续） |
| `w02ht` | W02 手测 prep | pc434-handtest / codex/w02-a1-handtest-434 | 434 same-stack prep（产品 v0 superseded） | inactive（可续） |
| `tutorsrv` | TUTOR-SIM 导师循环 | pc435-tutor / codex/tutor-sim-1-435 | 435 TUTOR-SIM-1 PASS 97708aee | inactive（可续） |
| `tutorui` | TUTOR-SIM UI 控件 | pc436-ui / codex/tutor-sim-2-ui-436 | 437 widgets 执行完成 b0c83c85（待手测） | inactive（可续） |
| `tutorprod438` | TUTOR-SIM 生产节点 | （未建——勘察停卡） | 438 勘察停卡：音频材料前提推翻，证据 coord/runs/TUTOR-SIM-4/ | inactive（可续，拆卡后领新腿） |
| `tutorprod438b` | TUTOR-SIM 生产节点 | pc438b / port/tutor-sim-4-438b（基线 841b9ef4） | 438 v2 续任实现完成：v1 schema+MIDI 材料装载+webui 材料卡，三门绿合入 main（950f1695→51eff149），done 待验收，证据 coord/runs/TUTOR-SIM-4/v2/ | inactive（待验收，可续真栈腿） |
| `w02fb` | W02 A2a 作品反馈装置 | pc433-feedback / codex/w02-a2a-feedback-433 | A2a device 073d5f37（真回合未跑） | inactive（可续） |
| `combo422` | 422 跨工作树验证装置 | pc422-combo / codex/conv-cross-combo-422 | 422 真栈 PASS 218dff3c | inactive（可续） |
| `scorecard441` | SIM 线对话流谱面卡 | pc441-scorecard / pc441-scorecard（基线 cd6032c7） | 441 SIM-MUSIC-WIDGET-1 零栈执行完成：谱面卡（midiScore 装置解析+renderScoreInto 复用）三门绿合入 main（d9daabf3→08f24c7b），done 待验收，证据 coord/runs/SIM-MUSIC-WIDGET-1/ | inactive（待验收，可续真栈数据链腿） |
| `scorexfer440` | YUE 纯器乐谱迁移线 | pc440 / scorexfer440（基线 7e77c7b7） | 440 腿A+腿B 零栈执行完成：金样对账逐字节（197→0/78→275）+两段式接线，两门绿合入 main（05be3d6f），done 待验收，证据 coord/runs/YUE-INSTRUMENTAL-SCORE-TRANSFER-1/ | inactive（待验收，可续腿C 真栈腿） |
| `forge334` | goal_status 伪造面清扫（chat 决策应答面） | pc334forge / port/goalstatus-forgery-sweep-1（基线 948d7b1c） | 334 全树普查+同型修 5 处（三罐头兜底+no_candidate+F4 finalize 回填/运行时双守卫）：红 4/4 活体复现→绿 7/7，chat 全包绿合入 main（c656d675→9c0fd8c3），done 待验收，证据 coord/runs/FIX-GOALSTATUS-FORGERY-SWEEP-1/ | inactive（待验收，可续同族存疑站点清扫） |
| `sighyg331` | VST3 信令卫生线（C++ engine） | wt_pc331 / pc331-sighyg（engine 分支 pc331-sighyg 在主模块仓库 .git/modules/tracktion_engine） | 331 VST3-SIGNAL-HYGIENE-1 零栈执行完成：engine 98fb0200（守卫模式感知+VIT331_SETUP/SIGNAL 断言面，env 门 VIT331_CONSISTENT_SETUP 默认关），四门绿（configure/冷构建/go build/定向 go test），done 待验收（真栈腿 lead 留决策侧），证据 coord/runs/VST3-SIGNAL-HYGIENE-1/ | inactive（待验收，可续真栈腿/翻默认） |
| `entry413` / `ringfix416` / `presmerge412` / `producer410` / `ready411` | 窗口线旧执行 | 各自 worktree | 已结案 | inactive（一般不再复用） |

## 复用规则

- 同一产线新卡：直接 `send_message` 给 owner，附卡路径+本次新增约束；不重述 AGENTS 全文。
- 小修/连续调参：合并给同一 owner，一次给多步指令。
- 仅当文件域不同/原 owner 阻塞/需并行不相交时新建子代理；新名字按产线命名便于入册。
- 跨会话：名册保留作参考；若原 agent 不可续，用同产线名新建，先读 `NEXT-SESSION-BRIEF` + 卡/裁定，不读旧聊天。

# SIM-LIVE-SCORE-LINK-1：谱面卡 live 数据链接线

- 池序 443；P1；PC；发卡=lead（2026-11-21 凌晨，441 真栈腿收口时确认接缝未接）；验收负责人=lead。
- 背景：441 谱面卡零栈+渲染面 PASS，但 live 链 `tutorScoreLoader` 接缝设计上由调用方接线、本卡未接——真栈实测：装载成功而谱面入口不渲染（fail-honest 正确，但产品要的是能看到谱）。

## 目标

1. **回读通道**：给 webui 提供已装载材料的音符序列回读——优先走 agent HTTP 既有面（如 /agent/invoke 工具族有 clip notes 读回命令则直接用；无则在 tutor 域加只读端点 `GET /agent/tutor/material_score?conversation_id=&title=`，服务端复用 438 的 tutorsim.ParseMidi 读材料文件返回音符序列——不新增内核命令）。
2. **接线**：App.tsx 的 project surface 容器给 ProjectSurface 传 `tutorScoreLoader`（调回读通道→midiScore.ts 同构数据或直接返回 Go 侧解析结果→ScoreModel）。
3. 441 既有测试不回归；新增 live 接线单测（loader 失败/非 midi_clip/成功三态）。

## 约束与验收

- 文件域：`agent/internal/chat/tutor_sim.go`(+test 只读端点如需)、`agent/webui/src/project/`、`agent/webui/src/App.tsx`（surface 容器一处）；不改内核/冻结件。
- 零栈：go test 定向+vitest+build 全绿；真栈手测（装载→查看谱面→42 音 4/4 渲染）=lead 窗口，PASS 后 441/443 同链收口。
- 与夜班在跑卡域互斥：331=Go vst3、398=webui-scoreworkbench、442=scripts——本卡 chat/tutor 域+project/App 容器；**注意 398 动 scoreworkbench 选择面，本卡读 scoreworkbench 只读**；若 398 尚未合入则基于其合入后 main 领取。

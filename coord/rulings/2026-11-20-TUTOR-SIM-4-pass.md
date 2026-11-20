# 裁定：TUTOR-SIM-4 v2（438）真栈 PASS

- 日期：2026-11-20 20:05 +0800
- 卡：438 TUTOR-SIM-4 v2（brief/MIDI 材料装载）
- 裁定：**零栈三门主管复跑全绿 + 真栈手测 PASS**。
- 代码：950f1695 → main 51eff149；coord 收尾 c745955c；被测 agent `d94e8168…` + dist `c55d1810…`；berth=coord/runs/TUTOR-SIM-4/berth_20261120_1944。

## 真栈事实（浏览器级 DOM + HTTP 双面）

- **v1 装置渲染**：`D:/Vit_DAW_handtest/TUTOR-SIM-2/live.vit`（sidecar=tutor_sim.yaml+assets 原样复制，MIDI sha 与冻结源一致）自动开场 1/4；「查看简报」折叠/展开；材料卡「小星星主题（42 音）｜42 音·48 拍」+「装载到工程」。
- **材料装载（核心腿）**：UI 点击 → 按钮变「已装载」禁用+「已挂音源」；HTTP 直调收据=**track 1015 / clip 1018 / plugin 1022 / instrument_attached=true**（真内核 add_track→rename→insert_midi_clip→add_midi_notes→rack_add_node 全链 ok）；复调 already_loaded=true 同 id（幂等亲证）。
- **四步推进**：42 →（词答案走对话输入框）模进 → 已装载 → 完成 → 「第 4/4 步 · 已完成」+输入双禁用。
- **v0 兼容**：TUTOR-SIM-1（v0 YAML）1/3 开场、无简报/材料卡，437 行为原样。

## 边界与观察（非缺陷）

- 步骤卡输入框为数字型，词答案走对话 composer 可用（打磨项：按 expect 类型切换输入型，另卡候选）。
- 材料装载态按导师会话记忆：跨会话 load 各建一轨（设计内；产品化时若需工程级去重另裁）。
- 音源挂载=best-effort rack_add_node（Surge identifier 384 腿 D provenance）；被拒时材料仍成立+instrument_error 披露（本轮真内核实挂成功）。
- 预设级音色（program/state）不在既有命令面（298 已证）——留待音色卡。
- teardown：进程/TCP 全零；recents/Settings/default_project 哈希还原亲证。

## 下一步

- SIM 线下一卡：音乐控件（谱面/Tap/播放器，W05/W06/W07）与材料-工程联动深化；生产节点判定器不在 SIM 线（既有边界）。

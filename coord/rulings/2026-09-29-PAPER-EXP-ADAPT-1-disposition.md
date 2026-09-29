# Ruling：PAPER-EXP-ADAPT-1 blocked 处置——目标 1/2/3 采信合入，目标 4 转新卡 RUNNER-GATE-ADAPT-1（2026-09-29，决策侧）

- **裁定**：部分交付采信（目标 1/2/3 pass 合入）+ 阻断面采纳执行侧上交的**选项①**（开 runner 门实验适配卡）；选项②（换 p05 当 PILOT 例，丢 p01 问题例+p07 控制例配对）与选项③（改素材，违 56b 定稿纪律）不采。
- **执行侧行为采信**：证据推翻卡面前提（"材料侧适配清单即可过 runner"被证伪）后停止实现并上交——AGENTS §11 处置正确；runner 断言/内核/素材零改动，纪律遵守；穷尽核对（无 skip 旗标/无清单侧合法绕行/ps1+py 参数族穷举）记录在案。
- **决策侧核验（非转述）**：主报告全文核读；工件链完整——七例工程构建报告（1051 行 JSON）+材料仓清单入库（258b1ba）+full access 六步探针（含 AUTHORITY-LOST-1 跨工程身份边界实测与 server.go:6826 锚）+全七例离线门复算（all7_qualify_results.json）+PILOT 两轮真栈 exit 1 实证（vit-pilot/ 两组工件，responses=[] LLM 前确定性拒绝）；泊位亲核（验收时端口 5555/5556/7878 与 VitApp/Godot/VitAgent 进程全清——报告 §0"栈仍泊位"为撰写时点表述，实际已复原）。
- **归因采信**：qualify_material 门（free_state_d1_smoke.py ~L234-259，全 flavor 无条件）为 D2-1.5-S2f-2 压缩 fixture 按 spv1 合成材料校准（crest 14.5-21dB）；pv1 为真实音乐 stems（MUSDB18 型六轨），安静轨（guitar/other/piano −50～−90 dBFS）是真实混音形态非素材缺陷。**实验对象已重锚为真实材料而门参数未随迁**——实验基础设施适配缺口，非材料问题。
- **目标 3 附带产出采信**：EXP-VIT-MAIN 正式轮脚本化序列（物化副本→project.open→同身份切 full→`--reuse-existing-project` 直跑 py）现行代码零开发可行——正式轮卡的设计输入已备。
- **处置**：目标 1/2/3 工件 cherry-pick 合入（7a55a97b+14aaa984）；卡归档（blocked→已处置）；新卡 **RUNNER-GATE-ADAPT-1** 入池（P0，PILOT 解锁前置）。
- **tag 影响评估**：阻断在 runner python 侧（scripts/），不在 Go/内核——`experiment-baseline-2026-09-29`（4b695354）继续有效，不需前移；RUNNER-GATE-ADAPT-1 改 runner 门后 PILOT 在同 tag 上跑，被测对象（混音诊断调控）未变。

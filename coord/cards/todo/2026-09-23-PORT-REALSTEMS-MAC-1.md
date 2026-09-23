# PORT-REALSTEMS-MAC-1：真实 stems 到 mac——旅程烟测真实素材验证 + 插件族动机面观察

- 优先级 / 预估 / 依赖：P1（mac 演示前置链）/ 0.5 天 / 用户裁定 2026-09-23（mac 一直没有真实测试 stems，旅程烟测跑在合成替身上）；反哺 PORT-WL-EQ-1 优先级判定
- 模型分级：L1 / GLM-5.3 flash 可接（驱动+记账，断言零改动）
- **执行侧（mac 侧卡）**：素材由**用户从 PC 本机转交**（机器本地目录，**绝不入仓**——§10 纪律，仓库脚本 --stems-dir 语义即为此设计）；回执写回 D:\Vit_DAW coord
- **背景（已核实+2026-09-23 三次核实修正）**：④⑤ 夹具=仓内合成音（test_100hz_10s.wav 100Hz 正弦+test_target_3s.wav 3s 目标音，双端同源）——**统计数字口径=工作流机制证明，不含内容质量证据**。**旅程/展示层素材真相（修正）**：912.vit 引用的六轨 20s"问题 stems"= **spv1 受控问题素材**（`D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\semantic_processor_project_smoke_v1_80085263a651cf20\`，3 案例 spv1_p01/p02/p06 ×6 轨×20.0s/44.1kHz）——由仓内脚本确定性生成（Goal5 干净受控素材 `scripts/semantic_mix_goal5_fixtures.py` → 配方化故障注入 `scripts/semantic_processor_open_fixtures.py`，种子化 RNG、波峰因数 14.5-21dB、可检测性门全过、盲密封真值）——**具备工程条件（有注入问题可观察可修复）**，与 ④⑤ 两个纯音不是一个层次；mac 旅程跑在自造替身上=从未用过这套真素材。商业歌曲 stems（茉莉花/Jade Bridge）与自动链无关，不入传交清单。
- **传交清单（用户打包，2026-09-23 敲定）**：①`fixtures\...\cases\`（61MB，18 个问题 stems=3 案例×6 轨）②`912.vit`（12KB，`D:\Godot\project\vit-daw-frontend\`）——合计 ~61MB；③可选 `sealed\`（+122MB，仅 mac 要跑盲评测时）；**`projects\`（875MB PC 构建 .vit 副本）不需要**。备选零传输方案：mac 用仓内两脚本重建（配方等价，但跨平台浮点舍入不保证逐字节同 sha256，盲评测哈希核对会失败——演示用途传输仍是最稳路径）。**路径重映射注记**：912.vit 引用 Windows 绝对路径，mac 侧 provisioning/--stems-dir 摆位适配由本卡执行面处理（踩不动走停止条件上交）。
- 目标：
  1. 用户打包传交上述清单到 mac（机器本地目录，不入仓），目录路径+文件 sha256 清单记回执；**文件名映射**：mac journey `--stems-dir` 约定 `<stem>.wav`——spv1 案例目录已是 bass.wav/drums.wav/guitar.wav/other.wav/piano.wav/vocals.wav 命名，直接可用（此前"茉莉花数字前缀改名映射"条目作废）
  2. `journey1_demo_journey_smoke_mac.sh --stems-dir <真实目录>` 跑通完整旅程（工程打开→权限→实验→装载→A/B）——**mac 首次真实素材端到端**
  3. **插件族动机面观察（反哺 EQ-1）**：记录真实素材旅程中模型提案/观察实际触及的插件族（EQ/broadband comp 是否被动机化、观察层是否出现频段占用/masking 事实）——PORT-WL-EQ-1 的优先级以此为准：若真实素材确动机化 EQ/comp → EQ-1 升 P1（演示剧本含插件场景时必须）；若仍无 → 维持 P2
  4. 顺带按 demo-capture SOP 采集 1-2 个真实素材成功轮的演示素材（若时机合适）
- 约束：stems 零入仓（.gitignore 无需动——放仓库外）；LLM key 零入工件；断言零改动（旅程断言即被测契约）；④⑤ 合成夹具**不换**（统计连续性——合成夹具是机制统计的特性不是缺陷，真实素材证据层由旅程承载）
- 验收：①真实素材旅程整轮 exit 0 工件；②插件族动机面观察表（哪族被动机化+观察事实指针）；③素材转交记录（来源/文件清单/哈希，不含文件本体）；④回执记两端 HEAD
- 停止条件：真实素材旅程断言红且红点在素材相关断言 → 如实记录红点与形态上交（素材与断言契约的适配是决策问题）
- 领取：
- 回执：
- 验收：

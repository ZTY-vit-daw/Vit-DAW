# PORT-REALSTEMS-MAC-1：真实 stems 到 mac——旅程烟测真实素材验证 + 插件族动机面观察

- 优先级 / 预估 / 依赖：P1（mac 演示前置链）/ 0.5 天 / 用户裁定 2026-09-23（mac 一直没有真实测试 stems，旅程烟测跑在合成替身上）；反哺 PORT-WL-EQ-1 优先级判定
- 模型分级：L1 / GLM-5.3 flash 可接（驱动+记账，断言零改动）
- **执行侧（mac 侧卡）**：素材由**用户从 PC 本机转交**（机器本地目录，**绝不入仓**——§10 纪律，仓库脚本 --stems-dir 语义即为此设计）；回执写回 D:\Vit_DAW coord
- **背景（已核实+2026-09-23 二次核实）**：④⑤ 夹具=仓内合成音（test_100hz_10s.wav 100Hz 正弦+test_target_3s.wav 3s 目标音，双端同源）——**统计数字口径=工作流机制证明，不含内容质量证据**；且 **PC 旅程工程 912.vit 引用的六轨 stems 也是合成的**（`D:\Vit_DAW\temp\semantic-processor-agent-project-smoke-v1\fixtures\...\spv1_p01\stems\`，六文件均 3.4MB=20s/44.1kHz 合成件，journey 工件 default_project.xml 副本核实）——**真实歌曲 stems 在两端都从未进过自动化链**。真实素材在 PC 本机（不入仓，已核体积）：`C:\Users\timoz\Downloads\茉莉花清影 Stems\`（147MB，5 轨：0 Lead Vocals/1 Drums/2 Bass/3 Keyboard/4 Synth——中文多轨）与 `C:\Users\timoz\Downloads\Jade Bridge Stems\`（192MB，7 轨）。后果：EQ/压缩类提案**无素材动机**（合成音无频段占用/动态问题），模型此类建议无工程依据；clarify 链失败取证卡已增"素材退化形态"假设。
- 目标：
  1. 用户转交真实 stems 到 mac（**推送不可行已裁定 2026-09-23**：147-192MB 商业音乐 WAV 不入 git 历史+当下 GitHub 断连；走 U 盘/局域网/网盘人工转交；顺带 912.vit 12KB）。**文件名映射**：mac journey `--stems-dir` 约定 `<stem>.wav`（bass.wav/drums.wav/…）——茉莉花目录带数字前缀需改名映射（0 Lead Vocals.wav→vocals.wav、1 Drums.wav→drums.wav、2 Bass.wav→bass.wav、3 Keyboard.wav→keyboard.wav、4 Synth.wav→synth.wav），映射表记回执；目录路径机器本地记录在回执
  2. `journey1_demo_journey_smoke_mac.sh --stems-dir <真实目录>` 跑通完整旅程（工程打开→权限→实验→装载→A/B）——**mac 首次真实素材端到端**
  3. **插件族动机面观察（反哺 EQ-1）**：记录真实素材旅程中模型提案/观察实际触及的插件族（EQ/broadband comp 是否被动机化、观察层是否出现频段占用/masking 事实）——PORT-WL-EQ-1 的优先级以此为准：若真实素材确动机化 EQ/comp → EQ-1 升 P1（演示剧本含插件场景时必须）；若仍无 → 维持 P2
  4. 顺带按 demo-capture SOP 采集 1-2 个真实素材成功轮的演示素材（若时机合适）
- 约束：stems 零入仓（.gitignore 无需动——放仓库外）；LLM key 零入工件；断言零改动（旅程断言即被测契约）；④⑤ 合成夹具**不换**（统计连续性——合成夹具是机制统计的特性不是缺陷，真实素材证据层由旅程承载）
- 验收：①真实素材旅程整轮 exit 0 工件；②插件族动机面观察表（哪族被动机化+观察事实指针）；③素材转交记录（来源/文件清单/哈希，不含文件本体）；④回执记两端 HEAD
- 停止条件：真实素材旅程断言红且红点在素材相关断言 → 如实记录红点与形态上交（素材与断言契约的适配是决策问题）
- 领取：
- 回执：
- 验收：

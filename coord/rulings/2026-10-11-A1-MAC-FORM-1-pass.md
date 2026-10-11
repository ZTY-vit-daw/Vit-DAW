# Ruling：A1-MAC-FORM-1 pass（2026-10-11，决策侧）

- **裁定：pass**。报告工件 cherry-pick 4c06b8f1 随本裁定链合 main；卡片状态链（领取 19cdb0a5/done 326754b6）执行侧已直推 main。
- **决策侧独立复验（Mac 决策会话亲核，非转述）**：
  - **数据源独立重数吻合**：`~/.vit/plugin_semantics.json`（built 2026-10-03）entries=719、is_instrument=true=14、primary_type=instrument=22——与报告及仓内工件 scan_summary.json（719 全 Waves VST3、双 shell 718+1、误判清单 CLA Drums×2/JJP-Drums×3/GW PianoCentric×3 在案）、instruments_14.json（14 条）逐项对账一致。
  - **锚点抽验命中**：VitGraphValidator.cpp:30-34（Z1/Z2/Z3 zone 准入——Z2 synth 槽结构预留）；harness.go:1955+（PCA 装载门 exact-identity promoted 解析面）——"agent 准入层=instrument 结构性无通道（七族全效果器）"的代码核证结论与源码结构一致。
  - **三节齐+验收口径三分类明确**：已有通道（发现/标记双源 is_instrument、kernel 装载原语通用、Z2 槽、渲染预检 instrument-ready、编排 zone 分配）/需新通道（agent 侧 instrument 准入=A1 本体，PC/Mac 同构）/无候选（无专用鼓机、无 GM 鼓组、CR8 零本机采样内容——"装上即得鼓声"选项不存在）。
  - **出声腿结论清晰**：最短可达=路径 B 音频文件腿（ingest→render→audition→take 切换全链 Mac 真栈先例实证，仅缺鼓 wav 生成器一环）；路径 A（MIDI 出声）收敛在准入层一处待 A1 裁定+装载实证。
  - **诚实边界采信**：零真栈声明（所有 ✅ 均指先例 run 非本卡复跑，如实）；占比=2026-10-03 索引快照（卡面指定数据源）注明不自动更新；PCA 拒信=代码结构核证非实测；Waves 授权/AU hosting/CR8 内容下载三项未取证如实申报；候选适配度排序标注含产品常识判断。
- **端测边界**：只读勘察零代码零真栈，合规。
- **上交（A1 产品形态用户裁定项的事实包已备齐）**：①Mac 插件库无开箱鼓源（14/719 全合成器/采样器系，GM 鼓组路线=内核内置则本仓 VitApp/Source 零内置合成器 grep 0 命中）；②三条路线成本——GM 内置（内核新件）/演示工程鼓机音源（内容+装载实证）/CR8 采样器（内容下载+授权未取证）；③路径 B 可零新通道先行出声（缺 wav 生成器一环）。裁定归用户，不在本卡。

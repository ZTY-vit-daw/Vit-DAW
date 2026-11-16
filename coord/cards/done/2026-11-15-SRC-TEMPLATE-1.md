# SRC-TEMPLATE-1：课程固定健康音源模板——旋律音源+辅助律动鼓预设（DG1-B）

- 发卡：GLM 主管决策流 / 2026-11-15 深夜（依据=[审计 H5 阻塞点](../../reports/2026-11-12-demo-execution-audit/README.md)+[STACKGATE DG1-B 已冻结](../../docs/STACKGATE_DG1_DG3_V1.md)；进度驱动无工期）
- 派发确认：已确认（主管发卡，Wave B 首卡）
- 验收负责人：主管决策流 + 用户听核节点（人耳真栈出声）
- 池序 373；P0；依赖=无（真栈验证候栈位排队：367→372→本卡）；PC

## 目标

1. **旋律音源模板**：固定健康旋律音源（Surge 系既往非静音对照在卷——309 卡；或等价确定性健康 VSTi）+ 本课配置（GM 映射声明、默认 program/音色参数固化），模板工程内可开、学生改音后**真实出声变化**、保存重开一致。
2. **辅助律动鼓预设**：鼓/打击预设模板（§13 裁定：鼓轨=agent 依拍号补充的辅助律动，模板库形态），2/4 拍可用律动至少一套。
3. **模板交付形态**：可移植课程模板（工程包/模板文件随包，不依赖本机绝对路径；创建乐局时应用 2/4+tempo+调号——与 W01 契约的 timing 字段衔接位标注）。
4. **DG1-B 过关**：按 [STACKGATE §DG1-B](../../docs/STACKGATE_DG1_DG3_V1.md)（B1-B7）真实栈验证 exit 0——含 **PCA 无越权红线**（不伪造授权、不绕 PCA；加载走既有插件装载门）。
5. 单测/装置：模板装配单测+DG1-B 断言工件。

## 非目标

- 通用插件选择器/preset 浏览器（另卡）；Waves 授权修复（329 判别腿另候）；谱面/拍号 handler 命令面（谱面线域）。

## 文件域

`VitApp/`（如需内核侧模板装载）+ `agent/internal/`（模板装配/乐局创建衔接）+ 模板资产文件（工程包内）+ 烟测装置扩展 + coord 工件；领取时锚定精确清单回填。禁入：chat/launcher/project webui 域、gen 域（372）、谱面域。

## 验收标准

DG1-B 真实栈 exit 0（B1-B7 逐项）+三门不回退+用户听核节点如实登记（人耳）。

## 停止条件

健康音源在真实栈静默且非配置可修→取证上交（对齐 309/329 取证链，不硬凑）。

## 领取/回执/验收

### 领取（2026-11-15 23:5x +0800）

- owner=GLM-5.3 L2 执行子代理（ZCode PC 会话，SRC-TEMPLATE-1 专项）；机器=PC。
- worktree=`D:/Vit_DAW_wt_src1`，分支=`port/src-template-1`；领取基线：origin/main=`430b234b1484bc80cc949594b996f5693e5b4eaf`（fetch 后亲读；领取前 night 并发推进两轮 9caedb84/430b234b 已核对——367 已 done、372 doing 零栈段完成，栈位仍 372→本卡），HEAD=`430b234b`，`git status --short`=空、`git diff --stat`=空（worktree 自 origin/main 新建检出，全净）。
- 文件域锚定（卡面"领取时锚定精确清单"兑现）：
  1. `coord/lesson-designs/music-creation-unit2/templates/`（新；模板资产=课程模板工程 XML+manifest+README，随课程设计包，沿 `coord/fixtures/legame/` 先例：SHA256 纪律+place-copy 隔离）；
  2. `agent/internal/coursetemplate/`（新 Go 包；模板装载/校验/装配与 W01 timing 衔接位标注，纯函数零栈）+ 单测；
  3. `scripts/src_template_smoke.py`（新；DG1-B 腿驱动，B1-B7）+ `scripts/dev_agent_smoke.ps1`（加 `-SrcTemplateSmoke` 透传缝，既有腿字节不变——309 `-WaveshellStacklegSmoke` 同款模式）；
  4. `coord/runs/SRC-TEMPLATE-1/`（RUNBOOK/§8 口径/run 工件）；
  5. 卡面/POOL-INDEX/PC-RUNTIME-STACK 登记（coord 惯例路径）。
  - 不涉 VitApp 内核源码（模板走既有 open/save/render 命令面，内核侧零改动预期；若真栈证伪此假设→取证上交不扩修）。
- doing 冲突核对：372（port/gen-yue-wire-1 @ a88580d1）文件域=genapi/harness.gen_*/tools.catalog+coord/runs/GEN-YUE-WIRE-1——与本卡零交叠；367 已 done。
- 真栈腿排队：372 第 2 位（候其真栈+生成腿）→本卡第 3；零栈段（模板/单测/装置/RUNBOOK）先行。
- 关键锚点（领取时已亲核）：309 Surge 非静音取证=`coord/runs/WAVESHELL-OFFRENDER-FIX-1/stackleg_20261109_1906/`（e1ctrl 实时 -10.97dB/e4 离线渲染 -12.6dBFS；fixture 带 Surge XT proven state blob+identifier `VST3-Surge XT-1e1bcfb-190e4fbd`）；Waves 全壳静音不可押（309 M1=RT-SILENT）；装置挂法=STACKGATE §7 第 2/3 行（berth 形态+`l2_render_probe` 探针面+`start_render` WAV 判据+agent `open_project` tool face）；VST3 program change 不在参数面（298 证）→program 固化走模板 state blob 固化；W01 timing 契约=`docs/LEGAME_DESIGN_CONTRACT_V0.md` workspace.timing 字段（G0-LEGAME-CONTRACT-1）。

### 回执（2026-11-16 00:2x，执行侧自验 done 待主管验收）

- **实现提交** @ `port/src-template-1`（已推远端）：零栈段 `ba14b914`（模板资产+装配包+装置+RUNBOOK）→真栈段 `c2f47f72`（DG1-B PASS 工件+装置修复）。文件域=登记清单全兑现：`coord/lesson-designs/music-creation-unit2/templates/melody-drum-v1/`（template.v1.xml[规范形 LF sha256 `a188defd…`]+manifest+README）+`agent/internal/coursetemplate/`（3 文件+6 单测）+`scripts/src_template_build.py`+`scripts/src_template_smoke.py`+`scripts/dev_agent_smoke.ps1`（仅加 `-SrcTemplateSmoke` 透传缝，既有腿字节不变）+`coord/runs/SRC-TEMPLATE-1/`。**VitApp 内核源码零改动**（登记预期兑现）。
- **DG1-B 真实栈 exit 0**：run=`coord/runs/SRC-TEMPLATE-1/run_20261116_0012/`（berth 形态；agent=worktree 现场构建；内核=主检出 build 目录只读复用 `eda7e179…` mtime 2026-11-09，含 key 面且回避了缺该面的 staging 旧内核）。逐断言（回指 STACKGATE §3.2）：
  - **B1 PASS**：agent `open_project` tool face ok+`project_history.project_uuid`=模板 uuid（身份绑定链）+五轨名齐（参考与示范/我的旋律/辅助律动（鼓）/我的八小节主题/我的成曲版本）；
  - **B2 PASS**：旋律轨 `l2_render_probe` ready/非零/max_abs **0.295**（阈 0.01）；**B3 PASS**：鼓轨 max_abs **0.461**（Surge XT 309 血统 state blob——既往 -12.6 dBFS 离线对照在本卡复现成立）；
  - **B4 PASS**：`mutate_midi_notes`（占位 60→79，beat 0，by vit_note_id）readback 命中+全编 WAV 前后差分 **20501**（阈 500；前后峰 13442/15320 双非静音，309 契约）；
  - **B5 PASS**：`save_project`→**生产语义发现：`normalizeProjectPathForSave`（VitProjectFile.cpp:15）把任意保存路径扩展名规范化为 .vit**——.xml 模板副本实际落加密 `lesson_project.vit`；装置按生产形态重开该 .vit（=用户真实保存/重开链，含加密装载）→音符回读逐键相等+重开探针 0.315 非静音；
  - **B6 PASS**：重开后五轨+三轨 Surge XT 插件行+`project_snapshot_export` 快照钉全中（TIMESIG 2/4/TEMPO 80.0/`vit_music_key_signature="C"`/channelNumber=10/改后音高 p="79"/Surge 块×3+role×3）；
  - **B7 PASS**：agent 日志窗口 admission 摘录在案（`b7_admission_excerpt.txt`）零 bypass/forged/unauthorized+零 agent 侧插件控制命令（instantiate/rack_add_node/set_plugin_param 零命中）——模板插件=内核原生装载，PCA 门零接触；Waves 静音族零挂载。
- **模板交付面**：①旋律+主题+鼓三轨 Surge XT state blob 固化（manifest 记 `b42e1e53…` provenance=309 E4 逐字）；②GM 映射声明=占位声明非兼容承诺（melody≈GM82 saw stand-in；鼓=GM 打击键位记谱 36/38/42+chromatic 合成音色分离声明）；③鼓预设 `groove-2-4-basic`（2/4×4 小节 18 音，B3 非静音亲证）；④可移植=零音频媒体引用（portability 断言防回归）+VST 按 uniqueId 解析（filename 仅提示）；⑤W01 timing 衔接=manifest `w01_seam.field_map` 四字段全落（2/4 模板固化免命令[内核无 set_time_signature，H4]/C 走 set_project_key 缝/80 缺省 R5 可覆盖/pickup 未固化注明）。
- **§8 对账**：无效轮 A-E 分记（RUNBOOK §6：全为本卡装置域修复——argparse 缺参/CRLF 规范形哈希/B1 取位/track_name 键/B5 生产语义适配；**零产品断言失败、零原样重跑、零环境中断**）；正式轮 F 单次 exit 0。**计费 ¥0 三重亲证**（telemetry absent+-NoChatSmoke+死端点 override）。
- **占用链**：占用 main `7a7bee6f`（00:0x 实际登记，行内 00:3x 为笔误如实申报）→释放=本批提交（recents 快照→还原哈希亲证 `d9b53ba7…` 一致；VIT_PROJECT_XML/草稿根全程 run 目录隔离；模板源规范形哈希前后一致；**TEARDOWN_ALL_ZERO 亲证 00:23:02**：VitApp=16880/VitAgent=12488 亲停，PROCS/TCP 7878/7879/8787/5555/5556/UDP 4445 全 0）。
- **端测覆盖边界（AGENTS §5）**：已覆盖=agent HTTP tool face+内核 ZMQ 命令面+l2_render_probe/start_render WAV 物理计量+快照 XML 断言；**未覆盖**=webui 渲染面（本卡零 webui 改动，vitest/构建面不适用）、Godot 旅程（DG2 串）、**M2 人耳听核（音色健康——待用户从 Godot 正常入口听核，`lesson_project.vit` 可作听核对象；机械非静音不替代）**。
- **遗留如实**：①鼓音色=Surge init saw 对 GM 打击键位的合成音（记谱/音色分离声明）——音色是否「可入课」属 M2 人耳判域，若用户听核否决则换预设属新裁定不属本卡返工；②B4 旋律轨探针 sum_abs 相对变化仅 0.02%（能量代理对单音改动不敏感，如实记录；主判据=WAV 差分）；③W01-C 创建乐局时消费 manifest 落位（另卡）；④轮 A 分支态失误（登记分支切换后工作树无装置接线）为流程教训：后续腿执行期间应留在实现分支。

### 验收：待主管

### 验收（主管 2026-11-16 凌晨）

- 裁定：**pass**——diff 9 文件零越域（VitApp 零改动=领取时假设兑现）；DG1-B 真栈 run_20261116_0012 主管亲核（driver exit 0、B1-B7 九断言、双轨非静音 0.295/0.461、WAV 差分 20501≫500、save→加密 .vit→重开回读相等、B7 零 bypass）；主管复跑 go build+coursetemplate 6 测绿；栈协议/S10/零计费合规；无效轮 A-E 装置域分记诚实。
- 采纳：**生产语义发现**（normalizeProjectPathForSave 恒落 .vit）入卷；鼓音色=合成占位声明（听核否决则换预设新裁定）；B4 能量代理不敏感如实（主判据 WAV 差分）。
- 合入：实现 ba14b914/c2f47f72 cherry-pick→main（6469f061/a453f4a6，合并 aa04434b）。
- **用户听核 M2 候用户**（lesson_project.vit 经 Godot 启动链，机械非静音不替代音色判定）。

## 用户听核 M2 回执（2026-11-16 晚，主管代记）

- 判定：**机械面 PASS / 音色域否决（触发预留口径的新裁定）**。
- 用户原话要点：①「这几个轨道的确都成立了」（五轨结构/装载/播放链成立）；②「基本都是单音固定写入」（模板旋律内容=固定单音序列，音乐性不足）；③「音色都没有正确匹配」——旋律+鼓音色双双否决；④「鼓组写的也不是很合理……写的和音色的选择都不好」——鼓 groove 编写+音色双否。
- 按本卡预留口径：③④属**换预设/换音源的新裁定**，不属本卡返工（本卡范围=模板结构+装载链+DG1-B 断言，均已 pass 收口）。
- 处置：音源换血裁定项（旋律音色选型/鼓预设重写/真 GM 打击音色 vs VST 选型）+ ②旋律内容与 380 材料转写流的衔接（教材曲例旋律替换单音固定序列）→ 主管整理裁定单候用户；裁定后开音源重做卡。
- M2 登记：**FAIL（音色域——预留路径生效，转新裁定）**；模板机械面维持 pass。

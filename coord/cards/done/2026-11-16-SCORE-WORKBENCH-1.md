# SCORE-WORKBENCH-1：谱面工作台——节奏/旋律模式编辑+内核真写回读+随谱游标（展演主要新开发）

- 发卡：GLM 主管决策流 / 2026-11-16 凌晨（依据=[369 选型定案](../runs/G0-SCORE-SPIKE-1/REPORT.md)：VexFlow 5+自研 beat-grid 模型；进度驱动无工期）
- 派发确认：已确认（主管发卡，展演关键路径主开发）
- 验收负责人：主管决策流（DG1-A+DG3 关卡）+ 用户手测节点（谱面编辑体验）
- 池序 374；P0；依赖=无（M3 的 2/4 工程语义候 375 拍号卡；M5 真栈腿候栈位）；PC

## 目标（里程碑顺序执行，每 M 独立可验收）

- **M1 模型+推导+渲染**：kernel 形状模型（携带内核 note id 一等公民）+推导规则成文（369 报告 R1-R3 规则+拍内附点归并/弱起延音链/休止填充表=**W18 契约基础**）+VexFlow 5 渲染与命中检测；vitest（jsdom env 成文）全绿。
- **M2 编辑交互**：点击放符/时值切换/附点开关/休止插入/延音建立解除/删除/拖动（16 分网格）全经自研模型推导重绘；**治理粒度=本地编辑缓冲+显式 apply 单次确认**（主管工作假设，RiskConfirm 语义对齐；候用户裁定单确认）。
- **M3 内核往返**：apply_midi_note_patch 真写（time_unit:beats）→get_midi_clip_notes 回读→模型重建回显；保存重开工程回读一致（2/4 工程语义候 375）。
- **M4 随谱游标**：/agent/state playhead_seconds→beat→x（tick→x 游标表 369 已证）+单 tempo fail-honest 门（多段 tempo 降级不静默错位）。
- **M5 端侧验收**：渲染面烟测（浏览器级 DOM 断言）+真栈烟测腿（DG1-A 口径，[STACKGATE §DG1-A](../../docs/STACKGATE_DG1_DG3_V1.md)）exit 0。

## 约束与风险（369 报告 §5 风险表全量继承）

R4 VF5 API 漂移→锁版本+薄适配层；R6 游标轮询粒度受限→必要时评估 events 流不改内核；R7 推导规则集可审查成文；R8 **Verovio 不捆绑本卡**（LGPL+wasm 单独裁定）；零内核/VitApp 改动（出声走 373 模板线）。

## 文件域

`agent/webui/src/scoreworkbench/**`（新目录全套）+`agent/webui/package.json`（+vexflow、+jsdom devDep）+composer/交互卡装配接线点（领取时锚定——与 367 域已合入基线核对）+`scripts/` 烟测扩展（同模式加参数）+coord 工件。禁入：内核/VitApp、gen 域（372）、模板域（373）、chat 内部。

## 验收标准

DG1-A 真栈腿 exit 0+渲染面烟测 PASS+vitest 全绿（基线 584+新增）+M1-M5 逐里程碑工件；用户手测节点=谱面编辑体验（DG3 M 清单）。

## 停止条件

VF5 在真实装配面不可用（npm/许可意外）→取证上交；治理粒度工作假设被用户否决→M2 确认面返工段停候新裁定。

## 领取

- 时间：2026-11-16 00:1x +0800（领取提交时间以 git 为准）
- 基线：origin/main `fea0829a`（fetch 后亲核：含 373 领取 bd7e8c49、372 真栈占用+释放 fea0829a、367 终验合入 4ce8862a/1837811f；首次推送被 372 释放提交顶掉一次，按协议在最新 main 上重应用）
- owner：GLM-5.3 执行子代理（ZCode L2 会话）/ PC
- 分支：`port/score-workbench-1`；worktree `C:/Users/timoz/wt_score1`（D 盘净空 1.78GB < 检出 1.66GB+node_modules，worktree 落 C 盘，路径如实登记）；协调 worktree `C:/Users/timoz/wt_score1coord`
- 领取时基线核对：HEAD=110c3aa5，status 干净，diff=0（协调 worktree 新建即净）
- 文件域锚定（精确清单）：
  - `agent/webui/src/scoreworkbench/**`（新目录全套：beat-grid 模型/记谱推导/VF5 渲染/游标表/编辑缓冲/内核桥 + ScoreWorkbenchPanel + 测试 + css）
  - `agent/webui/package.json`（仅 dependencies +`vexflow@5.0.0` 一行、devDependencies +`jsdom` 一行；`package-lock.json` 为 npm install 必然副产物一并入库）
  - composer 装配接线点：`agent/webui/src/App.tsx` **三点最小接线**——①L203 `WorkbenchTab` 类型 +`"score"`；②L237 `workbenchTabs` 数组 +谱面页签；③`Workbench()` 渲染分支（L7228 附近）+ScoreWorkbenchPane 分支与 import。核对：App.tsx 最近触碰=354 UI-ENTRY/LOADING（已合入），非 367 域（367=src/project/**+chat server+vitshell 脚本）；当前 doing 卡（372/373）文件域不涉 App.tsx
  - `scripts/dev_agent_smoke.ps1`（+`-ScoreWorkbenchSmoke` 场景开关，同模式扩展）
  - `scripts/score_workbench_smoke.mjs`（新：DG1-A berth 腿断言体——node+playwright 驱动真实 webui 面板经 agent HTTP/tool face 到内核 ZMQ）
  - `scripts/webui_rendered_dom_smoke.mjs`（+score 渲染面断言段，DG3 §5.1 门槛 1 扩展）
  - `coord/runs/SCORE-WORKBENCH-1/**`（run 工件，port 分支入库）
  - `coord/cards/` 卡状态 + `coord/resources/PC-RUNTIME-STACK.md` 真栈登记（M5 候栈队列=372 占用中 → 373 DG1-B 腿 → 本卡 DG1-A 腿）
- 边界预告（如实）：DG1-A A7/A8（真实出声+改音差分）依赖 MIDI 轨音源=373 模板线（内核无内建合成器，offline render 对无乐器 MIDI 轨 fail-honest 拒绝——VitProductionCoordinator.cpp:965-979 锚点）；A4 双拍号之 2/4 工程语义候 375。本卡 M5 腿对 A1-A6（+4/4 拍号面）exit 0，A7/A8/A4-2/4 作显式边界由决策侧裁定。

## 回执

- **M2 完成**（commit 634a2bc3 @ port/score-workbench-1）：edit.ts 编辑全集（放符/时值/附点表 2↔3/4↔6/休止/延音建立=相邻同音高合并/延音解除=链内拆分/删除/拖动；paint 语义）+buildApplyPatch=单次 replace_region 整段覆盖（治理粒度=本地缓冲+显式 apply 单次确认落实）；ScoreWorkbenchPanel（data-score-* 锚点族；无内核 fail-honest）；App.tsx 三点接线完成（类型+页签+分支+宽度行）。测试 +20（panel 交互 12+edit 纯函数 8；react-dom/client+React19 act 零新依赖）。三门：vitest 652/652/build exit 0。已知边界：vexflow 静态进主包（chunk 大小警告，懒加载候后续）。
- **M3 完成**：kernel.ts agent 桥（analyze_midi_transformations music_context_echo=拍号/tempo 披露缝+get_midi_clip_notes/data；apply=单次 confirmed replace_region+回读验证）；面板挂载自动装载+快照锚点；App.tsx Workbench 建桥。测试 +11（累计 65）；三门 vitest 663/663/build 0。保存重开一致（A6）留 M5 真栈腿断言；单 clip 定位边界如实（多 clip 报错候后续卡）。
- **M4 完成**：cursor.ts（秒→beat→x 锚点插值）+面板 250ms 轮询 /agent/ui/state ui_context；快照在场才画游标（单 tempo fail-honest 门=装载时 tempo_setting_count 守门+无快照不画）；data-score-cursor* 锚点。测试 +5（累计 70）；三门 vitest 668/668/build 0。
- **M5a 完成**（commit f93c567b）：渲染面烟测 exit 0 PASS——webui_rendered_dom_smoke.mjs +score-panel-S 段（S0-S7：页签/内核桥 fail-honest 装载/SVG/真实指针放符×2+时值/无内核 apply 显错+缓冲保留/放弃还原/无快照不画游标）；**装置域内适配**：FIX-START-DRAFT-SIDE-EFFECT-1 读路径不再物化 draft 后种子前置在当前 main 全红（干净基线 fea0829a 同 fatal 亲证），适配=种子前经生产缝 version.project_new 显式物化（报告 draft_materialization 字段）；**产品缺陷修复**：apply 失败路径关确认框（留开模态遮挡后续操作，烟测实证）。工件 coord/runs/SCORE-WORKBENCH-1/render_smoke_20261116_044124/。
- **M5b 完成**（commit 2718a863）：多 clip 工程 clip 选择器（面板 shadow 轨表清单+桥工厂换 clip 重建）——boot 演示工程 7 轨多 clip 实证「单 clip 自动解析」产品假设不成立（attempt3/4 取证：harness 多 clip 无指定正确拒绝，面板原样透传 fail-honest），clip 选择是课程模板多轨形态的真需求非测试便利。dev_agent_smoke.ps1 +ScoreWorkbenchSmoke 开关（含 recents 快照/还原/哈希亲证块）+score_workbench_smoke.mjs 断言体（真浏览器真实指针→/agent/invoke→harness→内核 ZMQ 全生产缝）。
- **M5c 完成**：DG1-A 真栈腿 **PASS exit 0**（占用 4136b54d/释放 f0d2106a；正式轮 attempt5=单次成功，十五断言 A1-A6 全绿；A7/A8=skipped-pending-373〔音源线已验未合入，内核拒绝无乐器 offline render〕；A4 之 2/4=375 已验未合入）。attempt 分记：attempt1=环境中断（worktree 无 Export 内核 exe）→run 目录拼装主仓 04:33 构建内核 a5947361…（预检亲证 music_context 120bpm/4/4 披露正常；**陈旧部署 b6565dcf 无披露面——部署面待主管重部署**）；attempt2=环境中断（同上内核）；attempt3/4=装置序重构取证（多 clip→M5b 产品修复；D1/D2 内核域外发现）——分记均非产品断言失败、零原样重跑。**域外发现上交（锚点，未扩修）**：D1=XML-boot 工程会话内加轨 save_project 后新轨被丢出工程状态（.vit-boot 不复现）；D2=save_project 后 get_project_state 的 clips[].clip_id 披露降级 null 直至 open_project 重开（产品影响：面板 save 后至重开间 clip 选择器失源；本腿 A6 以 save→open 序绕开）；D3=agent get_project_state 消毒面不披露 project_path（腿经 save_as 目标+--project-xml 双源）。工件=coord/runs/SCORE-WORKBENCH-1/run_dg1a_20261116_0455/（@port 322a82fd）。
- **三门终态**：vitest 668/668（基线 598 亲测+70 新增：M1 34/M2 20/M3 11/M4 5）/npm run build exit 0（0 TS 错）/go 不涉；渲染面烟测 exit 0+DG1-A 真栈腿 exit 0。**LLM 计费上界=0**（死端点 override+全确定性面；真栈 recents 哈希还原亲证 130B235F、TEARDOWN_ALL_ZERO 亲证）。
- **限额中断如实记录**：2026-11-16 04:0x 限额中断，04:1x 续跑（主管已核 M1-M4 已提交零丢失，worktree 干净）。
- **遗留与边界（如实）**：①A7/A8 出声断言候 373 音源线合入后补腿（内核侧 offline render 需乐器，VitProductionCoordinator.cpp:965-979 锚点）；②2/4 工程语义候 375 合入（模型/推导/面板 2/4 已就绪，桥读内核拍号真值）；③M4 游标 x 用上一完成渲染几何（250ms 内自校）、轮询粒度 250ms=R6 已知约束；④vexflow 静态进主包（chunk 警告非错误，懒加载候后续）；⑤单声部 v1（和弦 fail-honest，DERIVATION_RULES §6）；⑥R1.5 整小节归并不做（W18 候裁定）；⑦D1/D2/D3 域外锚点候决策侧处置（建议内核卡）；⑧端测覆盖边界：渲染面=浏览器级 DOM 断言（score-panel-S 七断言+DG1-A 面板重载断言）；用户手测节点 M4（谱面可视/手感抽检）候用户。
- **M1 完成**（commit 93a414d8 @ port/score-workbench-1 已推）：DERIVATION_RULES.md v1 成文（R1 切分/R2 符号表+休止填充表/R3 延音链；R1.5 整小节归并=不做，W18 候裁定）+model.ts（fail-honest 网格校验/序列化往返）+derive.ts（4/4、3/4 支持面）+render.ts（VF5 薄适配，唯一 vexflow import 点；tick→x 行表+命中检测）。测试：scoreworkbench 34 例全绿（jsdom env 文件头注释成文）；三门= vitest 632/632（基线 598 亲测+34 新增）/npm run build exit 0/go 不涉。VF5 类型面新漂移实证：Voice 参数 camelCase（numBeats/beatValue）——适配层吸收，另 Renderer 容器类型收窄 HTMLDivElement。

## 验收

（决策侧回填）

### 验收（主管 2026-11-16 晨）

- 裁定：**pass**——M1-M5 全里程碑亲核：vitest **668/668**（598+70）主管复跑+build+render 六段；**DG1-A 真栈 run_dg1a_20261116_0455 控制台亲证 PASS**（十五断言 A1-A6 全绿；A7/A8 skipped-pending-373 如实）；中断-续跑记录在卷（M1-M4 断点零丢失）；栈/S10/零 LLM 合规。
- 合入：M1-M5+工件链 cherry-pick→main（76838672..2862f1bc，合并 3ea8117a；ps1 透传缝四方并集=SrcTemplate+LauncherFlow+Timesig+ScoreWorkbench）。
- **承接路由**：A7/A8（出声+差分断言）→**377 卡**（373/375 已合入，解锁补完）；D1（XML-boot 加轨 save 丢轨）+D2（save 后 clip_id 披露降级至重开）+D3（agent 消毒面剥 project_path 影响面板）→**378 取证/修复卡**（域外锚点未扩修纪律正确）。
- **用户手测节点 M4**（谱面手感抽检）候用户——DG3 M 清单。
- 采纳：VF5 静态进主包（懒加载候后续）与单 clip 定位边界如实登记。
- **合并事故补记（主管如实）**：M1-M5 cherry-pick 链在两次冲突处理中丢失三件（cursor.ts/cursor.test.ts/App.tsx 锚点属性），主检出构建红+vitest 跌至 376——系统域比对后从分支终态补齐（两个 fix 提交），npm install 补依赖，**复验 build+vitest 668/668+render 全 PASS**。教训：多提交链 cherry-pick 后必须即时跑构建门再推远端。

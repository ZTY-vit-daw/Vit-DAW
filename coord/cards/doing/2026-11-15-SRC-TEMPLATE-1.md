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

### 回执：待回填
